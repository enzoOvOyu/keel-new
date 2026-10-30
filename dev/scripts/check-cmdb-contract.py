"""Validate only the real CMDB responses against the checked-in OpenAPI schemas."""
import json,yaml,urllib.request,http.cookiejar,time,sys
from pathlib import Path
from jsonschema import Draft202012Validator
config=dict(line.split('=',1) for line in Path('dev/backend/.env').read_text().splitlines() if '=' in line)
base='http://localhost:8081/api/v1';opener=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
def get(path,body=None):
 req=urllib.request.Request(base+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
 with opener.open(req) as r:return json.load(r)
get('/auth/login',{'username':config.get('KEEL_LOGIN_USER','admin'),'password':config['KEEL_LOGIN_PASSWORD']})
spec=yaml.safe_load(Path('docs/openapi.yaml').read_text());root={'components':spec['components']};checks=[];errors=[]
inv=get('/cmdb/inventory')['data'];switch=inv['switches'][0]['id'];project='proj-b300-demo'
paths=[('/cmdb/schemas','/cmdb/schemas'),('/cmdb/schemas/versions','/cmdb/schemas/versions'),('/cmdb/inventory','/cmdb/inventory'),('/cmdb/dossier/catalog?project_id='+project,'/cmdb/dossier/catalog'),('/cmdb/dossier/tree?project_id='+project,'/cmdb/dossier/tree'),('/cmdb/dossier/relations?root_id=srv-0001&project_id='+project,'/cmdb/dossier/relations'),('/cmdb/dossier/quality?project_id='+project,'/cmdb/dossier/quality'),('/cmdb/changesets?project_id='+project,'/cmdb/changesets')]
for asset in ['srv-0001',switch]:
 prefix='/cmdb/dossier/assets/'+asset
 for sub in ['','/provenance','/configuration','/lifecycle','/revisions']:
  paths.append((prefix+sub+'?project_id='+project,'/cmdb/dossier/assets/{asset_id}'+sub))
 for tab in (['inventory','mapping','ports','pcie','monitoring'] if asset=='srv-0001' else ['identity','configuration','scripts','events','runtime','lossless','optics','logs']):paths.append((prefix+'/drawer/'+tab+'?project_id='+project,'/cmdb/dossier/assets/{asset_id}/drawer/{tab}'))
paths.append(('/cmdb/published-assets?project_id='+project,'/cmdb/published-assets'))
total=len(paths);previous=None
if '--bom-compat' in sys.argv:
 previous=json.loads(Path('docs/CMDB-REAL-CONTRACT-CHECK.json').read_text());paths=[p for p in paths if p[1]=='/cmdb/published-assets']
if '--failed-only' in sys.argv:
 previous=json.loads(Path('docs/CMDB-REAL-CONTRACT-CHECK.json').read_text());failed={e['path'] for e in previous['errors']};paths=[p for p in paths if p[0] in failed]
for path,contract in paths:
 data=get(path);schema=spec['paths'][contract]['get']['responses']['200']['content']['application/json']['schema'];validator=Draft202012Validator(root|schema);found=sorted(validator.iter_errors(data),key=lambda e:str(list(e.path)))
 if found:
  for e in found[:4]:errors.append({'path':path,'field':list(e.path),'message':e.message[:400]})
 else:checks.append(path)
report={'status':'FAIL' if errors else 'PASS','responses_checked':total,'checked_this_run':len(paths),'passed':len(checks)+(previous['passed'] if previous else 0),'errors':errors}
Path('docs/CMDB-REAL-CONTRACT-CHECK.json').write_text(json.dumps(report,indent=2,ensure_ascii=False)+'\n');print(json.dumps(report,ensure_ascii=False));raise SystemExit(bool(errors))
