import concurrent.futures as F
import hashlib
import json
import subprocess as S
import time
from pathlib import Path

base = Path('/home/llmops/llm')
revision = '6128240ebaf4eaa7bad2b3d1c72c37d677c5f462'
dest = base / 'models' / 'Qwen3.8-27B-NVFP4'
dest.mkdir(parents=True, exist_ok=True)
files = json.loads((base / 'config' / 'model-files.json').read_text())

def check(path, item):
    if not path.exists() or path.stat().st_size != item['size']:
        return False
    lfs = item.get('lfs')
    hasher = hashlib.sha256() if lfs else hashlib.sha1()
    if not lfs:
        hasher.update(b'blob ' + str(item['size']).encode() + b'\0')
    with path.open('rb') as f:
        for block in iter(lambda: f.read(4 * 1024**2), b''):
            hasher.update(block)
    return hasher.hexdigest() == (lfs['oid'] if lfs else item['oid'])

def download(item):
    name = item['path']
    # No remote Python code is needed by this built-in model architecture.
    if item.get('type') != 'file' or name == '.gitattributes':
        return
    target = dest / name
    assert target.parent == dest
    if check(target, item):
        print('VERIFIED existing', name, flush=True)
        return
    partial = target.with_name(target.name + '.partial')
    for attempt in range(1, 9):
        url = f'https://hf-mirror.com/Inferact/Qwen3.8-27B-NVFP4/resolve/{revision}/{name}?download=true&attempt={int(time.time())}'
        cmd = ['curl', '--noproxy', '*', '-fsSL', '--connect-timeout', '15',
               '--max-time', '900', '--speed-limit', '1024', '--speed-time', '60',
               '--limit-rate', '40M', '--output', str(partial)]
        if partial.exists() and partial.stat().st_size:
            cmd += ['--continue-at', '-']
        result = S.run(cmd + [url], stdout=S.DEVNULL, stderr=S.PIPE, text=True)
        if result.returncode == 0 and check(partial, item):
            partial.replace(target)
            print('VERIFIED downloaded', name, item['size'], flush=True)
            return
        if result.returncode == 0:
            partial.unlink(missing_ok=True)
            print('Checksum mismatch; retrying', name, flush=True)
        else:
            print('Download retry', name, attempt, 'curl_status', result.returncode, flush=True)
        time.sleep(min(30, 3 * attempt))
    raise RuntimeError('Unable to download and verify ' + name)

with F.ThreadPoolExecutor(max_workers=6) as pool:
    for result in pool.map(download, files):
        pass
(base / 'config' / 'model-verified.json').write_text(json.dumps({
    'repo': 'Inferact/Qwen3.8-27B-NVFP4', 'revision': revision,
    'download_endpoint': 'https://hf-mirror.com',
    'integrity': 'All downloaded files match the LFS SHA-256 or Git blob SHA-1 from the primary Hugging Face API manifest.',
    'excluded': ['.gitattributes'],
}, indent=2) + '\n')
print('MODEL_DOWNLOAD_AND_VERIFICATION_COMPLETE', flush=True)
