"""Focused real HTTP identity acceptance; synthetic users are disabled after the run."""
import json, os, uuid, pathlib, requests, re, yaml, jsonschema
spec=yaml.safe_load(pathlib.Path(__file__).parents[2].joinpath("docs/openapi.yaml").read_text()); resolver=jsonschema.RefResolver.from_schema(spec); contract_responses=0
base=os.getenv('KEEL_TEST_API','http://localhost:8081/api/v1/')
env=dict(line.strip().split('=',1) for line in pathlib.Path(__file__).parents[1].joinpath('backend/.env').read_text().splitlines() if '=' in line and not line.startswith('#'))
password=env['KEEL_BOOTSTRAP_PASSWORD'].strip("'\"")
checks=[]; created=[]; prefix='authcheck-'+uuid.uuid4().hex[:7]; testpass=uuid.uuid4().hex+'!'
class Client:
 def __init__(self):self.s=requests.Session();self.csrf=''
 def call(self,method,path,body=None,expected=200):
  r=self.s.request(method,base+path,json=body,headers={'X-CSRF-Token':self.csrf},timeout=20)
  assert r.status_code==expected,(method,path,r.status_code,r.text[:250],expected)
  global contract_responses
  if expected<400 and r.content:
   operation=next((ops[method.lower()] for pattern,ops in spec['paths'].items() if re.fullmatch(re.sub(r'\{[^}]+\}',r'[^/]+',pattern),'/'+path.split('?')[0]) and method.lower() in ops),None)
   assert operation is not None,('undocumented',path)
   response=operation['responses'][str(expected)]
   if '$ref' in response:_,response=resolver.resolve(response['$ref'])
   schema=response.get('content',{}).get('application/json',{}).get('schema')
   if schema:jsonschema.Draft202012Validator(schema,resolver=resolver).validate(r.json());contract_responses+=1
  return r.json().get('data') if r.content and expected<400 else r
 def login(self,name,pw):
  d=self.call('POST','auth/login',{'username':name,'password':pw,'method':'local'});self.csrf=d['csrf_token'];return d
 def check(self,permission,project,expected=200,**kw):return self.call('POST','authorization/check',{'permission':permission,'project_id':project,**kw},expected)
a=Client();a.login('admin',password);p1='proj-b300-demo';p2='proj-b300-lab'
def new(role,grant=True):
 name=prefix+'-'+role; uid=a.call('POST','users',{'username':name,'display_name':'Synthetic '+role,'password':testpass})['id'];created.append(uid)
 if grant:
  if role=='admin':a.call('POST',f'users/{uid}/global-roles',{'role_id':role})
  else:a.call('POST',f'projects/{p1}/members',{'user_id':uid,'role_id':role,'reason':'synthetic acceptance'})
 c=Client();session=c.login(name,testpass);return uid,name,c,session
