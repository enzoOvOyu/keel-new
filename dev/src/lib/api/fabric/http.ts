import {bomRequest} from '../realBom';
import type {components} from '../../schemas/openapi';
import type {IdentitySnapshot} from '../identityBridge';
import {FabricProvider,initialState} from './mock';
import {setIdentity} from './provider';
import type {FabricFixture,Role} from './types';
type InputPage=components['schemas']['FabricInputPage'];
export class HttpFabricProvider extends FabricProvider {
 readonly mode='real' as const;
 require(_roles:Role[]):never {throw Error('当前真实切片仅提供已发布输入与CMDB回执读取；配置编译、审批、下发及回采尚未接入，不执行模拟写入。')}
 static async load(project:string,session:IdentitySnapshot,signal:AbortSignal){
  const page=await bomRequest<InputPage>('projects/'+encodeURIComponent(project)+'/fabric/inputs','GET',undefined,undefined,signal);
  const current=page.inputs[0];
  const fixture:FabricFixture={schemaVersion:1,projectId:project,fabricId:'fabric-workspace:'+project,fabricName:'Fabric',sourceHash:current?.digest||'',kickoffSourceHash:current?.digest||'',devices:[],interfaces:[],physicalLinks:[],logicalLinks:[],planes:[],snapshots:[],capabilities:[],profiles:[],ztpProfiles:[],tasks:[],actors:[],sources:[],switchDomains:[],hostDomains:[],rules:[],initialUnenrolled:[]};
  fixture.snapshots=page.inputs.map(input=>({id:input.id,projectId:project,kickoffPublishId:input.id,cmdbRevisionId:'asset-vector:'+input.digest,connectivityPlanRevisionId:input.modelRevisionId,networkPlanRevisionId:input.networkPlans[0]?.revision_id||'',deviceSetHash:input.digest,sourceHash:input.digest,publishedAt:input.publishedAt,freshnessState:input.missingAssetIds.length?'BLOCKED':'READY',missingFields:input.missingAssetIds.map(id=>'CMDB回执缺失:'+id),changedDeviceIds:[],description:'Kickoff 已发布修订 '+input.revision,parameters:{}}));
  if(current){
   const assets=current.assets;const ids=new Set(assets.filter(a=>['switch','server','device'].includes(a.type)).map(a=>a.id));
   fixture.devices=assets.filter(a=>ids.has(a.id)).map(a=>({id:a.id,name:a.name,kind:a.type==='switch'?'switch':'host',role:String(a.fields.role||''),model:String(a.fields.model||''),rackId:String(a.fields.rack_id||a.fields.rack_uid||''),serial:String(a.fields.serial||''),managementAddress:String(a.fields.management_address||''),planeIds:[],maintenanceState:String(a.fields.maintenance_state||''),sourceRef:a.source}));
   fixture.interfaces=assets.filter(a=>a.type==='port').map(a=>({id:a.id,deviceId:String(a.fields.device_id||a.fields.parent_uid||''),name:String(a.fields.physical_name||a.name),speed:String(a.fields.speed||''),mac:a.fields.mac?String(a.fields.mac):undefined,planeIds:[],sourceRef:a.source}));
   const ports=new Map(fixture.interfaces.map(p=>[p.id,p]));
   fixture.physicalLinks=assets.filter(a=>a.type==='cable').flatMap(a=>{const pa=ports.get(String(a.fields.endpoint_a_uid||'')),pb=ports.get(String(a.fields.endpoint_b_uid||''));return pa&&pb?[{id:a.id,a:pa.id,b:pb.id,deviceA:pa.deviceId,deviceB:pb.deviceId,medium:String(a.fields.medium||a.fields.media_type||''),sourceRef:a.source,lldp:'NOT_RUN' as const}]:[]});
   for(const plan of current.networkPlans){
    for(const plane of plan.planes){fixture.planes.push({id:plane.id,name:plane.name,role:plane.profile||'',deviceIds:plan.device_roles.filter(r=>r.plane_id===plane.id).map(r=>r.equipment_id),asnRange:'',addressRange:'',redundancyDomain:'',sourceRef:plan.revision_id})}
    for(const role of plan.device_roles){const device=fixture.devices.find(d=>d.id===role.equipment_id);if(device){device.role=role.role;if(!device.planeIds.includes(role.plane_id))device.planeIds.push(role.plane_id)}}
    const logicalPorts=new Map(plan.interface_mappings.map(m=>[m.id,m]));
    for(const mapping of plan.interface_mappings){const port=ports.get(mapping.physical_port_id);if(port&&!port.planeIds.includes(mapping.plane_id))port.planeIds.push(mapping.plane_id)}
    for(const bgp of plan.bgp){const ma=logicalPorts.get(bgp.a_interface_id),mb=logicalPorts.get(bgp.z_interface_id),a=ma&&ports.get(ma.physical_port_id),b=mb&&ports.get(mb.physical_port_id);if(a&&b)fixture.logicalLinks.push({id:bgp.id,fromDeviceId:a.deviceId,toDeviceId:b.deviceId,planeId:ma?.plane_id||'',protocol:'BGP',sourceRef:plan.revision_id})}
   }
   fixture.sources=[{id:current.id,kind:'kickoff-publication',revision:String(current.revision),path:current.modelRevisionId,label:'已发布CMDB范围'}];
  }
  const state=initialState(fixture);state.enrollments.forEach(e=>{e.status='UNKNOWN';e.matchedDeviceId=undefined;e.evidenceRefs=[];e.timeline=[]});setIdentity(fixture,state,project,session);
  const provider=new HttpFabricProvider(fixture,state);provider.dependency=page.message;return provider;
 }
 dependency='';
}
