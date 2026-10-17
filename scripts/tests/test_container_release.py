"""Adversarial release checks; no Docker daemon, credentials or network needed."""
import copy
import importlib.util
import json
import os
from pathlib import Path
import sqlite3
import subprocess
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('release', Path(__file__).resolve().parents[1] / 'container_release.py')
r = importlib.util.module_from_spec(spec)
spec.loader.exec_module(r)
REV = 'a' * 40

def manifest():
    return {'revision': REV, 'platform': 'linux/amd64', 'components': {
        n: {'image': r.REGISTRY + n + '@sha256:' + 'b' * 64, 'image_id': 'sha256:' + 'c' * 64,
            'archive_sha256': 'd' * 64, 'source_sha256': 'e' * 64} for n in r.COMPONENTS}}

def container(service, identity, running=True, project=None):
    return {'Id': identity, 'Name': '/' + service, 'Image': 'sha256:' + 'c' * 64,
            'Config': {'Labels': {'com.docker.compose.project': project or r.PROJECT,
                       'com.docker.compose.service': service}, 'Env': []},
            'State': {'Running': running, 'StartedAt': 'fixed', 'Status': 'running' if running else 'exited'}, 'Mounts': []}

class ReleaseTests(unittest.TestCase):
    def test_manifest_pins_exact_components_repositories_revision_and_hashes(self):
        self.assertEqual(r.validate_manifest(manifest(), REV), manifest())
        for field, value in [('image', 'evil.test/image@sha256:' + 'b' * 64), ('image', r.REGISTRY + 'api:latest'),
                             ('image_id', 'latest'), ('archive_sha256', 'invalid'), ('source_sha256', '../path')]:
            with self.subTest(field=field, value=value):
                data = manifest()
                data['components']['api'][field] = value
                with self.assertRaises(r.ReleaseError): r.validate_manifest(data, REV)
        for field, value in [('revision', 'main'), ('platform', 'linux/arm64'), ('components', {})]:
            data = manifest()
            data[field] = value
            with self.assertRaises(r.ReleaseError): r.validate_manifest(data, REV)

    def test_invalid_revision_cannot_touch_service(self):
        with patch.object(r, 'preflight', side_effect=AssertionError('must not touch service')):
            for value in ['main', '../etc', 'a' * 39, 'a' * 41]:
                with self.assertRaises(r.ReleaseError): r.deploy(value)
                with self.assertRaises(r.ReleaseError): r.acquire(value)

    def test_pinned_config_changes_only_three_images_and_build_instructions(self):
        config = {'name': r.PROJECT, 'services': {
            n: {'image': 'old', 'build': {'context': '/old'}, 'environment': {'TOKEN': 'secret$VALUE'},
                'ports': ['127.0.0.1:9000:80'], 'volumes': ['data:/app/db']} for n in (*r.COMPONENTS, 'redis', 'gateway')},
                'volumes': {'data': {'name': 'existing-data'}}, 'networks': {'default': {'name': 'existing-network'}}}
        before = copy.deepcopy(config)
        result = r.pinned_config(config, {n: 'sha256:' + 'c' * 64 for n in r.COMPONENTS})
        self.assertEqual(config, before)
        for n in ('redis', 'gateway'): self.assertEqual(result['services'][n], config['services'][n])
        self.assertEqual(result['volumes'], config['volumes'])
        self.assertEqual(result['networks'], config['networks'])
        for n in r.COMPONENTS:
            self.assertNotIn('build', result['services'][n])
            self.assertEqual(result['services'][n]['environment'], config['services'][n]['environment'])
            self.assertEqual(result['services'][n]['ports'], config['services'][n]['ports'])
            self.assertEqual(result['services'][n]['volumes'], config['services'][n]['volumes'])

    def test_snapshot_preserves_literal_dollars_and_is_private(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory).resolve() / 'config.json'
            r.config_file(path, {'services': {'api': {'environment': {'KEY': 'ab$CD$$end'}}}})
            self.assertEqual(json.loads(path.read_text())['services']['api']['environment']['KEY'], 'ab$$CD$$$$end')
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            self.assertEqual(list(path.parent.glob('.release-*')), [])

    def test_snapshot_rejects_symlink_destination(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            original = root / 'original'
            original.write_text('keep')
            link = root / 'link'
            link.symlink_to(original)
            with self.assertRaises(r.ReleaseError): r.save(link, {'replace': True})
            self.assertEqual(original.read_text(), 'keep')

    def test_inventory_does_not_claim_other_projects(self):
        objects = [container(n, n) for n in r.COMPONENTS] + [container('api', 'other', project='gewu')]
        with patch.object(r, 'containers', return_value=objects):
            own, others = r.inventory()
        self.assertEqual(set(own), set(r.COMPONENTS))
        self.assertEqual(others['/api'][0], 'other')

    def test_failed_container_is_visible_for_recovery_but_blocks_preflight(self):
        objects = [container(n, n, n != 'api') for n in r.COMPONENTS]
        with patch.object(r, 'containers', return_value=objects):
            with self.assertRaises(r.ReleaseError): r.inventory()
            self.assertEqual(set(r.inventory(require_running=False)[0]), set(r.COMPONENTS))

    def test_duplicate_component_blocks_migration(self):
        objects = [container(n, n) for n in r.COMPONENTS] + [container('api', 'duplicate')]
        with patch.object(r, 'containers', return_value=objects):
            with self.assertRaises(r.ReleaseError): r.inventory()

    def test_secret_arguments_and_stderr_are_not_disclosed(self):
        with patch.object(r.subprocess, 'run', return_value=subprocess.CompletedProcess([], 1, 'private', 'merchant-secret')):
            with self.assertRaises(r.ReleaseError) as caught: r.run('docker', 'secret-key')
        self.assertEqual(str(caught.exception), 'docker: command failed (exit 1)')
        with patch.object(r.subprocess, 'run', side_effect=subprocess.TimeoutExpired(['docker', 'secret-key'], 5)):
            with self.assertRaises(r.ReleaseError) as caught: r.run('docker', 'secret-key')
        self.assertNotIn('secret', str(caught.exception))

    def test_database_copy_is_consistent_and_content_changes_detected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            live, backup = root / 'live.db', root / 'backup.db'
            with sqlite3.connect(live) as db:
                db.execute('CREATE TABLE orders(id INTEGER PRIMARY KEY, value TEXT)')
                db.execute('INSERT INTO orders VALUES(1,?)', ('private-order',))
            before = r.database_signature(live)
            r.database_copy(live, backup)
            self.assertEqual(r.database_signature(backup), before)
            with sqlite3.connect(live) as db: db.execute('INSERT INTO orders VALUES(2,?)', ('new-order',))
            self.assertNotEqual(r.database_signature(live), before)
            self.assertEqual(r.database_signature(backup), before)
            with self.assertRaises(r.ReleaseError): r.database_copy(live, backup)

    def test_database_layout_refuses_remote_or_unexpected_sources(self):
        api = container('api', 'api')
        for dsn in ['postgres://remote', '/elsewhere/db.sqlite', '/app/db/../private.db', '/app/db/a.db?query=1']:
            api['Config']['Env'] = ['DATABASE_DRIVER=sqlite', 'DATABASE_DSN=' + dsn]
            with self.assertRaises(r.ReleaseError): r.db_location(api)
        api['Config']['Env'] = ['DATABASE_DRIVER=postgres']
        with self.assertRaises(r.ReleaseError): r.db_location(api)

    def test_probe_cleanup_refuses_unrelated_name(self):
        result = subprocess.CompletedProcess([], 0, json.dumps([{'Config': {'Labels': {}}}]), '')
        with patch.object(r, 'run', return_value=result) as call:
            with self.assertRaises(r.ReleaseError): r.remove_probe()
        self.assertEqual(call.call_count, 1)

    def test_probe_failure_removes_only_its_owned_container(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            db = root / 'app.db'
            with sqlite3.connect(db) as connection: connection.execute('CREATE TABLE item(id INTEGER)')
            calls = []
            def command(*args, **kwargs):
                calls.append((args, kwargs))
                if args[:2] == ('docker', 'inspect'): return subprocess.CompletedProcess(args, 1, '', '')
                if args[:2] == ('docker', 'run'): raise r.ReleaseError('injected start failure')
                raise AssertionError(args)
            env = {k: 'fixture-secret' for k in ('APP_SECRET_KEY', 'JWT_SECRET', 'USER_JWT_SECRET')}
            with patch.object(r, 'run', side_effect=command), patch.object(r, 'remove_probe') as cleanup:
                with self.assertRaises(r.ReleaseError): r.database_probe('sha256:' + 'c'*64, db, env, root)
                cleanup.assert_called_once()
            args, kwargs = calls[-1]
            self.assertIn('none', args)
            self.assertNotIn('fixture-secret', ' '.join(args))
            self.assertEqual(kwargs['env']['QUEUE_ENABLED'], 'false')

    def test_adoption_blocks_source_mismatch(self):
        with patch.object(r, 'source_fingerprint', return_value='f'*64):
            with self.assertRaises(r.ReleaseError): r.adoption_check(manifest())

    def test_process_environment_does_not_override_saved_compose(self):
        with patch.dict(os.environ, {'DATABASE_DSN': 'secret', 'COMPOSE_PROJECT_NAME': 'other', 'APP_SECRET_KEY': 'secret'}):
            env = r.clean_env()
            self.assertNotIn('DATABASE_DSN', env)
            self.assertNotIn('COMPOSE_PROJECT_NAME', env)
            self.assertNotIn('APP_SECRET_KEY', env)

class ManagedReleaseTests(unittest.TestCase):
    def test_forced_or_incomplete_api_shutdown_blocks_cutover(self):
        for state in ({'Running': True, 'ExitCode': 0}, {'Running': False, 'ExitCode': 137}):
            with patch.object(r,'run',side_effect=[subprocess.CompletedProcess([],0,'',''),subprocess.CompletedProcess([],0,json.dumps([{'State':state}]),'')]):
                with self.assertRaises(r.ReleaseError): r.stop_api('api-id')

    def test_saved_config_hash_and_environment_must_match_live_container(self):
        for problem in ('none', 'hash', 'environment'):
            with self.subTest(problem=problem), tempfile.TemporaryDirectory() as folder:
                root=Path(folder).resolve()
                config_path=root/'compose.json'
                config_path.write_text('{}')
                (root/'.env').write_text('fixture')
                owned={name:container(name,name) for name in r.COMPONENTS}
                model={'name':r.PROJECT,'services':{name:{'environment':{'KEY':'fixture-$value-$$'}} for name in r.COMPONENTS}}
                for item in owned.values():
                    item['Config']['Env']=['KEY=fixture-$value-$$']
                    item['Config']['Labels'].update({'com.docker.compose.project.config_files':str(config_path),'com.docker.compose.config-hash':'correct'})
                if problem=='environment': owned['api']['Config']['Env']=['KEY=changed']
                if problem=='hash': owned['api']['Config']['Labels']['com.docker.compose.config-hash']='drifted'
                responses=[subprocess.CompletedProcess([],0,json.dumps(model).replace('$','$$'),''),
                           subprocess.CompletedProcess([],0,'\n'.join(name+' correct' for name in r.COMPONENTS),'')]
                with patch.object(r,'ROOT',root),patch.object(r,'run',side_effect=responses):
                    if problem=='none': self.assertEqual(r.read_config(owned),model)
                    else:
                        with self.assertRaises(r.ReleaseError): r.read_config(owned)

    def test_compose_serialized_dollars_are_decoded_exactly_once(self):
        model = {'services': {'api': {'environment': {'KEY': 'a$VALUE$$end'}}}}
        encoded = json.dumps(model).replace('$', '$$')
        decoded = r.decoded_compose_config(encoded)
        self.assertEqual(decoded, model)
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder).resolve() / 'compose.json'
            r.config_file(path, decoded)
            self.assertEqual(json.loads(path.read_text()), json.loads(encoded))

    def test_previous_images_must_resolve_by_content_not_mutable_tag(self):
        owned={'api':container('api','api')}
        owned['api']['ImageManifestDescriptor']={'digest':'sha256:'+'d'*64}
        results=[subprocess.CompletedProcess([],1,'',''),subprocess.CompletedProcess([],0,json.dumps([{'Id':'sha256:'+'d'*64}]),'')]
        with patch.object(r,'run',side_effect=results) as call:
            self.assertEqual(r.current_images(owned),{'api':'sha256:'+'d'*64})
            self.assertEqual(call.call_args.args[-1],'sha256:'+'d'*64)
        with patch.object(r,'run',return_value=subprocess.CompletedProcess([],1,'','')):
            with self.assertRaises(r.ReleaseError): r.current_images(owned)

    def test_existing_release_requires_current_image_identity(self):
        with tempfile.TemporaryDirectory() as folder:
            state = Path(folder).resolve() / 'current.json'
            state.write_text(json.dumps({'revision': REV, 'components': {n: {'runtime_id': 'sha256:'+'c'*64} for n in r.COMPONENTS}}))
            owned = {n: container(n,n) for n in r.COMPONENTS}
            for item in owned.values(): item['Config']['Image'] = 'sha256:'+'c'*64
            with patch.object(r,'STATE',state), patch.object(r,'inventory',return_value=(owned,{})), patch.object(r,'source_fingerprint',side_effect=AssertionError('managed release uses record')):
                r.adoption_check(manifest())
                owned['api']['Config']['Image'] = 'unexpected:latest'
                with self.assertRaises(r.ReleaseError): r.adoption_check(manifest())

    def test_recovery_handles_partially_created_components(self):
        api = container('api','new-api')
        unrelated = container('api','other-api',project='gewu')
        with patch.object(r,'containers',return_value=[api,unrelated]), patch.object(r,'run') as call:
            r.stop_api_for_recovery()
            call.assert_called_once_with('docker','stop','--time','45','new-api',timeout=90)
        with patch.object(r,'containers',return_value=[unrelated]), patch.object(r,'run') as call:
            r.stop_api_for_recovery()
            call.assert_not_called()

    def test_source_fingerprint_detects_code_without_persisting_content(self):
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder).resolve()
            path=root/'apps/api/main.go'
            path.parent.mkdir(parents=True)
            path.write_text('original')
            with patch.object(r,'source_files',return_value=['apps/api/main.go']):
                first=r.source_fingerprint(root,'api')
                path.write_text('changed')
                self.assertNotEqual(r.source_fingerprint(root,'api'),first)
                path.unlink()
                with self.assertRaises(r.ReleaseError): r.source_fingerprint(root,'api')

    def test_registry_acquire_accepts_cross_store_id_but_checks_digest_and_labels(self):
        for bad_digest in (False,True):
            with self.subTest(bad_digest=bad_digest), tempfile.TemporaryDirectory() as folder:
                root=Path(folder).resolve()
                directory=root/REV
                directory.mkdir()
                data=manifest()
                (directory/'release.json').write_text(json.dumps(data))
                def command(*args,**kwargs):
                    name=next(n for n in r.COMPONENTS if args[-1]==data['components'][n]['image'])
                    info={'Id':'sha256:'+'f'*64,'RepoDigests':[] if bad_digest else [args[-1]],
                          'Architecture':'amd64','Os':'linux','Config':{'Labels':{
                          'org.opencontainers.image.revision':REV,'org.opencontainers.image.source':'https://github.com/'+r.REPO,'org.ruomu.component':name}}}
                    return subprocess.CompletedProcess(args,0,json.dumps([info]),'')
                with patch.object(r,'STORE',root),patch.object(r,'run',side_effect=command),patch.object(r,'download',side_effect=AssertionError('unexpected download')):
                    if bad_digest:
                        with self.assertRaises(r.ReleaseError): r.acquire(REV)
                    else:
                        result=r.acquire(REV)
                        self.assertTrue(all(x['runtime_id']=='sha256:'+'f'*64 for x in result['components'].values()))

if __name__ == '__main__': unittest.main()
