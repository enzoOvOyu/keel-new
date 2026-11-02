import {ArrowDownToLine,Check,FileSpreadsheet,Plus,Save,Trash2,Upload} from 'lucide-react';
import {useEffect,useRef,useState} from 'react';
import {useQueries,useQuery} from '@tanstack/react-query';
import {Button} from '../components/ui';
import {readInputCSV,writeInputCSV} from './engineeringInputCSV';
import {adoptEngineeringModelingInputs,engineeringModel,engineeringModelingInputs,engineeringRules,saveEngineeringModelingInput,importEngineeringModelingPackage,engineeringInputCatalog,engineeringInputTemplateURL,previewEngineeringInput,type EngineeringInputPreview} from '../lib/api/engineering';

const referenceTables:Record<string,string[]>={racks:['rooms'],equipment_specs:['terminations'],placements:['rooms','racks','equipment_specs'],route_nodes:['rooms','racks'],route_edges:['route_nodes'],rack_access:['racks','route_nodes'],power_connections:['power_circuits'],rack_capacity:['racks'],network_roles:['network_planes'],network_interfaces:['network_planes']};
const keyFields:Record<string,string[]>={rooms:['room_code'],racks:['room_code','rack_code'],placements:['serial'],equipment_specs:['serial'],route_nodes:['code'],route_edges:['code'],rack_access:['rack_code'],cabling_rules:['code','version'],cable_specs:['code'],optic_specs:['model'],rack_capacity:['rack_code'],power_circuits:['code']};
const label=(s:string)=>s.replace(/^a端/,'本端').replace(/^b端/,'对端');

