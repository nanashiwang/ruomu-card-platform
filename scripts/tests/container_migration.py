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
    baseline = r.STORE / 'baseline.json'
    r.config_file(baseline, config)
    r.apply_config(baseline)
    r.acquire = lambda _: json.loads(json.dumps(data))
    (r.STORE / revision).mkdir()
    own, other = r.inventory()
    baseline_env = dict(e.split('=', 1) for e in own['api']['Config']['Env'] if '=' in e)
    assert baseline_env['RUOMU_LITERAL_FIXTURE'] == 'literal-$VALUE-$$-end', 'baseline dollar escaping failed'
    database, _ = r.db_location(own['api'])
    before = r.database_signature(database)
    try:
        r.deploy(revision)
        assert r.database_signature(database) == before
        state = json.loads(r.STATE.read_text())
        current, _ = r.inventory()
        live_env = dict(e.split('=', 1) for e in current['api']['Config']['Env'] if '=' in e)
        assert live_env['RUOMU_LITERAL_FIXTURE'] == 'literal-$VALUE-$$-end'
        r.rollback(Path(state['backup']))
        assert not r.STATE.exists()
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
