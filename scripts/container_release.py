#!/usr/bin/env python3
"""Pinned three-image release. Existing Compose layout/data stay on this host.

Initial adoption requires matching source and config hashes. Only sqlite with a
named /app/db volume is supported. Unknown layouts fail before service mutation.
"""
from __future__ import annotations
import argparse
from contextlib import contextmanager
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import signal
import sqlite3
import subprocess
import sys
import tempfile
import time
from urllib.request import Request, urlopen

REPO = 'nanashiwang/ruomu-card-platform'
REGISTRY = 'ghcr.io/nanashiwang/ruomu-'
COMPONENTS = ('api', 'user', 'admin')
PROJECT = 'ruomu-card-platform'
ROOT = Path('/opt/ruomu-card-platform')
STORE = Path('/opt/ruomu-container-releases')
BACKUPS = Path('/opt/ruomu-backups/container-releases')
STATE = STORE / 'current.json'
PROBE = 'ruomu-release-probe'
OWNER = 'org.ruomu.release-probe'
SHA = r'[0-9a-f]{40}'
DIGEST = r'sha256:[0-9a-f]{64}'

class ReleaseError(RuntimeError):
    pass

def need(ok, message):
    if not ok:
        raise ReleaseError(message)

def run(*args, timeout=60, check=True, env=None):
    try:
        result = subprocess.run(args, text=True, capture_output=True, timeout=timeout, env=env)
    except (subprocess.TimeoutExpired, OSError):
        raise ReleaseError(f'{Path(args[0]).name}: command unavailable or timed out') from None
    if check and result.returncode:
        # stdout/stderr/arguments may contain environment values, never log them.
        raise ReleaseError(f'{Path(args[0]).name}: command failed (exit {result.returncode})')
    return result

def sha(path):
    h = hashlib.sha256()
    with Path(path).open('rb') as stream:
        for data in iter(lambda: stream.read(1024 * 1024), b''):
            h.update(data)
    return h.hexdigest()

def safe_path(path):
    need(path.is_absolute() and path.resolve() == path, 'Symlink/non-absolute path refused')
    return path

def save(path, data):
    safe_path(path)
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    fd, name = tempfile.mkstemp(prefix='.release-', dir=path.parent)
    try:
        with os.fdopen(fd, 'w') as stream:
            json.dump(data, stream, ensure_ascii=False, indent=2)
            stream.flush()
            os.fsync(stream.fileno())
        os.chmod(name, 0o600)
        os.replace(name, path)
    finally:
        Path(name).unlink(missing_ok=True)

def source_files(root, component):
    prefix = f'apps/{component}/'
    result = run('git', '-C', str(root), 'ls-files', '-z', prefix).stdout.split('\0')
    # Ignore tests/docs/build recipe: deployment migration must preserve the
    # application inputs, including lockfiles, public assets and nginx config.
    return sorted(p for p in result if p and not p.endswith(('_test.go', '.md'))
                  and Path(p).name not in {'Dockerfile', '.dockerignore'}
                  and not p.startswith(prefix + 'tests/'))

def source_fingerprint(root, component):
    files = source_files(root, component)
    need(files, 'Missing tracked application source')
    h = hashlib.sha256()
    for relative in files:
        path = safe_path(root / relative)
        need(path.is_file(), 'Application source file missing')
        h.update(relative.encode() + b'\0' + sha(path).encode() + b'\n')
    return h.hexdigest()

def validate_manifest(data, revision):
    need(re.fullmatch(SHA, revision), 'Expected a full commit SHA')
    need(isinstance(data, dict) and data.get('revision') == revision and data.get('platform') == 'linux/amd64', 'Invalid release revision/platform')
    need(set(data.get('components', {})) == set(COMPONENTS), 'Expected api/user/admin images')
    for name, image in data['components'].items():
        need(re.fullmatch(re.escape(REGISTRY + name) + '@' + DIGEST, image.get('image', '')), 'Unexpected repository/digest')
        need(re.fullmatch(DIGEST, image.get('image_id', '')), 'Invalid publisher image ID')
        for key in ('archive_sha256', 'source_sha256'):
            need(re.fullmatch(r'[0-9a-f]{64}', image.get(key, '')), 'Missing release checksum')
    return data

