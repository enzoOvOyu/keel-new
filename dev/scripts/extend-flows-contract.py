from pathlib import Path
import yaml,re
yaml.SafeDumper.ignore_aliases=lambda self,data:True
p=Path(__file__).resolve().parents[2]/'docs/openapi.yaml';text=p.read_text();d=yaml.safe_load(text);ss={}
S={'type':'string'};I={'type':'integer'};B={'type':'boolean'};J={'type':'object','additionalProperties':True}
def R(n):return {'$ref':'#/components/schemas/'+n}
def A(s):return {'type':'array','items':R(s) if isinstance(s,str) else s}
def O(p):return {'type':'object','additionalProperties':False,'properties':p,'required':list(p)}
def N(p):return {**p,'nullable':True}
ss['FlowsEngine']={**J,'description':'Authoritative Kestra DTO; preserve native/plugin extension properties.','properties':{'state':J,'tasks':A(J),'taskRunList':A(J),'outputs':J,'source':S,'disabled':B,'revision':I,'triggers':A(J)}}
ss['FlowsDefinition']=O(dict(id=S,project_id=S,namespace=S,flow_id=S,name=S,description=S,target_type=S,revision=I,source=S,disabled=B,engine=R('FlowsEngine')))
ss['FlowsSave']=O(dict(project_id=S,name=S,description=S,target_type=S,source=S,expected_revision=I))
ss['FlowsTarget']=O(dict(id=S,name=S,type=S,project_id=S,revision=I,fields=J,agent_id=S,agent_online=B,capabilities=A(S),ssh_profile_id=S))
ss['FlowsRun']=O(dict(project_id=S,flow_id=S,expected_revision=I,target_ids=A(S),kind={'type':'string','enum':['command','bash','python']},command=S,transport_policy={'type':'string','enum':['agent_preferred','agent_only','ssh_only']},timeout_seconds=I,target_concurrency=I,max_retries=I,source_module=S,parent_id=S));ss['FlowsRun']['required']=['project_id','target_ids','kind','command','transport_policy','timeout_seconds','target_concurrency','max_retries']
ss['FlowsDecision']=O(dict(asset_id=S,name=S,revision=I,transport=S,reason=S,ready=B,target=R('FlowsTarget')))
ss['FlowsTargetRun']=O(dict(id=S,snapshot=R('FlowsTarget'),selected_transport=S,actual_transport=N(S),state=S,attempt=I,exit_code=N(I),stdout=S,stderr=S,started_at=N(S),ended_at=N(S)))
ss['FlowsExecution']=O(dict(id=S,project_id=S,flow_id=N(S),kestra_id=N(S),engine_flow_id=S,flow_revision=I,source_module=S,mode=S,started_by=S,created_at=S,error_code=N(S),engine=R('FlowsEngine'),targets=A('FlowsTargetRun')))
ss['FlowsAgent']=O(dict(id=S,project_id=S,asset_id=S,identity={'type':'object','additionalProperties':S},version=S,capabilities=A(S),state=S,certificate_serial=N(S),certificate_expires_at=N(S),last_heartbeat=N(S),online=B))
ss['FlowsAudit']={**J,'properties':dict(id=S,actor=S,action=S,details=J,created_at=S)}
ss['FlowsCapabilities']=O(dict(engine=S,version=S,configured=B,agent_gateway=B,scheduling=S,native_plugins=S))
ss['FlowsTokenRequest']=O(dict(project_id=S,asset_id=S));ss['FlowsToken']=O(dict(id=S,token=S,asset_id=S,expires_at=S))
ss['FlowsReason']=O(dict(reason=S));ss['FlowsValidationRequest']=O(dict(source=S));ss['FlowsAction']=O(dict(id=S,state=S))
ss['FlowsSSHTrust']=O(dict(id=S,project_id=S,asset_id=S,host=S,port=I,username=S,secret_ref=S,known_hosts_ref=S,status=S,revision=I))
ss['FlowsSSHTrustInput']=O(dict(project_id=S,asset_id=S,host=S,port=I,username=S,secret_ref=S,known_hosts_ref=S,status=S,expected_revision=I))
new={}
def op(path,method,name,response,req=None,many=False,code='200'):
 env=name+'Envelope';ss[env]=O({'data':A(response) if many else R(response),'request_id':S});params=[{'in':'query','name':'project_id','schema':S}]+[{'in':'path','name':k,'required':True,'schema':S} for k in re.findall(r'\{([^}]+)\}',path)]
 v={'operationId':name,'tags':['FlowsReal'],'parameters':params,'responses':{code:{'description':'Real engine/domain result.','content':{'application/json':{'schema':R(env)}}},'403':{'description':'Permission or project scope denied'},'422':{'description':'Invalid input, precondition or unavailable capability'},'503':{'description':'Provider not configured'}}}
 if req:v['requestBody']={'required':True,'content':{'application/json':{'schema':R(req)}}}
 new.setdefault(path,{})[method]=v
