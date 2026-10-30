"""Deterministic incubation of workflow/evidence data; immutable hardware scope."""
import json,hashlib,datetime,collections
from pathlib import Path
root=Path(__file__).resolve().parents[1]
base=json.loads((root/'fixtures/demo-64-b300/demo-seed.json').read_text())
c=base['collections'];out={k:[] for k in ['mapping_pairs','step_reports','config_sets','network_assertions','device_operations','monitoring_rules','retirement_runs','ai_cases','scenario_profiles']}
T='2026-10-29T14:00:00Z';until='2026-10-30T14:00:00Z'
def record(id,**kw):return dict({'id':id,'project_id':'proj-b300-demo','execution_mode':'mock','source':'deterministic_incubation_v1','revision':1},**kw)
# Synthesize observations with explicit mock provenance. Physical endpoints come only from canonical cables.
ports={p['id']:p for p in c['ports']};switch_ports={p['id']:p for p in c['switch_ports']};cables={p['endpoint_a_uid']:p for p in c['cables']}
for s in c['servers']:
 gs=[g for g in c['gpus'] if g['server_id']==s['id']]
 ns=[n for n in c['nics'] if n['server_id']==s['id'] and n['network_role']=='roce']
 for i,(g,n) in enumerate(zip(gs,ns)):
  p=next((p for p in c['ports'] if p.get('component_id')==n['id']),None);cable=cables.get(p['id']) if p else None
  if not p or not cable:raise ValueError('Missing canonical cable '+n['id'])
  verdict=cable['line_check'];observed=cable['endpoint_b_uid'] if verdict=='PASS' else None
  out['mapping_pairs'].append(record(s['id']+'-mapping-'+str(i+1),server_id=s['id'],gpu_id=g['id'],gpu_bdf=g['pci_bdf'],nic_id=n['id'],nic_bdf=n['pci_bdf'],server_port_id=p['id'],switch_port_id=cable['endpoint_b_uid'],cable_id=cable['id'],os_interface='demo-rdma'+str(i),server_lldp_peer=observed,switch_lldp_peer=p['id'] if verdict=='PASS' else None,line_verdict=verdict if verdict=='PASS' else 'BLOCKED',status='needs_review',pairing_policy='synthetic profile: ordinal GPU to RoCE NIC; candidate only, requires human confirmation',evidence_refs=[cable['id'],p['id']],collected_at=T))
 for i in range(1,7):
  gate='G'+str(i);pairs=[p for p in out['mapping_pairs'] if p['server_id']==s['id']]
  existing=next((e for e in c['gate_evidence'] if e['subject_id']==s['id'] and e['gate_id']==gate),None)
  verdict=existing['verdict'] if existing else ('PASS' if s['health_status']=='healthy' else ('FAIL' if i in [1,5,6] else 'BLOCKED'))
  if i==2 and any(p['line_verdict']!='PASS' for p in pairs):verdict='BLOCKED'
  if i==3:verdict='BLOCKED' # candidate GPU/NIC mappings are explicitly unconfirmed
  metrics={'observed_gpu_count':len(gs),'observed_nic_count':len([n for n in c['nics'] if n['server_id']==s['id']])} if i==1 else {'coverage_targets':1,'observed_links':len(pairs)}
  out['step_reports'].append(record('report-'+s['id']+'-'+gate,subject_id=s['id'],gate_id=gate,name={'G1':'GPU / NIC / 系统预收集','G2':'BOM / 服务器LLDP / 交换机LLDP','G3':'GPU-NIC候选审核','G4':'批准基线比对','G5':'单设备RoCE测试','G6':'单设备NCCL测试'}[gate],verdict=verdict,metrics=metrics,test_scope='single_device',collected_at=T,valid_until=until,rule_versions=['synthetic-demo-profile-v1'],source_refs=[s['id'],s['source_bom_item_id']],notice='SIMULATION_ONLY; verdict is a synthetic scenario, not a production threshold evaluation'))
# Fill missing object-level evidence, never widen a single-object PASS into a project PASS.
extra_gates=list(c['gate_evidence'])
for r in out['step_reports']:
 if not any(e['subject_id']==r['subject_id'] and e['gate_id']==r['gate_id'] for e in extra_gates):extra_gates.append(record('inc-gate-'+r['subject_id']+'-'+r['gate_id'],subject_type='server',subject_id=r['subject_id'],gate_id=r['gate_id'],verdict=r['verdict'],evidence_refs=[r['id']],rule_versions=r['rule_versions'],collected_at=T,valid_until=until,waiver=None))
