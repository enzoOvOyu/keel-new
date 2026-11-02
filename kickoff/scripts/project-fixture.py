"""Project existing synthetic identities into this isolated UI; no real workbook input."""
import hashlib
import json
from pathlib import Path

root = Path(__file__).resolve().parents[2]
folder = root / 'fixtures/demo-full-64-b300'
raw = (folder / 'demo-seed.json').read_bytes()
c = json.loads(raw)['collections']
f = json.loads((folder / 'fabric-studio.json').read_bytes())
ids = {x['id'] for x in f['devices']}
assets = [x for x in c['servers'] + c['switches'] if x['id'] in ids]
rack_ids = {x['rack_id'] for x in assets}
clean = lambda value: value.replace(' (demo)', '').replace(' · DEMO', '')
def evidence(sheet, row, source='source-equipment'):
    return dict(sourceId=source, sheet=sheet, cell=f'A{row}:H{row}', method='source')

devices = [dict(id=x['id'], name=x['hostname'].split('.')[0], serial=x['serial'],
    type=x['category'], model=clean(x['model']), rackId=x['rack_id'],
    startU=x['rack_u'], heightU=x['u_height'], role=x['role'],
    managementAddress=x['management_address'], evidence=evidence('设备清单', i+2))
    for i, x in enumerate(assets)]
ports = [dict(id=x['id'], deviceId=x['device_id'], name=x['physical_name'],
    mac=x.get('mac'), speed=x.get('speed', ''), role=x.get('network_role', x.get('role', '')),
    componentId=x.get('component_id'), evidence=evidence('端口清单', i+2))
    for i, x in enumerate(c['ports'] + c['switch_ports']) if x['device_id'] in ids]
port_ids = {x['id'] for x in ports}
connections = [dict(id=x['id'], a=x['endpoint_a_uid'], b=x['endpoint_b_uid'],
    medium=x['sub_type'], evidence=evidence('双端连接', i+2, 'source-connectivity'))
    for i, x in enumerate(c['cables']) if x['endpoint_a_uid'] in port_ids and x['endpoint_b_uid'] in port_ids]
components = [dict(id=x['id'], deviceId=x['server_id'], type=x['category'],
    name=x['id'].split('-', 2)[-1].upper(), model=clean(x['model']), slot=x.get('pci_bdf', ''),
    evidence=evidence('组件清单', i+2)) for i, x in enumerate(c['gpus'] + c['nics']) if x['server_id'] in ids]
racks = [dict(id=x['id'], name=x['name'], room='hall-a', capacityU=x['u_height'],
    evidence=evidence('机柜清单', i+2, 'source-facility')) for i, x in enumerate(c['racks']) if x['id'] in rack_ids]
time = '2026-11-02T09:12:00Z'
sources = [
 dict(id='source-equipment', name='设备与组件清单.xlsx', size='186 KB', version=1, state='recognized', hash=hashlib.sha256(raw).hexdigest(), createdAt=time, originalAvailable=False, regions=[
  dict(id='region-devices',sheet='设备清单',range=f'A1:H{len(devices)+1}',category='设备身份',rowCount=len(devices),state='extracted',note='设备身份与型号已关联'),
  dict(id='region-components',sheet='组件清单',range=f'A1:H{len(components)+1}',category='组件关系',rowCount=len(components),state='extracted',note='父设备与模块关系已保留'),
  dict(id='region-ports',sheet='端口清单',range=f'A1:H{len(ports)+1}',category='端口',rowCount=len(ports),state='extracted',note='MAC缺失不阻断明确端口身份')]),
 dict(id='source-connectivity',name='设备端接与线缆.xlsx',size='94 KB',version=1,state='recognized',hash=hashlib.sha256(b'kickoff-connectivity-v1'+raw).hexdigest(),createdAt=time,originalAvailable=False,regions=[
  dict(id='region-links',sheet='双端连接',range=f'A1:H{len(connections)+1}',category='物理连接',rowCount=len(connections),state='extracted',note='两端设备与端口按稳定引用关联')]),
 dict(id='source-facility',name='机柜布局与位置.xlsx',size='38 KB',version=1,state='recognized',hash=hashlib.sha256(b'kickoff-facility-v1'+raw).hexdigest(),createdAt=time,originalAvailable=False,regions=[
  dict(id='region-racks',sheet='机柜清单',range=f'A1:H{len(racks)+1}',category='机房机柜',rowCount=len(racks),state='extracted',note='已识别机柜归属，空间坐标可后补'),
  dict(id='region-placement',sheet='设备落位',range=f'A1:H{len(devices)+1}',category='落位',rowCount=len(devices),state='needs_input',note='一台设备高度待核对；不影响设备身份入库')]),
]
models = []
def model(table, rows):
    models.append(dict(tableId=table, rows=rows, version=1, confirmed=True, updatedAt=time, source='资料提取'))
