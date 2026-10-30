import json,copy
from pathlib import Path
root=Path(__file__).resolve().parents[1];p=root/'fixtures/demo-full-64-b300/demo-seed.json';d=json.loads(p.read_text());c=d['collections'];stamp='2026-10-30T00:00:00Z';until='2026-10-31T00:00:00Z';pid='proj-b300-demo'
order=['KICKOFF','NETWORK_READINESS','PROVISIONING','VALIDATION','NCCL_VALIDATION','CLEANUP_HANDOVER'];labels=['开局配置','网络就绪','装机','验机','NCCL 测试','优雅退场'];migration={'G1':('VALIDATION','hardware_inventory'),'G2':('VALIDATION','lldp_line_order'),'G3':('VALIDATION','gpu_nic_mapping'),'G4':('PROVISIONING','baseline_apply'),'G5':('VALIDATION','roce_connectivity'),'G6':('NCCL_VALIDATION','nccl_test')}
for old,new in [('gate_evidence','legacy_gate_evidence'),('step_reports','legacy_step_reports')]:
 if old in c:c[new]=c.pop(old)
for k in ['legacy_gate_evidence','legacy_step_reports']:
 for r in c[k]:r['historical_only']=True;r['legacy_gate_id']=r.pop('gate_id',r.get('legacy_gate_id'));r['migration_target']={'stage_id':migration[r['legacy_gate_id']][0],'check_key':migration[r['legacy_gate_id']][1]}
c['delivery_stage_definitions']=[{'id':s,'name':labels[i],'order':i+1,'scope':'project','previous_stage_id':order[i-1] if i else None,'execution_mode':'mock'} for i,s in enumerate(order)]
c['validation_checks']=[];c['delivery_stage_records']=[];c['delivery_stage_events']=[];c['delivery_batches']=[];c['handover_records']=[]
for i,server in enumerate(c['servers']):
 sid=server['id'];gpu=[g for g in c['gpus'] if g['server_id']==sid];nic=[n for n in c['nics'] if n['server_id']==sid];ports=[p for p in c['ports'] if p['device_id']==sid];pairs=[m for m in c['mapping_pairs'] if m['server_id']==sid];base=next((b for b in c['server_config_baselines'] if b['server_id']==sid),{});obs=next((b for b in c['server_observed_configs'] if b['server_id']==sid),{});gpuports=[p for p in ports if p['network_role']=='roce'];ddm=[v for v in c['ddm_samples'] if any(p['peer_port_id']==v['switch_port_id'] for p in gpuports)]
 items=[('hardware_inventory','硬件在位与健康','FAIL' if server['health_status']=='critical' else 'PASS',[sid]),('gpu_nic_inventory','GPU / NIC / 默认配对','PASS' if len(gpu)==8 and len(nic)==10 and all(m['nic_id'] for m in pairs) else 'BLOCKED',[g['id'] for g in gpu]+[n['id'] for n in nic]),('pcie_topology','PCIe BDF与拓扑检查','PASS' if len({g['pci_bdf'] for g in gpu+nic})==len(gpu+nic) else 'FAIL',[g['id'] for g in gpu+nic]),('nic_mac','网卡状态与MAC','PASS' if all(p['oper_state']=='up' and p.get('mac') for p in gpuports) else 'FAIL',[p['id'] for p in gpuports]),('lldp_line_order','LLDP线序与BOM关系','PASS' if len(pairs)==8 and all(m['line_verdict']=='PASS' for m in pairs) else 'FAIL',[m['id'] for m in pairs]),('optics','光模块与DDM','BLOCKED' if not ddm else 'FAIL' if any(v['status'] in ['critical','warning'] for v in ddm) else 'PASS',[v['id'] for v in ddm]),('config_consistency','配置基线与实际一致性','PASS' if base and obs and all(base.get(k)==obs.get(k) for k in ['os','kernel','driver','cuda','nccl','bios','bmc','roce_profile']) else 'FAIL',[base.get('id'),obs.get('id')]),('roce_connectivity','RoCE连通性子任务','PASS' if all(p['oper_state']=='up' for p in gpuports) else 'FAIL',[p['id'] for p in gpuports])]
 checks=[]
 for key,name,verdict,refs in items:
  r={'id':'check-'+sid+'-'+key,'project_id':pid,'device_id':sid,'stage_id':'VALIDATION','check_key':key,'name':name,'verdict':verdict,'required':True,'evidence_refs':[x for x in refs if x],'port_ids':[p['id'] for p in gpuports] if key in ['lldp_line_order','nic_mac','optics','roce_connectivity'] else [],'execution_id':None,'source':'synthetic_validation_fixture_v2','observed_at':stamp,'valid_until':until,'freshness':'fresh','support':'supported','revision':1,'execution_mode':'mock'};checks.append(r);c['validation_checks'].append(r)
 switchids={p['device_id'] for p in c['switch_ports'] if any(x['peer_port_id']==p['id'] for x in ports)};switches=[s for s in c['switches'] if s['id'] in switchids];network_ok=bool(switches) and all(s['ztp_status']=='completed' for s in switches) and not any(x['switch_id'] in switchids and x['severity']=='critical' and x['status']=='open' for x in c['switch_config_diffs'])
 frontier=0 if i>=56 else 1 if i>=48 else 2 if i>=40 else 3 if i>=24 else 5
 blocked=False
 for j,stage in enumerate(order):
  if blocked:status='BLOCKED'
  elif j>frontier:status='NOT_STARTED'
  elif j==frontier:status='READY'
  elif stage=='NETWORK_READINESS' and not network_ok:status='BLOCKED';blocked=True
  elif stage=='VALIDATION' and any(x['verdict']!='PASS' for x in checks):status='FAILED';blocked=True
  else:status='PASSED'
  if stage=='CLEANUP_HANDOVER' and status=='PASSED':status='READY'
  rec={'id':'stage-'+sid+'-'+stage,'project_id':pid,'device_id':sid,'stage_id':stage,'status':status,'evidence_refs':[x['id'] for x in checks] if stage=='VALIDATION' and status in ['PASSED','FAILED'] else ['mock://delivery/'+sid+'/'+stage] if status=='PASSED' else [],'execution_ids':[],'check_ids':[x['id'] for x in checks] if stage=='VALIDATION' else [],'observed_at':stamp if status in ['PASSED','FAILED'] else None,'valid_until':until if status=='PASSED' else None,'waiver':None,'blocked_reasons':['前置阶段未通过'] if blocked else [],'source':'synthetic_six_stage_migration_v1','revision':1,'execution_mode':'mock'};c['delivery_stage_records'].append(rec)