def download(url, target, limit):
    safe_path(target)
    need(not target.exists(), 'Download destination already exists')
    fd, name = tempfile.mkstemp(prefix='.download-', dir=target.parent)
    try:
        with os.fdopen(fd, 'wb') as stream, urlopen(Request(url, headers={'User-Agent': 'ruomu-release/1'}), timeout=60) as response:
            total = 0
            while chunk := response.read(1024 * 1024):
                total += len(chunk)
                need(total <= limit, 'Release asset exceeds size limit')
                stream.write(chunk)
        os.replace(name, target)
    finally:
        Path(name).unlink(missing_ok=True)

def acquire(revision):
    need(re.fullmatch(SHA, revision), 'Expected full commit SHA')
    directory = safe_path(STORE / revision)
    directory.mkdir(parents=True, exist_ok=True, mode=0o700)
    base = f'https://github.com/{REPO}/releases/download/card-{revision}/'
    manifest = directory / 'release.json'
    if not manifest.exists():
        download(base + 'release.json', manifest, 65536)
    data = validate_manifest(json.loads(manifest.read_text()), revision)
    for name, item in data['components'].items():
        print(f'STEP: acquire {name} pinned image', flush=True)
        result = run('docker', 'image', 'inspect', item['image'], check=False)
        source = 'registry'
        if result.returncode:
            pulled = run('docker', 'pull', item['image'], timeout=300, check=False)
            if not pulled.returncode:
                result = run('docker', 'image', 'inspect', item['image'])
            else:
                archive = directory / f'{name}.tar.gz'
                if archive.exists():
                    need(sha(archive) == item['archive_sha256'], 'Cached archive checksum mismatch')
                else:
                    download(base + archive.name, archive, 2 * 1024**3)
                need(sha(archive) == item['archive_sha256'], 'Archive checksum mismatch')
                loaded = run('docker', 'load', '--input', str(archive), timeout=300)
                tag = f'{REGISTRY}{name}:{revision}'
                need(f'Loaded image: {tag}' in loaded.stdout.splitlines(), 'Archive did not load expected tag')
                result = run('docker', 'image', 'inspect', tag)
                source = 'archive'
        info = json.loads(result.stdout)[0]
        labels = info.get('Config', {}).get('Labels') or {}
        if source == 'registry':
            need(item['image'] in (info.get('RepoDigests') or []), 'Registry digest mismatch')
        need(re.fullmatch(DIGEST, info.get('Id', '')), 'Invalid local image ID')
        need(info.get('Architecture') == 'amd64' and info.get('Os') == 'linux', 'Wrong image platform')
        need(labels.get('org.opencontainers.image.revision') == revision and
             labels.get('org.opencontainers.image.source') == f'https://github.com/{REPO}' and
             labels.get('org.ruomu.component') == name, 'Image ownership/revision mismatch')
        item['runtime_id'] = info['Id']
    return data

def containers():
    ids = run('docker', 'ps', '-aq').stdout.split()
    return json.loads(run('docker', 'inspect', *ids).stdout) if ids else []

def inventory(*, require_running=True):
    owned, others = {}, {}
    for item in containers():
        labels = item.get('Config', {}).get('Labels') or {}
        if labels.get('com.docker.compose.project') == PROJECT and labels.get('com.docker.compose.service') in COMPONENTS:
            service = labels['com.docker.compose.service']
            need(service not in owned and (not require_running or item['State']['Running']), 'Expected exactly one container per component (running required for preflight)')
            owned[service] = item
        else:
            others[item['Name']] = (item['Id'], item['State']['StartedAt'], item['State']['Status'])
    need(set(owned) == set(COMPONENTS), 'Expected current api/user/admin containers')
    return owned, others

def clean_env():
    # Environment must come from the existing Compose env file, not this shell.
    return {k: v for k, v in os.environ.items() if k in {'PATH', 'HOME', 'DOCKER_CONFIG'}}