model('rooms', [dict(room_id='hall-a', name='Hall A', site='', address='', frame='', unit='')])
model('racks', [dict(rack_id=x['id'],room_id=x['room'],capacity_u=str(x['capacityU']),width_mm='',depth_mm='',x='',y='',orientation='',frame='',unit='') for x in racks])
model('equipment_specs',[dict(model=name,height_u=str(next(x['heightU'] for x in devices if x['model']==name)),width_mm='',depth_mm='',face='',panel_ref='') for name in sorted({x['model'] for x in devices})])
model('placements',[dict(device_id=x['id'],rack_id=x['rackId'],start_u=str(x['startU']),height_u='' if x['id']=='srv-0008' else str(x['heightU']),face='',position='') for x in devices])
model('management',[dict(device_id=x['id'],management_ip=x['managementAddress'],bmc_ip='',management_port='',credential_ref='',method='') for x in devices])
# Explicit planning references from the existing Fabric fixture, not inferred links.
model('network_planes',[dict(fabric=f['fabricId'],plane=x['id'],role=x['role'],architecture='Spine–Leaf',profile='') for x in f['planes']])
model('network_roles',[dict(device_id=x['id'],fabric=f['fabricId'],plane=p,role=x['role']) for x in f['devices'] for p in x['planeIds']])
model('power_profiles',[dict(device_id=x['id'],power_kw=str(x['power_kw']),basis='planned',cooling='',source='设备清单规划功率') for x in assets if x.get('power_kw')])
model('rack_capacity',[dict(rack_id=x['id'],capacity_kw=str(x['power_capacity_kw']),cooling='',cooling_kw='',source='机柜规划容量') for x in c['racks'] if x['id'] in rack_ids])
issues = [
 dict(id='issue-identity',scope='identity',title='leaf-08 身份关联待确认',description='来源名称已匹配到既有设备，需要确认沿用该设备身份。',entityId='sw-leaf-0008',field='identity',impact='仅影响此设备及关联端口、连接',resolved=False,evidence=evidence('设备清单',21)),
 dict(id='issue-placement',scope='placement',title='b300-008 占用高度待核对',description='已识别机柜与起始U；可采用明确匹配的型号高度。',entityId='srv-0008',field='heightU',impact='只影响该设备的精确占位',resolved=False,evidence=evidence('设备落位',9,'source-facility')),
 dict(id='issue-routing',scope='configuration',title='布线路径与余量规则待补充',description='物理连接已完整。计算线长时再补充路径、机柜接入和规则。',entityId='cabling',field='route',impact='只影响线长计算，不阻断入库',resolved=False,evidence=evidence('双端连接',2,'source-connectivity')),
]
for d in devices:
    if d['id']=='srv-0008': d['heightU']=None
state = dict(schemaVersion=1,projectId='proj-b300-demo',projectName='B300 集群交付',revision=1,
    devices=devices,ports=ports,connections=connections,components=components,racks=racks,sources=sources,
    issues=issues,models=models,calculations=[],publications=[],activity=[dict(id='activity-initial',action='资料识别结果已就绪',detail='设备、组件、端口和连接已完成来源关联',at=time)])
target=root/'kickoff/src/lib/api/fixture.json'
target.write_text(json.dumps(state,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({k:len(state[k]) for k in ['devices','ports','connections','components','racks']},ensure_ascii=False))
