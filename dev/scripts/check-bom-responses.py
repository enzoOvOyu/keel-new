import json,re,pathlib,yaml,jsonschema
spec=yaml.safe_load(pathlib.Path('docs/openapi.yaml').read_text());resolver=jsonschema.RefResolver.from_schema(spec);captures=json.loads(pathlib.Path('/tmp/keel-bom-contract-captures.json').read_text());checked=0;seen=set()
for capture in captures:
 path=capture['path'].split('?')[0];match=None
 for pattern,ops in spec['paths'].items():
  if re.fullmatch(re.sub(r'\{[^}]+\}',r'[^/]+',pattern),'/'+path) and capture['method'] in ops:match=ops[capture['method']];break
 if match is None:raise AssertionError('Unspecified actual endpoint '+path)
 response=match['responses'].get(capture['status'])
 if response is None:raise AssertionError('Undocumented status '+path+' '+capture['status'])
 if '$ref' in response:
  _,response=resolver.resolve(response['$ref'])
 schema=response.get('content',{}).get('application/json',{}).get('schema')
 if schema:
  errors=list(jsonschema.Draft202012Validator(schema,resolver=resolver).iter_errors(capture['response']));assert not errors,(path,str(errors[0].message) if errors else '');checked+=1;seen.add((capture['method'],path))
print(json.dumps({'response_schema_validation':'PASS','responses':checked,'distinct_calls':len(seen)}))
