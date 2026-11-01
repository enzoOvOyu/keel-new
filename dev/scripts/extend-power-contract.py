"""Power vertical slice and typed calculation application contracts."""
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
s=d['components']['schemas']['IDERackPowerPlan'];s['required']=[k for k in s['required'] if k!='capacity'];s['properties'].update(cooling_capacity=R('IDEQuantity'),cooling_type=S);s['required'].append('cooling_type');replace('IDERackPowerPlan',s)
s=d['components']['schemas']['IDEPhysicalModel'];s['properties']['power_plans']=A('IDEPowerPlan');s['required'].append('power_plans');replace('IDEPhysicalModel',s)
s=d['components']['schemas']['IDERegionMapping'];s['properties']['kind']['enum'].append('power');replace('IDERegionMapping',s)
s=d['components']['schemas']['IDETargetAction'];s['properties']['targets']['items']['enum'].append('ENGINEERING_POWER');replace('IDETargetAction',s)
s=d['components']['schemas']['IDECalculationRequest'];s['properties']['operation']={'type':'string','enum':['power_balance']};replace('IDECalculationRequest',s)
schemas={'IDEPowerEdit':O(dict(expected_revision={**I,'minimum':1},plan=R('IDEPowerPlan'),reason={**S,'minLength':1})),
 'IDEResultBinding':O(dict(entity_id=S,field={'type':'string','enum':['power_load','power_remaining','cable_calculated','cable_selected_stock']},output_name=S)),
 'IDECalculationApplication':O(dict(expected_revision={**I,'minimum':1},run_id=S,bindings={**A('IDEResultBinding'),'minItems':1},reason={**S,'minLength':1}))}
paths={};base='/projects/{projectId}/engineering-imports/{engineeringId}'
for suffix,method,id,request in [('power','put','editEngineeringPower','IDEPowerEdit'),('calculation-application','post','applyEngineeringCalculation','IDECalculationApplication')]:
 envelope=id+'Envelope';schemas[envelope]=O(dict(data=R('IDEWorkspace'),request_id=S))
 paths[base+'/'+suffix]={method:{'operationId':id,'tags':['IntelligentEngineering'],'parameters':[{'in':'path','name':n,'required':True,'schema':S} for n in ['projectId','engineeringId']], 'requestBody':{'required':True,'content':{'application/json':{'schema':R(request)}}},'responses':{'200':{'description':'New reviewed-input revision with validation','content':{'application/json':{'schema':R(envelope)}}},'403':{'description':'Permission denied'},'409':{'description':'Revision stale'},'422':{'description':'Invalid typed model or result binding'}}}}
a=raw.index('components:\n');raw=raw[:a]+block(paths,2)+raw[a:];a=raw.index('x-implementation-policy:');raw=raw[:a]+block(schemas,4)+raw[a:];p.write_text(raw)