out['gate_evidence']=extra_gates
for i,(code,name,verdict) in enumerate([('NP-1','链路两端与端口建模','PASS'),('NP-2','ZTP完成与Golden无关键漂移','BLOCKED' if any(s['config_status']!='IN_SYNC' for s in c['switches']) else 'PASS'),('NP-3','三方线序比对','BLOCKED' if any(p['line_verdict']!='PASS' for p in out['mapping_pairs']) else 'PASS'),('NP-4','LLDP与30秒就绪证据','PASS'),('NP-5','全网BGP / PTP / 光功率健康','BLOCKED'),('NP-6','无损基线与能力采集','BLOCKED')]):
 out['network_assertions'].append(record('assertion-'+code,code=code,name=name,verdict=verdict,collected_at=T,source_refs=['netbase-demo-v1'],valid_for_seconds=30 if code=='NP-4' else 86400,notice='Synthetic scenario; NP-4 is intentionally stale until explicit refresh'))
# Add complete per-target tasks/logs. Preserve original sample task and log IDs.
tasks=list(c['task_runs']);logs=list(c['execution_logs']);outputs=list(c['execution_outputs']);executions=list(c['executions']);flows=list(c['flows'])
for f in flows:
 f['script']='#!/usr/bin/env bash\nprintf "SIMULATION_ONLY: '+f['id']+'\\n"\n# Offline demo script; never contacts a device.'
 f['failure_policy']='continue';f['script_language']='bash'
# Additional historical runs over stable target scope; no extra hardware.
for i in range(13,49):
 f=flows[(i-13)%len(flows)];target_count=8 if i%3==0 else 16 if i%3==1 else 64;ids=[s['id'] for s in c['servers'][:target_count]];state=['success','partial','failed','warning','result_unknown','running'][i%6];counts=dict(queued=0,running=0,success=0,warning=0,failed=0,killed=0,result_unknown=0)
 if state=='success':counts['success']=target_count
 elif state=='partial':counts['success']=target_count-2;counts['failed']=2
 elif state=='failed':counts['failed']=target_count
 elif state=='warning':counts['success']=target_count-1;counts['warning']=1
 elif state=='result_unknown':counts['success']=target_count-1;counts['result_unknown']=1
 else:counts['running']=target_count
 executions.append(record('exec-'+str(i).zfill(4),flow_id=f['id'],flow_revision=f['revision'],status=state,trigger_source='schedule' if i%2==0 else 'manual',started_by='demo.delivery.engineer',target_snapshot_id='snapshot-inc-'+str(i),target_count=target_count,target_ids=ids,counts=counts,started_at=f'2026-10-29T{8+(i%6):02d}:{i%60:02d}:00Z',finished_at=None if state=='running' else T,duration_ms=1200*(i%20+1),parameter_snapshot_redacted={'execution_mode':'mock','scope':'frozen demo targets'},cmdb_projection_status='pending' if i%7==0 else 'complete',request_id='req-inc-'+str(i)))
for e in executions:
 existing=[t for t in tasks if t['execution_id']==e['id']];used={t['target_id'] for t in existing};ids=e.get('target_ids') or [s['id'] for s in c['servers'][:e['target_count']]];e['target_ids']=ids
 desired=collections.Counter(e['counts']);used_status=collections.Counter(t['status'] for t in existing)
 for target in ids:
  if target in used:continue
  status=next((s for s,n in desired.items() if n>used_status[s]),'success');used_status[status]+=1
  task=record(e['id']+'-target-'+target,execution_id=e['id'],task_key='run_script',target_id=target,status=status,attempt_count=2 if status=='failed' else 1,started_at=e['started_at'],finished_at=e.get('finished_at'),duration_ms=2400,exit_code=1 if status=='failed' else 0 if status=='success' else None,error_summary='Injected mock target failure' if status=='failed' else None,retryable=status=='failed');tasks.append(task)
  for j in range(2):logs.append(record('log-'+task['id']+'-'+str(j),execution_id=e['id'],task_run_id=task['id'],target_id=target,attempt=task['attempt_count'],timestamp=e['started_at'],level='ERROR' if status=='failed' and j==1 else 'WARN' if status in ['warning','result_unknown'] and j==1 else 'INFO',message_redacted='SIMULATION_ONLY '+('target accepted' if j==0 else 'fixed outcome: '+status),cursor='cursor-'+task['id']+'-'+str(j)))
 counts=collections.Counter(t['status'] for t in tasks if t['execution_id']==e['id']);e['counts']={s:counts[s] for s in ['queued','running','success','warning','failed','killed','result_unknown']}
 if not any(o['execution_id']==e['id'] for o in outputs) and e['status'] not in ['running','queued']:
  outputs.append(record('artifact-'+e['id'],execution_id=e['id'],target_id=ids[0],name='mock-result-summary.json',media_type='application/json',artifact_ref='mock://artifacts/'+e['id'],content={'scope':ids,'status':e['status'],'counts':e['counts'],'notice':'SIMULATION_ONLY'},size_bytes=512,sha256='sha256:'+hashlib.sha256(e['id'].encode()).hexdigest(),created_at=T))
