#!/usr/bin/env python3
"""Offline syntax and secret-pattern checks. Never execute deployment or GPU tests."""
import ast,json,re,subprocess,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
patterns={
 'private-key':re.compile(r'-----BEGIN (?:RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----'),
 'github-token':re.compile(r'(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{40,})'),
 'prefixed-api-key':re.compile(r'\bsk-[A-Za-z0-9_-]{24,}'),
 'encoded-tunnel-or-jwt':re.compile(r'\beyJ[A-Za-z0-9_+/=-]{60,}'),
 'aws-access-key':re.compile(r'\b(?:AKIA|ASIA)[A-Z0-9]{16}\b'),
 'credential-url':re.compile(r'https?://[^\s/:@]+:[^\s/@]+@'),
}
blocked_names={'service.env','tunnel.token','backend-key','client-keys.json','initial-credentials.json','munge.key','.env'}
blocked_suffixes={'.db','.sqlite','.sqlite3','.log','.pem','.key','.pfx','.p12','.safetensors','.gguf','.whl','.partial'}
errors=[]
r=subprocess.run(['git','ls-files','--cached','--others','--exclude-standard','-z'],cwd=ROOT,capture_output=True)
if r.returncode==0:
 files=sorted({ROOT/p for p in r.stdout.decode().split('\0') if p})
else:
 files=sorted(p for p in ROOT.rglob('*') if p.is_file() and '.git' not in p.parts)
for p in files:
 rel=p.relative_to(ROOT)
 if p.is_symlink():errors.append(f'{rel}: symlinks require manual review');continue
 if p.name in blocked_names or p.suffix in blocked_suffixes or (p.name.startswith('.env.') and p.name!='.env.example'):
  errors.append(f'{rel}: sensitive/runtime file forbidden');continue
 if not p.exists():continue
 try:s=p.read_text(encoding='utf-8')
 except (UnicodeError,OSError):errors.append(f'{rel}: non-text file forbidden');continue
 for n,line in enumerate(s.splitlines(),1):
  for label,pattern in patterns.items():
   if pattern.search(line):errors.append(f'{rel}:{n}: {label}')
 try:
  if p.suffix=='.py':ast.parse(s,filename=str(rel))
  elif p.suffix=='.json':json.loads(s)
 except (SyntaxError,ValueError) as e:errors.append(f'{rel}: invalid syntax ({type(e).__name__})')
 if p.suffix in {'.sh','.sbatch'} or s.startswith('#!/bin/bash') or s.startswith('#!/usr/bin/env bash') or p.name=='pre-commit':
  b=subprocess.run(['bash','-n',str(p)],capture_output=True,text=True)
  if b.returncode:errors.append(f'{rel}: invalid bash syntax')
if errors:
 print('\n'.join(errors));sys.exit(1)
print(f'PASS: {len(files)} text files; Python/JSON/Bash syntax and secret-pattern checks')
