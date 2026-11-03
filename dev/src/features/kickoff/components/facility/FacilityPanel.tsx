import {useEffect,useMemo,useState} from 'react';
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

type Gap={id:string;name:string;table:'racks'|'placements';field:string;message:string};
const number=(value:unknown)=>value===null||value===undefined||value===''||!Number.isFinite(Number(value))?null:Number(value);
export function FacilityPanel(_: {activeRack:string|null;selectRack:(id:string|null)=>void;edit:(table:string)=>void}){
 const {state:s,can,go}=useWorkspace();
 const [assets,setAssets]=useState<InfraAsset[]>([]),[loading,setLoading]=useState(true),[error,setError]=useState(''),[partial,setPartial]=useState(false),[retry,setRetry]=useState(0);
 const [snapshot,setSnapshot]=useState('');
 const [editor,setEditor]=useState<{table:string;ids?:string[];rows:ModelRow[]}>();
 const publicationKey=s.publications.map(p=>p.id+':'+p.state).join('|');
 useEffect(()=>{const controller=new AbortController();setLoading(true);setError('');setAssets([]);setPartial(false);
  void(async()=>{const all=new Map<string,InfraAsset>();let cursor:string|undefined,snapshot:string|undefined,incomplete=false;
   do{const page=await infrastructureApi.subgraph(s.projectId,{view:'hierarchy',depth:1,limit:200,cursor,snapshot_id:snapshot},controller.signal);snapshot=page.snapshot_id;for(const asset of page.assets)all.set(asset.id,asset);incomplete ||= page.truncated;cursor=page.page.has_more?page.page.next_cursor??undefined:undefined;if(page.page.has_more&&!cursor)throw Error('CMDB 分页未完成，请重新检查。');}while(cursor);
   if(!controller.signal.aborted){setAssets([...all.values()]);setPartial(incomplete);setSnapshot(snapshot??'')}
  })().catch(e=>{if(!controller.signal.aborted)setError(e.message)}).finally(()=>{if(!controller.signal.aborted)setLoading(false)});
  return()=>controller.abort();
 },[s.projectId,publicationKey,retry]);
 const check=useMemo(()=>{
  const racks=assets.filter(a=>a.type==='rack'),rooms=assets.filter(a=>a.type==='pod');
  const devices=assets.filter(a=>['server','switch','device','pdu','patch_panel'].includes(a.type));
  const gaps:Gap[]=[];
  const add=(a:InfraAsset,table:Gap['table'],field:string,message:string)=>gaps.push({id:a.id,name:a.name,table,field,message});
  for(const rack of racks){
   if(!rooms.some(r=>r.id===rack.fields.pod_id))add(rack,'racks','room_id','尚未关联机房');
   const capacity=number(rack.fields.u_height),items=devices.filter(d=>d.fields.rack_id===rack.id);
   if(capacity===null||capacity<=0)add(rack,'racks','capacity_u','机柜总 U 高未明确');
   else for(const issue of layoutRack(capacity,items.map(d=>geometry(d)),number(rack.fields.depth_mm)).issues){for(const id of issue.ids){const d=items.find(d=>d.id===id);if(d)add(d,'placements',issue.kind==='missing'?'start_u':'placement',issue.message)}}
  }
  for(const d of devices){if(!racks.some(r=>r.id===d.fields.rack_id))add(d,'placements','rack_id','设备尚未关联机柜');const g=geometry(d);if(g.height===null||g.height<0||!Number.isInteger(g.height*2))add(d,'placements','height_u','设备占用高度未明确');if(g.height!==0&&(g.u===null||g.u<1||!Number.isInteger(g.u*2)))add(d,'placements','start_u','设备起始 U 位未明确');}
  return {racks,rooms,devices,gaps:[...new Map(gaps.map(g=>[g.id+g.field+g.message,g])).values()]};
 },[assets]);
 const draft=(g:Gap)=>{
  if(g.table==='racks'){const r=s.racks.find(r=>r.id===g.id);const row=modelRows(s,'racks').find(r=>r.rack_id===g.id);return g.field==='room_id'?row?.room_id||r?.room:g.field==='capacity_u'?row?.capacity_u||r?.capacityU:null;}
  const d=s.devices.find(d=>d.id===g.id);return g.field==='rack_id'?d?.rackId:g.field==='height_u'?d?.heightU:g.field==='start_u'?d?.startU:null;
 };
 const open=(table:'racks'|'placements')=>{
  const ids=[...new Set(check.gaps.filter(g=>g.table===table).map(g=>g.id))];
  const key=table==='racks'?'rack_id':'device_id';const rows=structuredClone(modelRows(s,table));
  for(const id of ids){if(rows.some(r=>r[key]===id))continue;const a=assets.find(a=>a.id===id)!;const text=(v:unknown)=>v==null?'':String(v);
   rows.push(table==='racks'?{rack_id:id,room_id:text(a.fields.pod_id),capacity_u:text(a.fields.u_height),width_mm:text(a.fields.width_mm),depth_mm:text(a.fields.depth_mm)}:{device_id:id,rack_id:text(a.fields.rack_id),start_u:text(a.fields.u_start??a.fields.rack_u),height_u:text(a.fields.u_height),face:text(a.fields.mounting_face??a.fields.face)});
  }
  setEditor({table,ids,rows});
 };
 return <Section title="已入库信息 · 拓扑资料补齐" caption="对照数据中心拓扑需要的已入库信息，仅补充实际缺项。未入库的工程输入不计作完成。" action={<Button onClick={()=>setRetry(n=>n+1)}>刷新检查</Button>}>
  {loading?<p className="facility-relation-summary" role="status">正在检查已入库关联与机柜图所需信息…</p>:error?<Notice tone="error">{error}<Button onClick={()=>setRetry(n=>n+1)}>重新检查</Button></Notice>:<>
   <div className="facility-relation-summary"><span>{check.rooms.length} 个机房</span><span>{check.racks.length} 个已入库机柜</span><span>{check.devices.length} 台已入库设备</span></div>
   {partial&&<Notice tone="warning">CMDB 返回的数据不完整，暂不能判定全部满足。<Button onClick={()=>setRetry(n=>n+1)}>重新检查</Button></Notice>}
   <TopologyPrerequisites key={s.projectId+':'+snapshot+':'+retry} assets={assets} snapshot={snapshot} rackGaps={check.gaps.length} partial={partial} refresh={()=>setRetry(n=>n+1)}/>
   {!check.racks.length&&!check.devices.length&&<p className="facility-relation-summary">尚无已入库的机柜或设备；先在入库与版本确认已核对的数据。</p>}
   <DataTable data={check.racks} pageSize={5} columns={[{accessorKey:'name',header:'已入库机柜'},{id:'room',header:'关联机房',cell:({row})=>check.rooms.find(r=>r.id===row.original.fields.pod_id)?.name||'未关联'},{id:'u',header:'容量 U',cell:({row})=>row.original.fields.u_height??'未提供'},{id:'devices',header:'关联设备',cell:({row})=>check.devices.filter(d=>d.fields.rack_id===row.original.id).length},{id:'status',header:'机柜图信息',cell:({row})=>{const ids=new Set([row.original.id,...check.devices.filter(d=>d.fields.rack_id===row.original.id).map(d=>d.id)]);const n=check.gaps.filter(g=>ids.has(g.id)).length;return n?`${n} 项待补充或核对`:partial?'待完成检查':'已满足'}}]}/>
   {!!check.gaps.length&&<div className="facility-inline-gaps"><b>与机柜图要求的差距 · {check.gaps.length} 项</b><DataTable data={check.gaps} pageSize={10} columns={[{accessorKey:'name',header:'受影响对象'},{accessorKey:'message',header:'缺失或冲突信息'},{id:'draft',header:'补录进度',cell:({row})=>draft(row.original)?'工程输入已有候选值 · 待核对入库':'待填写'}]}/><div className="actions">{check.gaps.some(g=>g.field==='room_id')&&!modelRows(s,'rooms').length&&<Button disabled={!can('bom:edit')} onClick={()=>setEditor({table:'rooms',rows:[]})}>补充机房</Button>}{check.gaps.some(g=>g.table==='racks')&&<Button disabled={!can('bom:edit')} onClick={()=>open('racks')}>补录机房关联 / 机柜信息</Button>}{check.gaps.some(g=>g.table==='placements')&&<Button disabled={!can('bom:edit')} onClick={()=>open('placements')}>补录设备落位信息</Button>}<Button onClick={()=>go('publish')}>核对补录并入库</Button></div><p className="muted small-copy">补录窗口支持在线表格编辑和 XLSX / CSV 导入；确认入库后重新检查，不把未入库的候选值当成已满足。</p></div>}
  </>}
  {editor&&<ModelEditor tableId={editor.table} initialRows={editor.rows} targetIds={editor.ids} entityChoices={{room_id:check.rooms.map(a=>({id:a.id,label:a.name})),rack_id:check.racks.map(a=>({id:a.id,label:a.name})),device_id:check.devices.map(a=>({id:a.id,label:a.name}))}} close={()=>setEditor(undefined)}/>}
 </Section>;
}