export function EngineeringStepInputs({project,canEdit,sourceID,revision,onSaved,onAdopted,canUpload=false,initialStep=1,initialTable=''}:{project:string;canEdit:boolean;sourceID?:string;revision?:number;onSaved?:()=>Promise<void>;onAdopted?:()=>void;canUpload?:boolean;initialStep?:number;initialTable?:string;dialog?:boolean}) {
 const [csv,setCSV]=useState(''),[reason,setReason]=useState(''),[version,setVersion]=useState(0),[dirty,setDirty]=useState(false),[result,setResult]=useState<EngineeringInputPreview|null>(null),[error,setError]=useState(''),[busy,setBusy]=useState(false),[showPaste,setShowPaste]=useState(false),[message,setMessage]=useState('');
 const file=useRef<HTMLInputElement>(null);
 const catalog=useQuery({queryKey:['engineering-input-catalog',project],queryFn:({signal})=>engineeringInputCatalog(project,signal)});
 const table=catalog.data?.tables.find(t=>t.id===initialTable);
 const saved=useQuery({queryKey:['engineering-modeling-input',project,sourceID,initialTable,revision],queryFn:({signal})=>engineeringModelingInputs(project,sourceID!,initialTable,signal),enabled:!!sourceID,refetchOnWindowFocus:false});
 const physical=useQuery({queryKey:['engineering-input-references',project,sourceID,revision],queryFn:({signal})=>engineeringModel(project,sourceID!,signal),enabled:!!sourceID&&!!revision&&revision>1});
 const references=useQueries({queries:(referenceTables[initialTable]||[]).map(id=>({queryKey:['engineering-modeling-input',project,sourceID,id,revision],queryFn:({signal}:{signal:AbortSignal})=>engineeringModelingInputs(project,sourceID!,id,signal),enabled:!!sourceID}))});
 const rules=useQuery({queryKey:['engineering-rules',project],queryFn:({signal})=>engineeringRules(project,signal),enabled:['rack_access','cabling_rules'].includes(initialTable)});
 useEffect(()=>{const item=saved.data?.items[0];if(!dirty&&item){setCSV(item.csv||'');setReason(item.reason);setVersion(item.version);setResult(null)}},[saved.data,dirty]);
 const parsed=readInputCSV(csv),headers=table?.fields.map(f=>f.label)||[];
 const positions=parsed?.[0]?.map(h=>table?.fields.findIndex(f=>f.label===h.trim()||f.code===h.trim())??-1)||[];
 const editable=!!parsed&&(!parsed.length||positions.length===headers.length&&new Set(positions).size===headers.length&&positions.every(i=>i>=0))&&parsed.every(row=>row.length===headers.length);
 const rows=editable?(parsed?.slice(1).map(row=>headers.map((_,i)=>row[positions.indexOf(i)]||''))||[]):[];
 const canonical=writeInputCSV([headers,...rows]);
 const locked=busy||!canEdit||!!sourceID&&saved.isPending;
 const staged=references.flatMap(q=>q.data?.items||[]).flatMap(item=>{const t=catalog.data?.tables.find(t=>t.id===item.table_id);return (readInputCSV(item.csv||'')?.slice(1)||[]).map(row=>({table:item.table_id,values:Object.fromEntries(t?.fields.map((f,i)=>[f.code,row[i]])||[])}))});
 const model=physical.data?.model;
 function choices(code:string,row:Record<string,string>):string[]{
  if(code==='room_code'&&initialTable!=='rooms')return [...new Set([...(model?.facilities.filter(f=>f.kind==='room').map(f=>f.code||'')||[]),...staged.filter(s=>s.table==='rooms').map(s=>s.values.room_code)].filter(Boolean))];
  if(code==='rack_code'&&initialTable!=='racks')return [...new Set([...(model?.facilities.filter(f=>f.kind==='rack'&&(!row.room_code||model.facilities.find(p=>p.entity_id===f.parent_id)?.code===row.room_code)).map(f=>f.code||'')||[]),...staged.filter(s=>s.table==='racks'&&(!row.room_code||s.values.room_code===row.room_code)).map(s=>s.values.rack_code)].filter(Boolean))];
  if(code==='serial'||code==='device')return [...new Set([...(model?.equipment.map(e=>e.serial||e.label)||[]),...staged.filter(s=>s.table==='equipment_specs').map(s=>s.values.serial)].filter(Boolean))];
  if(initialTable==='route_edges'&&['from','to'].includes(code)||initialTable==='rack_access'&&code==='node')return staged.filter(s=>s.table==='route_nodes').map(s=>s.values.code);
  if(['lead_rule','slack_rule','length_rule'].includes(code)||initialTable==='rack_access'&&code==='rule')return rules.data?.filter(v=>v.rule.approval_status==='approved').map(v=>v.rule.id+'@'+v.rule.version)||[];
  if(code==='face')return ['front','rear','both'];
  if(code==='mount_type')return ['u','non_u'];
  if(code==='coordinate_unit'||code==='unit'&&['route_nodes','route_edges','cable_specs','optic_specs'].includes(initialTable))return ['m','cm','mm'];
  if(code==='strategy')return ['shortest_path'];
  return [];
 }
 const issues:{row:number;field:string;message:string}[]=[];
 if(!editable)issues.push({row:1,field:'',message:'表头、列数或引号格式不符合模板；未知列保留在原文中，请修正。'});
 if(!rows.length)issues.push({row:0,field:'',message:'尚无输入，请导入资料或添加一行。'});
 const seen=new Set<string>();
 rows.forEach((row,i)=>{
  const values=Object.fromEntries(table?.fields.map((f,j)=>[f.code,row[j]?.trim()||''])||[]);
  const add=(field:string,message:string)=>issues.push({row:i+2,field,message});
  table?.fields.forEach((f,j)=>{const v=row[j]?.trim();if(!v){if(f.required)add(f.code,label(f.label)+'不能为空');return}
   if(['number','positive','nonnegative','positive_integer'].includes(f.kind)&&(!Number.isFinite(Number(v))||f.kind==='positive'&&Number(v)<=0||f.kind==='nonnegative'&&Number(v)<0||f.kind==='positive_integer'&&(!Number.isInteger(Number(v))||Number(v)<=0)))add(f.code,label(f.label)+'数值不符合要求');
   if(f.kind==='mac'&&!/^(?:[0-9a-f]{2}[:-]){5}[0-9a-f]{2}$/i.test(v)&&!/^[0-9a-f]{12}$/i.test(v))add(f.code,label(f.label)+'需填写有效 MAC 地址');
   const reference=initialTable==='racks'&&f.code==='room_code'||initialTable==='placements'&&['serial','room_code','rack_code'].includes(f.code)||initialTable==='route_nodes'&&['room_code','rack_code'].includes(f.code)||initialTable==='route_edges'&&['from','to'].includes(f.code)||initialTable==='rack_access'&&['rack_code','node'].includes(f.code);
   if(reference&&references.every(q=>q.isSuccess)&&(!physical.isEnabled||physical.isSuccess)&&!choices(f.code,values).includes(v))add(f.code,label(f.label)+'未找到关联对象，请先补充对应模型');
  });
  if(keyFields[initialTable]){const key=keyFields[initialTable].map(k=>values[k]).join('\0');if(seen.has(key))add(keyFields[initialTable][0],'对象编码重复');seen.add(key)}
  if(values.orientation_deg&&(Number(values.orientation_deg)<0||Number(values.orientation_deg)>=360))add('orientation_deg','朝向应在 0（含）至 360（不含）度之间');
  if(initialTable==='route_edges'&&values.from&&values.from===values.to)add('to','路径起点和终点不能相同');
  if(initialTable==='placements'&&values.mount_type==='u'){
   if(!values.start_u||!values.height_u)add('start_u','按 U 安装必须填写起始 U 和设备高度');
   const rack=staged.find(s=>s.table==='racks'&&s.values.rack_code===values.rack_code&&s.values.room_code===values.room_code)?.values;
   const capacity=Number(rack?.capacity_u||model?.facilities.find(f=>f.kind==='rack'&&f.code===values.rack_code)?.capacity_u);
   if(capacity&&Number(values.start_u)+Number(values.height_u)-1>capacity)add('start_u','设备占位超出机柜容量');
   const overlap=rows.slice(0,i).some(r=>{const v=Object.fromEntries(table!.fields.map((f,j)=>[f.code,r[j]]));return v.rack_code===values.rack_code&&v.room_code===values.room_code&&(v.face===values.face||v.face==='both'||values.face==='both')&&Number(v.start_u)<Number(values.start_u)+Number(values.height_u)&&Number(values.start_u)<Number(v.start_u)+Number(v.height_u)});
   if(overlap)add('start_u','同一机柜安装面存在 U 位重叠');
  }
 });
 function change(value:string){setCSV(value);setDirty(true);setResult(null);setMessage('');setError('')}
 function updateRows(value:string[][]){change(writeInputCSV([headers,...value]))}
 async function run(action:()=>Promise<void>){setBusy(true);setError('');setMessage('');try{await action()}catch(e){setError((e as Error).message)}finally{setBusy(false)}}
 async function save(){if(!sourceID||!revision||!table)return null;const value=await saveEngineeringModelingInput(project,sourceID,{table_id:table.id,csv:canonical,input_version:version,expected_revision:revision,reason});setVersion(value.version);setDirty(false);await saved.refetch();await onSaved?.();return value}
 async function adopt(){
  if(!sourceID||!revision||!table)return;
  const preview=await previewEngineeringInput(project,{table_id:table.id,csv:canonical});setResult(preview);if(!preview.valid)return;
  const value=await save();if(!value)return;
  const current=await engineeringModelingInputs(project,sourceID);
  const tables=catalog.data?.tables.filter(t=>t.step===table.step)||[];
  const dependencies=[table.id,...(referenceTables[table.id]||[])];
  const inputs=table.step===3?current.items.filter(input=>tables.some(t=>t.id===input.table_id)):current.items.filter(input=>dependencies.includes(input.table_id)&&tables.some(t=>t.id===input.table_id));
  const incomplete=inputs.filter(input=>input.issues.length>0);
  if(incomplete.length)throw Error('请先补齐关联资料：'+incomplete.map(input=>tables.find(t=>t.id===input.table_id)?.name||input.table_id).join('、'));
  await adoptEngineeringModelingInputs(project,sourceID,{expected_revision:current.revision,step:table.step,inputs:inputs.map(input=>({table_id:input.table_id,version:input.version})),reason});
  await saved.refetch();await onSaved?.();setMessage('已核对采用，相关工程产物已更新。');onAdopted?.();
 }
 async function importFile(value:File){if(value.name.toLowerCase().endsWith('.csv')){change(await value.text());setReason('用户导入 · '+value.name);return}if(!sourceID||!revision)throw Error('请先建立工程任务');await importEngineeringModelingPackage(project,sourceID,value,initialStep,revision);setDirty(false);await saved.refetch();await onSaved?.();setMessage('已导入本工作簿中的关联建模输入，请核对。')}
 return <section className="es-model-editor"><div className="es-model-editor-body">
 {(catalog.isPending||!!sourceID&&saved.isPending)&&<p aria-busy="true">正在读取建模输入…</p>}
 {[catalog.error,saved.error,physical.error,...references.map(r=>r.error)].filter(Boolean).map((e,i)=><p role="alert" key={i}>{e?.message}</p>)}
 {table&&<>
 <div className="es-editor-note"><FileSpreadsheet size={17}/><span>{table.condition} 可导入 XLSX / CSV、粘贴或逐格填写。</span></div>
 <div className="es-editor-toolbar"><a className="btn" href={engineeringInputTemplateURL(project,table.id,'xlsx')}><ArrowDownToLine size={15}/>XLSX 模板</a><a className="btn" href={engineeringInputTemplateURL(project,table.id,'csv')}>CSV 模板</a><Button disabled={locked} onClick={()=>file.current?.click()}><Upload size={15}/>导入资料</Button><Button disabled={locked} onClick={()=>setShowPaste(v=>!v)}>粘贴表格</Button><span>{rows.length} 行 · {headers.length} 列 · v{version}</span></div>
 <input hidden ref={file} aria-label={'导入'+table.name} type="file" accept={canUpload?'.xlsx,.csv':'.csv'} onChange={e=>{const value=e.target.files?.[0];e.target.value='';if(value)void run(()=>importFile(value))}}/>
 {(showPaste||!editable)&&<div className="es-editor-paste"><label>CSV 原文与批量粘贴<textarea aria-label="标准数据 CSV" rows={5} disabled={locked} value={csv} onChange={e=>change(e.target.value)} placeholder={headers.join(',')}/></label><Button disabled={!editable} onClick={()=>{setCSV(canonical);setShowPaste(false)}}>解析到表格</Button></div>}
 {editable&&<><div className="es-editor-table"><table><thead><tr><th>#</th>{table.fields.map(f=><th key={f.code}>{label(f.label)}{f.required?' *':''}<small>{f.code}</small></th>)}<th/></tr></thead><tbody>{rows.map((row,i)=>{const values=Object.fromEntries(table.fields.map((f,j)=>[f.code,row[j]]));return <tr key={i}><td>{i+1}</td>{table.fields.map((f,j)=>{const options=choices(f.code,values),list=table.id+'-'+f.code+'-'+i,problem=issues.find(p=>p.row===i+2&&p.field===f.code);return <td key={f.code}><input aria-label={label(f.label)+' 第'+(i+1)+'行'} aria-invalid={!!problem} title={problem?.message} disabled={locked} list={options.length?list:undefined} value={row[j]||''} onChange={e=>updateRows(rows.map((r,k)=>k===i?r.map((v,l)=>l===j?e.target.value:v):r))}/>{options.length>0&&<datalist id={list}>{options.map(value=><option key={value} value={value}/>)}</datalist>}</td>})}<td><button className="es-editor-icon" aria-label={'删除第'+(i+1)+'行'} disabled={locked} onClick={()=>updateRows(rows.filter((_,k)=>k!==i))}><Trash2 size={15}/></button></td></tr>})}</tbody></table>{!rows.length&&<p className="es-editor-empty">导入资料或添加一行，缺失信息会在这里提示。</p>}</div><Button disabled={locked} onClick={()=>updateRows([...rows,headers.map(()=>'')])}><Plus size={15}/>添加一行</Button></>}
 <label className="es-editor-reason">输入依据 / 版本说明<input aria-label="输入依据 / 版本说明" disabled={locked} value={reason} onChange={e=>{setReason(e.target.value);setDirty(true)}} placeholder="来源文件、设计版本或人工核对依据"/></label>
 {error&&<p className="notice error" role="alert">{error==='REVISION_CONFLICT'?'输入或工程版本已变化，请重新打开核对最新版本。':error}</p>}{message&&<p role="status">{message}</p>}
 <div className={'es-editor-validation '+(issues.length?'invalid':'valid')} role={issues.length?'alert':'status'}><strong>{issues.length?issues.length+' 项需要处理':'字段校验通过，可核对并采用'}</strong>{issues.slice(0,6).map((v,i)=><p key={i}>{v.row?'第 '+Math.max(1,v.row-1)+' 行 · ':''}{v.message}</p>)}{issues.length>6&&<p>以及其他 {issues.length-6} 项；问题单元格已标注。</p>}</div>
 {result&&!result.valid&&<div className="es-editor-validation invalid" role="alert">{result.issues.map((v,i)=><p key={i}>第 {v.row} 行 · {v.message}</p>)}</div>}
 <p className="es-editor-help">采用时执行引用和领域校验。修改输入会使受影响的计算与审核失效，已发布版本保留。</p>
 {!sourceID&&<p className="notice">请先在资料入口上传工作簿，建立工程任务。</p>}
 </>}
 </div><div className="es-model-editor-footer"><span>{busy?'正在处理…':dirty?'有未保存修改':'输入 v'+version}</span><Button disabled={locked||!sourceID||!revision||!reason.trim()||!csv.trim()||!editable} onClick={()=>void run(async()=>{await save();setMessage('输入已保存。')})}><Save size={15}/>保存草稿</Button><Button variant="primary" disabled={locked||!sourceID||!revision||!reason.trim()||issues.length>0||!table||!table.supports_confirmation&&table.step!==3} onClick={()=>void run(adopt)}><Check size={15}/>校验并采用</Button></div></section>
}