try:
 roles=['viewer','delivery_engineer','delivery_lead','developer_sre','network_engineer','admin'];users={r:new(r) for r in roles}
 for role,(_,_,c,s) in users.items():
  c.check('bom:read',p1);c.check('bom:read',p2,200 if role=='admin' else 403)
  if role!='admin':assert {p['id'] for p in s['authorized_projects']}=={p1}
 checks.append('six role identities and cross-project isolation')
 rid,rname,rc,_=new('resource',False); imports=a.call('GET',f'projects/{p1}/bom-imports'); selected=imports[0]['id']; a.call('POST',f'projects/{p1}/members',{'user_id':rid,'role_id':'viewer','resource_type':'bom_import','resource_id':selected,'reason':'synthetic scope'}); rc.call('GET','bom-imports/'+selected); rc.call('GET','bom-imports/'+imports[1]['id'],expected=403); checks.append('resource scoped grant cannot read sibling import')
 uid,name,v,_=users['viewer'];v.call('POST',f'projects/{p1}/bom-imports',{},403);checks.append('viewer direct write denied')
 users['network_engineer'][2].call('POST',f'projects/{p1}/provision/reinstall',{},403);users['network_engineer'][2].check('cleanup:execute',p1,403);checks.append('NE cannot reinstall/cleanup')
 a.call('POST',f'projects/{p2}/members',{'user_id':uid,'role_id':'delivery_engineer','reason':'different project'})
 v.check('bom:edit',p2);v.check('bom:edit',p1,403);checks.append('one user, different project roles')
 nuid,nname,n,_=new('no-member',False);n.call('GET',f'projects/{p1}/bom-imports',expected=403);checks.append('non-member URL access denied')
 duid,_,de,_=users['delivery_engineer'];members=a.call('GET',f'projects/{p1}/members');grant=next(m for m in members if m['user_id']==duid and not m['revoked_at']);a.call('DELETE',f'projects/{p1}/members/'+grant['id']);de.check('bom:edit',p1,403);checks.append('membership revoke immediate for old session')
 a.call('POST',f'projects/{p1}/members',{'user_id':duid,'role_id':'delivery_engineer','reason':'restore synthetic test'})
 request={'action':'reinstall','resource_type':'device','resource_id':'synthetic-server','snapshot_version':'v1','target_ids':['synthetic-server'],'reason':'synthetic acceptance','risk':'normal'}
 aid=de.call('POST',f'projects/{p1}/approvals',request)['id'];de.call('POST',f'projects/{p1}/approvals/{aid}/sign',{'group':'DL','decision':'approve','reason':'self'},403);checks.append('requester cannot approve own high risk operation')
 dl=users['delivery_lead'][2];dl.call('POST',f'projects/{p1}/approvals/{aid}/sign',{'group':'DL','decision':'approve','reason':'verified synthetic'});de.call('POST',f'projects/{p1}/approvals/{aid}/check',{'resource_id':'synthetic-server','snapshot_version':'v1','target_ids':['synthetic-server']});checks.append('distinct DL approval and frozen scope check')
 request.update(action='network_config',resource_id='synthetic-switch',target_ids=['synthetic-switch']);aid=a.call('POST',f'projects/{p1}/approvals',request)['id']
 for group,role in [('NE','network_engineer'),('DE','delivery_engineer')]:users[role][2].call('POST',f'projects/{p1}/approvals/{aid}/sign',{'group':group,'decision':'approve','reason':'synthetic'})
 a.call('POST',f'projects/{p1}/approvals/{aid}/check',{'resource_id':'synthetic-switch','snapshot_version':'v1','target_ids':['synthetic-switch']});checks.append('network NE + DE distinct signatures')
 request['risk']='critical';aid=a.call('POST',f'projects/{p1}/approvals',request)['id'];a.call('POST',f'projects/{p1}/approvals/{aid}/check',{'resource_id':'synthetic-switch','snapshot_version':'v1','target_ids':['synthetic-switch']},422);checks.append('Critical blocked pending NH decision')
 a.call('PATCH',f'users/{uid}',{'status':'disabled'});v.call('GET','auth/session',expected=401);checks.append('disabled user old session revoked')
 sessions=n.call('GET','sessions');sid=next(s['id'] for s in sessions if not s['revoked_at']);a.call('DELETE','sessions/'+sid);n.call('GET','auth/session',expected=401);checks.append('explicit session revoke')
 n.login(nname,testpass);a.call('POST',f'users/{nuid}/reset-password',{'new_password':testpass+'new'});n.call('GET','auth/session',expected=401);n.login(nname,testpass+'new');n.call('POST','auth/password',{'current_password':testpass+'new','new_password':testpass+'changed'});n.call('GET','auth/session',expected=401);checks.append('admin reset and own password change revoke sessions')
 bad=Client()
 for i in range(5):bad.call('POST','auth/login',{'username':nname,'password':'incorrect-password'},401)
 bad.call('POST','auth/login',{'username':nname,'password':testpass+'changed'},429);a.call('POST',f'users/{nuid}/unlock',{});n.login(nname,testpass+'changed');checks.append('login lock and admin unlock')
 a.call('GET','roles'); a.call('GET','permissions'); a.call('GET','projects'); a.call('GET','users'); a.call('GET',f'users/{uid}'); a.call('GET',f'projects/{p1}/approvals');
 logs=a.call('GET','login-events');audits=a.call('GET','identity-audits');assert any(x['outcome']=='success' for x in logs);assert any(x['action']=='identity.membership.revoked' for x in audits);checks.append('login and transactional permission audit')
 print(json.dumps({'passed':checks,'count':len(checks)},ensure_ascii=False,indent=2))
 pathlib.Path(__file__).parents[2].joinpath('docs/IDENTITY-ACCEPTANCE.json').write_text(json.dumps({'passed':checks,'count':len(checks),'contract_responses':contract_responses,'scope':'synthetic HTTP acceptance; no device execution'},ensure_ascii=False,indent=2)+'\n')
finally:
 for uid in created:a.call('PATCH',f'users/{uid}',{'status':'disabled'})
