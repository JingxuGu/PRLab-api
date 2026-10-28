#!/bin/bash
set -euo pipefail
umask 077
cd /home/llmops/llm
test -s config/model-verified.json
python3 - <<'PY'
import json
from pathlib import Path
base=Path('/home/llmops/llm')
verified=json.loads((base/'config/model-verified.json').read_text())
assert verified['revision']=='6128240ebaf4eaa7bad2b3d1c72c37d677c5f462'
model=base/'models/Qwen3.8-27B-NVFP4'
index=json.loads((model/'model.safetensors.index.json').read_text())
assert all((model/f).is_file() for f in set(index['weight_map'].values()))
source=Path('/etc/slurm/slurm.conf').read_text()
backup=base/'config/slurm.conf.before-vllm'
if not backup.exists():
    backup.write_text(source)
lines=[]
matched=False
for line in source.splitlines():
    if line.startswith('PartitionName=llm '):
        fields=line.split()
        fields=[('State=UP' if f.startswith('State=') else f) for f in fields]
        if not any(f.startswith('State=') for f in fields):
            fields.append('State=UP')
        line=' '.join(fields)
        matched=True
    lines.append(line)
assert matched, 'LLM partition missing'
(base/'config/slurm.conf.with-vllm').write_text('\n'.join(lines)+'\n')
print('Verified model files present; preserving research partition configuration')
PY
sudo -n install -o root -g root -m 0644 config/slurm.conf.with-vllm /etc/slurm/slurm.conf
sudo -n scontrol reconfigure
sudo -n systemctl enable --now qwen38-slurm.service
sleep 2
systemctl is-active qwen38-slurm.service qwen38-gateway.service
squeue -o '%.12i %.12P %.20j %.12T %.30R'
