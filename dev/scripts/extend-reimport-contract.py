from pathlib import Path
import re,yaml
p=Path(__file__).resolve().parents[2]/'docs/openapi.yaml';raw=p.read_text();d=yaml.safe_load(raw)
yaml.SafeDumper.ignore_aliases=lambda self,data:True
def block(v,n):return ''.join(' '*n+l for l in yaml.safe_dump(v,sort_keys=False,allow_unicode=True,width=120).splitlines(True))
def replace(name,value):
 global raw
 raw,n=re.subn(r'^    '+re.escape(name)+r':\n.*?(?=^    [A-Za-z0-9_-]+:|^x-implementation-policy:)',lambda _:block({name:value},4),raw,count=1,flags=re.M|re.S);assert n==1
S={'type':'string'};I={'type':'integer','minimum':1}
def R(n):return {'$ref':'#/components/schemas/'+n}
def A(n):return {'type':'array','items':R(n)}
def O(p,optional=()):return {'type':'object','additionalProperties':False,'properties':p,'required':[k for k in p if k not in optional]}
s=d['components']['schemas']['IDEWorkspace'];s['properties'].update(base_revision_id=S,base_source_id=S);replace('IDEWorkspace',s)
schemas={'IDEReimportRequest':O(dict(expected_revision=I,import_id=S)), 'IDEDifferenceResolution':O(dict(target={'type':'string','enum':d['components']['schemas']['IDETargetAction']['properties']['targets']['items']['enum']},decision={'type':'string','enum':['accept_incoming','retain_base','retain_current']})), 'IDEDifferenceReview':O(dict(expected_revision=I,resolutions=A('IDEDifferenceResolution'),reason={**S,'minLength':1}))};paths={}
for action,name,request in [('reimport','reimportEngineeringSource','IDEReimportRequest'),('difference-review','reviewEngineeringDifferences','IDEDifferenceReview')]:
 path='/projects/{projectId}/engineering-imports/{engineeringId}/'+action
 schemas[name+'Envelope']=O(dict(data=R('IDEWorkspace'),request_id=S))
 paths[path]={'post':dict(operationId=name,tags=['IntelligentEngineering'],parameters=[dict(name=n,**{'in':'path'},required=True,schema=S) for n in ['projectId','engineeringId']],requestBody={'required':True,'content':{'application/json':{'schema':R(request)}}},responses={'200':dict(description='Immutable source or reviewed difference draft',content={'application/json':{'schema':R(name+'Envelope')}}),'403':dict(description='Permission denied'),'404':dict(description='Scoped source not found'),'409':dict(description='Revision conflict'),'422':dict(description='Invalid difference decision')})}
a=raw.index('components:\n');raw=raw[:a]+block(paths,2)+raw[a:];a=raw.index('x-implementation-policy:');raw=raw[:a]+block(schemas,4)+raw[a:];p.write_text(raw)
