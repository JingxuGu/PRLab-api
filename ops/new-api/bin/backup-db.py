#!/usr/bin/env python3
import datetime,os,sqlite3
from pathlib import Path
from zoneinfo import ZoneInfo
os.umask(0o077)
base=Path('/home/llmops/new-api')
stamp=datetime.datetime.now(ZoneInfo('Asia/Shanghai')).strftime('%Y%m%d-%H%M%S')
dest=base/'backups'/f'new-api-{stamp}.db'
with sqlite3.connect(f'file:{base}/data/new-api.db?mode=ro',uri=True) as src:
 with sqlite3.connect(dest) as dst:src.backup(dst)
os.chmod(dest,0o600)
print('Private SQLite backup created:',dest)
