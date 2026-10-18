#!/usr/bin/env python3
"""Read-only review of four named card build artifacts. Never delete or prune.
No database/log bodies, environment values, or raw Docker inspect are printed.
"""
import argparse
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys

VOLUMES = ('ruomu-go-build-cache', 'ruomu-go-build-cache-cgo', 'ruomu-go-mod-cache')
IMAGE = 'ruomu-api-test-builder:latest'
ROOT = Path('/opt/ruomu-card-platform')
STORE = Path('/opt/ruomu-container-releases')
BACKUPS = Path('/opt/ruomu-backups/container-releases')

class AuditError(RuntimeError):
    pass

def command(*args, timeout=30):
    env = {**os.environ, 'DOCKER_CONTEXT': 'default', 'DOCKER_HOST': 'unix:///var/run/docker.sock'}
    try:
        result = subprocess.run(args, capture_output=True, text=True, timeout=timeout, env=env)
    except (OSError, subprocess.TimeoutExpired):
        raise AuditError('Read-only metadata command unavailable or timed out') from None
    if result.returncode:
        raise AuditError('Read-only metadata command failed; details withheld')
    return result.stdout

def resolved(path):
    if not path.is_absolute() or path.resolve() != path:
        raise AuditError('Unexpected symlink or relative audit path')
    return path

def references(value, names):
    if isinstance(value, str):
        return value in names
    if isinstance(value, list):
        return any(references(v, names) for v in value)
    if isinstance(value, dict):
        return any(references(v, names) for v in value.values())
    return False

def saved_references(names):
    found = []
    paths = [STORE / 'current.json', *STORE.glob('*/compose.json'), *BACKUPS.glob('*/rollback.json')]
    if len(paths) > 500:
        raise AuditError('Too many release records; review manually')
    for path in paths:
        if not path.exists():
            continue
        resolved(path)
        try:
            if path.stat().st_size > 4 * 1024**2:
                raise AuditError('Release record exceeds audit limit')
            value = json.loads(path.read_text())
        except (OSError, ValueError):
            raise AuditError('Cannot inspect a release record safely') from None
        if references(value, names):
            found.append(str(path))
    return found

def cache_layout(path, name):
    resolved(path)
    if not path.is_dir():
        raise AuditError('Cache path is not a directory')
    names = []
    for child in path.iterdir():
        if len(names) >= 512 or child.is_symlink():
            return 'unrecognized-or-symlink'
        names.append(child.name)
    if name == 'ruomu-go-mod-cache':
        if (path / 'cache/download').is_dir() and (path / 'cache/download').resolve() == path / 'cache/download':
            return 'module-cache-layout; contents and offline dependencies still need review'
    elif names and all(re.fullmatch(r'[0-9a-f]{2}', n) or n in {'README', 'trim.txt'} for n in names):
        return 'go-build-cache-layout; contents still need review'
    return 'unrecognized-layout; retain'

def inspect_audit():
    ids = command('docker', 'ps', '-aq').split()
    containers = json.loads(command('docker', 'inspect', *ids)) if ids else []
    disk = shutil.disk_usage(ROOT)
    result = {'read_only': True, 'deleted_objects': 0,
              'disk_bytes': {'total': disk.total, 'used': disk.used, 'free': disk.free},
              'card_logging': [], 'candidates': [],
              'rule': 'Zero references is not deletion approval; no cleanup command is provided.'}
    for item in containers:
        labels = item['Config'].get('Labels') or {}
        if labels.get('com.docker.compose.project') != 'ruomu-card-platform':
            continue
        service = labels.get('com.docker.compose.service')
        if service not in {'api', 'user', 'admin'}:
            continue
        log = item['HostConfig'].get('LogConfig') or {}
        options = log.get('Config') or {}
        # No tag, external logging address or log content is exposed.
        result['card_logging'].append({'service': service, 'running': item['State']['Running'],
            'driver': log.get('Type'), 'max_size': options.get('max-size', 'driver-default'),
            'max_file': options.get('max-file', 'driver-default')})
    docker_root = resolved(Path(command('docker', 'info', '--format', '{{.DockerRootDir}}').strip()))
    existing_volumes = set(command('docker', 'volume', 'ls', '--format', '{{.Name}}').splitlines())
    for name in VOLUMES:
        row = {'kind': 'volume', 'name': name, 'deletion_approved': False, 'last_used': 'not-known'}
        if name not in existing_volumes:
            row['exists'] = False
            result['candidates'].append(row)
            continue
        volume = json.loads(command('docker', 'volume', 'inspect', name))[0]
        path = Path(volume.get('Mountpoint', ''))
        if volume.get('Driver') != 'local' or path != docker_root / 'volumes' / name / '_data' or volume.get('Options'):
            raise AuditError('Unexpected volume backend/options; retain all candidates')
        resolved(path)
        row.update({'exists': True, 'created_at': volume.get('CreatedAt'),
                    'container_references': [c['Name'] for c in containers if any(m.get('Name') == name or m.get('Source') == str(path) for m in c.get('Mounts', []))],
                    'release_references': saved_references({name, str(path)}),
                    'layout': cache_layout(path, name)})
        row['allocated_bytes'] = int(command('ionice', '-c3', 'nice', '-n19', 'du', '-skx', '--', str(path), timeout=30).split()[0]) * 1024
        result['candidates'].append(row)
    images = command('docker', 'image', 'ls', '--format', '{{.Repository}}:{{.Tag}}').splitlines()
    row = {'kind': 'image', 'name': IMAGE, 'deletion_approved': False, 'last_used': 'not-known'}
    if IMAGE in images:
        image = json.loads(command('docker', 'image', 'inspect', IMAGE))[0]
        names = {image['Id'], IMAGE, *(image.get('RepoDigests') or []), *(image.get('RepoTags') or [])}
        row.update({'exists': True, 'id': image['Id'], 'created_at': image.get('Created'), 'virtual_size_bytes': image.get('Size'),
            'container_references': [c['Name'] for c in containers if c['Image'] in names or c['Config']['Image'] in names or (c.get('ImageManifestDescriptor') or {}).get('digest') in names],
            'release_references': saved_references(names),
            'pending': 'Check build scripts and rebuild availability before deletion'})
    else:
        row['exists'] = False
    result['candidates'].append(row)
    return result

def main():
    argparse.ArgumentParser(description=__doc__).parse_args()
    if os.geteuid() != 0:
        raise AuditError('Run as root to inspect local volume metadata')
    print(json.dumps(inspect_audit(), ensure_ascii=False, indent=2))
    print('RUOMU_AUDIT_BACKGROUND_PROCESSES=0')

if __name__ == '__main__':
    try:
        main()
    except (AuditError, OSError, ValueError, KeyError) as error:
        print('AUDIT_FAILED: ' + (str(error) if isinstance(error, AuditError) else 'metadata invalid; details withheld'), file=sys.stderr)
        sys.exit(1)
