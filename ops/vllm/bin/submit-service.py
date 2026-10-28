"""Keep one named Slurm batch job; stop cancels only the job submitted here."""
import json
import signal
import subprocess as S
import time
from pathlib import Path

BASE = Path('/home/llmops/llm')
state = BASE / 'state'
state.mkdir(exist_ok=True)
job_id = None

def stop(signum, frame):
    if job_id is not None:
        S.run(['/usr/local/bin/scancel', str(job_id)], check=False)
    raise SystemExit(0)

signal.signal(signal.SIGTERM, stop)
signal.signal(signal.SIGINT, stop)
# Prevent duplicate service jobs if the submitter was restarted abnormally.
existing = S.check_output(['/usr/local/bin/squeue', '-h', '-u', 'llmops', '-n', 'qwen38-vllm', '-o', '%A'], text=True).split()
if existing:
    saved = state / 'job-id'
    if len(existing) != 1 or not saved.exists() or saved.read_text().strip() != existing[0]:
        raise SystemExit('Unexpected qwen38-vllm job found; inspect it before restarting the submitter')
    job_id = int(existing[0])
    print('Adopted existing qwen38-vllm Slurm job', job_id, flush=True)
    while S.check_output(['/usr/local/bin/squeue', '-h', '-j', str(job_id), '-o', '%A'], text=True).strip():
        time.sleep(10)
    raise SystemExit(0)
proc = S.Popen(['/usr/local/bin/sbatch', '--wait', '--parsable', str(BASE / 'config/qwen38-vllm.sbatch')],
               stdout=S.PIPE, text=True)
try:
    line = proc.stdout.readline().strip()
    job_id = int(line.split(';', 1)[0])
    (state / 'job-id').write_text(str(job_id) + '\n')
    print('Submitted qwen38-vllm Slurm job', job_id, flush=True)
    result = proc.wait()
    (state / 'last-exit.json').write_text(json.dumps({'job_id': job_id, 'sbatch_exit': result}) + '\n')
    raise SystemExit(result)
finally:
    if proc.poll() is None:
        proc.terminate()