out.update(flows=flows,executions=executions,task_runs=tasks,execution_logs=logs,execution_outputs=outputs)
# Operational evidence, rules, versions, alerts and issue history.
for i,p in enumerate(c['parameters'][:5]):
 out['config_sets'].append(record('config-demo-'+str(i+1),name='B300 服务器交付基线 '+str(i+1),status=['draft','validated','approved','executing','verified'][i],target_ids=[s['id'] for s in c['servers'][i*4:i*4+4]],media_refs={'os':c['media'][0]['id']},mapping_revision_id='mapping-rev-demo-v1',created_at=T))
for i,s in enumerate(c['servers'][:24]):out['device_operations'].append(record('device-job-'+str(i+1),operation=['信息采集','PXE装机','固件升级','带外重启'][i%4],target_id=s['id'],status=['queued','running','success','failed'][i%4],created_at=T,media_ref=c['media'][0]['id'],evidence_ref='mock://device-job-'+str(i+1)))
for i,key in enumerate(sorted(set(m['metric_key'] for m in c['metrics']))):out['monitoring_rules'].append(record('monitor-rule-'+str(i),name=key+' 演示监控规则',metric_key=key,operator='<',threshold=90 if i==0 else 1,scope='proj-b300-demo',status='approved',approved_by='demo.sre',notice='Synthetic demo threshold only; not a production rule'))
for i in range(1,8):out['ai_cases'].append(record('case-T'+str(i),issue_type='T'+str(i),title='T'+str(i)+' 离线证据检查建议',summary='建议只根据当前工单的链路、设备和作业证据定位，缺失来源保持 unknown；不执行修复命令。',source_refs=[x['id'] for x in c['issues'] if x['issue_type']=='T'+str(i)],suggestions=['核对影响对象和最近采集时间','对比 BOM 期望与两端采集证据','记录差异并向责任工程师申请定向重验']))
issues=list(c['issues']);alerts=list(c['alerts']);audits=list(c['audit_events']);events=list(c['lifecycle_events'])
for i in range(8,36):
 s=c['servers'][(i*3)%64];closed=i%4==0;issue=record('issue-inc-'+str(i).zfill(3),title=['GPU预收集异常','RoCE链路能力未知','配置基线版本漂移','NCCL测试部分目标失败'][i%4],issue_type='T'+str(i%7+1),status='closed' if closed else 'investigating' if i%3==0 else 'open',severity=['warning','critical','major'][i%3],source_domain='delivery',source_ref='report-'+s['id']+'-G'+str(i%6+1),affected_entities=[s['id']],evidence_refs=['report-'+s['id']+'-G'+str(i%6+1)],assignee_id='demo.delivery.engineer',resolution='模拟修复并完成限定范围重验' if closed else None,created_at=T,updated_at=T,closed_at=T if closed else None);issues.append(issue)
 events.append(record('event-'+issue['id'],entity_id=s['id'],event_type='issue_created',summary=issue['title'],source_domain='issues',source_ref=issue['id'],occurred_at=T,actor=issue['assignee_id']))
 audits.append(record('audit-'+issue['id'],actor=issue['assignee_id'],action='创建合成演示异常工单',resource=issue['id'],timestamp=T,request_id='req-'+issue['id'],before=None,after={'status':issue['status'],'subject':s['id']}))
 if i%2==0:alerts.append(record('alert-inc-'+str(i),rule_id=out['monitoring_rules'][0]['id'],severity=issue['severity'],status='resolved' if closed else 'firing',source='mock-telemetry',subject_ref=s['id'],first_seen_at=T,last_seen_at=T,count=i,summary=issue['title'],evidence_refs=issue['evidence_refs'],issue_id=issue['id']))
