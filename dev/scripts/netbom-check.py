import urllib.request,urllib.error,http.cookiejar,pathlib,json,time,hashlib,uuid,collections
config=dict(l.split('=',1) for l in pathlib.Path('dev/backend/.env').read_text().splitlines() if '=' in l);jar=http.cookiejar.CookieJar();urllib.request.install_opener(urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar)));base='http://127.0.0.1:5174/api/v1/'
req=urllib.request.Request(base+'auth/login',data=json.dumps({'username':config.get('KEEL_LOGIN_USER','admin'),'password':config['KEEL_LOGIN_PASSWORD']}).encode(),headers={'Content-Type':'application/json'},method='POST');csrf=json.load(urllib.request.urlopen(req))['data']['csrf_token']
def call(p,body=None):
 req=urllib.request.Request(base+p,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json','X-CSRF-Token':csrf,'Idempotency-Key':str(uuid.uuid4())},method='GET' if body is None else 'POST')
 try:
  with urllib.request.urlopen(req) as r:return r.status,json.load(r)['data']
 except urllib.error.HTTPError as e:return e.code,json.load(e)['error']
data=pathlib.Path('netbom.xlsx').read_bytes();boundary='netbom-check';body=('--'+boundary+'\r\nContent-Disposition: form-data; name="file"; filename="netbom.xlsx"\r\nContent-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet\r\n\r\n').encode()+data+('\r\n--'+boundary+'--\r\n').encode();req=urllib.request.Request(base+'projects/proj-b300-demo/bom-imports',data=body,headers={'Content-Type':'multipart/form-data; boundary='+boundary,'X-CSRF-Token':csrf},method='POST');upload=json.load(urllib.request.urlopen(req))['data'];id=upload['id'];p='bom-imports/'+id
for _ in range(120):
 _,task=call(p)
 if task['status'] not in ['uploaded','parsing']:break
 time.sleep(1)
assert task['status']=='mapping',task['status'];assert task['content_sha256']==hashlib.sha256(data).hexdigest();sheets=task['workbook']['sheets'];assert len(sheets)==15
status,_=call(p+'/validate',{'revision':task['revision']});assert status==202
for _ in range(120):
 _,task=call(p)
 if task['status']!='validating':break
 time.sleep(1)
assert task['status']=='review_required',task['status'];assert task['issues'];status,_=call(p+'/commit',{'expected_revision':task['revision'],'confirm':True});assert status==422
report={'source':'netbom.xlsx','mode':'rules','real_upload_parse':'PASS','sheet_count':len(sheets),'nonempty_cells':sum(len(s['source_cells']) for s in sheets),'merged_regions':sum(len(s['merged_cells']) for s in sheets),'formula_cells':sum(bool(c.get('formula')) for s in sheets for c in s['source_cells']),'source_sha256':task['content_sha256'],'profile_count':len(call(p+'/profiles')[1]),'validation_blocking':'PASS','commit_rejection':'PASS','formal_sample_commit':'BLOCKED_PENDING_HUMAN_REVIEW','issues_by_rule':dict(collections.Counter(i['rule_id'] for i in task['issues'])),'no_real_ai_call':True,'note':'No automatic approval, fabricated serial/port or publication of unreviewed real customer data.'}
pathlib.Path('/tmp/keel-netbom-real-acceptance.json').write_text(json.dumps(report,indent=2));print(json.dumps({k:v for k,v in report.items() if k!='issues_by_rule'}))