def compose(path, *args):
    return run('docker', 'compose', '--project-name', PROJECT, '--project-directory', str(ROOT),
               '--env-file', '/dev/null', '-f', str(path), *args, timeout=240, env=clean_env())

def read_config(owned):
    files = {x['Config']['Labels'].get('com.docker.compose.project.config_files', '') for x in owned.values()}
    need(len(files) == 1, 'Components use different Compose config files')
    paths = next(iter(files)).split(',')
    need(len(paths) == 1, 'Multiple Compose overrides require explicit migration review')
    config_path = safe_path(Path(paths[0]))
    need(config_path.is_file() and (config_path.is_relative_to(ROOT) or config_path.is_relative_to(STORE) or config_path.is_relative_to(BACKUPS)), 'Unexpected Compose location')
    envfile = Path('/dev/null') if config_path.is_relative_to(STORE) or config_path.is_relative_to(BACKUPS) else ROOT / '.env'
    need(envfile.is_file() or envfile == Path('/dev/null'), 'Missing existing production .env')
    if envfile != Path('/dev/null'):
        safe_path(envfile)
    command = ['docker', 'compose', '--project-name', PROJECT, '--project-directory', str(ROOT),
               '--env-file', str(envfile), '-f', str(config_path)]
    config = json.loads(run(*command, 'config', '--format', 'json', env=clean_env()).stdout)
    hashes = dict(line.split() for line in run(*command, 'config', '--hash', '*', env=clean_env()).stdout.splitlines())
    for service, item in owned.items():
        need(hashes.get(service) == item['Config']['Labels'].get('com.docker.compose.config-hash'),
             f'{service}: running container differs from saved Compose config; reconcile before migration')
    need(config.get('name') == PROJECT, 'Compose project mismatch')
    return config

def pinned_config(config, images):
    result = json.loads(json.dumps(config))
    for name, identity in images.items():
        need(re.fullmatch(DIGEST, identity), 'Expected immutable image ID')
        service = result['services'][name]
        service.pop('build', None)
        service['image'] = identity
        service['pull_policy'] = 'never'
    return result

def config_file(path, config):
    # Compose interpolates even JSON. Escape literal dollars in resolved values.
    def escape(value):
        if isinstance(value, str):
            return value.replace('$', '$$')
        if isinstance(value, list):
            return [escape(v) for v in value]
        if isinstance(value, dict):
            return {k: escape(v) for k, v in value.items()}
        return value
    save(path, escape(config))

def health(owned):
    for name, item in owned.items():
        port = 8080 if name == 'api' else 80
        run('docker', 'exec', item['Id'], 'wget', '-q', '-O', '/dev/null', f'http://127.0.0.1:{port}/health', timeout=15)

def protected_files(owned):
    paths = [ROOT / '.env', ROOT / 'deploy', Path('/etc/nginx')]
    for item in owned.values():
        for mount in item.get('Mounts', []):
            if mount['Type'] == 'bind':
                # Bind mounts must be configuration/static assets; protect all.
                paths.append(Path(mount['Source']))
    found = {}
    for root in paths:
        for path in [root] if root.is_file() else root.rglob('*') if root.is_dir() else []:
            if path.is_file():
                found[str(path)] = sha(path)
    return found

def db_location(api):
    env = dict(e.split('=', 1) for e in api['Config']['Env'] if '=' in e)
    need(env.get('DATABASE_DRIVER', 'sqlite') == 'sqlite', 'Only existing SQLite deployments are supported')
    dsn = env.get('DATABASE_DSN', '/app/db/ruomu.db')
    need(re.fullmatch(r'/app/db/[A-Za-z0-9_-]+\.db', dsn), 'Custom database DSN requires review')
    mounts = [m for m in api['Mounts'] if m['Destination'] == '/app/db']
    need(len(mounts) == 1 and mounts[0]['Type'] == 'volume' and mounts[0]['RW'], 'Expected named writable database volume')
    root = safe_path(Path(mounts[0]['Source']))
    path = safe_path(root / Path(dsn).name)
    need(path.is_file(), 'Existing SQLite database missing')
    return path, env

