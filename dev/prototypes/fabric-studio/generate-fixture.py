"""Read-only projection of shared synthetic identities plus explicit Fabric DEMO design."""
import json, hashlib
from pathlib import Path
root=Path(__file__).resolve().parents[3]
p=root/'fixtures/demo-full-64-b300'
raw=(p/'demo-seed.json').read_bytes(); c=json.loads(raw)['collections']; kickoff=(p/'kickoff-studio.json').read_bytes()
selected=c['switches'][:8]+[s for s in c['switches'] if s['role']=='spine']+c['servers'][:8]
ids={s['id'] for s in selected}; planes=[]
for i,label in enumerate(['Compute A','Compute B','Compute C']):
 planes.append(dict(id=f'plane-{i}',name=f'Plane {i}',role=label,deviceIds=sorted(ids),asnRange='65100–65119 · DEMO',addressRange='192.0.2.0/24 · TEST-NET',redundancyDomain=f'rd-{i}',sourceRef=f'np:planes/{i}'))
devices=[dict(id=s['id'],name=s['hostname'].split('.')[0],kind='host' if s['category']=='server' else 'switch',role='host' if s['category']=='server' else s['role'],model=s['model'],rackId=s['rack_id'],serial=s['serial'],managementAddress=s['management_address'],planeIds=[x['id'] for x in planes],maintenanceState=s['maintenance_state'],sourceRef='cmdb:'+s['id']) for s in selected]
ports=[dict(id=v['id'],deviceId=v['device_id'],name=v['physical_name'],speed=v['speed'],mac=v.get('mac'),planeIds=[f'plane-{(v["port_index"]-1)%3}'],sourceRef='cmdb:'+v['id']) for v in c['ports']+c['switch_ports'] if v['device_id'] in ids]
pi={p['id']:p for p in ports}
links=[dict(id=l['id'],a=l['endpoint_a_uid'],b=l['endpoint_b_uid'],deviceA=pi[l['endpoint_a_uid']]['deviceId'],deviceB=pi[l['endpoint_b_uid']]['deviceId'],medium=l['sub_type'],sourceRef='connectivity:'+l['id'],lldp='NOT_RUN') for l in c['cables'] if l['endpoint_a_uid'] in pi and l['endpoint_b_uid'] in pi]
# Explicit synthetic logical design, never derived from physical cables.
logical=[dict(id=f'logical-{i}-{j}',fromDeviceId=f'sw-leaf-{j:04}',toDeviceId=f'sw-spine-{i%4+1:04}',planeId=f'plane-{i}',protocol='eBGP',sourceRef=f'np:bgp/{i}/{j}') for i in range(3) for j in range(1,9)]
sources=[dict(id='kickoff:publish-demo-7',kind='Kickoff Published',revision='7',path='publish/network-planning',label='合成发布记录 · 非真实智能开局发布'),dict(id='profiles:v3',kind='Intent Profile',revision='3.2',path='profiles/roce-lossless',label='演示批准策略'),dict(id='capability:v1',kind='Capability',revision='demo-v1',path='capability/synthetic',label='模拟能力，未核验生产型号')]
for d in devices: sources.append(dict(id=d['sourceRef'],kind='CMDB projection',revision='cmdb-demo-12',path='collections/'+('servers' if d['kind']=='host' else 'switches')+'/'+d['id'],label=d['name']))
for pl in planes:sources.append(dict(id=pl['sourceRef'],kind='NetworkPlan',revision='np-demo-7',path='planes/'+pl['id'],label=pl['name']+' · 显式合成逻辑分区；非实测硬件隔离'))
params={}
for i,d in enumerate(devices):
 ref='np:device/'+d['id'];sources.append(dict(id=ref,kind='NetworkPlan',revision='np-demo-7',path='intent/'+d['id'],label=d['name']+' · 合成参数'))
 params[d['id']]=dict(asn=65100+i,address=f'192.0.2.{10+i}/32',pfc=3,ecn='roce-ecn-v2',bgpPeers=[dict(deviceId='sw-spine-0001',asn=65108,address='192.0.2.18')] if d['role']=='leaf' else [],vrf='DEMO-COMPUTE',vni=10101 if d['role']=='leaf' else None,sourceRefs=[ref,d['sourceRef'],'profiles:v3'])
snaps=[]
for v in [7,8]:
 ps=json.loads(json.dumps(params))
 if v==8:
  ps['sw-leaf-0001']['ecn']='roce-ecn-v3'
  ps['sw-leaf-0001']['sourceRefs'][0]='np:device/sw-leaf-0001@8'
  sources.append(dict(id='np:device/sw-leaf-0001@8',kind='NetworkPlan',revision='np-demo-8',path='intent/sw-leaf-0001/ecn',label='leaf-01 · 合成ECN修订'))
 snaps.append(dict(id=f'input-demo-{v}',projectId='proj-b300-demo',kickoffPublishId=f'publish-demo-{v}',cmdbRevisionId='cmdb-demo-12',connectivityPlanRevisionId='cable-demo-5',networkPlanRevisionId=f'np-demo-{v}',deviceSetHash=hashlib.sha256(','.join(sorted(ids)).encode()).hexdigest(),sourceHash=hashlib.sha256(json.dumps(ps,sort_keys=True).encode()).hexdigest(),publishedAt=f'2026-11-02T0{v-2}:00:00Z',freshnessState='READY',missingFields=[],changedDeviceIds=['sw-leaf-0001'] if v==8 else [],description='ECN 策略调整 · leaf-01' if v==8 else '三逻辑 Plane · 两层 Spine–Leaf · 显式合成发布',parameters=ps))
