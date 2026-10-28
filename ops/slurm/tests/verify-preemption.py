import json
import subprocess as S
import time
from pathlib import Path

base = Path('/home/llmops/slurm-setup/tests')
tracked = []

def run(*args):
    return S.check_output(args, text=True).strip()

def state(job):
    text = run('scontrol', 'show', 'job', str(job), '-o')
    return dict(token.split('=', 1) for token in text.split() if '=' in token)

def wait_for(fn, label, seconds=75):
    end = time.monotonic() + seconds
    while time.monotonic() < end:
        if fn():
            return
        time.sleep(1)
    raise RuntimeError('Timed out waiting for ' + label)

def submit(partition, name, script=None):
    cmd = ['sbatch', '--parsable', '-p', partition, '--gres=gpu:rtx_5090:1',
           '-n', '1', '-c', '1', '--mem=256M', '-t', '00:03:00', '--requeue',
           '--job-name=' + name, '--open-mode=append', '--output=' + str(base / (name + '-%j.log'))]
    if script:
        cmd.append(str(script))
    else:
        cmd += ['--wrap', '/bin/sleep 30']
    job = int(run(*cmd).split(';')[0])
    tracked.append(job)
    return job

low_script = base / 'preemption-low.sbatch'
low_script.write_text('#!/bin/bash\nprintf "START restart=%s\\n" "${SLURM_RESTART_COUNT:-0}"\nsleep 150\n')

S.check_call(['sudo', '-n', 'scontrol', 'update', 'PartitionName=llm', 'State=UP'])
try:
    low = submit('llm', 'verify-low', low_script)
    logfile = base / f'verify-low-{low}.log'
    wait_for(lambda: state(low)['JobState'] == 'RUNNING' and logfile.exists() and 'START' in logfile.read_text(), 'low-priority GPU job')
    second = submit('llm', 'verify-queued')
    time.sleep(2)
    queued = state(second)
    assert queued['JobState'] == 'PENDING', queued
    print('PASS full-GPU exclusivity: second LLM job pending:', queued.get('Reason'), flush=True)
    S.check_call(['scancel', str(second)])
    high = submit('research', 'verify-research')
    wait_for(lambda: state(high)['JobState'] == 'RUNNING', 'research preemption')
    low_state = state(low)
    assert low_state['JobState'] == 'PENDING', low_state
    assert int(low_state['Restarts']) >= 1, low_state
    print('PASS research preempted LLM; LLM requeued with Restarts=' + low_state['Restarts'], flush=True)
    S.check_call(['scancel', str(high)])
    wait_for(lambda: state(low)['JobState'] == 'RUNNING' and 'START restart=1' in logfile.read_text(), 'automatic LLM restart', seconds=180)
    print('PASS LLM automatically restarted after research released GPU', flush=True)
    (base / 'preemption-result.json').write_text(json.dumps({'low_job': low, 'queued_job': second, 'research_job': high, 'passed': True}, indent=2) + '\n')
finally:
    for job in tracked:
        S.run(['scancel', str(job)], check=False)
    S.run(['sudo', '-n', 'scontrol', 'update', 'PartitionName=llm', 'State=DOWN'], check=True)
