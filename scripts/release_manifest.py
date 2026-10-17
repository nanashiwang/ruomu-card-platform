#!/usr/bin/env python3
"""CI-only: package the already-tested immutable images as a release manifest."""
import argparse
import importlib.util
import json
from pathlib import Path
import subprocess

spec = importlib.util.spec_from_file_location('release', Path(__file__).with_name('container_release.py'))
r = importlib.util.module_from_spec(spec)
spec.loader.exec_module(r)

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('revision')
    parser.add_argument('directory', type=Path)
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[1]
    result = {'revision': args.revision, 'platform': 'linux/amd64', 'components': {}}
    for name in r.COMPONENTS:
        tag = r.REGISTRY + name + ':' + args.revision
        info = json.loads(subprocess.check_output(['docker', 'image', 'inspect', tag]))[0]
        digest = next(d for d in info['RepoDigests'] if d.startswith(r.REGISTRY + name + '@sha256:'))
        result['components'][name] = {
            'image': digest, 'image_id': info['Id'],
            'archive_sha256': r.sha(args.directory / (name + '.tar.gz')),
            'source_sha256': r.source_fingerprint(root, name),
        }
    r.validate_manifest(result, args.revision)
    (args.directory / 'release.json').write_text(json.dumps(result, indent=2) + '\n')

if __name__ == '__main__':
    main()
