import {useState} from 'react';
import {useWorkspace} from '../../context';
import {Button,DataTable,Field,Modal,Notice} from '../ui';
import {EntityField,entityOptions,filled,modelRows,resolveEntity} from './FacilityEntities';
import {FacilityBatchEditor} from './FacilityBatchEditor';
import type {ModelRow} from '../../lib/api';

type Preview={rows:ModelRow[];items:{id:string;name:string;position:string;before:string;after:string;changed:boolean}[];revision:number};
export function FacilityLayoutEditor({rackIds,close}:{rackIds:string[];close:()=>void}){
 const {state:s,api,run,can}=useWorkspace();
 const [values,setValues]=useState<Record<string,string>>({});
 const [order,setOrder]=useState(rackIds),[overwrite,setOverwrite]=useState(false),[advanced,setAdvanced]=useState(false);
 const [preview,setPreview]=useState<Preview>(),[error,setError]=useState(''),[saving,setSaving]=useState(false);
 const racks=order.map(id=>s.racks.find(r=>r.id===id)!).filter(Boolean);
 const rooms=entityOptions(s,'room_id'),rackOptions=entityOptions(s,'rack_id');
 const change=(key:string,value:string)=>{setValues(v=>({...v,[key]:value}));setPreview(undefined);setError('')};
 const input=(key:string,label:string,hint?:string)=><Field label={label} hint={hint}><input type="number" min="0" step="any" value={values[key]??''} onChange={e=>change(key,e.target.value)}/></Field>;
 const build=()=>{setError('');try{
  if(!racks.length)throw Error('请先选择机柜。');
  if(!rooms.some(r=>r.id===values.room))throw Error('请选择机房。');
  const number=(key:string,label:string,positive=false)=>{if(!filled(values[key])||!Number.isFinite(Number(values[key]))||Number(values[key])<0||positive&&Number(values[key])===0)throw Error(`请填写${label}${positive?'（大于0）':'（可为0）'}。`);return Number(values[key])};
  const perRow=number('perRow','每排机柜数',true);if(!Number.isInteger(perRow))throw Error('每排机柜数必须是整数。');
  const width=number('width','机柜宽度',true),depth=number('depth','机柜深度',true);
  const gap=number('gap','柜间净距'),aisle=racks.length>perRow?number('aisle','排间通道宽度'):0;
  const left=number('left','首柜左侧距机房左边界'),front=number('front','首排前沿距机房前边界');
  if(!['0','180'].includes(values.orientation))throw Error('请选择机柜正面朝向。');
  const rows=structuredClone(modelRows(s,'racks'));
  const frame=values.room+':room-front-left';
  const items=racks.map((rack,index)=>{
   const matches=rows.filter(r=>resolveEntity(rackOptions,r.rack_id)===rack.id);if(matches.length>1)throw Error(`${rack.name}存在重复机柜输入，请先合并。`);
   const row=matches[0]??{rack_id:rack.id};
   const room=resolveEntity(rooms,row.room_id||rack.room);
   if(room&&room!==values.room)throw Error(`${rack.name}属于其他机房，请先核对归属或调整选择。`);
   const line=Math.floor(index/perRow),column=index%perRow;
   const hasPosition=filled(row.x)||filled(row.y);
   const before=hasPosition?'已有平面位置':'尚未定位';
   if(hasPosition&&!overwrite)return {id:rack.id,name:rack.name,position:`第 ${line+1} 排 · 第 ${column+1} 位`,before,after:'保留原位置',changed:false};
   if(!overwrite&&((filled(row.width_mm)&&Number(row.width_mm)!==width)||(filled(row.depth_mm)&&Number(row.depth_mm)!==depth)))throw Error(`${rack.name}已有尺寸与本次规格不同，请分组设置或明确允许覆盖。`);
   const next={...row,room_id:values.room,width_mm:String(width),depth_mm:String(depth),x:String(left+column*(width+gap)+width/2),y:String(front+line*(depth+aisle)+depth/2),frame,unit:'mm',orientation:values.orientation};
   if(matches.length)rows[rows.indexOf(row)]=next;else rows.push(next);
   return {id:rack.id,name:rack.name,position:`第 ${line+1} 排 · 第 ${column+1} 位`,before,after:`距左边界 ${left+column*(width+gap)} mm · 距前边界 ${front+line*(depth+aisle)} mm`,changed:JSON.stringify(next)!==JSON.stringify(row)};
  });
  setPreview({rows,items,revision:s.revision});
 }catch(e){setError((e as Error).message);setPreview(undefined)}};
 if(advanced)return <FacilityBatchEditor mode="coordinates" rackIds={order} close={()=>setAdvanced(false)}/>;
 return <Modal title="设置机房布局" description="按实际排布生成机柜平面位置；尺寸与间距使用毫米，可先预览再保存。" wide close={()=>{if(!saving)close()}}>
  <fieldset className="facility-batch-fields" disabled={saving}>
   <Field label="所属机房"><EntityField label="布局所属机房" value={values.room??''} options={rooms} onChange={v=>change('room',v)}/></Field>
   {input('perRow','每排机柜数')}{input('width','机柜宽度（mm）')}{input('depth','机柜深度（mm）')}
   {input('gap','相邻机柜净距（mm）','并柜填写 0，不含机柜自身宽度')}{input('aisle','排间通道宽度（mm）','单排可不填；多排按相同间距排列')}
   {input('left','首柜左侧距机房左边界（mm）')}{input('front','首排前沿距机房前边界（mm）')}
   <Field label="机柜正面朝向"><select value={values.orientation??''} onChange={e=>change('orientation',e.target.value)}><option value="">请选择</option><option value="0">朝向机房前边界</option><option value="180">朝向机房后边界</option></select></Field>
   <label className="facility-overwrite"><input type="checkbox" checked={overwrite} onChange={e=>{setOverwrite(e.target.checked);setPreview(undefined)}}/>替换已有平面位置与尺寸（默认保留）</label>
  </fieldset>
  <Notice>以机房左前角为参照，从左到右排满一排，再向后排列。不同尺寸或不同朝向的机柜请分组设置；无平面布局不影响机柜立面入库。</Notice>
  <DataTable data={racks} pageSize={10} columns={[{accessorKey:'name',header:'机柜排列顺序'},{id:'order',header:'调整顺序',cell:({row})=>{const i=order.indexOf(row.original.id);const move=(delta:number)=>{setOrder(v=>{const copy=[...v];[copy[i],copy[i+delta]]=[copy[i+delta],copy[i]];return copy});setPreview(undefined)};return <div className="actions"><Button disabled={saving||i===0} onClick={()=>move(-1)}>前移</Button><Button disabled={saving||i===order.length-1} onClick={()=>move(1)}>后移</Button></div>}}]}/>
  {error&&<Notice tone="error">{error}</Notice>}
  {preview&&<><h3>布局预览 · {preview.items.filter(i=>i.changed).length} 个机柜变更</h3><DataTable data={preview.items} pageSize={10} columns={[{accessorKey:'name',header:'机柜'},{accessorKey:'position',header:'计划排位'},{accessorKey:'before',header:'当前'},{accessorKey:'after',header:'本次处理'}]}/></>}
  {preview&&preview.revision!==s.revision&&<Notice tone="warning">工程数据已变化，请重新预览。</Notice>}
  <div className="modal-actions"><Button kind="ghost" disabled={saving} onClick={()=>setAdvanced(true)}>高级位置参数</Button><Button disabled={saving} onClick={close}>取消</Button><Button disabled={saving||!can('bom:edit')} onClick={build}>预览布局</Button><Button kind="primary" disabled={saving||!can('bom:edit')||!preview?.items.some(i=>i.changed)||preview?.revision!==s.revision} onClick={async()=>{if(!preview)return;setSaving(true);const ok=await run('机房布局已保存',()=>api.saveModel('racks',preview.rows,'机房排布设置'));setSaving(false);if(ok)close()}}>{saving?'正在保存…':'保存布局'}</Button></div>
 </Modal>;
}
