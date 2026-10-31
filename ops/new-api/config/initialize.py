"""Fresh-install, administrator-only setup. Never rerun against a deployed application service."""
import http.cookiejar,json,os,secrets,urllib.request
from pathlib import Path
os.umask(0o077)
BASE='http://127.0.0.1:3001'
CONF=Path('/home/llmops/new-api/config')
credpath=CONF/'initial-credentials.json'
if credpath.exists():
 creds=json.loads(credpath.read_text())
else:
 creds={name:{'username':name,'password':secrets.token_urlsafe(15)} for name in ['example-admin']}
 credpath.write_text(json.dumps(creds,ensure_ascii=False,indent=2)+'\n')
def client():
 return urllib.request.build_opener(urllib.request.ProxyHandler({}),urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
def req(op,path,data=None,method=None,uid=None):
 h={'Content-Type':'application/json'}
 if uid:h['New-Api-User']=str(uid)
 r=urllib.request.Request(BASE+path,data=None if data is None else json.dumps(data).encode(),headers=h,method=method)
 with op.open(r,timeout=30) as res:v=json.load(res)
 if not v.get('success',False):
  # Errors can include server internals; do not expose response contents.
  raise RuntimeError('New API operation failed: '+path)
 return v
admin=client()
if not req(admin,'/api/setup')['data']['status']:
 a=creds['example-admin']
 req(admin,'/api/setup',{'username':a['username'],'password':a['password'],'confirmPassword':a['password'],'SelfUseModeEnabled':False,'DemoSiteEnabled':False})
aid=req(admin,'/api/user/login',creds['example-admin'])['data']['id']
settings={
 'RegisterEnabled':'false','PasswordRegisterEnabled':'false','PasswordLoginEnabled':'true',
 'AutomaticDisableChannelEnabled':'false','AutomaticEnableChannelEnabled':'false',
 'RetryTimes':'0','SystemName':'PRLab api',
 'ServerAddress':'http://localhost:13001','LogConsumeEnabled':'true',
 'DisplayInCurrencyEnabled':'false','DisplayTokenStatEnabled':'true',
 'QuotaForNewUser':'0','QuotaForInviter':'0','QuotaForInvitee':'0',
 'ModelRatio':json.dumps({'qwen3.8-27b':0}),
 'quota_setting.enable_free_model_pre_consume':'false',
 'CompletionRatio':json.dumps({'qwen3.8-27b':1}),
 'GroupRatio':json.dumps({'default':1}),'UserUsableGroups':json.dumps({'default':'实验室'}),
 'Notice':'科研任务优先。服务繁忙或资源调度期间，请稍后重试。',
}
for k,v in settings.items():req(admin,'/api/option/',{'key':k,'value':v},'PUT',aid)
channels=req(admin,'/api/channel/?p=1&page_size=100',uid=aid)['data']
items=channels.get('items',[]) if isinstance(channels,dict) else channels
if not any(x['name']=='Qwen3.8 local Slurm' for x in items):
 upstream=json.loads(Path('/home/llmops/llm/config/client-keys.json').read_text())['user01']
 req(admin,'/api/channel/',{'mode':'single','channel':{'name':'Qwen3.8 local Slurm','type':1,'key':upstream,'base_url':'http://127.0.0.1:8000','models':'qwen3.8-27b','group':'default','status':1,'auto_ban':0,'test_model':'qwen3.8-27b','priority':10,'weight':1,'setting':json.dumps({'force_format':False}),'param_override':json.dumps({'chat_template_kwargs':{'enable_thinking':False}})}},uid=aid)
req(admin,'/api/user/logout',uid=aid)
(CONF/'initialized.json').write_text(json.dumps({'initialized':True,'admin':'example-admin','users':0,'model':'qwen3.8-27b','upstream':'http://127.0.0.1:8000','retries':0,'auto_disable_channel':False,'registration':False},indent=2)+'\n')
print('PASS: initialized; administrator-only setup; members use email applications; credentials remain private on server')
