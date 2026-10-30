"""Initialize local New API without printing credentials or API responses."""
import http.cookiejar,json,os,secrets,time,urllib.request
from pathlib import Path
os.umask(0o077)
BASE='http://127.0.0.1:3001'
CONF=Path('/home/llmops/new-api/config')
credpath=CONF/'initial-credentials.json'
if credpath.exists():
 creds=json.loads(credpath.read_text())
else:
 creds={name:{'username':name,'password':secrets.token_urlsafe(15)} for name in ['example-admin']+[f'user{i:02}' for i in range(1,7)]}
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
 'ModelRatio':json.dumps({'qwen3.8-27b':1}),
 'CompletionRatio':json.dumps({'qwen3.8-27b':1}),
 'GroupRatio':json.dumps({'default':1}),'UserUsableGroups':json.dumps({'default':'实验室'}),
 'Notice':'科研任务优先。推理最多同时 4 个请求，忙时稍后重试；科研抢占期间服务暂不可用。支持单张图片，建议 data URL 上传；上下文 16384 token。',
}
for k,v in settings.items():req(admin,'/api/option/',{'key':k,'value':v},'PUT',aid)
channels=req(admin,'/api/channel/?p=1&page_size=100',uid=aid)['data']
items=channels.get('items',[]) if isinstance(channels,dict) else channels
if not any(x['name']=='Qwen3.8 local Slurm' for x in items):
 upstream=json.loads(Path('/home/llmops/llm/config/client-keys.json').read_text())['user01']
 req(admin,'/api/channel/',{'mode':'single','channel':{'name':'Qwen3.8 local Slurm','type':1,'key':upstream,'base_url':'http://127.0.0.1:8000','models':'qwen3.8-27b','group':'default','status':1,'auto_ban':0,'test_model':'qwen3.8-27b','priority':10,'weight':1,'setting':json.dumps({'force_format':False}),'param_override':json.dumps({'chat_template_kwargs':{'enable_thinking':False}})}},uid=aid)
users=req(admin,'/api/user/?p=1&page_size=100',uid=aid)['data']
users=users.get('items',[]) if isinstance(users,dict) else users
for name,a in creds.items():
 if name=='example-admin':continue
 if not any(x['username']==name for x in users):
  if len(a['password'])>20:
   a['password']=secrets.token_urlsafe(15)
   credpath.write_text(json.dumps(creds,ensure_ascii=False,indent=2)+'\n')
  req(admin,'/api/user/',dict(a,role=1,display_name=name),uid=aid)
users=req(admin,'/api/user/?p=1&page_size=100',uid=aid)['data']
users=users.get('items',[]) if isinstance(users,dict) else users
for name,a in creds.items():
 if name=='example-admin':continue
 uid=next(x['id'] for x in users if x['username']==name)
 current_quota=next(x['quota'] for x in users if x['id']==uid)
 if current_quota<100000000:
  req(admin,'/api/user/manage',{'id':uid,'action':'add_quota','mode':'add','value':100000000-current_quota},uid=aid)
 op=client();u=req(op,'/api/user/login',{'username':a['username'],'password':a['password']})['data']['id']
 ts=req(op,'/api/token/',uid=u)['data']; ts=ts.get('items',[]) if isinstance(ts,dict) else ts
 if not any(x['name']=='lab-qwen38' for x in ts):
  req(op,'/api/token/',{'name':'lab-qwen38','expired_time':-1,'unlimited_quota':True,'model_limits_enabled':True,'model_limits':'qwen3.8-27b','group':'default','cross_group_retry':False},uid=u)
  ts=req(op,'/api/token/',uid=u)['data'];ts=ts.get('items',[]) if isinstance(ts,dict) else ts
 tid=next(x['id'] for x in ts if x['name']=='lab-qwen38')
 a['api_key']='sk-'+req(op,f'/api/token/{tid}/key',{},uid=u)['data']['key']
 a['user_id']=u
 credpath.write_text(json.dumps(creds,ensure_ascii=False,indent=2)+'\n')
 time.sleep(1)
req(admin,'/api/user/logout',uid=aid)
(CONF/'initialized.json').write_text(json.dumps({'initialized':True,'admin':'example-admin','users':6,'model':'qwen3.8-27b','upstream':'http://127.0.0.1:8000','retries':0,'auto_disable_channel':False,'registration':False},indent=2)+'\n')
print('PASS: initialized; six users and restricted model tokens created; credentials remain private on server')
