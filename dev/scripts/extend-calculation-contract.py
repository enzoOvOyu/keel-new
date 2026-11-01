"""P3 contract delta. Keep unrelated paths and schema definitions byte-for-byte."""
from pathlib import Path
import re
import yaml
p=Path(__file__).resolve().parents[2]/'docs/openapi.yaml'
raw=p.read_text();doc=yaml.safe_load(raw)
yaml.SafeDumper.ignore_aliases=lambda self,data:True
def block(value,spaces):return ''.join(' '*spaces+l for l in yaml.safe_dump(value,sort_keys=False,allow_unicode=True,width=120).splitlines(True))
for name,field,values in [
 ('IDEFormulaEvidence','dependency_status',['complete','partial','unsupported','cycle','missing']),
 ('IDEFormulaDependency','kind',['cell','range','named_range','external','dynamic','external_or_structured','unresolved','missing_sheet']),
 ('IDERuleExpression','operator',['input','literal','add','subtract','multiply','divide','min','max','ceil','floor','abs','select_ceiling','outputs','output']),
]:
 schema=doc['components']['schemas'][name];schema['properties'][field]['enum']=values
 pattern=r'^    '+re.escape(name)+r':\n.*?(?=^    [A-Za-z0-9_-]+:|^x-implementation-policy:)'
 raw,n=re.subn(pattern,lambda _:block({name:schema},4),raw,count=1,flags=re.M|re.S);assert n==1
schema=doc['components']['schemas']['IDESourceEvidence'];schema['properties']['calculation_run_id']={'type':'string'}
raw,n=re.subn(r'^    IDESourceEvidence:\n.*?(?=^    [A-Za-z0-9_-]+:)',lambda _:block({'IDESourceEvidence':schema},4),raw,count=1,flags=re.M|re.S);assert n==1
S={'type':'string'};I={'type':'integer'};B={'type':'boolean'}
def R(n):return {'$ref':'#/components/schemas/'+n}
def A(n):return {'type':'array','items':R(n)}
def O(properties,optional=()):return {'type':'object','additionalProperties':False,'properties':properties,'required':[n for n in properties if n not in optional]}
schemas={
 'IDECalculationRequest':O(dict(expected_revision={**I,'minimum':1},rule=R('IDEVersionReference'),requested_cells={**A('IDEFormulaCell'),'maxItems':1000},inputs=A('IDENamedValue')),('rule',)),
 'IDECalculationPage':O(dict(run=R('IDECalculationRun'),total_formulas=I,offset=I,limit=I,stale=B)),
 'IDERuleDraft':O(dict(expected_version={**I,'minimum':0},rule=R('IDECalculationRule'),reason={**S,'minLength':1})),
 'IDERuleReview':O(dict(expected_digest=S,decision={'type':'string','enum':['approve','reject','retire']},reason={**S,'minLength':1})),
 'IDERuleView':O(dict(rule=R('IDECalculationRule'),digest=S)),
}
paths={}
def op(path,method,id,response,request=None,status='200',many=False,query=[]):
 envelope=id+'Envelope';schemas[envelope]=O(dict(data=A(response) if many else R(response),request_id=S))
 params=[{'in':'path','name':n,'required':True,'schema':I if n=='version' else S} for n in re.findall(r'\{([^}]+)\}',path)]+query
 value={'operationId':id,'tags':['IntelligentEngineering'],'parameters':params,'responses':{status:{'description':'Authorized engineering calculation/rule result','content':{'application/json':{'schema':R(envelope)}}},'403':{'description':'Permission denied'},'404':{'description':'Scoped resource not found'},'409':{'description':'Version conflict'},'422':{'description':'Rule, unit, approval, calculation or schema precondition failed'}}}
 if request:value['requestBody']={'required':True,'content':{'application/json':{'schema':R(request)}}}
 paths.setdefault(path,{})[method]=value
base='/projects/{projectId}/engineering-rules'
op(base,'get','listEngineeringRules','IDERuleView',many=True)
op(base,'post','createEngineeringRule','IDERuleView','IDERuleDraft','201')
op(base+'/{ruleId}/{version}','get','getEngineeringRule','IDERuleView')
op(base+'/{ruleId}/{version}/review','post','reviewEngineeringRule','IDERuleView','IDERuleReview')
base='/projects/{projectId}/engineering-imports/{engineeringId}'
op(base+'/calculation-provider','get','getEngineeringCalculationProvider','IDECalculationCapabilities')
op(base+'/calculations','get','listEngineeringCalculations','IDECalculationPage',many=True)
op(base+'/calculations','post','runEngineeringCalculation','IDECalculationPage','IDECalculationRequest','202')
op(base+'/calculations/{runId}','get','getEngineeringCalculation','IDECalculationPage',query=[{'in':'query','name':n,'schema':{'type':'integer','minimum':0 if n=='offset' else 1,**({'maximum':200} if n=='limit' else {})}} for n in ['offset','limit']])
assert not(set(schemas)&set(doc['components']['schemas']));assert not(set(paths)&set(doc['paths']))
a=raw.index('components:\n');raw=raw[:a]+block(paths,2)+raw[a:]
a=raw.index('x-implementation-policy:');raw=raw[:a]+block(schemas,4)+raw[a:];p.write_text(raw)
print('Added 8 calculation/rule operations and',len(schemas),'schemas; extended 3 enumerations.')
