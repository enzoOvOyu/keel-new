import {useState} from 'react';
import {useWorkspace} from '../../context';
import {Button,DataTable,Field,Modal,Notice} from '../ui';
import {EntityField,entityOptions,modelRows,resolveEntity} from './FacilityEntities';
import type {ModelRow} from '../../lib/api';
export function RackDetail({id,close,edit}:{id:string;close:()=>void;edit:(table:string)=>void}){
 const {state:s,api,run,can}=useWorkspace();
 const rack=s.racks.find(r=>r.id===id)!;
 const options=entityOptions(s,'rack_id');
 const original=modelRows(s,'racks').find(r=>resolveEntity(options,r.rack_id)===id);
 const [values,setValues]=useState<ModelRow>(()=>({...original,rack_id:id,room_id:original?.room_id||resolveEntity(entityOptions(s,'room_id'),rack.room),capacity_u:original?.capacity_u??(rack.capacityU==null?'':String(rack.capacityU))}));
 const [revision]=useState(s.revision),[saving,setSaving]=useState(false),[dirty,setDirty]=useState(false);
 const set=(key:string,value:string)=>{setValues(v=>({...v,[key]:value}));setDirty(true)};
 const finish=()=>{if(!saving&&(!dirty||window.confirm('放弃未保存的机柜修改？')))close()};
 const devices=s.devices.filter(d=>d.rackId===id);
 return <Modal drawer title={rack.name} description="查看归属、机柜规格和设备落位；保存后到入库与版本确认增量。" close={finish}>
  <fieldset disabled={saving||!can('bom:edit')} className="facility-batch-fields">
   <Field label="所属机房"><EntityField label="所属机房" value={values.room_id||''} options={entityOptions(s,'room_id')} onChange={v=>set('room_id',v)}/></Field>
   {([['capacity_u','机柜容量（U）'],['width_mm','机柜宽度（mm）'],['depth_mm','机柜深度（mm）']] as const).map(([key,label])=><Field key={key} label={label}><input type="number" min="1" step={key==='capacity_u'?'1':'any'} value={values[key]??''} onChange={e=>set(key,e.target.value)} placeholder="待补充"/></Field>)}
  </fieldset>
  {s.revision!==revision&&<Notice tone="warning">工程数据已更新，请关闭并重新打开后编辑。</Notice>}
  <div className="actions"><Button kind="primary" disabled={!dirty||saving||s.revision!==revision||!can('bom:edit')} onClick={async()=>{setSaving(true);const rows=structuredClone(modelRows(s,'racks'));const index=rows.findIndex(r=>resolveEntity(options,r.rack_id)===id);if(index<0)rows.push(values);else rows[index]=values;const ok=await run('机柜输入已保存',()=>api.saveModel('racks',rows,'机柜详情编辑'));setSaving(false);if(ok){setDirty(false);close()}}}>{saving?'正在保存…':'保存机柜修改'}</Button></div>
  <div className="facility-filter-heading"><b>设备落位 · {devices.length} 台</b><Button disabled={!can('bom:edit')||dirty||saving} onClick={()=>{close();edit('placements')}}>编辑落位</Button></div>
  <DataTable data={devices} pageSize={10} columns={[{accessorKey:'name',header:'设备'},{accessorKey:'model',header:'型号'},{accessorKey:'startU',header:'起始 U',cell:({getValue})=>getValue()??'待补充'},{accessorKey:'heightU',header:'高度 U',cell:({getValue})=>getValue()??'待补充'}]}/>
 </Modal>;
}
