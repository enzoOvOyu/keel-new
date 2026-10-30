from pathlib import Path
import yaml,copy
p=Path('docs/openapi.yaml');raw=p.read_text();doc=yaml.safe_load(raw);old=copy.deepcopy(doc);S=doc['components']['schemas']
class D(yaml.SafeDumper):
 def ignore_aliases(self,data):return True
st={'type':'string'};bo={'type':'boolean'};it={'type':'integer'};nullst={'type':['string','null']}
def ref(n):return {'$ref':'#/components/schemas/'+n}
def obj(v,required=None):return {'type':'object','properties':v,'required':list(v) if required is None else required,'additionalProperties':False}
fields=dict(name={'type':'string','minLength':2,'maxLength':120},protocol={'enum':['openai-chat','keel-mapping']},endpoint={'type':'string','format':'uri'},model=st,timeout_seconds={'type':'integer','minimum':1,'maximum':120},token_budget={'type':'integer','minimum':128,'maximum':32768},token_parameter={'enum':['max_tokens','max_completion_tokens']},json_mode=bo,enabled=bo,is_default=bo)
S['AIProviderConfiguration']=obj(dict(id=st,**fields,credential_ref=st,has_credential=bo,revision=it,last_test_status={'enum':['not_tested','passed','failed']},last_test_code=nullst,last_test_http_status={'type':['integer','null']},last_test_at={'type':['string','null']},last_test_ms={'type':['integer','null']}))
S['AIProviderConfigurationCommand']=obj(dict(**fields,revision=it,api_key={'type':'string','writeOnly':True,'description':'Sent once to controlled server secret storage. Empty retains an existing key. Never returned or audited.'}),list(fields))
S['AIProviderTestCommand']=obj(dict(revision=it))
S['AIProviderTestResult']=obj(dict(provider_id=st,revision=it,status={'enum':['passed','failed']},error_code=st,http_status=it,duration_ms=it,synthetic=bo,observed_at=st))
S['AIProviderOption']=obj(dict(id=st,name=st,model=st,protocol=fields['protocol'],is_default=bo))
S['IdentityCreateProject']['properties']['name'].update(minLength=2,maxLength=240)
S['IdentityCreateProject']['properties']['code'].update(pattern='^[A-Za-z0-9_-]{3,32}$')
for name,schema in S.items():
 if name.startswith('BOM') and schema.get('type')=='object' and set(['mode','task']).issubset(schema.get('properties',{})):
  schema['properties']['provider_id']=st
for name,schema in S.items():
 if 'diagnostic' in name.lower() and schema.get('type')=='object':
  schema.setdefault('properties',{}).update(provider_id=st,provider_revision=it,protocol=st)
def operation(path,method,operation,data,body=None):
 env=operation+'Envelope';S[env]=obj(dict(data=data,request_id=st));item=doc['paths'].setdefault(path,{});item['parameters']=[{'in':'path','name':'provider_id','required':True,'schema':st}] if '{provider_id}' in path else []
 o={'tags':['AI Configuration'],'operationId':operation,'security':[{'cookieSession':[]}],'description':'Real configuration. Platform admin required except authorized project provider options. Test sends synthetic data only. Credentials never appear in reads or audit.','responses':{'200':{'description':'Result; failed model connection is represented by test status=failed.','content':{'application/json':{'schema':ref(env)}}}}}
 if body:o['requestBody']={'required':True,'content':{'application/json':{'schema':ref(body)}}}
 if method!='get':o['parameters']=[{'in':'header','name':'X-CSRF-Token','required':True,'schema':st}]
 for code in ['400','401','403','404','409','422','500']:o['responses'][code]={'description':'Input, authorization, revision, configuration or storage error.'}
 item[method]=o
operation('/admin/ai-providers','get','listAIProviderConfigurations',{'type':'array','items':ref('AIProviderConfiguration')})
operation('/admin/ai-providers','post','createAIProviderConfiguration',ref('IdentityMutation'),'AIProviderConfigurationCommand')
operation('/admin/ai-providers/{provider_id}','put','updateAIProviderConfiguration',ref('IdentityMutation'),'AIProviderConfigurationCommand')
operation('/admin/ai-providers/{provider_id}/test','post','testAIProviderConfiguration',ref('AIProviderTestResult'),'AIProviderTestCommand')
operation('/ai/provider-options','get','listBOMProviderOptions',{'type':'array','items':ref('AIProviderOption')});doc['paths']['/ai/provider-options']['get']['parameters']=[{'in':'query','name':'project_id','required':True,'schema':st}]
# Replace only changed mapping entries, retaining unrelated formatting.
import re
def patch_section(raw,marker,level,previous,new):
 start=raw.index(marker)+len(marker)
 end_match=re.search(r'(?m)^\S' if level==2 else r'(?m)^(?:\S|  \S)',raw[start:]);end=start+end_match.start() if end_match else len(raw);text=raw[start:end]
 for key,value in new.items():
  if previous.get(key)==value:continue
  block=''.join(' '*level+line if line.strip() else line for line in yaml.dump({key:value},Dumper=D,allow_unicode=True,sort_keys=False,width=160).splitlines(True))
  pattern=r'(?m)^'+' '*level+re.escape(key)+r':[^\n]*\n(?:^(?:'+' '*(level+1)+r'[^\n]*|[ \t]*)\n)*';match=re.search(pattern,text)
  if match:text=text[:match.start()]+block+text[match.end():]
  else:text+=block
 return raw[:start]+text+raw[end:]
raw=patch_section(raw,'paths:\n',2,old['paths'],doc['paths']);raw=patch_section(raw,'  schemas:\n',4,old['components']['schemas'],S)
assert yaml.safe_load(raw)['components']['schemas']==S
p.write_text(raw)
