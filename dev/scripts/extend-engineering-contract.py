"""Add first-slice engineering operations without rewriting unrelated OpenAPI."""
from pathlib import Path
import yaml
p=Path(__file__).resolve().parents[2]/'docs/openapi.yaml';raw=p.read_text();doc=yaml.safe_load(raw)
S={'type':'string'};I={'type':'integer'};B={'type':'boolean'}
def R(n):return {'$ref':'#/components/schemas/'+n}
def A(x):return {'type':'array','items':R(x) if isinstance(x,str) else x}
def O(p,optional=()):return {'type':'object','additionalProperties':False,'properties':p,'required':[k for k in p if k not in optional]}
def E(values):return {'type':'string','enum':values.split('|')}
ss={}
ss['IDEColumnBinding']=O(dict(field=S,column={**I,'minimum':1},unit=S))
ss['IDEConstantBinding']=O(dict(field=S,value=S,evidence_cells=A(S)))
ss['IDERegionMapping']=O(dict(sheet=S,bounds=R('IDEBounds'),kind=E('equipment|assembly|site|room|rack|placement|other'),semantic=S,header_rows=A(I),columns=A('IDEColumnBinding'),constants=A('IDEConstantBinding')))
ss['IDEModuleDetection']=O(dict(module_id=S,applicable=B,reason=S,sheets=A(S)))
ss['IDEEngineeringProposal']=O(dict(summary=S,regions=A('IDERegionMapping'),modules=A('IDEModuleDetection'),questions=A(S)))
ss['IDESourceEvidence']=O(dict(id=S,sheet=S,cell=S,field=S,entity_id=S,formula=B))
ss['IDETargetStatus']=O(dict(target=S,state=doc['components']['schemas']['IDEModuleInstance']['properties']['state'],revision_id=S,digest=S,review_id=S,publication_id=S,error_code=S),('review_id','publication_id','error_code'))
ss['IDEEngineeringJob']=O(dict(id=S,state=E('pending|running|succeeded|failed'),error_code=S,created_at={**S,'format':'date-time'},updated_at={**S,'format':'date-time'}),('error_code',))
ss['IDEEngineeringStep']=O(dict(sequence=I,tool=S,detail=S,state=S,at={**S,'format':'date-time'}))
ss['IDEWorkspace']=O(dict(id=S,project_id=S,import_id=S,filename=S,sha256=S,revision=I,head_revision_id=S,summary=S,total_cells=I,mapped_cells=I,retained_cells=I,modules=A('IDEModuleDetection'),targets=A('IDETargetStatus'),issues=A('IDEValidationIssue'),questions=A(S),latest_job=R('IDEEngineeringJob'),steps=A('IDEEngineeringStep'),created_at={**S,'format':'date-time'}),('head_revision_id','latest_job'))
ss['IDERevisionEntry']=O(dict(id=S,sequence=I,actor=S,origin=E('agent|human'),reason=S,created_at={**S,'format':'date-time'}))
ss['IDERevisionDifference']=O(dict(entity_id=S,kind=S,change=E('added|removed|changed'),before=S,after=S))
ss['IDEModelView']=O(dict(model=R('IDEPhysicalModel'),evidence=A('IDESourceEvidence')))
ss['IDEAttachSource']=O(dict(import_id=S))
ss['IDEExpectedRevision']=O(dict(expected_revision={**I,'minimum':1}))
ss['IDEModelEdit']=O(dict(expected_revision={**I,'minimum':1},model=R('IDEPhysicalModel'),reason={**S,'minLength':1}))
ss['IDETargetAction']=O(dict(expected_revision={**I,'minimum':1},targets={'type':'array','minItems':1,'uniqueItems':True,'items':E('CMDB_PLANNED_EQUIPMENT|CMDB_PLANNED_FACILITY|CMDB_PLANNED_PLACEMENT')},decision=E('approve|reject|publish'),reason={**S,'minLength':1}))
paths={};base='/projects/{projectId}/engineering-imports'
def op(suffix,method,id,response,req=None,code='200',many=False,params=[]):
 name=id+'Envelope';ss[name]=O(dict(data=A(response) if many else R(response),request_id=S))
 v={'operationId':id,'tags':['IntelligentEngineering'],'parameters':[{'in':'path','name':'projectId','required':True,'schema':S}]+([{'in':'path','name':'engineeringId','required':True,'schema':S}] if '{engineeringId}' in suffix else [])+params,'responses':{code:{'description':'Project-authorized engineering draft/review/publication result','content':{'application/json':{'schema':R(name)}}},'403':{'description':'Project/permission denied'},'404':{'description':'Scoped resource not found'},'409':{'description':'Revision, active task or idempotency conflict'},'422':{'description':'Evidence, schema, dependency or review precondition failed'}}}
 if req:v['requestBody']={'required':True,'content':{'application/json':{'schema':R(req)}}}
 paths.setdefault(base+suffix,{})[method]=v
op('','get','listEngineeringImports','IDEWorkspace',many=True)
op('','post','attachEngineeringSource','IDEWorkspace','IDEAttachSource',code='201')
op('/{engineeringId}','get','getEngineeringWorkspace','IDEWorkspace')
op('/{engineeringId}/analyze','post','analyzeEngineeringSource','IDEWorkspace','IDEExpectedRevision',code='202')
op('/{engineeringId}/model','get','getEngineeringModel','IDEModelView',params=[{'in':'query','name':'revision_id','schema':S}])
op('/{engineeringId}/model','put','editEngineeringModel','IDEWorkspace','IDEModelEdit')
op('/{engineeringId}/review','post','reviewEngineeringTargets','IDEWorkspace','IDETargetAction')
op('/{engineeringId}/publish','post','publishEngineeringTargets','IDEWorkspace','IDETargetAction',code='202',params=[{'in':'header','name':'Idempotency-Key','required':True,'schema':S}])
op('/{engineeringId}/revisions','get','listEngineeringRevisions','IDERevisionEntry',many=True)
op('/{engineeringId}/diff','get','diffEngineeringRevision','IDERevisionDifference',many=True,params=[{'in':'query','name':'revision_id','required':True,'schema':S}])
op('/{engineeringId}/source','get','readEngineeringSource','BOMSourceCell',many=True,params=[{'in':'query','name':n,'required':True,'schema':S if n=='sheet' else I} for n in ['sheet','start_row','end_row','start_column','end_column']])
assert not(set(ss)&set(doc['components']['schemas']));assert not(set(paths)&set(doc['paths']))
yaml.SafeDumper.ignore_aliases=lambda self,data:True
def block(value,spaces):return ''.join(' '*spaces+l for l in yaml.safe_dump(value,sort_keys=False,allow_unicode=True,width=120).splitlines(True))
a=raw.index('components:\n');raw=raw[:a]+block(paths,2)+raw[a:];a=raw.index('x-implementation-policy:');raw=raw[:a]+block(ss,4)+raw[a:];p.write_text(raw)
print('Added',sum(len(v) for v in paths.values()),'first-slice operations and',len(ss),'schemas.')