out.update(issues=issues,alerts=alerts,audit_events=audits,lifecycle_events=events)
# Fixed scenario patches are fixture data, not component-local business mocks.
for name in ['execution-running','execution-partial','execution-result-unknown','fabric-unknown','cmdb-projection-pending','rack-u-conflict','bom-blocked','bom-unknown-columns']:
 patches=[]
 if name.startswith('execution-'):
  first=executions[0];state={'execution-running':'running','execution-partial':'partial','execution-result-unknown':'result_unknown'}[name];ts=[t for t in tasks if t['execution_id']==first['id']];counts=collections.Counter()
  for i,t in enumerate(ts):
   st=('running' if i<16 else 'queued') if state=='running' else ('failed' if i<2 else 'success') if state=='partial' else ('result_unknown' if i==0 else 'success');counts[st]+=1;patches.append({'resource':'task_runs','id':t['id'],'changes':{'status':st,'retryable':st=='failed','attempt_count':1}})
  patches.append({'resource':'executions','id':first['id'],'changes':{'status':state,'counts':dict(counts),'simulation_step':0,'finished_at':None}})
 elif name=='fabric-unknown':patches=[{'resource':'switches','id':d['id'],'changes':{'health_status':'unknown','freshness':'stale'}} for d in c['switches']]
 elif name=='cmdb-projection-pending':patches=[{'resource':'cmdb_entities','id':d['id'],'changes':{'projection_status':'pending'}} for d in c['cmdb_entities'][:12]]
 elif name=='rack-u-conflict':patches=[{'resource':'rack_placements','id':c['rack_placements'][1]['id'],'changes':{'u_start':c['rack_placements'][0]['u_start']}}]
 elif name.startswith('bom-'):
  patches=[{'resource':'bom_imports','id':c['bom_imports'][0]['id'],'changes':{'status':'review_required'}}]
  if name=='bom-blocked':patches.append({'resource':'bom_issues','id':'bom-issue-002','changes':{'severity':'BLOCK','resolution_status':'open'}})
 out['scenario_profiles'].append(record('scenario-'+name,name=name,patches=patches))

# Project new domain records into CMDB, retaining canonical identity and existing relation records.
entities=list(c['cmdb_entities']);relations=list(c['cmdb_relationships']);known={e['stable_ref'] for e in entities};rid={r['id'] for r in relations}
for key,rows in out.items():
 if key in ['execution_logs','audit_events','lifecycle_events']:continue
 for r in rows:
  if r['id'] not in known:
   entities.append(record('cmdb-'+r['id'],entity_type=key,stable_ref=r['id'],display_name=r.get('name',r.get('title',r['id'])),current_status=r.get('status',r.get('verdict','recorded')),source_event_id='inc-event-'+r['id'],source_refs=[r['id']],freshness='historical_demo',projection_status='complete',created_at=T,updated_at=T));known.add(r['id'])
  parent=r.get('subject_id') or r.get('server_id') or r.get('target_id') or r.get('flow_id')
  if parent:
   rel='inc-rel-'+r['id']
   if rel not in rid:relations.append(record(rel,from_ref=parent,to_ref=r['id'],relation_type='has_evidence' if key in ['step_reports','gate_evidence'] else 'associated_with',source_event_id='inc-event-'+r['id'],valid_from=T,valid_to=None));rid.add(rel)
out['cmdb_entities']=entities;out['cmdb_relationships']=relations
# Every incubated CMDB event has a resolvable lifecycle record.
event_ids={e['id'] for e in events}
for obj in entities+relations:
 event_id=obj.get('source_event_id','')
 if event_id.startswith('inc-event-') and event_id not in event_ids:
  ref=event_id.removeprefix('inc-event-');events.append(record(event_id,entity_id=ref,event_type='synthetic_resource_recorded',summary='合成演示记录与CMDB投影',source_domain='incubation',source_ref=ref,occurred_at=T,actor='demo.sre'));event_ids.add(event_id)
out['lifecycle_events']=events
meta={'dataset_id':'keel-demo-b300-64-incubated-v1','source_dataset_id':base['meta']['dataset_id'],'generated_at':T,'deterministic':True,'hardware_scope_unchanged':True,'mapping_policy':'new GPU/NIC ordinal candidates are explicitly needs_review; LLDP observations are mock evidence','base_sha256':hashlib.sha256((root/'fixtures/demo-64-b300/demo-seed.json').read_bytes()).hexdigest(),'counts':{k:len(v) for k,v in out.items()}}
(root/'fixtures/demo-64-b300/expanded-seed.json').write_text(json.dumps({'meta':meta,'collections':out},ensure_ascii=False,indent=2)+'\n')
print(json.dumps(meta['counts'],ensure_ascii=False))