def database_copy(source, target):
    need(not target.exists(), 'Database copy destination exists')
    with sqlite3.connect(source.as_uri() + '?mode=ro', uri=True) as original, sqlite3.connect(target) as copy:
        original.backup(copy)
        need(copy.execute('PRAGMA integrity_check').fetchone() == ('ok',), 'SQLite integrity check failed')
    target.chmod(0o600)

def database_signature(path):
    # Includes schema, all application rows and PRAGMA user_version. Never output
    # SQL, keys, card secrets or order data; hash stays in private local backup.
    h = hashlib.sha256()
    with sqlite3.connect(path.as_uri() + '?mode=ro', uri=True) as db:
        need(db.execute('PRAGMA integrity_check').fetchone() == ('ok',), 'SQLite integrity check failed')
        for row in db.iterdump():
            h.update(row.encode() + b'\n')
        h.update(str(db.execute('PRAGMA user_version').fetchone()).encode())
    return h.hexdigest()

def remove_probe():
    result = run('docker', 'inspect', PROBE, check=False)
    if result.returncode:
        return
    item = json.loads(result.stdout)[0]
    need(item['Config'].get('Labels', {}).get(OWNER) == 'true', 'Refusing unrelated probe container')
    run('docker', 'rm', '-f', PROBE)

def database_probe(image, database, original_env, folder):
    need(run('docker', 'inspect', PROBE, check=False).returncode != 0, 'Probe name already exists')
    work = folder / ('probe-' + secrets.token_hex(4))
    work.mkdir(mode=0o700)
    copied = work / database.name
    database_copy(database, copied)
    before = database_signature(copied)
    # No provider/merchant tokens or default-admin credentials enter this probe.
    env = {key: original_env[key] for key in ('APP_SECRET_KEY', 'JWT_SECRET', 'USER_JWT_SECRET') if key in original_env}
    need(len(env) == 3, 'Required application secret names missing')
    env.update({'SERVER_HOST': '127.0.0.1', 'SERVER_PORT': '8080', 'SERVER_MODE': 'release',
                'DATABASE_DRIVER': 'sqlite', 'DATABASE_DSN': '/app/db/' + database.name,
                'REDIS_ENABLED': 'false', 'QUEUE_ENABLED': 'false', 'EMAIL_ENABLED': 'false',
                'BOOTSTRAP_DEFAULT_ADMIN_USERNAME': '', 'BOOTSTRAP_DEFAULT_ADMIN_PASSWORD': ''})
    args = ['docker', 'run', '--name', PROBE, '--label', OWNER + '=true', '-d', '--pull', 'never',
            '--network', 'none', '--cpus', '0.5', '--memory', '512m', '--memory-swap', '512m',
            '--pids-limit', '128', '--log-driver', 'none', '--read-only', '--cap-drop', 'ALL',
            '--security-opt', 'no-new-privileges', '--mount', f'type=bind,src={work},dst=/app/db',
            '--tmpfs', '/app/logs:rw,size=16m', '--tmpfs', '/app/uploads:rw,size=16m', '--tmpfs', '/tmp:rw,size=16m']
    for key in env:
        args += ['--env', key]
    try:
        run(*args, image, './ruomu-api', '-mode', 'api', env={**clean_env(), **env})
        for _ in range(30):
            status = run('docker', 'exec', PROBE, 'wget', '-q', '-O', '/dev/null', 'http://127.0.0.1:8080/health', check=False, timeout=5)
            if status.returncode == 0:
                break
            time.sleep(1)
        else:
            raise ReleaseError('Isolated database compatibility probe failed')
    finally:
        remove_probe()
    need(database_signature(copied) == before, 'New/rollback image changes database on startup; separate migration review required')
    print('PASS: isolated startup leaves database schema and rows unchanged', flush=True)

def adoption_check(data):
    if STATE.exists():
        safe_path(STATE)
        current = json.loads(STATE.read_text())
        need(re.fullmatch(SHA, current.get('revision', '')), 'Invalid installed release record')
        owned, _ = inventory()
        for component, item in owned.items():
            need(item['Config']['Image'] == current['components'][component]['runtime_id'],
                 'Running images differ from the managed release record')
        print('PASS: running images match managed release record', flush=True)
        return
    for component, image in data['components'].items():
        need(source_fingerprint(ROOT, component) == image['source_sha256'],
             f'{component}: server source differs from release; do not bundle business changes into migration')
    print('PASS: tracked application source matches release', flush=True)

