import json
import subprocess as S
from pathlib import Path

base = Path('/home/llmops/slurm-setup/tests')
info = S.check_output(['scontrol', 'show', 'job', '7', '-o'], text=True)
fields = dict(token.split('=', 1) for token in info.split() if '=' in token)
assert fields['Restarts'] == '1', fields
assert 'START restart=1' in (base / 'verify-low-7.log').read_text()
controller = S.check_output(['sudo', '-n', 'cat', '/var/log/slurm/slurmctld.log'], text=True)
events = [line for line in controller.splitlines() if any(tag in line for tag in ['Allocate JobId=7 ', 'Requeuing JobId=7', 'Allocate JobId=9 ', 'preempted JobId=7'])]
assert any('preempted JobId=7' in e for e in events), events
assert any('Allocate JobId=7 ' in e for e in events), events
assert any('Requeuing JobId=7' in e for e in events), events
assert any('Allocate JobId=9 ' in e for e in events), events
requeue_pos = next(i for i, e in enumerate(events) if 'Requeuing JobId=7' in e)
research_pos = next(i for i, e in enumerate(events) if 'Allocate JobId=9 ' in e)
restart_pos = next(i for i, e in enumerate(events) if 'Allocate JobId=7 ' in e)
assert requeue_pos < research_pos < restart_pos, events
result = {
    'passed': True,
    'low_job': 7,
    'queued_job': 8,
    'research_job': 9,
    'llm_restarts': 1,
    'events': events,
    'validation_note': 'Initial live assertion assumed appended batch output. The restarted output was overwritten by Slurm default. Recovery verified independently using restart marker, job Restarts counter, and chronological controller allocation records. The reusable verifier now requests appended output and checks the restart marker.',
}
(base / 'preemption-result.json').write_text(json.dumps(result, indent=2) + '\n')
print('PASS automatic restart verified by controller events, Restarts=1, and START restart=1')
print('\n'.join(events))
