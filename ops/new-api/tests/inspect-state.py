"""Report only non-secret state; never dump users, tokens or channel rows."""
import json,sqlite3
from pathlib import Path
p=Path('/home/llmops/new-api')
with sqlite3.connect('file:'+str(p/'data/new-api.db')+'?mode=ro',uri=True) as c:
 c.row_factory=sqlite3.Row
 r={
  'users':[dict(x) for x in c.execute('select username,role,status,used_quota,request_count from users where deleted_at is null')],
  'channels':[dict(x) for x in c.execute('select name,type,status,base_url,models,auto_ban from channels')],
  'token_count':c.execute('select count(*) from tokens where deleted_at is null').fetchone()[0],
  'usage_log_count':c.execute('select count(*) from logs where type=2').fetchone()[0],
  'options':[dict(x) for x in c.execute("select key,value from options where key in ('RegisterEnabled','PasswordRegisterEnabled','RetryTimes','AutomaticDisableChannelEnabled','ServerAddress','LogConsumeEnabled')")]
 }
(p/'config/state-verification.json').write_text(json.dumps(r,ensure_ascii=False,indent=2)+'\n')
print(json.dumps(r,ensure_ascii=False,indent=2))
