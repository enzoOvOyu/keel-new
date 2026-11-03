import {bomRequest} from '../realBom';
import type {components} from '../../schemas/openapi';
import type {IdentitySnapshot} from '../identityBridge';
import type * as C from './types';
type InputPage=components['schemas']['FabricInputPage'];
type Envelope=components['schemas']['FabricWorkspace'];
export const last=<T,>(values:T[])=>values[values.length-1];
function projectData(page:InputPage,state:C.FabricState,tasks:C.AutomationTaskCatalogDTO[],project:string,session:IdentitySnapshot):C.FabricFixture {
  const current=page.inputs.find(i=>i.id===state.inputId)||page.inputs[0];
  const fixture:C.FabricFixture={schemaVersion:1,projectId:project,fabricId:'fabric-workspace:'+project,fabricName:'Fabric',sourceHash:current?.digest||'',kickoffSourceHash:current?.digest||'',devices:[],interfaces:[],physicalLinks:[],logicalLinks:[],planes:[],snapshots:[],capabilities:[],profiles:[],ztpProfiles:[],tasks:tasks,actors:[],sources:[],switchDomains:[],hostDomains:[],rules:[],initialUnenrolled:[]};
  fixture.snapshots=page.inputs.map(input=>({id:input.id,projectId:project,kickoffPublishId:input.id,cmdbRevisionId:'asset-vector:'+input.digest,connectivityPlanRevisionId:input.modelRevisionId,networkPlanRevisionId:input.networkPlans[0]?.revision_id||'',deviceSetHash:input.digest,sourceHash:input.digest,publishedAt:input.publishedAt,freshnessState:input.missingAssetIds.length?'BLOCKED':'READY',missingFields:input.missingAssetIds.map(id=>'CMDB回执缺失:'+id),changedDeviceIds:[],description:'Kickoff 已发布修订 '+input.revision,parameters:{}}));
  if(current){
   const assets=current.assets;const ids=new Set(assets.filter(a=>['switch','server'].includes(a.type)).map(a=>a.id));
   fixture.devices=assets.filter(a=>ids.has(a.id)).map(a=>({id:a.id,name:a.name,kind:a.type==='switch'?'switch':'host',role:String(a.fields.role||''),model:String(a.fields.model||''),rackId:String(a.fields.rack_id||a.fields.rack_uid||''),serial:String(a.fields.serial||''),managementAddress:String(a.fields.management_address||''),planeIds:[],maintenanceState:String(a.fields.maintenance_state||''),sourceRef:a.source}));
   fixture.devices.sort((a,b)=>a.kind===b.kind?a.name.localeCompare(b.name):a.kind==='switch'?-1:1);
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

 const roles=[...(session.global_roles||[]),...(session.project_roles[project]||[])];
 const role=(roles.includes('admin')?'admin':roles[0]||'viewer') as C.Role;
 fixture.actors=[{id:session.user.id,name:session.user.display_name,role,projectIds:[project]}];
 fixture.rules=['Input','Capability','Physical','Logical','QoS / RoCE','Operational Health','Change Safety'];
 return fixture;
}
export class HttpFabricProvider {
 readonly mode:'real'='real'; state:C.FabricState; fixture:C.FabricFixture; revision:number;
 private listeners=new Set<()=>void>(); private pending=false;
 constructor(private project:string,private session:IdentitySnapshot,private envelope:Envelope){this.state=envelope.state;this.revision=envelope.revision;this.fixture=projectData(envelope.inputs,this.state,envelope.tasks,project,session)}
 static async load(project:string,session:IdentitySnapshot,signal:AbortSignal){return new HttpFabricProvider(project,session,await bomRequest<Envelope>('projects/'+encodeURIComponent(project)+'/fabric/workspace','GET',undefined,undefined,signal))}
 subscribe=(fn:()=>void)=>{this.listeners.add(fn);return()=>{this.listeners.delete(fn)}};
 snapshot=()=>this.state;
 private apply(e:Envelope){this.envelope=e;this.revision=e.revision;this.state=e.state;this.fixture=projectData(e.inputs,e.state,e.tasks,this.project,this.session);this.listeners.forEach(fn=>fn())}
 async refresh(){if(this.pending)return this.envelope;const e=await bomRequest<Envelope>('projects/'+encodeURIComponent(this.project)+'/fabric/workspace');this.apply(e);return e}
 private async command(action:string,values:Record<string,unknown>={}){if(this.pending)throw Error('当前操作正在提交，请等待结果');this.pending=true;try{const e=await bomRequest<Envelope>('projects/'+encodeURIComponent(this.project)+'/fabric/commands','POST',{action,expectedRevision:this.revision,...values});this.apply(e);return e.resultId}finally{this.pending=false}}
 enrollment(deviceId:string):C.ZtpEnrollmentDTO{return this.state.enrollments.find(e=>e.deviceId===deviceId)||{id:deviceId,deviceId,profileId:'',profileRevision:'',bootstrapArtifactId:'',discoveredIdentity:{},status:'UNKNOWN',evidenceRefs:[],timeline:[],attempt:0}}
 get actor(){return this.fixture.actors[0]}
 get input(){return this.fixture.snapshots.find(i=>i.id===this.state.inputId)}
 get blueprint(){return last(this.state.blueprints)}
 get build(){return last(this.state.builds)}
 get validation(){return this.state.validations.slice().reverse().find(v=>v.configBuildId===this.build?.id)}
 get profiles(){return [...this.fixture.profiles,...this.state.profileRevisions]}
 get dependency(){return this.envelope.inputs.message}
 get automation(){return {catalog:()=>this.fixture.tasks}}
 adopt(id:string){return this.command('adopt',{id})}
 commitBlueprint(){return this.command('commitBlueprint')}
 reviseBlueprint(){return this.command('reviseBlueprint')}
 revertBlueprint(){return this.command('revertBlueprint')}
 compile(domain:C.Domain,ids:string[],profile?:string){return this.command('compile',{domain,ids,profile})}
 validate(_scenario?:string){return this.command('validate')}
 createChange(task:string,strategy:C.RolloutPlanDTO['strategy'],purpose:string,window:string,acknowledgement:string,ids?:string[]){return this.command('createChange',{task,strategy,purpose,window,acknowledgement,ids})}
 submit(id:string){return this.command('submit',{id})}
 async approve(id:string,decision:'APPROVED'|'REJECTED',reason:string){const change=this.state.changes.find(c=>c.id===id);if(!change?.approvalId)throw Error('变更尚未提交统一审批');const roles=this.session.project_roles[this.project]||[];const group=roles.includes('network_engineer')?'NE':roles.includes('delivery_engineer')?'DE':null;if(!group)throw Error('网络变更需要具备项目 NE 或 DE 签署资格；管理员不能代签');await bomRequest('projects/'+encodeURIComponent(this.project)+'/approvals/'+encodeURIComponent(change.approvalId)+'/sign','POST',{group,decision:decision==='APPROVED'?'approve':'reject',reason});return this.command('approve',{id})}
 deploy(id:string,_scenario?:string){return this.command('deploy',{id})}
 advance(id:string){return this.command('advance',{id})}
 control(id:string,control:'pause'|'resume'|'cancel'){return this.command('control',{id,control})}
 match(id:string){return this.command('match',{id})}
 startZtp(ids:string[],task:string,_scenario?:string){return this.command('startZtp',{ids,task})}
 finishZtp(id:string){return this.command('finishZtp',{id})}
 collect(_scenario?:string){return this.command('collect')}
 ready(){return this.command('ready')}
 toggleProvider(){return this.refresh()}
 publishProfile(id:string,reason:string){return this.command('publishProfile',{id,reason})}
 plan(ids:string[],strategy:C.RolloutPlanDTO['strategy'],taskId:string):C.RolloutPlanDTO {const safe=strategy==='SERIAL'||ids.length<=1;const task=this.fixture.tasks.find(t=>t.id===taskId);return {id:'preview',strategy,waves:ids.map((id,i)=>({id:'preview-'+i,sequence:i+1,targetDeviceIds:[id],taskBindings:[taskId],maxConcurrency:1,executionRefs:[],dependsOn:i?['preview-'+(i-1)]:[]})),maxConcurrency:1,timeoutSeconds:task?.timeoutSeconds||0,retryPolicyRef:task?.retryPolicyRef||'',safe,warnings:safe?[]:['尚无已审批冗余关系，多个目标请使用 Serial。']}}
}
