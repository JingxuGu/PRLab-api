import json
import subprocess as S
import time
from pathlib import Path
import httpx

BASE = Path('/home/llmops/llm')

def job_info(job):
    text = S.check_output(['scontrol', 'show', 'job', str(job), '-o'], text=True)
    return dict(t.split('=', 1) for t in text.split() if '=' in t)

def ready():
    try:
        return httpx.get('http://127.0.0.1:8000/health', timeout=5, trust_env=False).status_code == 200
    except httpx.HTTPError:
        return False

def wait_for(fn, label, timeout):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if fn():
            return
        time.sleep(2)
    raise RuntimeError('Timed out waiting for ' + label)

assert ready(), 'Model must be ready before the preemption test'
low = int((BASE / 'state/job-id').read_text())
restarts_before = int(job_info(low)['Restarts'])
started = time.monotonic()
high = int(S.check_output([
    'sbatch', '--parsable', '-p', 'research', '--gres=gpu:rtx_5090:1', '-n', '1', '-c', '1',
    '--mem=512M', '-t', '00:01:00', '--job-name=verify-real-model-preemption',
    '--output=' + str(BASE / 'logs/preempt-research-%j.log'),
    '--wrap', '/home/llmops/llm/venv/bin/python /home/llmops/slurm-setup/tests/check-gpu.py && sleep 15',
], text=True).strip().split(';', 1)[0])
print('Submitted short research test:', high, 'preempting model job:', low, flush=True)
try:
    wait_for(lambda: job_info(high)['JobState'] == 'RUNNING', 'research allocation', 180)
    wait_for(lambda: not ready(), 'gateway unavailable during preemption', 30)
    wait_for(lambda: (BASE / f'logs/preempt-research-{high}.log').exists() and 'PASS CUDA' in (BASE / f'logs/preempt-research-{high}.log').read_text(), 'research CUDA validation', 30)
    used = int(S.check_output(['nvidia-smi', '--query-gpu=memory.used', '--format=csv,noheader,nounits'], text=True).strip())
    assert used < 3000, 'Model GPU memory was not released: ' + str(used)
    research_start_seconds = round(time.monotonic() - started, 2)
    print('PASS research acquired GPU and CUDA works; model VRAM released:', used, 'MiB', flush=True)
    wait_for(lambda: job_info(high)['JobState'] == 'COMPLETED', 'research completion', 120)
    wait_for(lambda: ready() and int(job_info(low)['Restarts']) > restarts_before, 'model requeue and readiness', 900)
    result = {'passed': True, 'model_job': low, 'research_job': high,
              'research_start_seconds': research_start_seconds,
              'gpu_memory_during_research_mib': used,
              'model_restarts_after': int(job_info(low)['Restarts']),
              'total_test_seconds': round(time.monotonic() - started, 2)}
    (BASE / 'config/preemption-verification.json').write_text(json.dumps(result, indent=2) + '\n')
    print('PASS model automatically reloaded and API ready:', result, flush=True)
finally:
    info = job_info(high)
    if info['JobState'] in ('PENDING', 'RUNNING'):
        S.run(['scancel', str(high)], check=False)
