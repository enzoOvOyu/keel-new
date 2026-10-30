import urllib.request,urllib.error,json,time,pathlib,http.cookiejar,os,uuid
jar=http.cookiejar.CookieJar();urllib.request.install_opener(urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar)))
config=dict(line.split("=",1) for line in pathlib.Path("dev/backend/.env").read_text().splitlines() if "=" in line)
login=urllib.request.Request("http://127.0.0.1:5174/api/v1/auth/login",data=json.dumps({"username":"developer","password":config["KEEL_LOGIN_PASSWORD"]}).encode(),headers={"Content-Type":"application/json"},method="POST")
csrf=json.load(urllib.request.urlopen(login))["data"]["csrf_token"]
RUN_KEY='bom-acceptance-'+str(uuid.uuid4())
CAPTURES=[]
BASE='http://127.0.0.1:5174/api/v1/'
def call(path,method='GET',body=None,key=None):
 data=None if body is None else json.dumps(body).encode();req=urllib.request.Request(BASE+path,data=data,method=method,headers={'Content-Type':'application/json','Idempotency-Key':key or RUN_KEY,'X-CSRF-Token':csrf})
 try:
  with urllib.request.urlopen(req) as r:
   envelope=json.load(r);CAPTURES.append({'path':path,'method':method.lower(),'status':str(r.status),'response':envelope});return r.status,envelope['data']
 except urllib.error.HTTPError as e:return e.code,json.load(e)['error']
def upload(path,declared=None,template='manual-v1',data_override=None):
 boundary='bom-acceptance';data=pathlib.Path(path).read_bytes() if data_override is None else data_override;extra=('--'+boundary+'\r\nContent-Disposition: form-data; name="template_revision_id"\r\n\r\n'+template+'\r\n').encode();
 if declared:extra+=('--'+boundary+'\r\nContent-Disposition: form-data; name="declared_batch_id"\r\n\r\n'+declared+'\r\n').encode()
 body=extra+('--'+boundary+'\r\nContent-Disposition: form-data; name="file"; filename="synthetic.xlsx"\r\nContent-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet\r\n\r\n').encode()+data+('\r\n--'+boundary+'--\r\n').encode();req=urllib.request.Request(BASE+'projects/proj-b300-demo/bom-imports',data=body,headers={'Content-Type':'multipart/form-data; boundary='+boundary,'X-CSRF-Token':csrf},method='POST');return json.load(urllib.request.urlopen(req))['data']['id']
id=upload('/tmp/keel-api-acceptance.xlsx');base='bom-imports/'+id
for _ in range(30):
 _,task=call(base)
 if task['status'] not in ['uploaded','parsing']:break
 time.sleep(1)
assert task['status']=='mapping',task.get('status');rev=task['revision']
code,_=call(base+'/commit','POST',{'expected_revision':rev,'confirm':True});assert code==422,code
_,profiles=call(base+'/profiles');p=profiles[0];p.update({'purpose':'assets','candidate_entity':'server','confirmed':True,'reason':'Synthetic explicit test headers'})
code,x=call(base+'/profiles','PATCH',{'revision':rev,'profiles':profiles,'reason':'Confirm synthetic profile'});assert code==200,(code,x);rev=x['revision']
_,task=call(base);items=task['draft'];assert len(items)==5,len(items)
items[2]['parent_uid']=items[0]['item_uid'];items[3]['parent_uid']=items[1]['item_uid'];items[4]['endpoint_a_uid']=items[2]['item_uid'];items[4]['endpoint_b_uid']=items[3]['item_uid']
code,x=call(base+'/draft','PATCH',{'revision':rev,'items':items,'reason':'Explicit source-backed endpoint review'});assert code==200,(code,x);rev=x['revision']
for category,model in [('server','synthetic-server-model'),('switch','synthetic-switch-model')]:
 code,_=call('bom-models','POST',{'category':category,'model':model,'capabilities':{},'reason':'Synthetic acceptance only'});assert code==201
code,issues=call(base+'/validate','POST',{'revision':rev});assert code==202,(code,issues)
for _ in range(20):
 _,task=call(base)
 if task['status']!='validating':break
 time.sleep(1)
assert not task['issues'],task['issues']
code,_=call(base+'/review','POST',{'revision':rev,'reason':'Accept synthetic source and relations'});assert code==200,code
code,receipt=call(base+'/commit','POST',{'expected_revision':rev,'confirm':True});assert code==201,(code,receipt)
code,again=call(base+'/commit','POST',{'expected_revision':rev,'confirm':True});assert receipt['id']==again['id']
for _ in range(15):
 _,pub=call(base+'/publication')
 if pub['cmdb_status']=='published':break
 time.sleep(1)
assert pub['cmdb_status']=='published',pub
_,assets=call('bom-batches/'+receipt['id']+'/items');assert len(assets)==5,len(assets)
bytype={i['category']:i for i in assets};assert bytype['cable']['endpoint_a_uid'] in {i['item_uid'] for i in assets if i['category']=='port'}
for scenario in ['invalid-json','schema-failure','timeout','provider-error','low-confidence','ambiguity']:
 code,result=call(base+'/ai-tasks','POST',{'mode':'mock','task':'field','scenario':scenario});assert code==409 # committed imports are immutable
