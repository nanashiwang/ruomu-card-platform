"""Adversarial release checks; no Docker daemon, credentials or network needed."""
import copy
import importlib.util
import io
import json
import os
from pathlib import Path
import sqlite3
import subprocess
import tempfile
import unittest
from unittest.mock import patch, mock_open

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
    @staticmethod
    def release(tag, *, draft=False, prerelease=False, assets=None, published_at='2026-11-10T00:00:00Z'):
        return {
            'tag_name': tag,
            'draft': draft,
            'prerelease': prerelease,
            'published_at': published_at,
            'assets': [{'name': name, 'state': 'uploaded', 'size': 100} for name in (
                r.RELEASE_ASSETS if assets is None else assets
            )],
        }

    def test_latest_release_skips_drafts_prereleases_and_incomplete_versions(self):
        latest, older = 'f' * 40, 'e' * 40
        releases = [
            self.release('card-' + 'd' * 40, draft=True),
            self.release('card-' + 'c' * 40, prerelease=True),
            self.release('main'),
            self.release('card-' + latest, assets={'api.tar.gz', 'user.tar.gz'}),
            self.release('card-' + older, published_at='2026-11-10T01:00:00Z'),
            self.release('card-' + latest, published_at='2026-11-10T02:00:00Z'),
        ]

        class Response(io.BytesIO):
            def __enter__(self): return self
            def __exit__(self, *_): self.close()

        with patch.object(r, 'urlopen', return_value=Response(json.dumps(releases).encode())) as open_url:
            self.assertEqual(r.resolve_latest_release(), latest)
        request = open_url.call_args.args[0]
        self.assertIn('/releases?per_page=100&page=1', request.full_url)
        self.assertEqual(request.get_header('Accept'), 'application/vnd.github+json')

    def test_latest_release_paginates_and_rejects_unavailable_or_invalid_lists(self):
        old_release = self.release('card-' + 'a' * 40)
        full_page = [self.release('main') for _ in range(r.RELEASE_API_PAGE_SIZE)]
        full_page[0] = self.release('card-' + 'b' * 40, published_at='2026-11-09T00:00:00Z')

        class Response(io.BytesIO):
            def __enter__(self): return self
            def __exit__(self, *_): self.close()

        responses = [Response(json.dumps(full_page).encode()), Response(json.dumps([old_release]).encode())]
        with patch.object(r, 'urlopen', side_effect=responses) as open_url:
            self.assertEqual(r.resolve_latest_release(), 'a' * 40)
        self.assertIn('page=2', open_url.call_args_list[1].args[0].full_url)

        with patch.object(r, 'urlopen', side_effect=OSError('private transport detail')):
            with self.assertRaisesRegex(r.ReleaseError, 'provide a full 40-character SHA'):
                r.resolve_latest_release()
        with patch.object(r, 'urlopen', return_value=Response(b'{invalid')):
            with self.assertRaisesRegex(r.ReleaseError, 'provide a full 40-character SHA'):
                r.resolve_latest_release()
        with patch.object(r, 'urlopen', return_value=Response(json.dumps([]).encode())):
            with self.assertRaisesRegex(r.ReleaseError, 'No complete published'):
                r.resolve_latest_release()

    def test_latest_release_requires_uploaded_assets_and_valid_metadata(self):
        valid = self.release('card-' + REV)
        uploading = copy.deepcopy(valid)
        uploading['published_at'] = '2026-11-11T00:00:00Z'
        uploading['assets'][0]['state'] = 'new'
        empty = copy.deepcopy(valid)
        empty['assets'][0]['size'] = 0
        bad_date = {**valid, 'published_at': 'invalid'}
        missing_flag = {**valid, 'draft': None}
        malformed = {**valid, 'tag_name': ['card-' + REV]}
        with patch.object(r, 'urlopen', return_value=io.BytesIO(json.dumps([
            uploading, empty, bad_date, missing_flag, malformed, valid
        ]).encode())):
            self.assertEqual(r.resolve_latest_release(), REV)

    def test_update_latest_does_not_redeploy_current_release(self):
        with tempfile.TemporaryDirectory() as directory:
            state = Path(directory).resolve() / 'current.json'
            owned = {name: container(name, name) for name in r.COMPONENTS}
            for item in owned.values(): item['Config']['Image'] = item['Image']
            state.write_text(json.dumps({'revision': REV, 'components': {
                name: {'runtime_id': item['Image']} for name, item in owned.items()}}))
            with patch.object(r, 'inventory', return_value=(owned, {})), \
                 patch.object(r, 'read_config'), patch.object(r, 'health'), \
                 patch.object(r, 'STATE', state), \
                 patch.object(r, 'resolve_latest_release', return_value=REV), \
                 patch.object(r, 'deploy') as deploy:
                r.update_latest()
            deploy.assert_not_called()

    def test_update_latest_does_not_hide_runtime_drift(self):
        with tempfile.TemporaryDirectory() as directory:
            state = Path(directory).resolve() / 'current.json'
            state.write_text(json.dumps({'revision': REV, 'components': {
                name: {'runtime_id': 'sha256:' + 'a' * 64} for name in r.COMPONENTS}}))
            owned = {name: container(name, name) for name in r.COMPONENTS}
            for item in owned.values(): item['Config']['Image'] = item['Image']
            with patch.object(r, 'inventory', return_value=(owned, {})), \
                 patch.object(r, 'read_config'), patch.object(r, 'health'), \
                 patch.object(r, 'STATE', state), \
                 patch.object(r, 'resolve_latest_release', return_value=REV), \
                 patch.object(r, 'deploy') as deploy:
                with self.assertRaisesRegex(r.ReleaseError, 'Running images differ'):
                    r.update_latest()
            deploy.assert_not_called()

    def test_update_latest_deploys_resolved_release_and_rejects_bad_state(self):
        latest = 'b' * 40
        with tempfile.TemporaryDirectory() as directory:
            state = Path(directory).resolve() / 'current.json'
            state.write_text(json.dumps({'revision': REV}))
            with patch.object(r, 'STATE', state), \
                 patch.object(r, 'resolve_latest_release', return_value=latest), \
                 patch.object(r, 'deploy') as deploy:
                r.update_latest()
            deploy.assert_called_once_with(latest)

            state.write_text('{invalid')
            with patch.object(r, 'STATE', state), \
                 patch.object(r, 'resolve_latest_release', return_value=latest), \
                 patch.object(r, 'deploy') as deploy:
                with self.assertRaisesRegex(r.ReleaseError, 'Invalid installed release record'):
                    r.update_latest()
            deploy.assert_not_called()

    def test_cli_optional_revision_and_resolution_failure_do_not_mutate_services(self):
        for invoked, args, automatic in [('rcard-update', [], True),
                                         ('rcard', ['update'], True),
                                         ('rcard-update', [REV], False),
                                         ('rcard', ['update', REV], False)]:
            with self.subTest(invoked=invoked, args=args), \
                 patch.object(r.sys, 'argv', [invoked]), \
                 patch.object(r.os, 'geteuid', return_value=0), \
                 patch.object(r.os, 'umask'), patch.object(r.signal, 'signal'), \
                 patch.object(r.fcntl, 'flock'), patch('builtins.open', mock_open()), \
                 patch.object(r, 'update_latest') as latest, patch.object(r, 'deploy') as deploy:
                r.main(args)
                if automatic:
                    latest.assert_called_once_with()
                    deploy.assert_not_called()
                else:
                    latest.assert_not_called()
                    deploy.assert_called_once_with(REV)
        with patch.object(r, 'resolve_latest_release', side_effect=r.ReleaseError('offline')), \
             patch.object(r, 'deploy') as deploy:
            with self.assertRaisesRegex(r.ReleaseError, 'offline'):
                r.update_latest()
            deploy.assert_not_called()

    def test_latest_release_limits_payload_and_pagination(self):
        with patch.object(r, 'urlopen', return_value=io.BytesIO(b' ' * (r.RELEASE_API_MAX_BYTES + 1))):
            with self.assertRaisesRegex(r.ReleaseError, 'size limit'):
                r.resolve_latest_release()
        with patch.object(r, 'RELEASE_API_MAX_PAGES', 1), \
             patch.object(r, 'urlopen', return_value=io.BytesIO(json.dumps([
                 self.release('card-' + REV) for _ in range(r.RELEASE_API_PAGE_SIZE)
             ]).encode())):
            with self.assertRaisesRegex(r.ReleaseError, 'pagination limit'):
                r.resolve_latest_release()

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

    def test_inventory_can_model_only_the_fully_uninstalled_state(self):
        with patch.object(r, 'containers', return_value=[]):
            own, others = r.inventory(require_running=False, allow_missing=True)
        self.assertEqual(own, {})
        self.assertEqual(others, {})
        with patch.object(r, 'containers', return_value=[container('api', 'api')]):
            with self.assertRaises(r.ReleaseError):
                r.inventory(require_running=False, allow_missing=True)

    def test_shortcut_names_dispatch_to_lifecycle_subcommands(self):
        cases = {
            'rcard-update': ('update', ['a' * 40]),
            'rcard-stop': ('stop', []),
            'rcard-start': ('start', []),
            'rcard-restart': ('restart', []),
            'rcard-reopen': ('restart', []),
            'rcard-uninstall': ('uninstall', ['--confirm']),
        }
        for name, (command, args) in cases.items():
            with self.subTest(name=name):
                self.assertEqual(r.normalize_shortcut(args, name), [command, *args])
        self.assertEqual(r.normalize_shortcut([], 'rcard-update'), ['update'])

    def test_stop_only_stops_application_services(self):
        path = Path('/opt/ruomu-card-platform/deploy/compose.yml')
        owned = {name: container(name, name) for name in r.COMPONENTS}
        stopped = {name: container(name, name, running=False) for name in r.COMPONENTS}
        for item in stopped.values():
            item['State']['ExitCode'] = 0
        with patch.object(r, 'lifecycle_context', return_value=(path, {}, owned, {})), \
             patch.object(r, 'compose') as compose_call, \
             patch.object(r, 'inventory', return_value=(stopped, {})), \
             patch.object(r, 'verify_other_containers'):
            r.stop_services()
        compose_call.assert_called_once_with(path, 'stop', '--timeout', '45', *r.COMPONENTS)

    def test_start_uses_saved_compose_without_starting_dependencies(self):
        path = Path('/opt/ruomu-card-platform/deploy/compose.yml')
        config = {'services': {'api': {'depends_on': {}}}}
        with patch.object(r, 'lifecycle_context', return_value=(path, config, {}, {})) as context, \
             patch.object(r, 'check_dependencies') as dependencies, \
             patch.object(r, 'compose') as compose_call, \
             patch.object(r, 'wait_health') as wait_health, \
             patch.object(r, 'verify_other_containers'):
            r.start_services()
        context.assert_called_once_with(allow_uninstalled=True)
        dependencies.assert_called_once_with(config)
        compose_call.assert_called_once_with(path, 'up', '-d', '--no-build', '--pull', 'never', '--no-deps', *r.COMPONENTS)
        wait_health.assert_called_once_with()

    def test_start_after_uninstall_requires_saved_existing_volumes(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder).resolve()
            store = root / 'store'
            backups = root / 'backups'
            store.mkdir()
            backups.mkdir()
            compose_path = store / REV / 'compose.json'
            compose_path.parent.mkdir()
            runtime_id = 'sha256:' + 'c' * 64
            config = {'name': r.PROJECT, 'services': {
                name: {'image': runtime_id} for name in r.COMPONENTS}}
            compose_path.write_text(json.dumps(config))
            state_path = store / 'current.json'
            state_path.write_text(json.dumps({
                'revision': REV,
                'compose': str(compose_path),
                'components': {name: {'runtime_id': runtime_id} for name in r.COMPONENTS},
                'preserved_volumes': ['ruomu-db'],
            }))
            with patch.object(r, 'STORE', store), patch.object(r, 'BACKUPS', backups), \
                 patch.object(r, 'STATE', state_path), patch.object(r, 'containers', return_value=[]), \
                 patch.object(r, 'compose', return_value=subprocess.CompletedProcess([], 0, json.dumps(config), '')), \
                 patch.object(r, 'run', return_value=subprocess.CompletedProcess([], 0, '[]', '')) as run:
                path, restored, owned, others = r.lifecycle_context(allow_uninstalled=True)
            self.assertEqual(path, compose_path)
            self.assertEqual(restored, config)
            self.assertEqual(owned, {})
            self.assertEqual(others, {})
            run.assert_called_once_with('docker', 'volume', 'inspect', 'ruomu-db')

    def test_start_after_uninstall_refuses_missing_volume_inventory(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder).resolve()
            store = root / 'store'
            backups = root / 'backups'
            store.mkdir()
            backups.mkdir()
            compose_path = store / REV / 'compose.json'
            compose_path.parent.mkdir()
            runtime_id = 'sha256:' + 'c' * 64
            config = {'name': r.PROJECT, 'services': {
                name: {'image': runtime_id} for name in r.COMPONENTS}}
            compose_path.write_text(json.dumps(config))
            state_path = store / 'current.json'
            state_path.write_text(json.dumps({
                'revision': REV,
                'compose': str(compose_path),
                'components': {name: {'runtime_id': runtime_id} for name in r.COMPONENTS},
                'preserved_volumes': [],
            }))
            with patch.object(r, 'STORE', store), patch.object(r, 'BACKUPS', backups), \
                 patch.object(r, 'STATE', state_path), patch.object(r, 'containers', return_value=[]), \
                 patch.object(r, 'compose', return_value=subprocess.CompletedProcess([], 0, json.dumps(config), '')):
                with self.assertRaisesRegex(r.ReleaseError, 'preserved-volume inventory'):
                    r.lifecycle_context(allow_uninstalled=True)

    def test_restart_only_restarts_application_services(self):
        path = Path('/opt/ruomu-card-platform/deploy/compose.yml')
        config = {'services': {'api': {'depends_on': {}}}}
        owned = {name: container(name, name) for name in r.COMPONENTS}
        with patch.object(r, 'lifecycle_context', return_value=(path, config, owned, {})), \
             patch.object(r, 'check_dependencies'), patch.object(r, 'compose') as compose_call, \
             patch.object(r, 'wait_health'), patch.object(r, 'verify_other_containers'):
            r.restart_services()
        compose_call.assert_called_once_with(path, 'restart', '--timeout', '45', *r.COMPONENTS)

    def test_lifecycle_start_requires_healthy_shared_dependencies(self):
        redis = container('redis', 'redis')
        redis['State']['Health'] = {'Status': 'healthy'}
        config = {'services': {'api': {'depends_on': {'redis': {'condition': 'service_healthy'}}}, 'redis': {}}}
        with patch.object(r, 'containers', return_value=[redis]):
            r.check_dependencies(config)
        redis['State']['Health']['Status'] = 'starting'
        with patch.object(r, 'containers', return_value=[redis]):
            with self.assertRaisesRegex(r.ReleaseError, 'not healthy'):
                r.check_dependencies(config)

    def test_uninstall_removes_only_app_containers_without_volume_flags(self):
        path = Path('/opt/ruomu-card-platform/deploy/compose.yml')
        owned = {name: container(name, name) for name in r.COMPONENTS}
        owned['api']['Mounts'] = [{'Type': 'volume', 'Name': 'ruomu-db', 'Destination': '/app/db'}]
        stopped = {name: container(name, name, running=False) for name in r.COMPONENTS}
        for item in stopped.values():
            item['State']['ExitCode'] = 0
        state = {'compose': str(path), 'revision': REV, 'components': {name: {'runtime_id': 'sha256:' + 'c' * 64} for name in r.COMPONENTS}}
        with patch.object(r, 'lifecycle_context', return_value=(path, {}, owned, {})), \
             patch.object(r, 'STATE') as state_path, \
             patch.object(r, 'json') as json_module, \
             patch.object(r, 'save') as save_state, \
             patch.object(r, 'run') as run, \
             patch.object(r, 'compose') as compose_call, \
             patch.object(r, 'inventory', side_effect=[(stopped, {}), ({}, {})]), \
             patch.object(r, 'verify_other_containers'):
            state_path.is_file.return_value = True
            state_path.read_text.return_value = 'state'
            json_module.loads.return_value = state
            r.uninstall_services()
        self.assertEqual(compose_call.call_args_list[0].args, (path, 'stop', '--timeout', '45', *r.COMPONENTS))
        self.assertEqual(compose_call.call_args_list[1].args, (path, 'rm', '--force', *r.COMPONENTS))
        self.assertFalse(any('-v' in call.args or 'down' in call.args for call in compose_call.call_args_list))
        run.assert_any_call('docker', 'volume', 'inspect', 'ruomu-db')
        save_state.assert_called_once_with(state_path, {**state, 'preserved_volumes': ['ruomu-db']})

    def test_uninstall_shortcut_requires_explicit_confirmation(self):
        with patch.object(r.sys, 'argv', ['rcard-uninstall']):
            with self.assertRaises(SystemExit) as caught:
                r.main([])
        self.assertEqual(caught.exception.code, 2)

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
    def test_existing_compose_working_directory_controls_relative_mounts(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder).resolve()
            directory = root / 'deploy'
            directory.mkdir()
            config_path = directory / 'compose.json'
            config_path.write_text('{}')
            (root / '.env').write_text('fixture')
            own = {name: container(name, name) for name in r.COMPONENTS}
            for item in own.values():
                item['Config']['Labels'].update({
                    'com.docker.compose.project.config_files': str(config_path),
                    'com.docker.compose.project.working_dir': str(directory),
                    'com.docker.compose.config-hash': 'verified-original-context',
                })
            model = {'name': r.PROJECT, 'services': {name: {'environment': {}, 'volumes': [{
                'type': 'bind', 'source': str(directory / 'nginx' / (name + '.conf')),
                'target': '/etc/nginx/conf.d/default.conf', 'read_only': True,
            }]} for name in r.COMPONENTS}}
            def command(*args, **kwargs):
                self.assertEqual(args[args.index('--project-directory') + 1], str(directory))
                self.assertEqual(args[args.index('--env-file') + 1], str(root / '.env'))
                output = '\n'.join(name + ' verified-original-context' for name in r.COMPONENTS) if '--hash' in args else json.dumps(model)
                return subprocess.CompletedProcess(args, 0, output, '')
            with patch.object(r, 'ROOT', root), patch.object(r, 'run', side_effect=command):
                self.assertEqual(r.read_config(own), model)

    def test_missing_ambiguous_or_external_working_directory_is_not_guessed(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder).resolve()
            config_path = root / 'compose.json'
            config_path.write_text('{}')
            (root / '.env').write_text('fixture')
            own = {name: container(name, name) for name in r.COMPONENTS}
            for item in own.values():
                item['Config']['Labels']['com.docker.compose.project.config_files'] = str(config_path)
            for directories in [('', '', ''), (str(root), str(root / 'deploy'), str(root)),
                                ('/etc', '/etc', '/etc'), ('deploy', 'deploy', 'deploy'),
                                (str(root / 'missing'),) * 3]:
                with self.subTest(directories=directories):
                    for item, directory in zip(own.values(), directories):
                        item['Config']['Labels']['com.docker.compose.project.working_dir'] = directory
                    with patch.object(r, 'ROOT', root), patch.object(r, 'run') as call:
                        with self.assertRaises(r.ReleaseError): r.read_config(own)
                        call.assert_not_called()
            link = root / 'link'
            link.symlink_to(root, target_is_directory=True)
            for item in own.values():
                item['Config']['Labels']['com.docker.compose.project.working_dir'] = str(link)
            with patch.object(r, 'ROOT', root), patch.object(r, 'run') as call:
                with self.assertRaises(r.ReleaseError): r.read_config(own)
                call.assert_not_called()

    def test_docker_operations_cannot_follow_remote_operator_context(self):
        with patch.dict(os.environ, {'DOCKER_CONTEXT':'other-server','DOCKER_HOST':'tcp://elsewhere:2375'}), patch.object(r.subprocess,'run',return_value=subprocess.CompletedProcess([],0,'','')) as call:
            r.run('docker','info')
            self.assertEqual(call.call_args.kwargs['env']['DOCKER_CONTEXT'],'default')
            self.assertEqual(call.call_args.kwargs['env']['DOCKER_HOST'],'unix:///var/run/docker.sock')

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
                    item['Config']['Labels'].update({'com.docker.compose.project.config_files':str(config_path),'com.docker.compose.project.working_dir':str(root),'com.docker.compose.config-hash':'correct'})
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

