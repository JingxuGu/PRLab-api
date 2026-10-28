import os
import subprocess as S
import time
from pathlib import Path

base = Path('/home/llmops/slurm-setup/tests')
marker = base / 'gpu-guard-marker'
marker.unlink(missing_ok=True)
env = dict(os.environ, GPU_TEST_HOLD_SECONDS='15', GPU_TEST_MARKER=str(marker))
with (base / 'gpu-guard-context.log').open('w') as log:
    job = S.Popen(['srun', '-p', 'research', '--gres=gpu:rtx_5090:1', '-n', '1', '-c', '1',
                   '--mem=512M', '-t', '00:01:00', '--job-name=verify-gpu-guard',
                   '/usr/bin/python3', str(base / 'check-gpu.py')], env=env, stdout=log, stderr=S.STDOUT)
    try:
        end = time.monotonic() + 45
        while not marker.exists():
            if job.poll() is not None:
                raise RuntimeError('GPU validation ended before its context was ready')
            if time.monotonic() > end:
                raise RuntimeError('GPU validation did not become ready')
            time.sleep(.25)
        pid = int(marker.read_text())
        guard = S.run(['/etc/slurm/gpu-idle-prolog.sh'], env=dict(os.environ, SLURM_JOB_GPUS='0', SLURM_JOB_ID='manual-guard-validation'), capture_output=True, text=True)
        assert guard.returncode == 1, guard
        os.kill(pid, 0)
        print('PASS busy GPU guard refused admission and left the existing CUDA context alive', flush=True)
        assert job.wait(timeout=30) == 0
    finally:
        if job.poll() is None:
            job.terminate()
            job.wait(timeout=20)
        marker.unlink(missing_ok=True)
