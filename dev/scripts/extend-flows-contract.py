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
ss['FlowsDefinition']=O(dict(draft=B,id=S,project_id=S,namespace=S,flow_id=S,name=S,description=S,target_type=S,revision=I,source=S,disabled=B,engine=R('FlowsEngine')))
ss['FlowsSave']=O(dict(draft=B,project_id=S,name=S,description=S,target_type=S,source=S,expected_revision=I))
ss['FlowsTarget']=O(dict(id=S,name=S,type=S,project_id=S,revision=I,fields=J,agent_id=S,agent_online=B,capabilities=A(S),ssh_profile_id=S))
ss['FlowsRun']=O(dict(project_id=S,flow_id=S,expected_revision=I,target_ids=A(S),kind={'type':'string','enum':['command','bash','python','file.upload','file.download','inventory.collect']},command=S,transport_policy={'type':'string','enum':['agent_preferred','agent_only','ssh_only']},timeout_seconds=I,target_concurrency=I,max_retries=I,source_module=S,parent_id=S));ss['FlowsRun']['properties'].pop('parent_id',None);ss['FlowsRun']['properties'].update({'inputs':{'type':'object','additionalProperties':S},'target_revisions':{'type':'object','additionalProperties':I}});ss['FlowsRun']['required']=['project_id','target_ids','kind','command','transport_policy','timeout_seconds','target_concurrency','max_retries']
ss['FlowsDecision']=O(dict(asset_id=S,name=S,revision=I,transport=S,reason=S,ready=B,target=R('FlowsTarget')))
ss['FlowsTargetRun']=O(dict(id=S,snapshot=R('FlowsTarget'),selected_transport=S,actual_transport=N(S),state=S,attempt=I,exit_code=N(I),stdout=S,stderr=S,started_at=N(S),ended_at=N(S)))
ss['FlowsExecution']=O(dict(parent_id=N(S),id=S,project_id=S,flow_id=N(S),kestra_id=N(S),engine_flow_id=S,flow_revision=I,source_module=S,mode=S,started_by=S,created_at=S,error_code=N(S),engine=R('FlowsEngine'),targets=A('FlowsTargetRun')))
ss['FlowsAgent']=O(dict(id=S,project_id=S,asset_id=S,identity={'type':'object','additionalProperties':S},version=S,capabilities=A(S),state=S,certificate_serial=N(S),certificate_expires_at=N(S),last_heartbeat=N(S),online=B))
ss['FlowsAudit']={**J,'properties':dict(id=S,actor=S,action=S,details=J,created_at=S)}
ss['FlowsCapabilities']=O(dict(namespace=S,engine=S,version=S,configured=B,agent_gateway=B,scheduling=S,native_plugins=S))
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
for a in ['pause','resume','kill','restart','retry-failed-targets']:op('/flows/executions/{executionId}/'+a,'post','actionExecution'+a.replace('-','').title(),'FlowsExecution')
op('/flows/agents/{agentId}','get','getRealAgent','FlowsAgent');op('/flows/agents/enrollment-tokens','post','issueAgentEnrollment','FlowsToken','FlowsTokenRequest')
for a in ['approve','quarantine','revoke']:op('/flows/agents/{agentId}/'+a,'post','agent'+a.title(),'FlowsAction','FlowsReason')
for a in ['tasks','audit']:op('/flows/agents/{agentId}/'+a,'get','agent'+a.title(),'FlowsAudit',many=True)
op('/flows/ssh-trust','post','saveFlowsSSHTrust','FlowsSSHTrust','FlowsSSHTrustInput')
new['/flows/executions/{executionId}/events']={'get':{'operationId':'followRealExecution','tags':['FlowsReal'],'parameters':[{'name':'executionId','in':'path','required':True,'schema':S}],'responses':{'200':{'description':'Authorized Kestra SSE proxy; reconnect refetches current state.','content':{'text/event-stream':{'schema':S}}}}}}
ss['FlowsNamespaceFile']=O(dict(path=S,content=S))
ss['FlowsTemplate']=O(dict(name=S,kind=S,source=S))
ss['FlowsBootstrap']=O(dict(project_id=S,asset_id=S,version=S))
ss['FlowsBootstrapResult']=O(dict(asset_id=S,state=S,version=S))
op('/flows/templates','get','getFlowsTemplate','FlowsTemplate')
op('/flows/{flowId}/namespace-file','get','getNamespaceFile','FlowsNamespaceFile')
op('/flows/{flowId}/namespace-file','put','saveNamespaceFile','FlowsNamespaceFile','FlowsNamespaceFile')
op('/flows/agents/bootstrap','post','bootstrapFlowsAgent','FlowsBootstrapResult','FlowsBootstrap')
new['/flows/executions/{executionId}/artifacts']={'get':{'operationId':'downloadExecutionArtifact','tags':['FlowsReal'],'parameters':[{'name':'executionId','in':'path','required':True,'schema':S},{'name':'path','in':'query','required':True,'schema':S},{'name':'project_id','in':'query','schema':S}],'responses':{'200':{'description':'Only an artifact URI in this authorized execution outputs.','content':{'application/octet-stream':{'schema':{'type':'string','format':'binary'}}}}}}}
ss['FlowsDependencyGraph']=O(dict(nodes=A(O(dict(uid=S,namespace=S,flow_id=S,binding_id=S))),edges=A(O(dict(source=S,target=S,relation=J))),partial=B))
op('/flows/{flowId}/dependencies','get','getFlowDependencies','FlowsDependencyGraph')
ss['FlowsFileMetadata']={'type':'object','additionalProperties':True,'properties':dict(fileName=S,type=S,size=I)}
op('/flows/{flowId}/files','get','getFlowFiles','FlowsFileMetadata',many=True)
ss['FlowsSSHTrustResult']=O(dict(asset_id=S,status=S))
op('/flows/ssh-trust','post','saveFlowsSSHTrust','FlowsSSHTrustResult','FlowsSSHTrustInput')
ss['FlowsAgentTaskSummary']=O(dict(id=S,state=S,execution_id=S,updated_at=S))
op('/flows/agents/{agentId}/tasks','get','agentTasks','FlowsAgentTaskSummary',many=True)
ss['FlowsLogEntry']={'type':'object','additionalProperties':True,'properties':dict(timestamp=S,level=S,message=S,taskRunId=S,executionId=S)}
op('/flows/executions/{executionId}/logs','get','getExecutionLogs','FlowsLogEntry',many=True)
logs=new['/flows/executions/{executionId}/logs']['get'];logs['parameters'].append({'name':'follow','in':'query','schema':B});logs['responses']['200']['content']['text/event-stream']={'schema':S}
ss['FlowsValidationResult']={'type':'object','additionalProperties':False,'properties':dict(index=I,namespace=S,flow=S,constraints=S,outdated=B,deprecationPaths=A(S),warnings=A(S),infos=A(S)),'required':['index','outdated']}
op('/flows/{flowId}/validate','post','validateRealFlow','FlowsValidationResult','FlowsValidationRequest',many=True)
ss['FlowsEnrollmentInput']=O(dict(token=S,csr=S,identity={'type':'object','additionalProperties':S},version=S,capabilities=A(S)))
ss['FlowsEnrollmentInput']['properties']['token']={**S,'writeOnly':True}
ss['FlowsEnrollmentResult']=O(dict(id=S,state=S,certificate=S,ca=S,expires_at=S))
op('/flows/agents/register','post','registerFlowsAgent','FlowsEnrollmentResult','FlowsEnrollmentInput',code='201')
new['/flows/agents/register']['post']['security']=[]
new['/flows/agents/register']['post']['description']='TLS-only enrollment listener; one-use scoped token, no Session. Returns public certificate/CA only.'
# Text-file query path and binary artifact are explicit; no implicit generic object endpoint.
new['/flows/{flowId}/namespace-file']['get']['parameters'].append({'name':'path','in':'query','required':True,'schema':S})
ss['FlowsQueryPage']=O(dict(limit=I,has_more=B,next_cursor=N(S)))
ss['FlowsExecutionPage']=O(dict(items=A('FlowsExecution'),page=R('FlowsQueryPage'),state_filter_source=S))
op('/flows/executions','get','listRealExecutions','FlowsExecutionPage')
for name in ['cursor','q','state','source','flow_id','started_by','transport']:new['/flows/executions']['get']['parameters'].append({'name':name,'in':'query','schema':S})
new['/flows/executions']['get']['parameters'].append({'name':'limit','in':'query','schema':{'type':'integer','minimum':1,'maximum':100,'default':50}})
ss['FlowsJobStep']=O(dict(id=S,kind={'type':'string','enum':['command','bash','python']},command=S,depends_on=A(S),inputs={'type':'object','additionalProperties':S}))
ss['FlowsRun']['properties'].update(dict(steps=A('FlowsJobStep'),approval_id=S,approval_action=S))
ss['FlowsSchedule']=O(dict(cron=S,timezone=S,enabled=B,run=R('FlowsRun'),revision=I,last_error=N(S)))
ss['FlowsFileRevision']=O(dict(id=S,path=S,revision=I,sha256=S,size=I,created_at=S))
ss['FlowsFileUpload']=O(dict(path=S,data_base64={**S,'writeOnly':True},expected_revision=I))
ss['FlowsStepResult']=O(dict(asset_id=S,step_id=S,state=S,exit_code=N(I),stdout=S,stderr=S))
ss['FlowsApprovalRequest']=O(dict(run=R('FlowsRun'),reason=S))
ss['FlowsApprovalResult']=O(dict(id=S,status=S))
ss['FlowsEvent']=O(dict(id=I,type=S,data=J))
op('/flows/{flowId}/schedule','get','getTargetSchedule','FlowsSchedule')
op('/flows/{flowId}/schedule','put','saveTargetSchedule','FlowsSchedule','FlowsSchedule')
op('/flows/{flowId}/namespace-artifacts','get','listNamespaceFileVersions','FlowsFileRevision',many=True)
op('/flows/{flowId}/namespace-artifacts','post','uploadNamespaceBinary','FlowsFileRevision','FlowsFileUpload')
op('/flows/{flowId}/namespace-artifacts/{revisionId}/distribute','post','distributeNamespaceRevision','FlowsExecution','FlowsRun')
new['/flows/{flowId}/namespace-artifacts/{revisionId}']={'get':{'operationId':'downloadNamespaceRevision','tags':['FlowsReal'],'parameters':[{'in':'path','name':k,'required':True,'schema':S} for k in ['flowId','revisionId']]+[{'in':'query','name':'project_id','schema':S}],'responses':{'200':{'description':'Immutable authorized binary revision.','content':{'application/octet-stream':{'schema':{'type':'string','format':'binary'}}}}}}}
op('/flows/approval-request','post','requestFlowsApproval','FlowsApprovalResult','FlowsApprovalRequest')
op('/flows/executions/{executionId}/steps','get','getDeviceSteps','FlowsStepResult',many=True)
events=new['/flows/executions/{executionId}/events']['get'];events['description']='Persisted Keel event journal; Last-Event-ID scoped to execution; replay ascending IDs. Permissions checked on every reconnect and stream lifetime.';events['parameters'] += [{'name':'project_id','in':'query','schema':S},{'name':'Last-Event-ID','in':'header','schema':S},{'name':'after','in':'query','schema':I},{'name':'history','in':'query','schema':B}]
ss['FlowsEventPageEnvelope']=O(dict(data=A('FlowsEvent'),request_id=S));events['responses']['200']['content']['application/json']={'schema':R('FlowsEventPageEnvelope')}

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
