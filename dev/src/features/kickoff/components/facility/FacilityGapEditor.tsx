import {useState} from 'react';
import {useWorkspace} from '../../context';
import {Button,DataTable,Field,Modal,Notice} from '../ui';
import {EntityField,entityOptions,modelRows,positiveU} from './FacilityEntities';
import type {ModelRow} from '../../lib/api';

export function FacilityGapEditor({mode,close}:{mode:'height'|'placement';close:()=>void}){
 const {state:s,api,run,can}=useWorkspace();
 const [revision]=useState(s.revision),[saving,setSaving]=useState(false),[error,setError]=useState(''),[preview,setPreview]=useState<ModelRow[]>();
 const missing=s.devices.filter(d=>mode==='height'?!positiveU(d.heightU):!s.racks.some(r=>r.id===d.rackId)||!positiveU(d.startU)||!d.model&&!positiveU(d.heightU));
 const models=[...new Set(missing.map(d=>d.model).filter(Boolean))];
 const [values,setValues]=useState<Record<string,ModelRow>>({});
 const update=(id:string,key:string,value:string)=>{setValues(v=>({...v,[id]:{...v[id],[key]:value}}));setPreview(undefined)};
 const table=mode==='height'?'equipment_specs':'placements';
 const prepare=()=>{setError('');try{
  const rows=structuredClone(modelRows(s,table));let changes=0;
  for(const [id,input] of Object.entries(values)){
   const key=mode==='height'?'model':'device_id';
   const matches=rows.filter(r=>r[key]===id);if(matches.length>1)throw Error('同一对象有多条输入，请先通过批量导入入口核对重复记录。');
   const device=s.devices.find(d=>d.id===id);
   const row=matches[0]??(mode==='height'?{model:id}:{device_id:id,rack_id:device?.rackId||'',start_u:device?.startU==null?'':String(device.startU),height_u:device?.heightU==null?'':String(device.heightU)});
   let changed=false;
   for(const [field,value] of Object.entries(input)){
    if(!value.trim()||(field==='rack_id'?s.racks.some(r=>r.id===row[field]):row[field]))continue;
    if(field.endsWith('_u')&&(!Number.isInteger(Number(value))||Number(value)<=0))throw Error('U 位和高度必须为正整数。');
    row[field]=value;changed=true;
   }
   if(changed){if(!matches.length)rows.push(row);changes++}
  }
  if(!changes)throw Error('尚未填写可补充的信息；已有值保持不变。');
  setPreview(rows);
 }catch(e){setError((e as Error).message)}};
 const numberInput=(id:string,key:string,label:string)=><input aria-label={label} type="number" min="1" step="1" value={values[id]?.[key]??''} placeholder="待补充" onChange={e=>update(id,key,e.target.value)}/>;
 return <Modal title={mode==='height'?'按型号批量补录高度':'补录设备安装位置'} description={`已自动带入 ${missing.length} 台缺信息的设备；只填写有依据的空缺，已有值保留。`} wide close={()=>{if(!saving)close()}}>
  <fieldset disabled={saving} className="facility-batch-fields">
  {mode==='height'?<><DataTable data={models.map(model=>({model,count:missing.filter(d=>d.model===model).length}))} pageSize={12} columns={[{accessorKey:'model',header:'设备型号'},{accessorKey:'count',header:'待补充设备数'},{id:'height',header:'统一高度（U）',cell:({row})=>numberInput(row.original.model,'height_u',row.original.model+'高度')}]}/>{missing.some(d=>!d.model)&&<Notice>有 {missing.filter(d=>!d.model).length} 台设备没有明确型号，请通过“补录安装位置”逐设备填写高度，不推测型号。</Notice>}</>:<>
   <Field label="将未关联机柜的设备统一关联到"><EntityField label="批量关联机柜" value="" options={entityOptions(s,'rack_id')} onChange={id=>{setValues(v=>{const next={...v};for(const d of missing)if(!s.racks.some(r=>r.id===d.rackId))next[d.id]={...next[d.id],rack_id:id};return next});setPreview(undefined)}}/></Field>
   <DataTable data={missing} pageSize={10} columns={[{accessorKey:'name',header:'设备'},{accessorKey:'model',header:'型号'},{id:'rack',header:'所在机柜',cell:({row})=>s.racks.some(r=>r.id===row.original.rackId)?s.racks.find(r=>r.id===row.original.rackId)?.name:<EntityField label={row.original.name+'机柜'} value={values[row.original.id]?.rack_id??''} options={entityOptions(s,'rack_id')} onChange={v=>update(row.original.id,'rack_id',v)}/>},{id:'u',header:'起始 U 位',cell:({row})=>positiveU(row.original.startU)?row.original.startU:numberInput(row.original.id,'start_u',row.original.name+'起始U')},{id:'height',header:'高度（U）',cell:({row})=>positiveU(row.original.heightU)?row.original.heightU:numberInput(row.original.id,'height_u',row.original.name+'高度')}]}/>
  </>}
  </fieldset>
  {error&&<Notice tone="error">{error}</Notice>}
  {preview&&<Notice>将应用上方已填写的空缺；未填写项和已有信息保留。保存后缺口数量自动更新。</Notice>}
  {s.revision!==revision&&<Notice tone="warning">资料已更新，请重新打开补录窗口。</Notice>}
  <div className="modal-actions"><Button disabled={saving} onClick={close}>取消</Button><Button disabled={saving||!can('bom:edit')} onClick={prepare}>核对本次补录</Button><Button kind="primary" disabled={saving||!preview||s.revision!==revision||!can('bom:edit')} onClick={async()=>{if(!preview)return;setSaving(true);const ok=await run('缺失信息已补录',()=>api.saveModel(table,preview,'缺口批量补录'));setSaving(false);if(ok)close()}}>{saving?'正在保存…':'确认补录'}</Button></div>
 </Modal>;
}
