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
for name in ['IDEMaterialEstimate','IDEWorkloadEstimate']:
 s=d['components']['schemas'][name];s['properties']['source_model_label']=S;s['properties']['required_calculation_run_id' if name=='IDEMaterialEstimate' else 'quantity_calculation_run_id']=S;s['required'].append('source_model_label');replace(name,s)
s=d['components']['schemas']['IDEPhysicalModel'];s['properties'].update(materials=A('IDEMaterialEstimate'),workloads=A('IDEWorkloadEstimate'));s['required']+=['materials','workloads'];replace('IDEPhysicalModel',s)
s=d['components']['schemas']['IDERegionMapping'];s['properties']['kind']['enum']+=['material','workload'];replace('IDERegionMapping',s)
s=d['components']['schemas']['IDETargetAction'];s['properties']['targets']['items']['enum']+=['ENGINEERING_MATERIAL','ENGINEERING_WORKLOAD'];replace('IDETargetAction',s)
s=d['components']['schemas']['IDECalculationRequest'];s['properties']['operation']['enum'].append('workload_summary');replace('IDECalculationRequest',s)
s=d['components']['schemas']['IDEResultBinding'];s['properties']['field']['enum']+=['material_required','material_gap','workload_quantity','workload_hours'];replace('IDEResultBinding',s)
schemas={'IDEEstimateEdit':O(dict(expected_revision={**I,'minimum':1},material=R('IDEMaterialEstimate'),workload=R('IDEWorkloadEstimate'),reason={**S,'minLength':1}),('material','workload'))};name='editEngineeringEstimate';schemas[name+'Envelope']=O(dict(data=R('IDEWorkspace'),request_id=S))
path='/projects/{projectId}/engineering-imports/{engineeringId}/estimates'
paths={path:{'put':{'operationId':name,'tags':['IntelligentEngineering'],'parameters':[{'in':'path','name':n,'required':True,'schema':S} for n in ['projectId','engineeringId']],'requestBody':{'required':True,'content':{'application/json':{'schema':R('IDEEstimateEdit')}}},'responses':{'200':{'description':'New estimate revision with validation','content':{'application/json':{'schema':R(name+'Envelope')}}},'403':{'description':'Permission denied'},'409':{'description':'Revision conflict'},'422':{'description':'Invalid typed estimate'}}}}}
a=raw.index('components:\n');raw=raw[:a]+block(paths,2)+raw[a:];a=raw.index('x-implementation-policy:');raw=raw[:a]+block(schemas,4)+raw[a:];p.write_text(raw)
