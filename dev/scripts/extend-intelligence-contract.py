from pathlib import Path
import re,yaml
p=Path(__file__).resolve().parents[2]/'docs/openapi.yaml';raw=p.read_text();d=yaml.safe_load(raw);schemas={};paths={}
yaml.SafeDumper.ignore_aliases=lambda self,data:True
def block(v,n):return ''.join(' '*n+l for l in yaml.safe_dump(v,sort_keys=False,allow_unicode=True,width=120).splitlines(True))
def replace(name,value):
 global raw
 raw,n=re.subn(r'^    '+re.escape(name)+r':\n.*?(?=^    [A-Za-z0-9_-]+:|^x-implementation-policy:)',lambda _:block({name:value},4),raw,count=1,flags=re.M|re.S);assert n==1
S={'type':'string'};I={'type':'integer'}
def R(n):return {'$ref':'#/components/schemas/'+n}
def A(n):return {'type':'array','items':R(n)}
def O(p,optional=()):return {'type':'object','additionalProperties':False,'properties':p,'required':[k for k in p if k not in optional]}
schemas['IDECellBinding']=O(dict(field=S,sheet=S,cell=S,unit=S))
schemas['IDERecordBinding']=O(dict(cells=A('IDECellBinding')))
s=d['components']['schemas']['IDERegionMapping'];s['properties']['records']=A('IDERecordBinding');replace('IDERegionMapping',s)
s=d['components']['schemas']['IDESourceEvidence'];s['properties']['source_id']=S;replace('IDESourceEvidence',s)
schemas['IDEPlanBlock']=O(dict(instance_id=S,inputs={'type':'array','items':S}))
schemas['IDEPlanView']=O(dict(plan=R('IDEImportPlan'),definitions=A('IDEEngineeringModule'),batches={'type':'array','items':{'type':'array','items':S}},unavailable=A('IDEPlanBlock')))
schemas['IDEIntelligenceView']=O(dict(regions=A('IDEWorkbookRegion'),facts=A('IDESemanticFact'),total_facts=I,offset=I))
for action,name,response in [('plan','getEngineeringPlan','IDEPlanView'),('intelligence','getEngineeringIntelligence','IDEIntelligenceView')]:
 path='/projects/{projectId}/engineering-imports/{engineeringId}/'+action
 schemas[name+'Envelope']=O(dict(data=R(response),request_id=S))
 params=[dict(name=n,**{'in':'path'},required=True,schema=S) for n in ['projectId','engineeringId']]
 if action=='intelligence':params += [dict(name='offset',**{'in':'query'},schema={**I,'minimum':0})]
 paths[path]={'get':dict(operationId=name,tags=['IntelligentEngineering'],parameters=params,responses={'200':dict(description='Typed revision intelligence',content={'application/json':{'schema':R(name+'Envelope')}}),'403':dict(description='Permission denied'),'404':dict(description='Scoped source not found')})}
a=raw.index('components:\n');raw=raw[:a]+block(paths,2)+raw[a:];a=raw.index('x-implementation-policy:');raw=raw[:a]+block(schemas,4)+raw[a:];p.write_text(raw)
