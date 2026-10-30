"""Repair synthetic observation references; preserve imported records and explicit mismatch scenarios."""
import json,copy
from pathlib import Path
root=Path(__file__).resolve().parents[1];p=root/'fixtures/demo-full-64-b300/demo-seed.json';d=json.loads(p.read_text());c=d['collections'];source=c['lldp_neighbors'];all_ports={x['id']:x for x in c['ports']+c['switch_ports']};servers={x['id']:x for x in c['servers']};switches={x['id']:x for x in c['switches']};roce=[p for p in c['ports'] if p['network_role']=='roce'];roce.sort(key=lambda x:x['id']);assert len(roce)==len(source)==512
faults=[i for i,r in enumerate(source) if r['match_status']=='mismatch'];report={'source_records':len(source),'canonical_roce_ports':len(roce),'source_bom_uid_mismatches':sum(all_ports[r['remote_port_id']].get('peer_port_id')!=r['local_port_id'] for r in source),'explicit_mismatch_scenarios_retained':len(faults),'method':'Keep confirmed BOM unchanged. Normalize synthetic LLDP to the 512 RoCE ports. Preserve 28 explicit fault scenarios as a permutation of existing physical endpoints. Keep immutable source_original on each record.'};(root/'docs/IA24-LLDP-NORMALIZATION.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
fixed=[]
for i,planned in enumerate(roce):
 imported=source[i];local=all_ports[planned['peer_port_id']];remote=roce[faults[(faults.index(i)+1)%len(faults)]] if i in faults else planned;device=servers[remote['device_id']];switch=switches[local['device_id']]
 fixed.append({'id':imported['id'],'project_id':planned['project_id'],'local_switch_id':switch['id'],'local_port_id':local['id'],'local_port_name':local['physical_name'],'remote_device_id':device['id'],'remote_device_name':device['hostname'],'remote_port_id':remote['id'],'remote_port_name':remote['physical_name'],'chassis_id':device['serial'],'port_id':remote['physical_name'],'sys_name':device['hostname'],'sys_desc':imported['sys_desc'],'observed_at':imported['observed_at'],'freshness':imported['freshness'],'source':'synthetic_lldp_normalized_fixture_v1','match_status':'mismatch' if i in faults else 'matched','source_original':copy.deepcopy(imported.get('source_original',imported)),'normalization_rule':'confirmed_bom_roce_links_with_preserved_fault_permutation_v1','is_demo':True})
c['lldp_neighbors']=fixed
for n in c['server_lldp_observations']:
 peer=next((r for r in fixed if r['remote_port_id']==n['port_id']),None)
 if peer:n.update(peer_port_id=peer['local_port_id'],freshness=peer['freshness'],observed_at=peer['observed_at'],source='synthetic_dual_lldp_fixture_v1',evidence_ref=peer['id'])
d['meta']['counts']={k:len(v) for k,v in c.items()};d['meta']['collection_counts']=d['meta']['counts'];p.write_text(json.dumps(d,ensure_ascii=False,indent=2)+'\n');print(report)