def preflight():
    need(os.geteuid() == 0, 'Run as root')
    need(run('uname', '-m').stdout.strip() == 'x86_64', 'This release requires x86_64')
    safe_path(ROOT)
    need(ROOT.is_dir(), 'Expected existing card installation')
    for path in (STORE, BACKUPS):
        safe_path(path)
        path.mkdir(parents=True, exist_ok=True, mode=0o700)
        need(path.stat().st_mode & 0o077 == 0, 'Release/backup directory must be private (0700)')
    need(shutil.disk_usage(ROOT).free >= 2 * 1024**3, 'At least 2 GiB free disk space required')
    mem = dict((k, int(v.split()[0])) for k, v in (line.split(':', 1) for line in Path('/proc/meminfo').read_text().splitlines()))
    need(mem['MemAvailable'] >= 704 * 1024, 'Insufficient RAM for isolated compatibility check')
    owned, others = inventory()
    config = read_config(owned)
    health(owned)
    database, env = db_location(owned['api'])
    # Non-target services and volume/network identity must survive unchanged.
    for name in COMPONENTS:
        need(config['services'][name].get('scale', 1) == 1, 'Scaled services require review')
    need(not any(m['Type'] == 'bind' for m in owned['api']['Mounts']), 'API bind mounts require explicit probe support')
    db_mount = next(m for m in owned['api']['Mounts'] if m['Destination'] == '/app/db')
    for item in containers():
        if item['Id'] != owned['api']['Id'] and item['State']['Running']:
            need(not any(m.get('Source') == db_mount['Source'] for m in item.get('Mounts', [])), 'Another container can access the card database')
    print('PASS: existing project, config hashes, health and database layout; no secret values shown', flush=True)
    return owned, others, config, database, env

def stop_api_for_recovery():
    # compose can fail after replacing only one of the three components.
    # Recovery must not require every new component to be running/present.
    apis = [x for x in containers() if (x['Config'].get('Labels') or {}).get('com.docker.compose.project') == PROJECT
            and (x['Config'].get('Labels') or {}).get('com.docker.compose.service') == 'api']
    need(len(apis) <= 1, 'Ambiguous API containers during recovery')
    for item in apis:
        if item['State']['Running']:
            run('docker', 'stop', '--time', '45', item['Id'], timeout=90)


def restart_previous(path):
    compose(path, 'up', '-d', '--no-build', '--pull', 'never', '--no-deps', '--force-recreate', *COMPONENTS)
    wait_health()

def wait_health():
    for _ in range(45):
        try:
            owned, _ = inventory()
            health(owned)
            return owned
        except ReleaseError:
            time.sleep(1)
    raise ReleaseError('Application health did not recover; inspect card services locally')

def apply_config(path):
    compose(path, 'up', '-d', '--no-build', '--pull', 'never', '--no-deps', '--force-recreate', *COMPONENTS)
    return wait_health()

