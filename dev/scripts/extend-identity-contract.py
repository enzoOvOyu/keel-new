"""Keep the concrete identity API schemas alongside the shared domain contract."""
from pathlib import Path
import yaml, re, copy
class InlineDumper(yaml.SafeDumper):
 def ignore_aliases(self,data):return True
p=Path('docs/openapi.yaml');raw=p.read_text();s=yaml.safe_load(raw);previous=copy.deepcopy(s);S=s['components']['schemas']
str_={'type':'string'};dt={'type':'string','format':'date-time'};nullable={'type':['string','null'],'format':'date-time'}
def obj(fields,required=None):return {'type':'object','properties':fields,'required':list(fields) if required is None else required}
def ref(n):return {'$ref':'#/components/schemas/'+n}
def arr(v):return {'type':'array','items':v}
roles=['viewer','delivery_engineer','delivery_lead','developer_sre','admin','network_engineer']
S['IdentityRoleId']={'type':'string','enum':roles}
S['IdentityUser']=obj(dict(id=str_,username=str_,display_name=str_,status={'type':'string','enum':['active','disabled']},locked_until=nullable,permission_version={'type':'integer'},created_at=dt),['id','username','display_name','status','locked_until'])
S['IdentityProject']=obj(dict(id=str_,name=str_,code=str_,status=str_,template_id=str_),['id','name','code','status'])
S['IdentityPermission']=obj(dict(id=str_,module=str_,action=str_,description=str_))
S['IdentityRole']=obj(dict(id=ref('IdentityRoleId'),name=str_,description=str_,permissions=arr(str_),assignable_to_project={'type':'boolean'}))
S['IdentityGrant']=obj(dict(id=str_,project_id=str_,role_id=ref('IdentityRoleId'),resource_type=str_,resource_id=str_,permissions=arr(str_)))
S['IdentitySession']=obj(dict(actor=str_,user=obj(dict(id=str_,username=str_,display_name=str_)),role=str_,project_id=str_,csrf_token=str_,permissions=arr(str_),ai_configured={'type':'boolean'},global_permissions=arr(str_),global_roles=arr(ref('IdentityRoleId')),project_permissions={'type':'object','additionalProperties':arr(str_)},project_roles={'type':'object','additionalProperties':arr(ref('IdentityRoleId'))},grants=arr(ref('IdentityGrant')),authorized_projects=arr(ref('IdentityProject')),permission_revision={'type':'integer'}))
S['BOMSession']=ref('IdentitySession')
S['IdentityProviders']=obj(dict(status={'const':'ready'},methods={'type':'array','items':{'const':'local'}},environment=str_))
S['IdentityLogin']=obj(dict(username=str_,password={'type':'string','writeOnly':True},method={'const':'local'}),['username','password'])
S['IdentityMutation']=obj(dict(id=str_,status={'const':'updated'}))
S['IdentityStatus']=obj(dict(status=str_))
S['IdentityCreateUser']=obj(dict(username={'type':'string','minLength':3,'maxLength':64},display_name=str_,password={'type':'string','minLength':12,'maxLength':256,'writeOnly':True}))
S['IdentityUpdateUser']=obj(dict(display_name=str_,status={'type':'string','enum':['active','disabled']}),[])
S['IdentityPassword']=obj(dict(current_password={'type':'string','writeOnly':True},new_password={'type':'string','minLength':12,'maxLength':256,'writeOnly':True}))
S['IdentityResetPassword']=obj(dict(new_password={'type':'string','minLength':12,'maxLength':256,'writeOnly':True}))
S['IdentityGlobalRoleCommand']=obj(dict(role_id={'const':'admin'},revoke={'type':'boolean'},expires_at=nullable),['role_id'])
S['IdentityMembership']=obj(dict(id=str_,user_id=str_,username=str_,display_name=str_,role_id=ref('IdentityRoleId'),resource_type=str_,resource_id=str_,valid_from=dt,expires_at=nullable,revoked_at=nullable))
S['IdentityMembershipCommand']=obj(dict(user_id=str_,username=str_,role_id={'type':'string','enum':[r for r in roles if r!='admin']},resource_type=str_,resource_id=str_,expires_at=nullable,reason={'type':'string','minLength':1}),['role_id','reason'])
S['IdentitySessionRecord']=obj(dict(id=str_,user_id=str_,username=str_,ip=str_,user_agent=str_,created_at=dt,expires_at=dt,revoked_at=nullable,last_seen_at=dt))
S['IdentityLoginEvent']=obj(dict(id=str_,username=str_,outcome=str_,ip=str_,created_at=dt))
S['IdentityAuditSnapshot']=obj(dict(user={'type':['object','null'],'properties':S['IdentityUser']['properties']},global_roles=arr(obj(dict(role_id=str_,expires_at=nullable,revoked_at=nullable))),memberships=arr(obj(dict(id=str_,project_id=str_,role_id=str_,resource_type=str_,resource_id=str_,expires_at=nullable,revoked_at=nullable))),active_sessions={'type':'integer'}))
S['IdentityAuditEvent']=obj(dict(id=str_,actor=str_,action=str_,resource_id=str_,created_at=dt,details=obj(dict(method=str_,path=str_,before=ref('IdentityAuditSnapshot'),after=ref('IdentityAuditSnapshot'),group=str_,decision=str_,reason=str_),[])))
S['IdentityApproval']=obj(dict(id=str_,requester_id=str_,action=str_,resource_type=str_,resource_id=str_,snapshot_version=str_,risk={'enum':['normal','critical']},state={'enum':['pending','approved','rejected','blocked_policy']},required_groups=arr({'enum':['NE','DE','DL','NH']}),target_ids=arr(str_),reason=str_,expires_at=dt))
S['IdentityApprovalCommand']=obj(dict(action={'enum':['network_config','bios','firmware','bmc_power','reinstall','cleanup']},resource_type=str_,resource_id=str_,snapshot_version=str_,risk={'enum':['normal','critical']},target_ids=arr(str_),reason=str_,expires_at=nullable),['action','resource_type','resource_id','snapshot_version','risk','target_ids','reason'])
S['IdentitySignCommand']=obj(dict(group={'enum':['NE','DE','DL','NH']},decision={'enum':['approve','reject']},reason=str_))
S['IdentityApprovalCheck']=obj(dict(resource_id=str_,snapshot_version=str_,target_ids=arr(str_)))
S['IdentityAuthorizationCheck']=obj(dict(permission=str_,project_id=str_,resource_type=str_,resource_id=str_),['permission','project_id'])
S['IdentityCreateProject']=obj(dict(name=str_,code=str_,template_id=str_),['name','code'])
def op(path,method,name,reply,body=None,public=False):
 params=[{'in':'path','name':x[1:-1],'required':True,'schema':str_} for x in path.split('/') if x.startswith('{')]
 item=s['paths'].setdefault(path,{})
 # Replace inherited legacy paging params for the concrete identity resource.
 item['parameters']=params
 envname=name+'Envelope';S[envname]=obj(dict(data=reply,request_id=str_))
 o={'tags':['Identity'],'operationId':name,'security':[] if public else [{'cookieSession':[]}],'description':'Real identity API. Project grants are checked server-side on every request. Mutations use X-CSRF-Token; permissions and approval are separate checks.','responses':{'200':{'description':'Success','content':{'application/json':{'schema':ref(envname)}}}}}
 for code,desc in [('400','Invalid request'),('401','Unauthenticated or session expired'),('403','Scope, CSRF or permission denied'),('409','Identity state conflict'),('422','Invalid input or approval policy unresolved'),('429','Login rate limited')]:o['responses'][code]={'description':desc}
 if body:o['requestBody']={'required':True,'content':{'application/json':{'schema':ref(body)}}}
 if method not in ['get'] and not public:o['parameters']=[{'in':'header','name':'X-CSRF-Token','required':True,'schema':str_}]
 item[method]=o
