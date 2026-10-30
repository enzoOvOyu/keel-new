"""Only deterministic mock extensions of supplied IA 2.4 data; never overwrite supplied facts."""
import json
from pathlib import Path
p=Path(__file__).resolve().parents[1]/'fixtures/demo-full-64-b300/demo-seed.json';d=json.loads(p.read_text());c=d['collections'];stamp='2026-10-30T00:00:00Z';project='proj-b300-demo'
def row(id,**kw):return dict(id=id,project_id=project,execution_mode='mock',source='full_demo_deterministic_extension_v1',revision=1,**kw)
ports={r['id']:r for r in c['ports']+c['switch_ports']}
if not c.get('server_lldp_observations'):
 c['server_lldp_observations']=[row('server-lldp-'+x['id'],server_id=x['device_id'],port_id=x['id'],peer_port_id=x.get('peer_port_id') if x['oper_state']=='up' else None,observed_at=stamp,freshness='stale',notice='Explicit synthetic collector result based on confirmed port fixture; not a real observation') for x in c['ports'] if x.get('peer_port_id')]
if not c['network_assertions']:
 labels=['统一BOM两端建模','ZTP与Golden无Critical漂移','网络授时能力','双侧LLDP线序证据','网络无损基线一致性','网络测试前置健康']
 for i,label in enumerate(labels,1):c['network_assertions'].append(row('np-full-'+str(i),code='NP-'+str(i),name=label,verdict='BLOCKED',collected_at=stamp,source_refs=['network-baseline-demo-v1'],valid_for_seconds=30,notice='Fresh mock collection and domain precheck required; never default to PASS'))
# Supplemental per-device reports use disclosed scenarios, not unprovided real benchmark results.
existing={(r['subject_id'],r['gate_id']) for r in c['step_reports']}
for i,s in enumerate(c['servers']):
 for gate in ['G1','G2','G3','G4','G5','G6']:
  if (s['id'],gate) in existing:continue
  actual=next((r for r in c['server_observed_configs'] if r['server_id']==s['id']),None)
  verdict='BLOCKED' if gate=='G3' or s['lifecycle_status']=='maintained' else 'FAIL' if s['health_status']=='critical' or (gate=='G4' and actual and actual['diff_status']!='IN_SYNC') else 'PASS'
  metrics={'observed_gpu_count':sum(g['server_id']==s['id'] for g in c['gpus']),'observed_nic_count':sum(g['server_id']==s['id'] for g in c['nics'])} if gate=='G1' else {'scenario_index':i,'evidence_kind':'synthetic_profile'}
  c['step_reports'].append(row('full-report-'+s['id']+'-'+gate,subject_id=s['id'],gate_id=gate,name={'G1':'硬件预收集','G2':'三方线序模拟','G3':'人工Mapping确认','G4':'配置基线比较','G5':'RoCE模拟场景','G6':'NCCL模拟场景'}[gate],verdict=verdict,metrics=metrics,test_scope='single_device',collected_at=stamp,valid_until='2026-10-31T00:00:00Z',rule_versions=['synthetic-demo-scenario-v1'],source_refs=[s['id'],s['source_bom_item_id']],notice='Synthetic scenario only. G5/G6 values are not real benchmark evidence. Gate prerequisites still enforced.'))
# ConfigSets reference existing approved source versions and media; do not invent network parameters.
if not c['config_sets']:
 approved=[m for m in c['media'] if m['approval_state']=='approved' and m['retirement_state']=='active' and m['type']=='os-image']
 if approved:
  for i in range(4):c['config_sets'].append(row('full-configset-'+str(i+1),name='B300 批次配置 '+str(i+1),status='draft',target_ids=[s['id'] for s in c['servers'][i*4:i*4+4]],media_refs={'os':approved[0]['id']},mapping_revision_id=c['mapping_revisions'][0]['id'],created_at=stamp))
# Project missing source evidence as CMDB links; metric points stay in their dedicated collection.
ids={e['stable_ref'] for e in c['cmdb_entities']};relations={r['id'] for r in c['cmdb_relationships']}
for key,ref in [('server_runtime','server_id'),('bmc_inventory','server_id'),('firmware_inventory','device_id'),('server_config_baselines','server_id'),('server_observed_configs','server_id'),('switch_runtime','switch_id'),('switch_config_baselines','switch_id'),('switch_running_configs','switch_id'),('switch_config_diffs','switch_id'),('collection_jobs','target_id')]:
 for item in c[key]:
  if item['id'] not in ids:c['cmdb_entities'].append(row('cmdb-full-'+item['id'],stable_ref=item['id'],entity_type=key,display_name=item['id'],current_status=item.get('status',item.get('compliance','recorded')),source_refs=[item['id']],source_event_id='seed-full-projection',projection_status='complete',freshness=item.get('freshness','unknown'),created_at=stamp));ids.add(item['id'])
  rid='rel-full-'+item['id']
  if rid not in relations:c['cmdb_relationships'].append(row(rid,from_ref=item[ref],to_ref=item['id'],relation_type='HAS_'+key.upper(),source_event_id='seed-full-projection',valid_from=stamp,valid_to=None));relations.add(rid)
d['meta']['counts']={k:len(v) for k,v in c.items()};d['meta']['collection_counts']=d['meta']['counts'];p.write_text(json.dumps(d,ensure_ascii=False,indent=2)+'\n');print({k:len(c[k]) for k in ['step_reports','server_lldp_observations','config_sets','cmdb_entities','cmdb_relationships']})
