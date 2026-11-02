import { useRef, useState } from 'react';
import { Download, Plus, Save, Trash2, Upload } from 'lucide-react';
import { useWorkspace } from '../context';
import { tableById, type ModelRow, type ImportPreview } from '../lib/api';
import { exportWorkbook, readModelFile } from '../lib/api/workbook';
import { Button, Modal, Notice } from './ui';

export function ModelEditor({tableId,close}:{tableId:string;close:()=>void}){
  const {api,state,run}=useWorkspace();
  const def=tableById(tableId);
  const existing=state.models.find(m=>m.tableId===tableId);
  const [rows,setRows]=useState<ModelRow[]>(()=>structuredClone(existing?.rows??[]));
  const [preview,setPreview]=useState<ImportPreview>(),[error,setError]=useState(''),[source,setSource]=useState('页面编辑');
  const [page,setPage]=useState(0),[dirty,setDirty]=useState(false);
  const picker=useRef<HTMLInputElement>(null);
  const pageSize=12,pageRows=rows.slice(page*pageSize,(page+1)*pageSize);
  async function importFile(file:File){try{setError('');setPreview(await readModelFile(file,def));setSource(file.name);}catch(e){setError((e as Error).message);}}
  const finish=()=>{if(dirty&&!window.confirm('有尚未应用的编辑。放弃这些编辑并关闭？'))return;close();};
  return <Modal title={def.name+' · 编辑输入'} description={def.description} close={finish} wide>
    <div className="editor-tools"><div><b>{rows.length} 行</b><span>空配置可以保留，使用时再检查对应条件。</span></div><div className="actions"><Button onClick={()=>exportWorkbook(def,[],true)}><Download size={14}/>下载XLSX模板</Button><Button onClick={()=>picker.current?.click()}><Upload size={14}/>导入XLSX / CSV</Button><Button onClick={()=>{setRows([...rows,Object.fromEntries(def.fields.map(f=>[f.key,'']))]);setPage(Math.floor(rows.length/pageSize));setDirty(true);}}><Plus size={14}/>新增行</Button></div></div>
    <input ref={picker} type="file" hidden accept=".xlsx,.csv" onChange={e=>{const file=e.target.files?.[0];if(file)void importFile(file);e.target.value='';}}/>
    {error&&<Notice tone="error">{error}</Notice>}
    {preview&&<div className="import-preview"><div><b>导入预览 · {preview.sheet}</b><span>{preview.rows.length} 行 · {preview.errors.length?'需要处理表头':'字段已匹配'}</span></div>{preview.errors.map(e=><Notice key={e} tone="error">{e}</Notice>)}<div className="table-scroll"><table><thead><tr>{def.fields.map(f=><th key={f.key}>{f.label}</th>)}</tr></thead><tbody>{preview.rows.slice(0,3).map((r,i)=><tr key={i}>{def.fields.map(f=><td key={f.key}>{r[f.key]||'—'}</td>)}</tr>)}</tbody></table></div><div className="actions"><Button onClick={()=>setPreview(undefined)}>取消导入</Button><Button kind="primary" disabled={!!preview.errors.length} onClick={()=>{setRows(preview.rows);setPreview(undefined);setPage(0);setDirty(true);}}>采用 {preview.rows.length} 行到编辑区</Button></div></div>}
    <div className="editor-grid table-scroll"><table><thead><tr><th>#</th>{def.fields.map(f=><th key={f.key}><b>{f.label}</b><code>{f.key}</code></th>)}<th>操作</th></tr></thead><tbody>{pageRows.map((row,index)=>{const absolute=page*pageSize+index;return <tr key={absolute}><td>{absolute+1}</td>{def.fields.map(f=><td key={f.key}><input aria-label={`${f.label} 第${absolute+1}行`} inputMode={f.kind==='positive'||f.kind==='number'?'decimal':undefined} value={row[f.key]??''} placeholder="待补充" onChange={e=>{const next=rows.map((r,i)=>i===absolute?{...r,[f.key]:e.target.value}:r);setRows(next);setDirty(true);}}/></td>)}<td><button className="icon-button" aria-label={`删除第${absolute+1}行`} onClick={()=>{setRows(rows.filter((_,i)=>i!==absolute));setPage(Math.min(page,Math.max(0,Math.ceil((rows.length-1)/pageSize)-1)));setDirty(true);}}><Trash2 size={14}/></button></td></tr>;})}</tbody></table>{!rows.length&&<div className="editor-empty">从 XLSX 导入，或新增一行开始填写。暂时没有依据时可以留空。</div>}</div>
    <div className="editor-pagination"><span>{rows.length>pageSize?'分段编辑，所有已填内容会一起保存':'每个字段只保存明确提供的值'}</span><div className="actions"><Button disabled={page===0} onClick={()=>setPage(page-1)}>上一页</Button><span>{page+1} / {Math.max(1,Math.ceil(rows.length/pageSize))}</span><Button disabled={(page+1)*pageSize>=rows.length} onClick={()=>setPage(page+1)}>下一页</Button></div></div>
    <div className="editor-impact"><b>本次输入用于</b><span>{def.consumer}</span><small>其他工程域可以继续工作；计算结果按依赖标记过期。</small></div>
    <div className="modal-actions"><Button onClick={finish}>取消</Button><Button kind="primary" disabled={!!preview} onClick={async()=>{if(await run('工程输入已更新',()=>api.saveModel(tableId,rows,source))){setDirty(false);close();}}}><Save size={14}/>应用输入</Button></div>
  </Modal>;
}
