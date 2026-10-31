"""Only read the repository synthetic fixture. Never reads DB/private Excel."""
import json,pathlib
root=pathlib.Path(__file__).resolve().parents[3]
c=json.loads((root/'fixtures/demo-full-64-b300/demo-seed.json').read_text())['collections']
def pick(x,keys):return {k:x[k] for k in keys.split() if k in x}
a=[]
for key,kind in [('racks','rack'),('servers','server'),('switches','switch')]:
 for x in c[key]:
  f=pick(x,'rack_id rack_u u_height model role gpu_count nic_count pod_id serial management_address maintenance_state freshness health_status nos_version')
  a.append(dict(id=x['id'],type=kind,name=x.get('hostname',x.get('name',x['id'])).replace('.demo.keel.invalid',''),project_id=x['project_id'],revision=x.get('revision',1),source='synthetic:demo-full-64-b300',fields=f))
ports=[]
for x in c['ports']+c['switch_ports']:
 ports.append(dict(id=x['id'],type='port',name=x['physical_name'],project_id=x['project_id'],revision=x.get('revision',1),source='synthetic:demo-full-64-b300',fields={**pick(x,'speed oper_state port_index network_role'), 'owner_device_id':x['device_id']}))
by={x['id']:x for x in ports}
cables=[]
for x in c['cables']:
 pa=x['endpoint_a_uid'];pb=x['endpoint_b_uid']
 cables.append(dict(id=x['id'],a=pa,b=pb,device_a=by.get(pa,{}).get('fields',{}).get('owner_device_id',''),device_b=by.get(pb,{}).get('fields',{}).get('owner_device_id',''),fact='planning',source='synthetic:demo-full-64-b300',fields=pick(x,'cable_type length_m')))
# Isolated adverse examples are overlays, not changes to canonical fixture or CMDB.
cables[1]['fields']['example']='stale';cables[2]['fields']['example']='conflict';cables[3]['b']='';cables[3]['device_b']='';cables[3]['fields']['example']='incomplete'
out=dict(meta={'synthetic':True,'source':'fixtures/demo-full-64-b300/demo-seed.json','observed_at':'2026-10-30T00:00:00Z','scope':'design review only; three cable evidence overlays; no live health'},assets=a,ports=ports,cables=cables)
(pathlib.Path(__file__).parent/'fixture.json').write_text(json.dumps(out,ensure_ascii=False,separators=(',',':')))