class LoggingTests(unittest.TestCase):
    def policy_fixture(self, driver='json-file', options=None):
        config = {'services': {name: {'environment': {'KEY': 'unchanged'}, 'image': 'old'} for name in (*r.COMPONENTS, 'redis', 'gateway')}}
        owned = {name: {'HostConfig': {'LogConfig': {'Type': driver, 'Config': options or {}}}} for name in r.COMPONENTS}
        return config, owned

    def test_default_logs_are_bounded_without_touching_other_services(self):
        config, owned = self.policy_fixture()
        original = copy.deepcopy(config)
        result, policies = r.bounded_logging(config, owned)
        self.assertEqual(config, original)
        for name in ('redis', 'gateway'):
            self.assertEqual(result['services'][name], original['services'][name])
        for name in r.COMPONENTS:
            self.assertEqual(result['services'][name]['logging'], {'driver': 'json-file', 'options': {'max-size': '20m', 'max-file': '3'}})
            self.assertEqual(result['services'][name]['environment'], original['services'][name]['environment'])
        self.assertEqual(set(policies), set(r.COMPONENTS))

    def test_smaller_limits_and_extra_options_survive(self):
        config, owned = self.policy_fixture(options={'max-size': '5MB', 'max-file': '2', 'compress': 'true', 'tag': 'existing-tag'})
        result, policies = r.bounded_logging(config, owned)
        self.assertEqual(policies['api']['options'], owned['api']['HostConfig']['LogConfig']['Config'])
        config, owned = self.policy_fixture(options={'max-size': '1m'})
        self.assertEqual(r.bounded_logging(config, owned)[1]['api']['options']['max-file'], '1')

    def test_large_and_unlimited_limits_are_reduced(self):
        for driver, options in [('json-file', {'max-size': '-1', 'max-file': '8'}), ('json-file', {'max-size': '200m', 'max-file': '6'}), ('local', {})]:
            with self.subTest(driver=driver, options=options):
                config, owned = self.policy_fixture(driver, options)
                _, policies = r.bounded_logging(config, owned)
                self.assertEqual(policies['api']['driver'], driver)
                self.assertEqual(policies['api']['options']['max-size'], '20m')
                self.assertEqual(policies['api']['options']['max-file'], '3')

    def test_custom_drivers_and_disabled_logging_are_not_replaced(self):
        for driver in ('none', 'journald', 'syslog', 'fluentd'):
            config, owned = self.policy_fixture(driver, {'tag':'keep'})
            for name in r.COMPONENTS:
                config['services'][name]['logging'] = {'driver':driver, 'options':{'tag':'keep'}}
            original = copy.deepcopy(config)
            updated, policies = r.bounded_logging(config, owned)
            self.assertEqual(updated, original)
            self.assertEqual(policies, {})

    def test_invalid_log_limits_stop_before_service_changes(self):
        for options in ({'max-size':'bad'}, {'max-size':'0'}, {'max-file':'0'}, {'max-file':'-1'}, {'max-file':'x'}):
            config, owned = self.policy_fixture(options=options)
            with self.subTest(options=options), self.assertRaises(r.ReleaseError):
                r.bounded_logging(config, owned)

    def test_verify_checks_actual_docker_configuration(self):
        config, owned = self.policy_fixture()
        _, policies = r.bounded_logging(config, owned)
        for name in r.COMPONENTS:
            owned[name]['HostConfig']['LogConfig']['Config'] = dict(policies[name]['options'])
        r.verify_logging(owned, policies)
        owned['api']['HostConfig']['LogConfig']['Config']['max-file'] = '9'
        with self.assertRaises(r.ReleaseError): r.verify_logging(owned, policies)

if __name__ == '__main__': unittest.main()
