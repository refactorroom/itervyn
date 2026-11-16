#!/usr/bin/env python3
"""Build/rebuild four deterministic local archives. Never tags or publishes."""
import argparse
import gzip
import hashlib
import io
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--go', default='go', help='selected Go 1.27.4+ executable')
    args = parser.parse_args()
    root = Path(__file__).resolve().parent.parent
    dist = root / 'dist'
    dist.mkdir(exist_ok=True)
    checksums = []
    for system, arch in [('darwin', 'arm64'), ('darwin', 'amd64'), ('linux', 'arm64'), ('linux', 'amd64')]:
        env = dict(os.environ, GOOS=system, GOARCH=arch, CGO_ENABLED='0',
                   GOTOOLCHAIN='local', GOPROXY='off', GOSUMDB='off')
        with tempfile.TemporaryDirectory(prefix='repro-', dir=dist) as tmp:
            first, second = Path(tmp) / 'first', Path(tmp) / 'second'
            for path in [first, second]:
                subprocess.run([args.go, 'build', '-trimpath', '-buildvcs=false',
                                '-ldflags=-s -w -buildid=', '-o', str(path),
                                './cmd/itervyn'], cwd=root, env=env, check=True)
            if first.read_bytes() != second.read_bytes():
                raise SystemExit(f'reproducibility mismatch: {system}/{arch}')
            payload = io.BytesIO()
            with tarfile.open(fileobj=payload, mode='w') as archive:
                files = [('itervyn', first)]
                for name in ['LICENSE', 'README.md', 'CONTRIBUTING.md', 'SECURITY.md']:
                    files.append((name, root / name))
                for directory in ['docs', 'schemas', 'examples']:
                    files += [(str(p.relative_to(root)), p) for p in sorted((root / directory).rglob('*'))
                              if p.is_file() and '.itervyn' not in p.parts and p.name != 'guard-bin']
                for name, path in files:
                    data = path.read_bytes()
                    info = tarfile.TarInfo(name)
                    info.size = len(data)
                    info.mode = 0o755 if name == 'itervyn' else 0o644
                    info.mtime = 0
                    archive.addfile(info, io.BytesIO(data))
            name = f'itervyn_v0.1.0_{system}_{arch}.tar.gz'
            output = dist / name
            with output.open('wb') as f:
                with gzip.GzipFile(filename='', fileobj=f, mode='wb', mtime=0) as compressed:
                    compressed.write(payload.getvalue())
            digest = hashlib.sha256(output.read_bytes()).hexdigest()
            checksums.append(f'{digest}  {name}\n')
            print(f'{system}/{arch}: reproducible binary; archive {digest}')
    (dist / 'SHA256SUMS').write_text(''.join(checksums))


if __name__ == '__main__':
    main()
