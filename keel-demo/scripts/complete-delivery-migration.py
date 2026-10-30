import json
from collections import Counter
from pathlib import Path
root=Path(__file__).resolve().parents[1];p=root/'fixtures/demo-full-64-b300/demo-seed.json';d=json.loads(p.read_text());c=d['collections'];stamp='2026-10-30T00:00:00Z';types={'hardware_inventory':'T5','gpu_nic_inventory':'T7','pcie_topology':'T7','nic_mac':'T5','lldp_line_order':'T2','optics':'T3','config_consistency':'T6','roce_connectivity':'T1'}
# Seed failed validation states carry the same device/check/port/Execution/Issue references as runtime commands.
for r in c['delivery_stage_records']:
 if r['stage_id']!='VALIDATION' or r['status']!='FAILED':continue
 checks=[x for x in c['validation_checks'] if x['device_id']==r['device_id']];eid='migrated-validation-'+r['device_id'];r['execution_ids']=[eid];r['blocked_reasons']=[x['name']+'未通过' for x in checks if x['verdict']!='PASS']
 if not any(e['id']==eid for e in c['executions']):c['executions'].append({'id':eid,'project_id':r['project_id'],'stage_id':'VALIDATION','flow_id':'delivery-stage-VALIDATION','flow_revision':1,'status':'failed','target_ids':[r['device_id']],'target_count':1,'target_snapshot_id':r['id'],'counts':{'failed':1},'started_at':stamp,'finished_at':stamp,'trigger_source':'migration_fixture','started_by':'demo.delivery.engineer','execution_mode':'mock','snapshot_complete':True})
 for check in checks:
  check['execution_id']=eid;tid='task-'+check['id']
  if not any(t['id']==tid for t in c['task_runs']):c['task_runs'].append({'id':tid,'project_id':r['project_id'],'execution_id':eid,'stage_id':'VALIDATION','task_key':check['check_key'],'target_id':r['device_id'],'status':'success' if check['verdict']=='PASS' else 'failed','attempt_count':1,'started_at':stamp,'finished_at':stamp,'exit_code':0 if check['verdict']=='PASS' else 1,'execution_mode':'mock','source':'validation_check_projection'})
  if check['verdict']=='PASS':continue
  iid='validation-issue-'+check['id'];check['issue_id']=iid
  if not any(i['id']==iid for i in c['issues']):c['issues'].append({'id':iid,'project_id':r['project_id'],'issue_type':types[check['check_key']],'source_domain':'validation','source_ref':check['id'],'source_check_id':check['id'],'stage_id':'VALIDATION','device_id':r['device_id'],'port_ids':check['port_ids'],'execution_id':eid,'status':'open','severity':'critical' if check['verdict']=='FAIL' else 'warning','title':'验机异常 · '+check['name']+' · '+r['device_id'],'affected_entities':[r['device_id']]+check['port_ids'],'evidence_refs':[check['id']]+check['evidence_refs'],'resolution':None,'retest_status':'not_started','created_at':stamp,'execution_mode':'mock'})
# NCCL readiness is independent from (and cannot alter) validation.
for r in c['delivery_stage_records']:
 if r['stage_id']!='NCCL_VALIDATION' or r['status']!='PASSED':continue
 serverports=[p for p in c['ports'] if p['device_id']==r['device_id']];sp=[p for p in c['switch_ports'] if any(s['peer_port_id']==p['id'] for s in serverports)];switchids={p['device_id'] for p in sp};bgp=[n for n in c['switch_bgp_neighbors'] if n['switch_id'] in switchids];ddm=[n for n in c['ddm_samples'] if any(p['id']==n['switch_port_id'] for p in sp)]
 if not bgp or any(n['state']!='established' for n in bgp) or not ddm or any(n['status'] in ['warning','critical'] for n in ddm):
  r['status']='BLOCKED';r['blocked_reasons']=['NP-5：网络测试前置证据未满足'];r['valid_until']=None
  for nxt in c['delivery_stage_records']:
   if nxt['device_id']==r['device_id'] and nxt['stage_id']=='CLEANUP_HANDOVER':nxt['status']='BLOCKED';nxt['blocked_reasons']=['NCCL测试尚未通过']
# Explicit stage transition evidence. Older result snapshots are not interpreted as the new pipeline.
c['delivery_stage_events']=[{'id':'migration-event-'+r['id'],'project_id':r['project_id'],'device_id':r['device_id'],'stage_id':r['stage_id'],'from_status':'NOT_STARTED','to_status':r['status'],'evidence_refs':r['evidence_refs'] or [r['id']],'reason':'新六阶段合成数据迁移快照；非真实设备执行','actor':'demo.admin','occurred_at':stamp,'source':'migration_snapshot','historical_snapshot':True} for r in c['delivery_stage_records'] if r['status']!='NOT_STARTED']
for r in c['delivery_runs']:r['stage_summary']={sid:dict(Counter(x['status'] for x in c['delivery_stage_records'] if x['stage_id']==sid)) for sid in [x['id'] for x in c['delivery_stage_definitions']]}
for r in c['overview']:r['stage_summary']=c['delivery_runs'][0]['stage_summary']
# Keep raw old gate associations only inside explicitly marked history.
for e in c['audit_events']:
 if 'G1' in str(e) or 'G2' in str(e) or 'G3' in str(e):e['historical_only']=True
c.setdefault('validation_remediations',[]);c.setdefault('provisioning_receipts',[]);c.setdefault('port_models',[])
d['meta']['counts']={k:len(v) for k,v in c.items()};d['meta']['collection_counts']=d['meta']['counts'];p.write_text(json.dumps(d,ensure_ascii=False,indent=2)+'\n')
print('Completed seed associations for six-stage model; no tests executed')