incomplete=json.loads(json.dumps(snaps[0]));incomplete.update(id='input-demo-missing',networkPlanRevisionId='np-demo-6',freshnessState='NEEDS_INPUT',missingFields=['leaf-01 · local ASN'],description='缺少ASN · 不能采用');incomplete['parameters']['sw-leaf-0001']['asn']=None;snaps.append(incomplete)
switch_domains=['System / Management','Interface','Port Breakout','IP / MAC','RoCE Lossless','QoS / Traffic Pool','PFC','ECN','Adaptive Routing','BGP','BGP Neighbor / Peer Group','Route Distribution','BFD','WECMP','BGP DC PIC','BGP-LLDP Redistribution','EVPN','VXLAN / NVE','Tenant VRF','VNI / RD']
host_domains=['DOCA','NCCL Net Plugin','NIC Firmware','NIC Port Split','Port Rename','eSwitch','PF / VF','VF Representor','DOCA X-Plane','GDR','PFC / ECN','Adaptive Routing','SPCX-CC','IPG']
capabilities=[dict(deviceId=d['id'],manufacturer='NVIDIA · DEMO',model=d['model'],nos='Linux' if d['kind']=='host' else 'Cumulus Linux',nosVersion='DEMO capability 5.18' if d['kind']=='switch' else 'DEMO Linux',nvueVersion='5.18-demo' if d['kind']=='switch' else None,firmwareVersion='fixture-v1',supportedFeatures=['BGP','PFC','ECN','Interface','BFD','RoCE Lossless'] if d['kind']=='switch' else ['DOCA','PFC / ECN','NIC Firmware'],unsupportedFeatures=['WECMP','BGP DC PIC','SPCX-CC','Hardware Multiplane','DOCA X-Plane'],breakoutProfiles=['1x800G','4x200G'],capabilityRevision='cap-demo-1',sourceRef='capability:v1') for d in devices]
profiles=[]
for domain,role,name,features in [('SWITCH',['leaf'],'RoCE Leaf',['BGP','PFC','ECN','BFD']),('SWITCH',['spine'],'Fabric Spine',['BGP','ECN','BFD']),('HOST',['host'],'Host Network',['DOCA','PFC / ECN','NIC Firmware']),('MULTIPLANE',['host'],'Hardware Multiplane',['Hardware Multiplane','DOCA X-Plane'])]:
 profiles.append(dict(id='profile-'+role[0] if domain!='MULTIPLANE' else 'profile-dms',name=name,version='3.2',domain=domain,roleSelector=role,policyRefs=['policy-roce-3'],capabilityConstraints=features,schemaVersion='intent/v1',status='PUBLISHED',features=features,description='版本化合成策略 · 能力不足时阻断'))
tasks=[]
for key,name,kinds,tag,rollback in [('ztp','Cumulus Day-0 enrollment',['switch'],'ztp',False),('switch','NVUE staged apply',['switch'],'switch.apply',True),('host','Host network configure',['host'],'host.apply',False),('dms','DMS prepare / configure',['host'],'dms.apply',False)]:
 tasks.append(dict(id='mock-task-'+key,provider='MOCK',namespace='demo.fabric',flowId='demo-'+key,flowRevision=3,name=name,inputSchemaVersion='v1',inputSchema=[dict(name=n,type='string[]' if n=='target_ids' else 'string',required=True) for n in ['target_ids','artifact_ref','source_snapshot','approval_ref']],outputSchema=['applied_ref','verification_ref','task_log'],supportedTargetKinds=kinds,capabilityTags=[tag],supportsDryRun=True,supportsRollback=rollback,supportsPause=True,supportsCancel=True,retryPolicyRef='failed-targets-new-approval',concurrencyPolicy='reviewed-redundancy-aware',available=key!='dms',source='MOCK_CATALOG'))
actors=[dict(id=role,name=name,role=role,projectIds=['proj-b300-demo']) for role,name in [('network_engineer','陈工 · 网络工程师'),('delivery_lead','林工 · 交付负责人'),('delivery_engineer','周工 · 交付工程师'),('viewer','访客 · 只读'),('developer_sre','沈工 · SRE'),('admin','管理员')]]
data=dict(schemaVersion=1,projectId='proj-b300-demo',fabricId='fabric-demo-roce',fabricName='Spectrum-X · Fabric A',sourceHash=hashlib.sha256(raw).hexdigest(),kickoffSourceHash=hashlib.sha256(kickoff).hexdigest(),devices=devices,interfaces=ports,physicalLinks=links,logicalLinks=logical,planes=planes,snapshots=snaps,capabilities=capabilities,profiles=profiles,ztpProfiles=[dict(id='ztp-cumulus',name='Cumulus 5.18 · Secure bootstrap',revision='2.1',nos='Cumulus Linux',method='DHCP Option 239 · Cumulus 5.18',bootstrapUrl='https://bootstrap.demo.keel.invalid/cumulus/v2.sh',scriptRevision='sha256:demo-bootstrap-v2',osImage='cumulus-demo-image-ref',secretRef='secret-ref://demo/bootstrap',expires='15 min · DEMO policy',cleanup='Revoke bootstrap reference after completion')],tasks=tasks,actors=actors,sources=sources,switchDomains=switch_domains,hostDomains=host_domains,rules=['Input','Capability','Physical','Logical','QoS / RoCE','Operational Health','Change Safety'],initialUnenrolled=['sw-leaf-0007','sw-leaf-0008'])
(p/'fabric-studio.json').write_text(json.dumps(data,ensure_ascii=False,indent=2)+'\n')
print(f'Generated {len(devices)} referenced devices, {len(ports)} ports, {len(links)} physical cables, {len(planes)} explicit DEMO planes')
