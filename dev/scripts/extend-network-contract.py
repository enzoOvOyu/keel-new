from pathlib import Path
import re,yaml
p=Path(__file__).resolve().parents[2]/'docs/openapi.yaml';raw=p.read_text();d=yaml.safe_load(raw)
yaml.SafeDumper.ignore_aliases=lambda self,data:True
def block(v,n):return ''.join(' '*n+l for l in yaml.safe_dump(v,sort_keys=False,allow_unicode=True,width=120).splitlines(True))
def replace(name,value):
 global raw
 raw,n=re.subn(r'^    '+re.escape(name)+r':\n.*?(?=^    [A-Za-z0-9_-]+:|^x-implementation-policy:)',lambda _:block({name:value},4),raw,count=1,flags=re.M|re.S);assert n==1
S={'type':'string'};I={'type':'integer'}
def R(n):return {'$ref':'#/components/schemas/'+n}
def A(n):return {'type':'array','items':R(n)}
def O(p,optional=()):return {'type':'object','additionalProperties':False,'properties':p,'required':[k for k in p if k not in optional]}
s=d['components']['schemas']['IDEPhysicalModel'];s['properties']['network_plans']=A('IDENetworkPlan');s['required'].append('network_plans');replace('IDEPhysicalModel',s)
s=d['components']['schemas']['IDENetworkAllocation'];s['properties']['vrf_ref']=S;replace('IDENetworkAllocation',s)
s=d['components']['schemas']['IDERegionMapping'];s['properties']['kind']['enum']+=['network_fabric','network_plane','network_role','network_interface','network_allocation','network_bgp','network_bfd','network_qos','network_ar','network_evpn'];replace('IDERegionMapping',s)
s=d['components']['schemas']['IDETargetAction'];s['properties']['targets']['items']['enum'].append('NETWORK_REVISION');replace('IDETargetAction',s)
schemas={'IDENetworkEdit':O(dict(expected_revision={**I,'minimum':1},plan=R('IDENetworkPlan'),reason={**S,'minLength':1})),
 'IDEPhysicalRevisionOption':O(dict(revision_id=S,source_id=S,publication_id=S,target=S,filename=S,published_at={'type':'string','format':'date-time'})),
 'IDEGoldenInput':O(dict(publication_id=S,revision_id=S,plan=R('IDENetworkPlan'),physical_revision_id=S))}
paths={}
def operation(path,method,name,response,request=None,many=False):
 schemas[name+'Envelope']=O(dict(data=A(response) if many else R(response),request_id=S));op={'operationId':name,'tags':['IntelligentEngineering'],'parameters':[{'in':'path','name':n,'required':True,'schema':S} for n in re.findall(r'{([^}]+)}',path)],'responses':{'200':{'description':'Authorized typed engineering result','content':{'application/json':{'schema':R(name+'Envelope')}}},'403':{'description':'Permission denied'},'404':{'description':'Scoped revision not found'},'409':{'description':'Revision stale or not published'},'422':{'description':'Invalid network planning facts or dependencies'}}}
 if request:op['requestBody']={'required':True,'content':{'application/json':{'schema':R(request)}}}
 paths[path]={method:op}
base='/projects/{projectId}'
operation(base+'/engineering-imports/{engineeringId}/network','put','editEngineeringNetwork','IDEWorkspace','IDENetworkEdit')
operation(base+'/engineering-physical-revisions','get','listEngineeringPhysicalRevisions','IDEPhysicalRevisionOption',many=True)
operation(base+'/engineering-imports/{engineeringId}/golden-input','get','getEngineeringGoldenInput','IDEGoldenInput')
a=raw.index('components:\n');raw=raw[:a]+block(paths,2)+raw[a:];a=raw.index('x-implementation-policy:');raw=raw[:a]+block(schemas,4)+raw[a:];p.write_text(raw)