op('/auth/providers','get','getIdentityProviders',ref('IdentityProviders'),public=True)
op('/auth/login','post','loginIdentity',ref('IdentitySession'),'IdentityLogin',True)
op('/auth/session','get','getIdentitySession',ref('IdentitySession'))
op('/auth/logout','post','logoutIdentity',ref('IdentityStatus'))
op('/auth/password','post','changeIdentityPassword',ref('IdentityMutation'),'IdentityPassword')
for path,name,resource in [('/users','IdentityUsers','IdentityUser'),('/roles','IdentityRoles','IdentityRole'),('/permissions','IdentityPermissions','IdentityPermission'),('/sessions','IdentitySessions','IdentitySessionRecord'),('/login-events','IdentityLoginEvents','IdentityLoginEvent'),('/identity-audits','IdentityAudits','IdentityAuditEvent'),('/projects','IdentityProjects','IdentityProject'),('/projects/{project_id}/members','IdentityMembers','IdentityMembership'),('/projects/{project_id}/approvals','IdentityApprovals','IdentityApproval')]:op(path,'get','list'+name,arr(ref(resource)))
op('/users','post','createIdentityUser',ref('IdentityMutation'),'IdentityCreateUser')
op('/users/{user_id}','get','getIdentityUser',ref('IdentityUser'))
op('/users/{user_id}','patch','updateIdentityUser',ref('IdentityMutation'),'IdentityUpdateUser')
op('/users/{user_id}/reset-password','post','resetIdentityPassword',ref('IdentityMutation'),'IdentityResetPassword')
op('/users/{user_id}/unlock','post','unlockIdentityUser',ref('IdentityMutation'))
op('/users/{user_id}/global-roles','post','grantIdentityGlobalRole',ref('IdentityMutation'),'IdentityGlobalRoleCommand')
op('/sessions/{session_id}','delete','revokeIdentitySession',ref('IdentityMutation'))
op('/projects','post','createIdentityProject',ref('IdentityMutation'),'IdentityCreateProject')
op('/projects/{project_id}/members','post','grantIdentityMembership',ref('IdentityMutation'),'IdentityMembershipCommand')
op('/projects/{project_id}/members/{membership_id}','delete','revokeIdentityMembership',ref('IdentityMutation'))
op('/projects/{project_id}/approvals','post','requestIdentityApproval',ref('IdentityMutation'),'IdentityApprovalCommand')
op('/projects/{project_id}/approvals/{approval_id}/sign','post','signIdentityApproval',obj(dict(id=str_,state=str_)),'IdentitySignCommand')
op('/projects/{project_id}/approvals/{approval_id}/check','post','checkIdentityApproval',obj(dict(allowed={'type':'boolean'},permission_allowed={'type':'boolean'},approval_satisfied={'type':'boolean'},device_execution={'const':'not_performed'})),'IdentityApprovalCheck')
op('/authorization/check','post','checkIdentityAuthorization',obj(dict(allowed={'type':'boolean'})),'IdentityAuthorizationCheck')
# Go implementation accepts Cookie Sessions, never a legacy development bearer token.
s['security']=[{'cookieSession':[]}]
if not any(t['name']=='Identity' for t in s['tags']):s['tags'].append({'name':'Identity','description':'Real local authentication, RBAC + project/resource scope, sessions and approvals'})
def patch_section(raw, marker, level, old, new):
 start=raw.index(marker)+len(marker)
 # Section ends at the next sibling/ancestor key.
 end_match=re.search(r'(?m)^\S' if level==2 else r'(?m)^(?:\S|  \S)',raw[start:])
 end=start+end_match.start() if end_match else len(raw)
 section=raw[start:end]
 for key,value in new.items():
  if old.get(key)==value:continue
  dumped=yaml.dump({key:value},Dumper=InlineDumper,allow_unicode=True,sort_keys=False,width=160)
  block=''.join(' '*level+line if line.strip() else line for line in dumped.splitlines(True))
  pattern=r'(?m)^'+' '*level+re.escape(key)+r':[^\n]*\n(?:^(?:'+' '*(level+1)+r'[^\n]*|[ \t]*)\n)*'
  match=re.search(pattern,section)
  if match:section=section[:match.start()]+block+section[match.end():]
  else:section+=block
 return raw[:start]+section+raw[end:]
raw=patch_section(raw,'paths:\n',2,previous['paths'],s['paths'])
raw=patch_section(raw,'  schemas:\n',4,previous['components']['schemas'],S)
raw=re.sub(r'(?m)^security:\n(?:^  [^\n]*\n)*','security:\n  - cookieSession: []\n',raw,count=1)
if not any(t['name']=='Identity' for t in previous['tags']):raw=raw.replace('tags:\n','tags:\n  - name: Identity\n    description: Local authentication, project/resource RBAC and approvals\n',1)
assert yaml.safe_load(raw)['paths']==s['paths']
assert yaml.safe_load(raw)['components']['schemas']==S
p.write_text(raw)