op('/flows','get','listRealFlows','FlowsDefinition',many=True);op('/flows','post','createRealFlow','FlowsDefinition','FlowsSave',code='201')
for path,name,res in [('capabilities','getFlowsCapabilities','FlowsCapabilities'),('targets','listFlowsTargets','FlowsTarget'),('executions','listRealExecutions','FlowsExecution'),('agents','listRealAgents','FlowsAgent'),('ssh-trust','listFlowsSSHTrust','FlowsSSHTrust')]:op('/flows/'+path,'get',name,res,many=path!='capabilities')
op('/flows/preflight','post','preflightFlowsTargets','FlowsDecision','FlowsRun',many=True);op('/flows/quick-run','post','createQuickRun','FlowsExecution','FlowsRun',code='201')
op('/flows/{flowId}','get','getRealFlow','FlowsDefinition');op('/flows/{flowId}','put','updateRealFlow','FlowsDefinition','FlowsSave')
op('/flows/{flowId}/validate','post','validateRealFlow','FlowsEngine','FlowsValidationRequest',many=True);op('/flows/{flowId}/execute','post','executeRealFlow','FlowsExecution','FlowsRun',code='201')
for suffix in ['revisions','dependencies','files']:op('/flows/{flowId}/'+suffix,'get','getFlow'+suffix.title(),'FlowsEngine',many=True)
op('/flows/executions/{executionId}','get','getRealExecution','FlowsExecution')
for suffix,res in [('targets','FlowsTargetRun'),('logs','FlowsEngine'),('outputs','FlowsEngine'),('audit','FlowsAudit')]:op('/flows/executions/{executionId}/'+suffix,'get','getExecution'+suffix.title(),res,many=suffix!='outputs')
for a in ['pause','resume','kill','retry-failed-targets']:op('/flows/executions/{executionId}/'+a,'post','actionExecution'+a.replace('-','').title(),'FlowsExecution')
op('/flows/agents/{agentId}','get','getRealAgent','FlowsAgent');op('/flows/agents/enrollment-tokens','post','issueAgentEnrollment','FlowsToken','FlowsTokenRequest')
for a in ['approve','quarantine','revoke']:op('/flows/agents/{agentId}/'+a,'post','agent'+a.title(),'FlowsAction','FlowsReason')
for a in ['tasks','audit']:op('/flows/agents/{agentId}/'+a,'get','agent'+a.title(),'FlowsAudit',many=True)
op('/flows/ssh-trust','post','saveFlowsSSHTrust','FlowsSSHTrust','FlowsSSHTrustInput')
new['/flows/executions/{executionId}/events']={'get':{'operationId':'followRealExecution','tags':['FlowsReal'],'parameters':[{'name':'executionId','in':'path','required':True,'schema':S}],'responses':{'200':{'description':'Authorized Kestra SSE proxy; reconnect refetches current state.','content':{'text/event-stream':{'schema':S}}}}}}
# Replace whole affected path blocks while keeping all unrelated source text untouched.
for path,definition in new.items():
 fragment=yaml.safe_dump({path:definition},sort_keys=False,allow_unicode=True,width=120)
 fragment=''.join('  '+line if line.strip() else line for line in fragment.splitlines(True))
 pattern=r'^  '+re.escape(path)+r':\n.*?(?=^  /|^[A-Za-z]|\Z)'
 if re.search(pattern,text,re.M|re.S):text=re.sub(pattern,lambda _:fragment,text,count=1,flags=re.M|re.S)
 else:text=text.replace('\ncomponents:', '\n'+fragment+'\ncomponents:',1)
# Fresh schemas inserted before existing schemas, allowing deterministic reruns.
for n,definition in ss.items():
 fragment=yaml.safe_dump({n:definition},sort_keys=False,allow_unicode=True,width=120);fragment=''.join('    '+l if l.strip() else l for l in fragment.splitlines(True))
 pattern=r'^    '+re.escape(n)+r':\n.*?(?=^    [A-Za-z]|^  [A-Za-z]|^[A-Za-z]|\Z)'
 if re.search(pattern,text,re.M|re.S):text=re.sub(pattern,lambda _:fragment,text,count=1,flags=re.M|re.S)
 else:text=text.replace('  schemas:\n','  schemas:\n'+fragment,1)
yaml.safe_load(text);p.write_text(text)
