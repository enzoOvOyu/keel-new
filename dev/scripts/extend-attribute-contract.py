from pathlib import Path
import re,yaml
p=Path(__file__).resolve().parents[2]/'docs/openapi.yaml';raw=p.read_text();d=yaml.safe_load(raw)
yaml.SafeDumper.ignore_aliases=lambda self,data:True
def block(v,n):return ''.join(' '*n+l for l in yaml.safe_dump(v,sort_keys=False,allow_unicode=True,width=120).splitlines(True))
S={'type':'string'};I={'type':'integer','minimum':0}
def R(n):return {'$ref':'#/components/schemas/'+n}
def O(p):return {'type':'object','additionalProperties':False,'properties':p,'required':list(p)}
schemas={'IDEAttributeDraft':O(dict(expected_version=I,definition=R('IDEAttributeDefinition'),reason={**S,'minLength':1})), 'IDEAttributeReview':O(dict(version={'type':'integer','minimum':1},decision={'type':'string','enum':['approve','reject']},reason={**S,'minLength':1}))};paths={}
for path,method,name,response,request in [('/projects/{projectId}/engineering-attributes','get','listEngineeringAttributes',{'type':'array','items':R('IDEAttributeDefinition')},None),('/projects/{projectId}/engineering-attributes','post','saveEngineeringAttribute',R('IDEAttributeDefinition'),'IDEAttributeDraft'),('/projects/{projectId}/engineering-attributes/{attributeId}/review','post','reviewEngineeringAttribute',R('IDEAttributeDefinition'),'IDEAttributeReview')]:
 schemas[name+'Envelope']=O(dict(data=response,request_id=S))
 op=dict(operationId=name,tags=['IntelligentEngineering'],parameters=[dict(name=n,**{'in':'path'},required=True,schema=S) for n in re.findall(r'{([^}]+)}',path)],responses={'200':dict(description='Typed project attribute definition',content={'application/json':{'schema':R(name+'Envelope')}}),'403':dict(description='Permission denied'),'404':dict(description='Scoped definition not found'),'409':dict(description='Definition version conflict'),'422':dict(description='Invalid typed attribute')})
 if request:op['requestBody']={'required':True,'content':{'application/json':{'schema':R(request)}}}
 paths.setdefault(path,{})[method]=op
a=raw.index('components:\n');raw=raw[:a]+block(paths,2)+raw[a:];a=raw.index('x-implementation-policy:');raw=raw[:a]+block(schemas,4)+raw[a:];p.write_text(raw)
