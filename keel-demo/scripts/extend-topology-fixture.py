"""Add explicitly synthetic, versioned logical intent/observation. Not inferred from cables."""
import json
from pathlib import Path
p=Path('fixtures/demo-full-64-b300/demo-seed.json');d=json.loads(p.read_text());c=d['collections'];pid='proj-b300-demo';stamp='2026-10-30T00:00:00Z'
meta={'project_id':pid,'source':'explicit_synthetic_topology_v1','observed_at':stamp,'freshness':'fresh','support':'supported','revision':1,'execution_mode':'mock','is_demo':True}
c['topology_fabrics']=[dict(meta,id='fabric-demo-roce',name='B300 Demo RoCE Fabric',baseline_id='netbase-demo-v1',pod_ids=[f'demo-pod-{i}' for i in range(1,5)])]
c['topology_networks']=[];c['topology_memberships']=[];c['topology_observed_networks']=[];c['topology_bgp_sessions']=[];c['topology_snapshots']=[]
for pod in range(1,5):
 for rail in range(1,9):
  netid=f'network-pod-{pod}-rail-{rail}';c['topology_networks'].append(dict(meta,id=netid,name=f'Pod {pod} / Rail {rail}',fabric_id='fabric-demo-roce',pod_id=f'demo-pod-{pod}',rail=f'rail-{rail}',network_role='compute',vlan_id=[100,110,120,130][(rail-1)%4],vrf=f'demo-compute-pod-{pod}',vni=10000+pod*100+rail,baseline_id='netbase-demo-v1',approval_state='approved_demo_only',planning_source='explicit_mock_plan_not_physical_inference'))
for pod in range(1,5):
 for role,vlan in [('storage',130),('management',100)]:
  c['topology_networks'].append(dict(meta,id=f'network-pod-{pod}-{role}',name=f'Pod {pod} / {role}',fabric_id='fabric-demo-roce',pod_id=f'demo-pod-{pod}',rail=role,network_role=role,vlan_id=vlan,vrf=f'demo-{role}-pod-{pod}',vni=20000+pod*100+(1 if role=='storage' else 2),baseline_id='netbase-demo-v1',approval_state='approved_demo_only',planning_source='explicit_mock_plan_not_physical_inference'))
for i,sw in enumerate(c['switches']):
 pod=(i%16)//4+1 if sw['role']=='leaf' else i%4+1
 for net in c['topology_networks']:
  if sw['role']=='leaf' and net['pod_id']!=f'demo-pod-{pod}':continue
  ports=[p for p in c['switch_ports'] if p['device_id']==sw['id'] and ((p['port_index']-1)%8==int(net['rail'].split('-')[1])-1 if net['rail'].startswith('rail-') else p.get('role')=='server-downlink')]
  base=next(b for b in c['switch_config_baselines'] if b['switch_id']==sw['id']);run=next(r for r in c['switch_running_configs'] if r['switch_id']==sw['id'])
  mid=f'membership-{sw["id"]}-{net["id"]}';c['topology_memberships'].append(dict(meta,id=mid,network_id=net['id'],device_id=sw['id'],port_ids=[p['id'] for p in ports],fabric_id=net['fabric_id'],pod_id=net['pod_id'],rail=net['rail'],baseline_id=base['id'],asn=base['asn'],vlan_id=net['vlan_id'],vrf=net['vrf'],vni=net['vni'],pfc_priorities=base['pfc_priorities'],ecn_profile=base['ecn_profile']))
  # Deliberately leave rail8 planned-only; rail7 has an explicit unsupported/unknown sample.
  if net['rail']=='rail-8':continue
  idx=len(c['topology_observed_networks']);c['topology_observed_networks'].append(dict(meta,id='observed-'+mid,membership_id=mid,network_id=net['id'],device_id=sw['id'],running_config_id=run['id'],vlan_id=net['vlan_id'] if idx%19 else net['vlan_id']+1,vrf=net['vrf'],vni=None if net['rail']=='rail-7' else net['vni'],asn=run['asn'],pfc_priorities=run['pfc_priorities'],ecn_profile=run['ecn_profile'],freshness='stale' if idx%23==5 else 'fresh',support='unknown' if net['rail']=='rail-7' else 'supported'))