def deploy(revision):
    need(re.fullmatch(SHA, revision), 'Expected full commit SHA')
    owned, others, config, database, env = preflight()
    data = acquire(revision)
    adoption_check(data)
    folder = BACKUPS / (time.strftime('%Y%m%dT%H%M%SZ', time.gmtime()) + '-' + secrets.token_hex(3))
    folder.mkdir(mode=0o700)
    previous = pinned_config(config, {name: item['Image'] for name, item in owned.items()})
    next_config = pinned_config(config, {name: item['runtime_id'] for name, item in data['components'].items()})
    old_path, new_path = folder / 'previous.json', STORE / revision / 'compose.json'
    config_file(old_path, previous)
    config_file(new_path, next_config)
    before_files = protected_files(owned)
    before_state = json.loads(STATE.read_text()) if STATE.exists() else None
    save(folder / 'rollback.json', {'state': before_state, 'config': previous, 'revision': revision})
    save(folder / 'protected.json', {'files': before_files, 'others': others})
    # Validate generated config without printing its secret-bearing values.
    compose(new_path, 'config', '--quiet')
    stopped = False
    switched = False
    try:
        print('STEP: brief card maintenance; stopping API and queue worker together', flush=True)
        stopped = True
        run('docker', 'stop', '--time', '45', owned['api']['Id'], timeout=90)
        info = json.loads(run('docker', 'inspect', owned['api']['Id']).stdout)[0]
        need(not info['State']['Running'] and info['State']['ExitCode'] == 0, 'API did not exit gracefully; refusing database cutover')
        database_copy(database, folder / database.name)
        sig = database_signature(database)
        database_probe(data['components']['api']['runtime_id'], database, env, folder)
        need(database_signature(database) == sig, 'Database changed while API stopped')
        switched = True
        current = apply_config(new_path)
        for name, item in current.items():
            need(item['Config']['Image'] == data['components'][name]['runtime_id'], 'Running image reference differs from release')
        need(protected_files(current) == before_files, 'Protected configuration changed')
        need(inventory()[1] == others, 'Another project/container changed during deployment')
        save(STATE, {'revision': revision, 'components': data['components'], 'backup': str(folder), 'compose': str(new_path)})
        stopped = False
        print('PASS: preserved configuration, volumes and other project containers', flush=True)
    finally:
        if stopped:
            if switched:
                # Never restore the database. Old program is permitted only after
                # proving compatibility against a copy of the CURRENT database.
                stop_api_for_recovery()
                database_probe(owned['api']['Image'], database, env, folder)
                restart_previous(old_path)
            else:
                run('docker', 'start', owned['api']['Id'])
                wait_health()
            print('ROLLBACK_SUCCESS: previous programs restored; live database retained', flush=True)
    print('RUOMU_LEFTOVER_TEST_CONTAINERS=0')
    print('DEPLOY_SUCCESS=' + revision)
    print('ROLLBACK_DIRECTORY=' + str(folder))
    print('No caches, volumes, images or backups were deleted.')

def rollback(folder):
    safe_path(folder)
    need(folder.parent == BACKUPS and folder.is_dir(), 'Unexpected rollback directory')
    old = json.loads((folder / 'rollback.json').read_text())
    owned, others, config, database, env = preflight()
    current_path = folder / 'rollback-recovery.json'
    config_file(current_path, pinned_config(config, {name: item['Image'] for name, item in owned.items()}))
    run('docker', 'stop', '--time', '45', owned['api']['Id'], timeout=90)
    try:
        database_probe(old['config']['services']['api']['image'], database, env, folder)
        path = folder / 'rollback-target.json'
        config_file(path, old['config'])
        apply_config(path)
        need(inventory()[1] == others, 'Another project/container changed')
        if old['state'] is None:
            STATE.unlink(missing_ok=True)
        else:
            save(STATE, old['state'])
    except (Exception, KeyboardInterrupt):
        stop_api_for_recovery()
        database_probe(owned['api']['Image'], database, env, folder)
        restart_previous(current_path)
        raise
    print('ROLLBACK_SUCCESS: previous programs restored; live database retained')

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest='command', required=True)
    sub.add_parser('preflight')
    sub.add_parser('deploy').add_argument('revision')
    sub.add_parser('rollback').add_argument('directory', type=Path)
    args = parser.parse_args()
    need(os.geteuid() == 0, 'Run as root')
    os.umask(0o077)
    for sig in (signal.SIGTERM, signal.SIGHUP):
        signal.signal(sig, lambda *a: (_ for _ in ()).throw(ReleaseError('Deployment interrupted')))
    with open('/run/lock/ruomu-container-release.lock', 'a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        if args.command == 'preflight': preflight()
        elif args.command == 'deploy': deploy(args.revision)
        else: rollback(args.directory)

if __name__ == '__main__':
    try:
        main()
    except (Exception, KeyboardInterrupt) as error:
        print('FAIL: ' + (str(error) if isinstance(error, ReleaseError) else type(error).__name__ + ' (details withheld)'), file=sys.stderr)
        sys.exit(1)
