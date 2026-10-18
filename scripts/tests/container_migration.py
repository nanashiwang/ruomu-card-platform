#!/usr/bin/env python3
"""Destructive only inside a unique GitHub-hosted disposable routing fixture."""
import importlib.util
import json
import os
import shutil
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('release', ROOT / 'scripts/container_release.py')
r = importlib.util.module_from_spec(spec)
spec.loader.exec_module(r)

def main():
    assert os.environ.get('GITHUB_ACTIONS') == 'true'
    assert os.environ.get('RUNNER_ENVIRONMENT') == 'github-hosted'
    assert os.geteuid() == 0
    project, folder, revision = sys.argv[1:]
    assert project.startswith('ruomu-routing-')
    temp = Path(folder).resolve()
    assert temp.name.startswith('ruomu-routing-') and temp.parent == Path('/tmp')
    r.PROJECT, r.ROOT = project, ROOT
    r.STORE, r.BACKUPS = temp / 'releases', temp / 'backups'
    r.STATE = r.STORE / 'current.json'
    r.STORE.mkdir(mode=0o700)
    r.BACKUPS.mkdir(mode=0o700)
    command = ['docker', 'compose', '--project-name', project, '--project-directory', str(ROOT),
               '--env-file', str(temp / '.env'), '-f', str(ROOT / 'deploy/docker-compose.prod.yml'),
               '-f', str(temp / 'images.json')]
    config = r.decoded_compose_config(r.run(*command, 'config', '--format', 'json', env=r.clean_env()).stdout)
    data = {'revision': revision, 'platform': 'linux/amd64', 'components': {}}
    for name in r.COMPONENTS:
        identity = json.loads(r.run('docker', 'image', 'inspect', f'{r.REGISTRY}{name}:{revision}').stdout)[0]['Id']
        data['components'][name] = {'runtime_id': identity, 'source_sha256': r.source_fingerprint(ROOT, name)}
    config = r.pinned_config(config, {name: item['runtime_id'] for name, item in data['components'].items()})
    # Test real dollar values survive resolved Compose -> private snapshot.
    config['services']['api']['environment']['RUOMU_LITERAL_FIXTURE'] = 'literal-$VALUE-$$-end'
    # Start with unbounded json-file logs, as on a legacy installation.
    for name in r.COMPONENTS:
        config['services'][name]['logging'] = {'driver': 'json-file', 'options': {'max-size': '-1'}}
    # Reproduce production: original Compose workdir is a deploy subdirectory,
    # and frontends mount relative ./nginx/*.conf paths. Reading with ROOT would
    # silently resolve different paths and must fail the config-hash guard.
    legacy = r.STORE / 'legacy/deploy'
    (legacy / 'nginx').mkdir(parents=True, mode=0o700)
    for name in ('user', 'admin'):
        shutil.copyfile(ROOT / f'apps/{name}/nginx.conf', legacy / f'nginx/{name}.conf')
        config['services'][name]['volumes'] = [{
            'type': 'bind', 'source': f'./nginx/{name}.conf',
            'target': '/etc/nginx/conf.d/default.conf', 'read_only': True,
        }]
    baseline = legacy / 'compose.json'
    r.config_file(baseline, config)
    r.run('docker', 'compose', '--project-name', project, '--project-directory', str(legacy),
          '--env-file', '/dev/null', '-f', str(baseline), 'up', '-d', '--no-build', '--pull', 'never',
          '--no-deps', '--force-recreate', *r.COMPONENTS, env=r.clean_env(), timeout=240)
    r.wait_health()
    r.acquire = lambda _: json.loads(json.dumps(data))
    (r.STORE / revision).mkdir()
    own, other = r.inventory()
    baseline_env = dict(e.split('=', 1) for e in own['api']['Config']['Env'] if '=' in e)
    assert baseline_env['RUOMU_LITERAL_FIXTURE'] == 'literal-$VALUE-$$-end', 'baseline dollar escaping failed'
    parsed = r.read_config(own)
    for name in ('user', 'admin'):
        expected = str(legacy / f'nginx/{name}.conf')
        assert parsed['services'][name]['volumes'][0]['source'] == expected
    print('PASS: original deploy working directory resolves relative nginx mounts and exact hashes')
    database, _ = r.db_location(own['api'])
    before = r.database_signature(database)
    try:
        r.deploy(revision)
        assert r.database_signature(database) == before
        state = json.loads(r.STATE.read_text())
        current, _ = r.inventory()
        r.verify_logging(current, state['logging'])
        assert all(current[n]['HostConfig']['LogConfig']['Config']['max-size'] == '20m' for n in r.COMPONENTS)
        assert all(current[n]['HostConfig']['LogConfig']['Config']['max-file'] == '3' for n in r.COMPONENTS)
        print('PASS: actual application containers have bounded Docker logs')
        live_env = dict(e.split('=', 1) for e in current['api']['Config']['Env'] if '=' in e)
        assert live_env['RUOMU_LITERAL_FIXTURE'] == 'literal-$VALUE-$$-end'
        for name in ('user', 'admin'):
            mounts = current[name]['Mounts']
            assert len(mounts) == 1 and mounts[0]['Source'] == str(legacy / f'nginx/{name}.conf') and not mounts[0]['RW']
        r.rollback(Path(state['backup']))
        assert not r.STATE.exists()
        restored, _ = r.inventory()
        assert all(restored[n]['HostConfig']['LogConfig']['Config']['max-size'] == '-1' for n in r.COMPONENTS)
        print('PASS: rollback restores previous log configuration')
        assert r.database_signature(database) == before
        print('PASS: real source/config checks, isolated DB clone, three-image migration and rollback')
        original_apply = r.apply_config
        def fail_after_cutover(path):
            original_apply(path)
            raise r.ReleaseError('injected post-cutover failure')
        r.apply_config = fail_after_cutover
        try:
            r.deploy(revision)
            raise AssertionError('injected failure did not fail')
        except r.ReleaseError as error:
            assert str(error) == 'injected post-cutover failure'
        r.health(r.inventory()[0])
        assert r.database_signature(database) == before
        assert r.inventory()[1] == other
        print('PASS: injected cutover failure restores programs without restoring live database')
    finally:
        r.remove_probe()
        assert r.run('docker', 'inspect', r.PROBE, check=False).returncode != 0
        for path in (r.STORE, r.BACKUPS):
            assert path.parent == temp
            shutil.rmtree(path)
        print('RUOMU_LEFTOVER_COMPATIBILITY_CONTAINERS=0')

if __name__ == '__main__':
    main()