for idx,rack in enumerate(c['racks']):c['delivery_batches'].append({'id':'batch-'+rack['id'],'project_id':pid,'name':rack['name']+'交付批次','rack_id':rack['id'],'target_ids':[s['id'] for s in c['servers'] if s['rack_id']==rack['id']],'revision':1})
# Keep old assertions internal and use the approved PRD meanings; they are not stage numbers.
npnames=['统一BOM链路建模','ZTP与Golden配置就绪','验机线序通过并写入PortModel','网络就绪30秒保鲜与LLDP可读','全网健康BGP/DDM/PTP适用检查','无损基线可用']
for a in c['network_assertions']:a['name']=npnames[int(a['code'].split('-')[1])-1];a['verdict']='BLOCKED';a['notice']='Network-domain assertion, never a delivery stage';a['valid_for_seconds']=30
for e in c['executions']:
 f=next(f for f in c['flows'] if f['id']==e['flow_id']);e['stage_id']='NCCL_VALIDATION' if 'roce' in f['id'] else 'CLEANUP_HANDOVER' if 'cleanup' in f['id'] else 'NETWORK_READINESS' if f['target_types']==['switch'] else 'VALIDATION';e['legacy_stage_association']=True
for f in c['flows']:f['stage_id']='NCCL_VALIDATION' if f['id']=='flow-roce' else 'CLEANUP_HANDOVER' if 'cleanup' in f['id'] else 'NETWORK_READINESS' if f['target_types']==['switch'] else 'VALIDATION'
for r in c['delivery_runs']:
 r['legacy_gate_summary']=r.pop('gate_summary',{});r['stage_order']=order;r['current_stage']='VALIDATION';r['stage_summary']={s:dict(__import__('collections').Counter(x['status'] for x in c['delivery_stage_records'] if x['stage_id']==s)) for s in order}
for r in c['overview']:r['legacy_gate_summary']=r.pop('gate_summary',{});r['stage_summary']=c['delivery_runs'][0]['stage_summary']
for timeline in c['project_timelines']:
 timeline['legacy_stages']=timeline.pop('stages',[]);timeline['stages']=[{'stage_id':s,'name':labels[i],'order':i+1} for i,s in enumerate(order)];timeline['current_stage']='VALIDATION' if timeline['project_id']==pid else 'KICKOFF';timeline['progress_source']='delivery_stage_records';timeline['completion_percent']=0
for raw in c['legacy_step_reports']:
 if raw['legacy_gate_id']=='G6':pass
c['test_reports']=[{**r,'id':'test-'+r['id'],'stage_id':migration[r['legacy_gate_id']][0],'test_kind':'nccl' if r['legacy_gate_id']=='G6' else 'roce','historical_only':True} for r in c['legacy_step_reports'] if r['legacy_gate_id'] in ['G5','G6']]
# Historical data are preserved explicitly; active views never use legacy_gate_summary/legacy_* to unlock stages.
for r in c['cmdb_entities']:
 if r.get('entity_type')=='gate_evidence':r['entity_type']='legacy_gate_evidence'
d['meta']['delivery_model']='ordered-six-stage-v1';d['meta']['counts']={k:len(v) for k,v in c.items()};d['meta']['collection_counts']=d['meta']['counts'];p.write_text(json.dumps(d,ensure_ascii=False,indent=2)+'\n');(root/'docs/DELIVERY-MIGRATION.json').write_text(json.dumps({'stage_order':order,'legacy_mapping':migration,'stage_records':len(c['delivery_stage_records']),'validation_checks':len(c['validation_checks']),'legacy_gate_records':len(c['legacy_gate_evidence'])},ensure_ascii=False,indent=2)+'\n');print('Migrated:',len(c['delivery_stage_records']),'stage records,',len(c['validation_checks']),'validation checks')