# Check failures independently of committed state.
fresh=upload('/tmp/keel-api-acceptance.xlsx')
for _ in range(30):
 _,task=call('bom-imports/'+fresh)
 if task['status']=='mapping':break
 time.sleep(1)
for scenario in ['invalid-json','schema-failure','timeout','provider-error']:
 code,result=call('bom-imports/'+fresh+'/ai-tasks','POST',{'mode':'mock','task':'field','scenario':scenario});assert code==200 and result['status']=='FAILED',(code,result)
_,before=call('bom-imports/'+fresh)
code,recognition=call('bom-imports/'+fresh+'/ai-tasks','POST',{'mode':'rules','task':'field'});assert code==200 and recognition['diagnostic']['candidates']
code,applied=call('bom-imports/'+fresh+'/ai-candidates/'+recognition['id']+'/apply','POST',{'revision':before['revision'],'indices':[0],'reason':'Human verifies source-bound semantic candidate'});assert code==200 and applied['revision']==before['revision']+1
code,conflict=call('bom-imports/'+fresh+'/ai-tasks','POST',{'mode':'mock','task':'field','scenario':'conflict'});assert code==200 and len(conflict['diagnostic']['candidates'])==2
code,_=call('bom-imports/'+fresh+'/ai-candidates/'+conflict['id']+'/apply','POST',{'revision':applied['revision'],'indices':[0,1],'reason':'Conflicting proposals must not be merged'});assert code==422
for scenario in ['low-confidence','ambiguity','unknown-field']:
 code,result=call('bom-imports/'+fresh+'/ai-tasks','POST',{'mode':'mock','task':'field','scenario':scenario});assert code==200 and result['status']=='SUCCEEDED'
code,_=call('bom-imports/'+fresh+'/ai-tasks','POST',{'mode':'provider','task':'field'});assert code==422
code,_=call('projects/unauthorized/bom-imports');assert code==403
declared=RUN_KEY+'-declared'
first=upload('/tmp/keel-api-acceptance.xlsx',declared=declared)
assert upload('/tmp/keel-api-acceptance.xlsx',declared=declared)==first
try:upload('/tmp/keel-api-acceptance.xlsx',declared=declared,data_override=pathlib.Path('/tmp/keel-api-acceptance.xlsx').read_bytes()+b'changed');raise AssertionError('Different batch digest accepted')
except urllib.error.HTTPError as e:assert e.code==409 and json.load(e)['error']['code']=='BATCH_DIGEST_CONFLICT'
code,t=call('bom-mapping-templates','POST',{'import_id':id,'name':RUN_KEY+'-template','revision':rev,'reason':'Synthetic confirmed template reuse'});assert code==201
templated=upload('/tmp/keel-api-acceptance.xlsx',template=t['name']+'@'+str(t['revision']))
for _ in range(30):
 _,templated_task=call('bom-imports/'+templated)
 if templated_task['status']=='mapping':break
 time.sleep(1)
assert len(templated_task['draft'])==5 and call('bom-imports/'+templated+'/profiles')[1][0]['confirmed']
assert all(i['fields'].get('model_version_ref') for i in assets if i['category'] in ['server','switch'])
# Revisions, source, package and diagnostics use the same authoritative API.
code,_=call(base+'/sheets');assert code==200
code,_=call(base+'/mappings');assert code==200
code,_=call(base+'/preview');assert code==200
code,_=call(base+'/issues');assert code==200
code,_=call(base+'/history');assert code==200
code,_=call('bom-imports/'+fresh+'/evaluate','POST',{'mode':'rules','task':'field'});assert code==200
package=urllib.request.urlopen(BASE+base+'/package').read()
import zipfile,io
with zipfile.ZipFile(io.BytesIO(package)) as z:
 assert 'manifest.json' in z.namelist() and len(z.namelist())==8
 manifest=json.loads(z.read('manifest.json'));assert len(manifest['files'])==7
# CSRF cannot be bypassed by an authenticated cookie.
req=urllib.request.Request(BASE+base+'/review',data=b'{}',headers={'Content-Type':'application/json'},method='POST')
try:urllib.request.urlopen(req);raise AssertionError('CSRF accepted')
except urllib.error.HTTPError as e:assert e.code==403
pathlib.Path('/tmp/keel-bom-contract-captures.json').write_text(json.dumps(CAPTURES))
pathlib.Path('/tmp/keel-bom-acceptance-import').write_text(id)
pathlib.Path('/tmp/keel-bom-open-import').write_text(fresh)
print(json.dumps({'status':'PASS','rules_without_provider':True,'blocker_rejection':True,'source_backed_review':True,'normalized_cable_references':True,'idempotent_commit':True,'cmdb_outbox_published':True,'ai_failure_isolation':True,'project_isolation':True,'import_id':id}))