leaves=[s for s in c['switches'] if s['role']=='leaf'];spines=[s for s in c['switches'] if s['role']=='spine']
for i,leaf in enumerate(leaves):
 for j,spine in enumerate(spines):
  lp=[p for p in c['switch_ports'] if p['device_id']==leaf['id'] and p.get('role')=='fabric-uplink'];sp=[p for p in c['switch_ports'] if p['device_id']==spine['id']]
  c['topology_bgp_sessions'].append(dict(meta,id=f'topology-bgp-{i}-{j}',fabric_id='fabric-demo-roce',pod_id=f'demo-pod-{i//4+1}',source_device_id=leaf['id'],target_device_id=spine['id'],source_port_id=lp[j%len(lp)]['id'] if lp else None,target_port_id=sp[(i+j)%len(sp)]['id'],local_asn=next(b['asn'] for b in c['switch_config_baselines'] if b['switch_id']==leaf['id']),remote_asn=next(b['asn'] for b in c['switch_config_baselines'] if b['switch_id']==spine['id']),state='active' if i==0 and j==0 else 'unknown' if i==2 and j==1 else 'established',baseline_id='netbase-demo-v1',planned=True,evidence_class='explicit_mock_bgp_observation',freshness='stale' if i==4 and j==0 else 'fresh'))
c['topology_snapshots']=[dict(meta,id='topology-snapshot-demo-20261030',captured_at=stamp,snapshot_kind='logical_plan_observation',network_ids=[n['id'] for n in c['topology_networks']],description='Initial deterministic synthetic snapshot; older times unavailable')]
p.write_text(json.dumps(d,ensure_ascii=False,indent=2)+'\n')
# Explicit server network membership uses the fixture's declared NIC ordinal and role plan,
# never an inferred VLAN from a physical cable. Asset/port IDs remain unchanged.
for srv in c['servers']:
 pod=next(r['pod_id'] for r in c['racks'] if r['id']==srv['rack_id'])
 for ni in range(1,9):
  net=next(n for n in c['topology_networks'] if n['pod_id']==pod and n['rail']==f'rail-{ni}')
  port=next(p for p in c['ports'] if p['device_id']==srv['id'] and p['component_id']==f'{srv["id"]}-nic-{ni:02d}')
  c['topology_memberships'].append(dict(meta,id=f'membership-{srv["id"]}-{ni}',network_id=net['id'],device_id=srv['id'],port_ids=[port['id']],fabric_id=net['fabric_id'],pod_id=pod,rail=net['rail'],baseline_id='netbase-demo-v1',asn=None,vlan_id=net['vlan_id'],vrf=net['vrf'],vni=net['vni'],pfc_priorities=[3],ecn_profile='roce-ecn-v2'))
# Normalize synthetic dual-sided LLDP against the previously normalized switch evidence;
# keep originals and all observed endpoint faults. Do not alter confirmed Cable endpoints.
rep=[]
for r in c['server_lldp_observations']:
 actual=next((l for l in c['lldp_neighbors'] if l['remote_port_id']==r['port_id']),None)
 if not actual:continue
 if r.get('peer_port_id')!=actual['local_port_id']:rep.append(r['id'])
 r.setdefault('source_original',{k:v for k,v in r.items() if k!='source_original'});r.update(peer_port_id=actual['local_port_id'],evidence_ref=actual['id'],source='synthetic_dual_lldp_normalized_v2',freshness=actual['freshness'],support='supported')
c['topology_port_observations']=[]
for i,port in enumerate(c['ports']):
 c['topology_port_observations'].append(dict(meta,id='topology-port-observed-'+port['id'],physical_port_id=port['id'],device_id=port['device_id'],oper_state=port['oper_state'],speed='400G' if i%101==7 and port['speed']=='800G' else port['speed'],source='explicit_synthetic_server_interface_v1',freshness='stale' if i%41==9 else 'fresh'))
keys=['servers','switches','racks','ports','switch_ports','cables','switch_interfaces','lldp_neighbors','server_lldp_observations','ddm_samples','topology_networks','topology_memberships','topology_observed_networks','topology_bgp_sessions','topology_fabrics','topology_port_observations']
c['topology_snapshots'][0]['data']={key:[{k:v for k,v in row.items() if k!='source_original'} for row in c[key]] for key in keys}
Path('docs/TOPOLOGY-DATA-NORMALIZATION.json').write_text(json.dumps({'source':'synthetic_demo_only','changed_server_lldp_refs':rep,'rule':'switch-normalized-observation reverse relation, preserve endpoint mismatch and source_original','cable_endpoints_changed':False,'verification':'NOT RUN'},ensure_ascii=False,indent=2)+'\n')
p.write_text(json.dumps(d,ensure_ascii=False,indent=2)+'\n')
