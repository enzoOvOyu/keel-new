import {useRef,useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {FileSpreadsheet,Upload,X,CheckCircle2,ArrowRight} from 'lucide-react';
import * as Dialog from '@radix-ui/react-dialog';
import {Button} from '../components/ui';
import {engineeringInputCatalog,engineeringModelingInputs,importEngineeringModelingPackage,saveEngineeringModelingInput,type EngineeringWorkspace} from '../lib/api/engineering';
import {studioModels} from './engineeringStudioCatalog';
import {readInputCSV} from './engineeringInputCSV';

type UploadItem={file:File;target:string;state:'pending'|'uploading'|'done'|'error';error?:string};
export function EngineeringSources({project,workspace,canUpload,canEdit,busy,onRawUpload,onSaved,onEvidence,onAnalysis}:{project:string;workspace?:EngineeringWorkspace;canUpload:boolean;canEdit:boolean;busy:boolean;onRawUpload:(file:File)=>Promise<void>;onSaved:()=>Promise<void>;onEvidence:()=>void;onAnalysis:()=>void}){
 const picker=useRef<HTMLInputElement>(null),[files,setFiles]=useState<UploadItem[]>([]),[working,setWorking]=useState(false),[error,setError]=useState('');
 const inputs=useQuery({queryKey:['engineering-modeling-inputs',project,workspace?.id,workspace?.revision],queryFn:({signal})=>engineeringModelingInputs(project,workspace!.id,'',signal),enabled:!!workspace});
 const catalog=useQuery({queryKey:['engineering-input-catalog',project],queryFn:({signal})=>engineeringInputCatalog(project,signal)});
 const sources=new Map<string,{name:string;steps:Set<number>;rows:number}>();
 for(const item of inputs.data?.items||[]){const name=item.reason||'页面补充';const old=sources.get(name)||{name,steps:new Set<number>(),rows:0};const step=catalog.data?.tables.find(t=>t.id===item.table_id)?.step;if(step)old.steps.add(step);old.rows+=item.rows;sources.set(name,old)}
 async function choose(selected:File[]){
  const values:UploadItem[]=[];
  for(const file of selected){let target='source';if(file.name.toLowerCase().endsWith('.csv')){const header=readInputCSV(await file.text())?.[0];const table=catalog.data?.tables.find(t=>header?.length===t.fields.length&&header.every(h=>t.fields.some(f=>f.label===h.trim()||f.code===h.trim())));target=table?.id||'rooms'}values.push({file,target,state:'pending'})}
  setFiles(values);setError('');
 }
 async function upload(){
  setWorking(true);setError('');
  try{
   if(files.some(f=>f.target==='source')&&files.length!==1)throw Error('原始工作簿单独建立工程任务；建模资料可在任务内批量导入。');
   for(let i=0;i<files.length;i++){
    const item=files[i];if(item.state==='done')continue;
    setFiles(v=>v.map((f,j)=>i===j?{...f,state:'uploading',error:undefined}:f));
    try{
     if(item.target==='source'){await onRawUpload(item.file);setFiles([]);return}
     if(!workspace)throw Error('请先上传原始工作簿建立任务，再导入建模资料。');
     const table=catalog.data?.tables.find(t=>t.id===item.target);if(!table)throw Error('请选择资料对应的模型。');
     if(item.file.name.toLowerCase().endsWith('.csv')){
      const current=await engineeringModelingInputs(project,workspace.id,item.target);
      await saveEngineeringModelingInput(project,workspace.id,{expected_revision:workspace.revision,input_version:current.items[0]?.version||0,table_id:item.target,csv:await item.file.text(),reason:'用户导入 · '+item.file.name});
     }else await importEngineeringModelingPackage(project,workspace.id,item.file,table.step,workspace.revision);
     setFiles(v=>v.map((f,j)=>i===j?{...f,state:'done'}:f));
    }catch(e){setFiles(v=>v.map((f,j)=>i===j?{...f,state:'error',error:(e as Error).message}:f));throw e}
   }
  }catch(e){setError((e as Error).message)}finally{await inputs.refetch();await onSaved();setWorking(false)}
 }
 return <section className="panel padded es-source-panel"><div className="es-section-title"><h3>资料入口</h3><Button disabled={!canUpload||busy||working} onClick={()=>picker.current?.click()}><Upload size={15}/>选择资料</Button></div><p className="muted">原始工作簿建立任务；机房、机柜、路径、规则等建模资料可批量补充到同一任务。</p><input ref={picker} hidden multiple type="file" accept=".xlsx,.csv" aria-label="选择开局资料" onChange={e=>{const selected=Array.from(e.target.files||[]);e.target.value='';if(selected.length)void choose(selected)}}/>
 <div className="es-source-list">{workspace&&<div><FileSpreadsheet size={22}/><article><b>{workspace.filename}</b><small>原始资料 · 原件已保留 · v{workspace.revision}</small></article><Button onClick={onEvidence}>查看来源<ArrowRight size={13}/></Button></div>}{[...sources.values()].map(s=><div key={s.name}><FileSpreadsheet size={20}/><article><b>{s.name}</b><small>{s.rows} 行输入</small></article><span>用于步骤 {[...s.steps].sort().join(' / ')}</span></div>)}{!workspace&&<div className="empty">选择开局 XLSX，建立第一份工程任务。</div>}</div>
 <div className="es-source-footer"><span>支持 XLSX 工作簿和标准 CSV；未识别字段在建模中补充。</span><Button onClick={onAnalysis}>智能识别设置<ArrowRight size={13}/></Button></div>
 {inputs.error&&<p role="alert">{inputs.error.message}</p>}
 {files.length>0&&<Dialog.Root open onOpenChange={open=>{if(!open&&!working)setFiles([])}}><Dialog.Portal><Dialog.Overlay className="es-model-dialog-overlay"/><Dialog.Content className="es-model-dialog es-upload-dialog"><div className="es-model-dialog-header"><div><Dialog.Title>导入开局资料</Dialog.Title><Dialog.Description>选择每份资料的用途，导入后在工程模型中统一核对。</Dialog.Description></div><Dialog.Close disabled={working} aria-label="关闭" className="es-editor-icon"><X size={18}/></Dialog.Close></div><div className="es-model-editor-body"><div className="es-source-imports">{files.map((f,i)=><div key={i}><FileSpreadsheet size={18}/><div><b>{f.file.name}</b><small>{f.state==='done'?'已导入':f.state==='uploading'?'正在导入…':f.error||'待导入'}</small></div><select aria-label={'资料用途 '+f.file.name} disabled={working||f.state==='done'} value={f.target} onChange={e=>setFiles(v=>v.map((a,j)=>i===j?{...a,target:e.target.value}:a))}>{f.file.name.endsWith('.xlsx')&&<option value="source">{workspace?'原始资料新版本':'原始开局工作簿'}</option>}<option value="terminations">设备关联 · 八个端接字段</option>{studioModels.map(m=><optgroup key={m.id} label={m.name}>{m.tables.map(id=><option value={id} key={id}>{catalog.data?.tables.find(t=>t.id===id)?.name||m.name}</option>)}</optgroup>)}</select>{f.state==='done'&&<CheckCircle2 size={18}/>}</div>)}</div>{error&&<p className="notice error" role="alert">{error}</p>}<p className="muted">非规范资料的自动分流待 Provider 配置后接入；当前选择明确资料用途，原文和缺口保留。</p></div><div className="es-model-editor-footer"><Button disabled={working} onClick={()=>setFiles([])}>{files.every(f=>f.state==='done')?'完成，核对建模':'取消'}</Button>{!files.every(f=>f.state==='done')&&<Button variant="primary" disabled={working||busy||!canUpload||!canEdit&&files.some(f=>f.target!=='source')} onClick={()=>void upload()}>{working?'正在导入…':'导入所选资料'}</Button>}</div></Dialog.Content></Dialog.Portal></Dialog.Root>}
 </section>;
}
