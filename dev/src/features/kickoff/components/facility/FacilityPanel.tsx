import {useMemo,useState} from 'react';
import {useQuery,keepPreviousData} from '@tanstack/react-query';
import {identitySnapshot} from '../../../../lib/api/identityBridge';
import {useWorkspace} from '../../context';
import {infrastructureApi,type InfraAsset} from '../../../../lib/api/infrastructure';
import {geometry} from '../../../infrastructure/approved/rack/model';
import {layoutRack} from '../../../infrastructure/approved/rack/geometry';
import {Button,DataTable,Notice,Section} from '../ui';
import {ModelEditor} from '../ModelEditor';
import {modelRows} from './FacilityEntities';
import type {ModelRow} from '../../lib/api';
import {TopologyPrerequisites} from './TopologyPrerequisites';
import './facility.css';

type Gap={id:string;name:string;table:'racks'|'placements'|'power_profiles'|'rack_capacity';field:string;message:string};
const number=(value:unknown)=>value===null||value===undefined||value===''||!Number.isFinite(Number(value))?null:Number(value);
export function FacilityPanel(_: {activeRack:string|null;selectRack:(id:string|null)=>void;edit:(table:string)=>void}){
 const {state:s,can,go}=useWorkspace();
 const [editor,setEditor]=useState<{rack?:string;table:string;view?:'global'|'rack'}>();
 const publicationKey=s.publications.map(p=>p.id+':'+p.state).join('|');
 const identity=identitySnapshot();
 const checkKey=['kickoff-facility-check',identity?.user.id,identity?.permission_revision,s.projectId,s.revision,publicationKey];
 const query=useQuery({queryKey:checkKey,staleTime:Infinity,gcTime:30*60*1000,refetchOnMount:false,refetchOnWindowFocus:false,placeholderData:keepPreviousData,
  queryFn:async({signal})=>{const all=new Map<string,InfraAsset>();let cursor:string|undefined,snapshot:string|undefined,incomplete=false;
   do{const page=await infrastructureApi.subgraph(s.projectId,{view:'hierarchy',depth:1,limit:200,cursor,snapshot_id:snapshot},signal);snapshot=page.snapshot_id;for(const asset of page.assets)all.set(asset.id,asset);incomplete ||= page.truncated;cursor=page.page.has_more?page.page.next_cursor??undefined:undefined;if(page.page.has_more&&!cursor)throw Error('CMDB 分页未完成，请重新检查。');}while(cursor);
   return {assets:[...all.values()],partial:incomplete,snapshot:snapshot??'',checkedAt:Date.now()};
  }});
 const assets=query.data?.assets??[],partial=query.data?.partial??false,snapshot=query.data?.snapshot??'';
 const refresh=()=>{void query.refetch()};
 const check=useMemo(()=>{
  const racks=assets.filter(a=>a.type==='rack'),rooms=assets.filter(a=>a.type==='pod');
  const devices=assets.filter(a=>['server','switch','device','pdu','patch_panel'].includes(a.type));
  const gaps:Gap[]=[];
  const add=(a:InfraAsset,table:Gap['table'],field:string,message:string)=>gaps.push({id:a.id,name:a.name,table,field,message});
  for(const rack of racks){
   if(!rooms.some(r=>r.id===rack.fields.pod_id))add(rack,'racks','room_id','尚未关联机房');
   const capacity=number(rack.fields.u_height),items=devices.filter(d=>d.fields.rack_id===rack.id);
   const powerCapacity=number(rack.fields.planned_power_capacity_kw??rack.fields.power_capacity_kw);
   if(powerCapacity===null||powerCapacity<=0)add(rack,'rack_capacity','capacity_kw','缺少机柜可用供电容量（kW）');
   const powers=items.map(d=>number(d.fields.planned_power_kw));
   if(powerCapacity!==null&&powers.every(n=>n!==null)&&powers.reduce<number>((sum,n)=>sum+(n??0),0)>powerCapacity)add(rack,'rack_capacity','capacity_kw','规划设备负荷超过机柜供电容量，需核对容量或调整设备');
   if(capacity===null||capacity<=0)add(rack,'racks','capacity_u','机柜总 U 高未明确');
   else for(const issue of layoutRack(capacity,items.map(d=>geometry(d)),number(rack.fields.depth_mm)).issues){for(const id of issue.ids){const d=items.find(d=>d.id===id);if(d)add(d,'placements',issue.kind==='missing'?'start_u':'placement',issue.message)}}
  }
  for(const d of devices){if(number(d.fields.planned_power_kw)===null)add(d,'power_profiles','power_kw','缺少设备规划功率依据（kW），不能按零计算');if(!racks.some(r=>r.id===d.fields.rack_id))add(d,'placements','rack_id','设备尚未关联机柜');const g=geometry(d);if(g.height===null||g.height<0||!Number.isInteger(g.height*2))add(d,'placements','height_u','设备占用高度未明确');if(g.height!==0&&(g.u===null||g.u<1||!Number.isInteger(g.u*2)))add(d,'placements','start_u','设备起始 U 位未明确');}
  return {racks,rooms,devices,gaps:[...new Map(gaps.map(g=>[g.id+g.field+g.message,g])).values()]};
 },[assets]);
 const draft=(g:Gap)=>{
  if(g.table==='power_profiles')return modelRows(s,'power_profiles').find(r=>r.device_id===g.id&&r.basis==='planned'&&r.power_kw!=='')?.power_kw;
  if(g.table==='rack_capacity')return modelRows(s,'rack_capacity').find(r=>r.rack_id===g.id)?.capacity_kw;
  if(g.table==='racks'){const r=s.racks.find(r=>r.id===g.id);const row=modelRows(s,'racks').find(r=>r.rack_id===g.id);return g.field==='room_id'?row?.room_id||r?.room:g.field==='capacity_u'?row?.capacity_u||r?.capacityU:null;}
  const d=s.devices.find(d=>d.id===g.id);return g.field==='rack_id'?d?.rackId:g.field==='height_u'?d?.heightU:g.field==='start_u'?d?.startU:null;
 };
 const rackGaps=(id?:string)=>{if(!id)return check.gaps;const ids=new Set([id,...check.devices.filter(d=>d.fields.rack_id===id).map(d=>d.id)]);return check.gaps.filter(g=>ids.has(g.id))};
 const viewGaps=(id?:string,view?:'global'|'rack')=>rackGaps(id).filter(g=>view!=='global'||['room_id','rack_id'].includes(g.field));
 const tablesFor=(id?:string,view?:'global'|'rack')=>{const gaps=viewGaps(id,view);const tables=[...new Set(gaps.map(g=>g.table))] as string[];if(gaps.some(g=>g.field==='room_id'))tables.unshift('rooms');return tables};
 const open=(rack?:string,view?:'global'|'rack')=>{const tables=tablesFor(rack,view);if(tables.length)setEditor({rack,view,table:tables[0]})};
 const editorInput=()=>{
  const table=editor!.table,ids=[...new Set(viewGaps(editor!.rack,editor!.view).filter(g=>g.table===table).map(g=>g.id))];
  const key=['racks','rack_capacity'].includes(table)?'rack_id':'device_id';const rows=structuredClone(modelRows(s,table));
  for(const id of ids){if(rows.some(r=>r[key]===id&&(table!=='power_profiles'||r.basis==='planned')))continue;const a=assets.find(a=>a.id===id)!;const text=(v:unknown)=>v==null?'':String(v);
   const row:ModelRow=table==='racks'?{rack_id:id,room_id:text(a.fields.pod_id),capacity_u:text(a.fields.u_height),width_mm:text(a.fields.width_mm),depth_mm:text(a.fields.depth_mm)}:table==='rack_capacity'?{rack_id:id,capacity_kw:text(a.fields.planned_power_capacity_kw??a.fields.power_capacity_kw)}:table==='power_profiles'?{device_id:id,power_kw:text(a.fields.planned_power_kw),basis:'planned',source:''}:{device_id:id,rack_id:text(a.fields.rack_id),start_u:text(a.fields.u_start??a.fields.rack_u),height_u:text(a.fields.u_height),face:text(a.fields.mounting_face??a.fields.face)};
   rows.push(row);
  }
  return {initialRows:rows,targetIds:table==='rooms'?undefined:ids};
 };
 const labels:Record<string,string>={rooms:'机房信息',racks:'机柜关联与规格',placements:'设备落位',power_profiles:'设备规划功率',rack_capacity:'机柜供电容量'};
 return <Section title="已入库信息 · 拓扑资料补齐" caption="对照数据中心拓扑需要的已入库信息，仅补充实际缺项。未入库的工程输入不计作完成。" action={<Button disabled={query.isFetching} onClick={refresh}>{query.isFetching?'检查中…':'刷新检查'}</Button>}>
  {!query.data&&query.isPending?<p className="facility-relation-summary" role="status">正在检查已入库关联与机柜图所需信息…</p>:!query.data&&query.error?<Notice tone="error">{query.error.message}<Button onClick={refresh}>重新检查</Button></Notice>:<>
   <div className="facility-relation-summary"><span>{check.rooms.length} 个机房</span><span>{check.racks.length} 个已入库机柜</span><span>{check.devices.length} 台已入库设备</span><span>{query.isFetching?'正在更新检查':'检查于 '+new Date(query.dataUpdatedAt).toLocaleTimeString()}</span></div>
   {query.error&&<Notice tone="error">检查更新失败，当前显示上次结果：{query.error.message}</Notice>}
   <p className="muted small-copy">供电检查采用已入库规划功率和机柜容量；实测功率不替代规划依据。完整可安装容量还依赖深度、承重等约束，不能仅凭剩余U位或功率判断。</p>
   {partial&&<Notice tone="warning">CMDB 返回的数据不完整，暂不能判定全部满足。<Button onClick={refresh}>重新检查</Button></Notice>}
   <TopologyPrerequisites key={snapshot+':'+query.data?.checkedAt} cacheRevision={query.data?.checkedAt??0} assets={assets} snapshot={snapshot} rackGaps={check.gaps.length} partial={partial} refresh={refresh} supplement={view=>open(undefined,view)}/>
   {!check.racks.length&&!check.devices.length&&<p className="facility-relation-summary">尚无已入库的机柜或设备；先在入库与版本确认已核对的数据。</p>}
   <DataTable data={check.racks} pageSize={5} columns={[{accessorKey:'name',header:'已入库机柜'},{id:'room',header:'关联机房',cell:({row})=>check.rooms.find(r=>r.id===row.original.fields.pod_id)?.name||'未关联'},{id:'u',header:'容量 U',cell:({row})=>row.original.fields.u_height??'未提供'},{id:'devices',header:'关联设备',cell:({row})=>check.devices.filter(d=>d.fields.rack_id===row.original.id).length},{id:'status',header:'质量状态',cell:({row})=>{const ids=new Set([row.original.id,...check.devices.filter(d=>d.fields.rack_id===row.original.id).map(d=>d.id)]);const n=check.gaps.filter(g=>ids.has(g.id)).length;return n?`${n} 项待补充或核对`:partial?'待完成检查':'已满足'}}]}/>
   {!!check.gaps.length&&<div className="facility-inline-gaps"><b>空间与供电资料缺项 · {check.gaps.length} 项</b><DataTable data={check.gaps} pageSize={10} columns={[{accessorKey:'name',header:'受影响对象'},{accessorKey:'message',header:'缺失或冲突信息'},{id:'draft',header:'补录进度',cell:({row})=>draft(row.original)?'工程输入已有候选值 · 待核对入库':'待填写'}]}/><div className="actions"><Button onClick={()=>go('publish')}>核对补录并入库</Button></div><p className="muted small-copy">补录窗口支持在线表格编辑和 XLSX / CSV 导入；确认入库后重新检查，不把未入库的候选值当成已满足。</p></div>}
  </>}
  {editor&&<ModelEditor key={editor.table} keepOpenOnSave fieldKeys={editor.view==='global'?(editor.table==='placements'?['device_id','rack_id']:editor.table==='racks'?['rack_id','room_id']:['room_id','name']):undefined} tableId={editor.table} title={(editor.rack?check.racks.find(r=>r.id===editor.rack)?.name:editor.view==='global'?'全局拓扑':'机柜与机房')+' · 补充信息'} {...editorInput()} relatedTables={tablesFor(editor.rack,editor.view).map(id=>({id,label:labels[id]}))} onTableChange={table=>setEditor({...editor,table})} entityChoices={{room_id:check.rooms.map(a=>({id:a.id,label:a.name})),rack_id:check.racks.map(a=>({id:a.id,label:a.name})),device_id:check.devices.map(a=>({id:a.id,label:a.name}))}} close={()=>setEditor(undefined)}/>}
 </Section>;
}
