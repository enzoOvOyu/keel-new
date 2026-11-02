import {ArrowDownToLine,Check,FileSpreadsheet,Plus,Save,Trash2,Upload} from 'lucide-react';
import {useEffect,useRef,useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {Button} from '../components/ui';
import {readInputCSV,writeInputCSV} from './engineeringInputCSV';
import {adoptEngineeringModelingInputs,engineeringModelingInputs,saveEngineeringModelingInput,importEngineeringModelingPackage,engineeringInputCatalog,engineeringInputTemplateURL,previewEngineeringInput,type EngineeringInputPreview} from '../lib/api/engineering';

export function EngineeringStepInputs({project,canEdit,sourceID,revision,onSaved,canUpload=false,initialStep=1,initialTable=''}:{project:string;canEdit:boolean;sourceID?:string;revision?:number;onSaved?:()=>Promise<void>;canUpload?:boolean;initialStep?:number;initialTable?:string;dialog?:boolean}) {
 const [csv,setCSV]=useState(''),[reason,setReason]=useState(''),[version,setVersion]=useState(0),[dirty,setDirty]=useState(false),[result,setResult]=useState<EngineeringInputPreview|null>(null),[error,setError]=useState(''),[busy,setBusy]=useState(false),[showPaste,setShowPaste]=useState(false),[message,setMessage]=useState('');
 const file=useRef<HTMLInputElement>(null);
 const catalog=useQuery({queryKey:['engineering-input-catalog',project],queryFn:({signal})=>engineeringInputCatalog(project,signal)});
 const table=catalog.data?.tables.find(t=>t.id===initialTable);
 const saved=useQuery({queryKey:['engineering-modeling-input',project,sourceID,initialTable],queryFn:({signal})=>engineeringModelingInputs(project,sourceID!,initialTable,signal),enabled:!!sourceID,refetchOnWindowFocus:false});
 useEffect(()=>{const item=saved.data?.items[0];if(!dirty&&item){setCSV(item.csv||'');setReason(item.reason);setVersion(item.version);setResult(null)}},[saved.data,dirty]);
 const parsed=readInputCSV(csv),headers=table?.fields.map(f=>f.label)||[],rows=parsed?.slice(1)||[];
 const editable=!!parsed&&(!parsed.length||JSON.stringify(parsed[0])===JSON.stringify(headers))&&parsed.every(row=>row.length===headers.length);
 const locked=busy||!canEdit||!!sourceID&&saved.isPending;
 function change(value:string){setCSV(value);setDirty(true);setResult(null);setMessage('')}
 function updateRows(value:string[][]){change(writeInputCSV([headers,...value]))}
 async function run(action:()=>Promise<void>){setBusy(true);setError('');setMessage('');try{await action()}catch(e){setError((e as Error).message)}finally{setBusy(false)}}
 async function save(){if(!sourceID||!revision||!table)return null;const value=await saveEngineeringModelingInput(project,sourceID,{table_id:table.id,csv,input_version:version,expected_revision:revision,reason});setDirty(false);setVersion(value.version);await saved.refetch();await onSaved?.();return value}
 async function adopt(){if(!sourceID||!revision||!table||!result?.valid)return;const value=await save();if(!value)return;await adoptEngineeringModelingInputs(project,sourceID,{expected_revision:revision,step:table.step,inputs:[{table_id:table.id,version:value.version}],reason});await saved.refetch();await onSaved?.();setMessage('已采用，工程版本已更新。')}
 async function importFile(value:File){if(value.name.toLowerCase().endsWith('.csv')){change(await value.text());setReason('用户导入 · '+value.name);return}if(!sourceID||!revision)throw Error('请先建立工程任务');await importEngineeringModelingPackage(project,sourceID,value,initialStep,revision);setDirty(false);await saved.refetch();await onSaved?.();setMessage('工作簿已保存为建模草稿，请核对后统一采用。')}
 return <section className="es-model-editor"><div className="es-model-editor-body">
  {(catalog.isPending||!!sourceID&&saved.isPending)&&<p aria-busy="true">正在读取建模输入…</p>}
  {[catalog.error,saved.error].filter(Boolean).map((e,i)=><p role="alert" key={i}>{e?.message}<Button onClick={()=>void Promise.all([catalog.refetch(),saved.refetch()])}>重试</Button></p>)}
  {table&&<>
   <div className="es-editor-note"><FileSpreadsheet size={17}/><span>{table.condition}</span></div>
   <div className="es-editor-toolbar"><a className="btn" href={engineeringInputTemplateURL(project,table.id,'xlsx')}><ArrowDownToLine size={15}/>XLSX 模板</a><a className="btn" href={engineeringInputTemplateURL(project,table.id,'csv')}>CSV 模板</a><Button disabled={locked} onClick={()=>file.current?.click()}><Upload size={15}/>导入资料</Button><Button disabled={locked} onClick={()=>setShowPaste(v=>!v)}>粘贴表格</Button><span>{rows.length} 行 · 输入 v{version}{dirty?' · 未保存':''}</span></div>
   <input hidden ref={file} aria-label={'导入'+table.name} type="file" accept={canUpload?'.xlsx,.csv':'.csv'} onChange={e=>{const value=e.target.files?.[0];e.target.value='';if(value)void run(()=>importFile(value))}}/>
   {(showPaste||!editable)&&<div className="es-editor-paste"><label>CSV 原文与批量粘贴<textarea aria-label="标准数据 CSV" rows={5} disabled={locked} value={csv} onChange={e=>change(e.target.value)} placeholder={headers.join(',')}/></label>{!editable&&<p role="alert">请保留模板表头、列数和引号格式。</p>}<Button disabled={!editable} onClick={()=>setShowPaste(false)}>返回表格</Button></div>}
   {editable&&<><div className="es-editor-table"><table><thead><tr><th>#</th>{table.fields.map(f=><th key={f.code}>{f.label.replace(/^a端/,'本端').replace(/^b端/,'对端')}{f.required?' *':''}<small>{f.code}</small></th>)}<th/></tr></thead><tbody>{rows.map((row,i)=><tr key={i}><td>{i+1}</td>{table.fields.map((f,j)=><td key={f.code}><input aria-label={f.label+' 第'+(i+1)+'行'} disabled={locked} value={row[j]||''} onChange={e=>updateRows(rows.map((r,k)=>k===i?r.map((v,l)=>l===j?e.target.value:v):r))}/></td>)}<td><button className="es-editor-icon" aria-label={'删除第'+(i+1)+'行'} disabled={locked} onClick={()=>updateRows(rows.filter((_,k)=>k!==i))}><Trash2 size={15}/></button></td></tr>)}</tbody></table>{!rows.length&&<p className="es-editor-empty">导入资料或添加一行；未知字段留空，保存后可集中补充。</p>}</div><Button disabled={locked} onClick={()=>updateRows([...rows,headers.map(()=>'')])}><Plus size={15}/>添加一行</Button></>}
   <label className="es-editor-reason">输入依据 / 版本说明<input aria-label="输入依据 / 版本说明" disabled={locked} value={reason} onChange={e=>{setReason(e.target.value);setDirty(true)}} placeholder="来源文件、设计版本或人工核对依据"/></label>
   {error&&<p className="notice error" role="alert">{error==='REVISION_CONFLICT'?'输入或工程版本已变化，请关闭后重新打开，核对新版本再保存。':error}</p>}{message&&<p role="status">{message}</p>}
   {result&&<div className={'es-editor-validation '+(result.valid?'valid':'invalid')} role={result.valid?'status':'alert'}><strong>{result.valid?'格式校验通过':'需要补充或修正 '+result.issues.length+' 项'}</strong>{result.issues.map((v,i)=><p key={i}>第 {v.row||'—'} 行 · {table.fields.find(f=>f.code===v.field)?.label||v.field}：{v.message}</p>)}</div>}
   <p className="es-editor-help">保存保留未完成输入；确认采用会执行领域校验并创建工程版本。依赖多张表的步骤在建模中心统一采用。</p>
   {!sourceID&&<p className="notice">先上传资料建立工程任务，再保存建模输入。</p>}
  </>}
 </div><div className="es-model-editor-footer"><span>{busy?'正在处理…':dirty?'未保存的输入':'保存草稿 → 核对 → 采用'}</span><Button disabled={locked||!csv.trim()||!editable} onClick={()=>void run(async()=>{setResult(await previewEngineeringInput(project,{table_id:initialTable,csv}))})}>校验</Button><Button disabled={locked||!sourceID||!revision||!reason.trim()||!csv.trim()||!editable} onClick={()=>void run(async()=>{await save();setMessage('草稿已保存，可关闭后继续补充。')})}><Save size={15}/>保存草稿</Button><Button variant="primary" disabled={locked||!sourceID||!revision||!reason.trim()||!result?.valid||!table?.supports_confirmation} onClick={()=>void run(adopt)}><Check size={15}/>确认采用</Button></div></section>
}
