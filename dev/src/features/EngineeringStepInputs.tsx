import {useEffect,useRef,useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {Button} from '../components/ui';
import {engineeringInputCatalog,engineeringInputTemplateURL,previewEngineeringInput,type EngineeringInputPreview} from '../lib/api/engineering';

const steps=['设备关联','机房与落位','连线与布线','逻辑网络','电力相关'];
export function EngineeringStepInputs({project,canEdit}:{project:string;canEdit:boolean}) {
 const [step,setStep]=useState(1),[tableID,setTableID]=useState(''),[csv,setCSV]=useState(''),[result,setResult]=useState<EngineeringInputPreview|null>(null),[error,setError]=useState(''),[busy,setBusy]=useState(false);
 const scope=useRef(project);scope.current=project;const pending=useRef<AbortController|null>(null);
 const catalog=useQuery({queryKey:['engineering-input-catalog',project],queryFn:({signal})=>engineeringInputCatalog(project,signal)});
 const tables=catalog.data?.tables.filter(t=>t.step===step)||[];const table=tables.find(t=>t.id===tableID)||tables[0];
 useEffect(()=>{setCSV('');setResult(null);setError('');setBusy(false);pending.current?.abort();return()=>pending.current?.abort()},[project,step,tableID]);
 async function preview(){if(!table)return;pending.current?.abort();const controller=new AbortController();pending.current=controller;const frozen=project;setBusy(true);setError('');setResult(null);try{const value=await previewEngineeringInput(project,{table_id:table.id,csv},controller.signal);if(scope.current===frozen&&!controller.signal.aborted)setResult(value)}catch(e){if(!controller.signal.aborted&&scope.current===frozen)setError((e as Error).message)}finally{if(!controller.signal.aborted&&scope.current===frozen)setBusy(false)}}
 return <section className="panel padded engineering-step-inputs">
  <div className="work-heading"><div><h3>五步标准输入</h3><p>下载模板，填写或导入资料，先定位缺项与格式问题。</p></div><span>契约版本 {catalog.data?.version||'—'}</span></div>
  <nav className="tabs" aria-label="开局五步">{steps.map((name,i)=><button key={name} aria-current={step===i+1?'step':undefined} className={step===i+1?'active':''} onClick={()=>{setStep(i+1);setTableID('')}}>{i+1}. {name}</button>)}</nav>
  {catalog.isPending&&<p aria-busy="true">正在读取输入要求…</p>}
  {catalog.error&&<p role="alert">{catalog.error.message} <Button onClick={()=>void catalog.refetch()}>重试</Button></p>}
  {table&&<>
   <div className="work-heading"><label>资料类型 <select value={table.id} onChange={e=>setTableID(e.target.value)}>{tables.map(t=><option key={t.id} value={t.id}>{t.name}</option>)}</select></label><div><a className="btn" href={engineeringInputTemplateURL(project,table.id,'xlsx')}>下载 XLSX 模板</a> <a className="btn" href={engineeringInputTemplateURL(project,table.id,'csv')}>下载 CSV 模板</a></div></div>
   <p className="notice">{table.condition}</p>
   <details><summary>字段要求 · {table.fields.length} 列</summary><div className="table-wrap"><table><thead><tr><th>列名</th><th>要求</th><th>类型</th></tr></thead><tbody>{table.fields.map(f=><tr key={f.code}><td>{f.label}</td><td>{f.required?'必填':'按适用条件补充'}</td><td>{({text:'文本',mac:'MAC 地址',number:'数值',positive:'正数',positive_integer:'正整数'})[f.kind]}</td></tr>)}</tbody></table></div></details>
   <label>选择已填写的 CSV <input type="file" accept=".csv,text/csv" disabled={!canEdit||busy} onChange={e=>{const file=e.target.files?.[0];if(!file)return;const frozen=project;void file.text().then(v=>{if(scope.current===frozen){setCSV(v);setResult(null)}});e.target.value=''}}/></label>
   <label style={{display:'block'}}>标准数据（CSV）<textarea aria-label="标准数据 CSV" value={csv} disabled={!canEdit||busy} rows={7} style={{width:'100%',fontFamily:'monospace'}} onChange={e=>{setCSV(e.target.value);setResult(null)}} placeholder={table.fields.map(f=>f.label).join(',')}/></label>
   <Button variant="primary" disabled={!canEdit||!csv.trim()||busy} onClick={()=>void preview()}>{busy?'正在校验…':'校验输入'}</Button>
   {!canEdit&&<p>当前账号没有编辑权限。</p>}{error&&<p role="alert">{error}</p>}
   {result&&<><p role="status">{result.valid?'格式校验通过':'需要补充或修正'} · {result.rows.length} 行 · {result.issues.length} 个问题</p><p className="muted">此预览尚未保存模型；身份、引用、关系及工程校验在确认建模时执行。</p>{result.issues.length>0&&<div className="table-wrap"><table><thead><tr><th>行</th><th>字段</th><th>问题</th></tr></thead><tbody>{result.issues.map((issue,i)=><tr key={i}><td>{issue.row||'整表'}</td><td>{table.fields.find(f=>f.code===issue.field)?.label||'整行'}</td><td>{issue.message}</td></tr>)}</tbody></table></div>}<details><summary>查看前 20 行</summary><div className="table-wrap"><table><thead><tr>{table.fields.map(f=><th key={f.code}>{f.label}</th>)}</tr></thead><tbody>{result.rows.slice(0,20).map((row,i)=><tr key={i}>{row.map((v,j)=><td key={j}>{v||'—'}</td>)}</tr>)}</tbody></table></div></details></>}
  </>}
 </section>
}
