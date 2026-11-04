import { useEffect, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { ArrowRight, Check, ChevronRight, FileSpreadsheet, FolderOpen, Plus, Search, Trash2 } from 'lucide-react';
import './sources.css';
import { understandingStepIds, terminationNames, understandingStates, understandingDescription, identityGuidance } from '../lib/understanding';
import {SourceAgentPanel} from '../components/SourceAgentPanel';
import { useWorkspace } from '../context';
import type { SourceRegionPreview, SourceUnderstanding } from '../lib/api';
import { Badge, Button, Empty, Heading, Notice, Tabs, formatDate } from '../components/ui';

const columnLabel=(column:number)=>{let label='';for(let n=column;n>0;n=Math.floor((n-1)/26))label=String.fromCharCode(65+(n-1)%26)+label;return label;};
const sourceLabels:Record<string,string> = {historical:'历史资料复用',partial:'部分识别',recognized:'已有识别结果',awaiting_analysis:'待分析',needs_input:'待补充'};
const objectKind=(kind:string):'device'|'port'|'connection'|'component'|'evidence'=>['device','port','connection','component'].includes(kind)?kind as 'device'|'port'|'connection'|'component':'evidence';

export default function Sources(){
 const {api,state:s,go,notify,inspect,can}=useWorkspace();
 const [params,setParams]=useSearchParams();
 const source=s.sources.find(v=>v.id===params.get('source'))??s.sources[0];
 const sourceId=source?.id;
 const [report,setReport]=useState<SourceUnderstanding>();
 const [regionId,setRegionId]=useState('');
 const [preview,setPreview]=useState<SourceRegionPreview>();
 const [offset,setOffset]=useState(0),[view,setView]=useState<'source'|'normalized'>('source');
 const [loading,setLoading]=useState(false),[previewLoading,setPreviewLoading]=useState(false);
 const [error,setError]=useState(''),[previewError,setPreviewError]=useState('');
 const [retry,setRetry]=useState(0),[previewRetry,setPreviewRetry]=useState(0);
 const [uploading,setUploading]=useState(false);
 const picker=useRef<HTMLInputElement>(null);
 const currentReport=report?.sourceId===sourceId?report:undefined;
 const regions=currentReport?.flowVersion==='termination-v1'?currentReport.regions:[];
 const sourceState=currentReport?.flowVersion==='termination-v1'&&!currentReport.revisionId?'awaiting_analysis':source?.state;
 const region=regions.find(r=>r.id===regionId)??regions[0];
 useEffect(()=>{let active=true;setLoading(report?.sourceId!==sourceId);setError('');if(report?.sourceId!==sourceId){setReport(undefined);setOffset(0);setRegionId('');}
  if(!sourceId){setLoading(false);return;}
  void api.sourceUnderstanding(sourceId).then(data=>{if(active){if(report?.revisionId!==data.revisionId){setOffset(0);setRegionId('');}setReport(data);}}).catch(e=>{if(active)setError(e.message);}).finally(()=>{if(active)setLoading(false);});
  return()=>{active=false;};
 },[api,sourceId,s.revision,retry]);
 useEffect(()=>{let active=true;setPreview(undefined);setPreviewError('');
  if(!sourceId||!region||!currentReport)return;
  setPreviewLoading(true);
  void api.sourceRegionPreview(sourceId,region.id,offset,currentReport.revisionId,view).then(data=>{if(active){if(data.revisionId!==currentReport.revisionId)throw Error('资料版本已变化，请重新读取识别结果');setPreview(data);}}).catch(e=>{if(active)setPreviewError(e.message);}).finally(()=>{if(active)setPreviewLoading(false);});
  return()=>{active=false;};
 },[api,sourceId,region?.id,currentReport?.revisionId,offset,view,previewRetry]);
 const add=async(files:File[])=>{setError('');setUploading(true);try{await api.addSources(files);notify('资料已添加，可点击开始分析');}catch(e){setError((e as Error).message);}finally{setUploading(false);}};
 const columns=[...new Set(preview?.rows.flatMap(row=>row.cells.map(cell=>cell.column))??[])].sort((a,b)=>a-b);
 const sourceEvidence=(cell:string)=>({sourceId:sourceId!,sheet:region?.sheet??'',cell,method:'source' as const});
 return <>
  <Heading eyebrow="SOURCES & INTELLIGENCE" title="资料与识别" description="从端接表建立设备与连接，关联机柜、U 位和线缆；采用后进入数据核对，再确认入库。" action={<Button kind="primary" onClick={()=>picker.current?.click()} disabled={uploading||!can('bom:import')}><Plus size={15}/>添加资料</Button>}/>
  <input ref={picker} hidden type="file" multiple accept=".xlsx" onChange={e=>{void add([...e.target.files??[]]);e.target.value='';}}/>

  <div className={'source-layout source-page-layout'+(!source?' source-page-empty':'')}><aside className="source-rail"><div className="rail-heading"><b>资料包</b><span>{s.sources.length}</span></div>{s.sources.map(item=><button className={'source-file '+(sourceId===item.id?'selected':'')} key={item.id} onClick={()=>{setParams({source:item.id});setRegionId('');setOffset(0);}}><FileSpreadsheet size={20}/><span><b title={item.name}>{item.name}</b><small>{item.size} · v{item.version}</small><Badge state={item.id===sourceId?sourceState??item.state:item.state}>{sourceLabels[(item.id===sourceId?sourceState:item.state)??'awaiting_analysis']}</Badge></span></button>)}<div className="source-rail-foot"><FolderOpen size={15}/><span>端接表建立设备与连接，增补资料补齐位置和规格。</span></div></aside>
   <div className="source-main">{source?<>
    <div className="source-header"><FileSpreadsheet size={22}/><div><h2 title={source.name}>{source.name}</h2><p>{source.size} · 添加于 {formatDate(source.createdAt)}</p></div><Badge state={sourceState??'awaiting_analysis'}>{sourceLabels[sourceState??'awaiting_analysis']??sourceState}</Badge><Button onClick={()=>go('review')}>核对识别结果<ArrowRight size={14}/></Button></div>
    <SourceAgentPanel key={source.id} sourceId={source.id} sourceName={source.name} onResult={()=>setRetry(n=>n+1)}>{analysis=>{const stageReport=currentReport?.origin==='agent'&&analysis?.jobId&&currentReport.revisionId!==analysis.jobId?undefined:currentReport;return <div className="recognition-pipeline" aria-label="资料理解五步状态">{understandingStepIds.map((id,i)=>{const step=stageReport?.flowVersion==='termination-v1'?stageReport.steps.find(x=>x.id===id):undefined;return <div key={id} className={`recognition-step ${step?.state??'pending'}`} title={step?.note??'本流程尚未执行；历史任务的证据可在右侧执行记录查看'}><span>{step?.state==='complete'?<Check size={12}/>:<Search size={12}/>}</span><b>{terminationNames[id]}<small>{step?understandingStates[step.state]:loading?'读取中':'待分析'}{step&&step.total>0?` ${step.completed}/${step.total}`:''}</small></b>{i<4&&<ChevronRight size={13}/>}</div>;})}<small className="recognition-explanation">{stageReport?.flowVersion==='termination-v1'?understandingDescription(stageReport):'按端接关系建立设备与连接，再关联位置和线缆。点击右侧开始分析，生成设备与连接候选。'}</small></div>;}}</SourceAgentPanel>
    <div className="source-understanding-note">{identityGuidance}{currentReport?.limitations.length? <details><summary>识别范围与限制</summary>{currentReport.limitations.map((note,i)=><p key={i}>{note}</p>)}</details>:null}</div>
    {error&&<Notice tone="error">{error}<Button onClick={()=>setRetry(n=>n+1)}>重新读取</Button></Notice>}
    {loading?<div className="empty" aria-busy="true" role="status">正在读取资料理解与历史证据…</div>:currentReport&&<>

     {regions.length?<div className="source-region-layout"><div className="region-rail"><small>工作表与数据区域</small>{regions.map(r=><button className={region?.id===r.id?'active':''} key={r.id} onClick={()=>{setRegionId(r.id);setOffset(0);setPreview(undefined);}}><span><b>{r.sheet}</b><small>{r.range} · {r.category==='unknown'?'待分类':r.category}</small></span><span className="region-count">{r.rowCount}</span></button>)}</div><div className="region-content">
      <div className="region-heading"><div><h3>{region?.sheet}</h3><p>{region?.note}</p></div><Badge state={region?.state??'needs_input'}/></div>
      <Tabs items={[{id:'source',label:'来源数据'},{id:'normalized',label:'标准化对象'}]} value={view} onChange={value=>{setView(value as 'source'|'normalized');setOffset(0);setPreview(undefined);}}/>
      {previewLoading?<div className="empty" role="status" aria-busy="true">正在读取所选区域…</div>:previewError?<Notice tone="error">{previewError}<Button onClick={()=>setPreviewRetry(n=>n+1)}>重新加载区域</Button><Button onClick={()=>setRetry(n=>n+1)}>刷新资料版本</Button></Notice>:preview&&<>
       {view==='source'?<div className="sheet-preview"><table><thead><tr><th aria-label="Excel 行号"/>{columns.map(col=><th key={col}>{columnLabel(col)}</th>)}</tr></thead><tbody>{preview.rows.map(row=><tr key={row.row}><th>{row.row}</th>{columns.map(col=>{const cell=row.cells.find(c=>c.column===col);return <td key={col} className={cell?.fields.some(field=>field.includes('_candidate'))?'candidate-cell':cell?.fields.length?'highlight-cell':''}>{cell&&<button className="source-cell" title={[cell.cell,!cell.sensitive&&cell.formula?`公式：${cell.formula}`:'',...cell.fields].filter(Boolean).join(' · ')} onClick={()=>inspect('evidence',`${source.id}:${cell.cell}`,sourceEvidence(cell.cell))}>{cell.sensitive?'[敏感信息已遮蔽]':cell.value||'—'}</button>}</td>;})}</tr>)}</tbody></table>{!preview.rows.length&&<Empty title="此区域暂无可预览单元格" description={region?.note??'来源范围仍然保留，可继续核对历史对象。'}/>}</div>:<div className="source-normalized">{preview.rows.flatMap(row=>row.objects??[]).map(item=><button className="relationship-row" key={item.id} onClick={()=>{const kind=objectKind(item.kind);if(kind==='evidence')inspect('evidence',item.id,{...sourceEvidence(region?.range??''),note:`区域来源，非字段证据。对象：${item.label||item.id}（${item.kind}）；此处仅表示该对象与当前来源区域有关联。`});else inspect(kind,item.id);}}><span><b>{item.label||item.id}</b><small>{item.kind} · {item.fields.map(f=>`${f.field}: ${f.value}`).join('、')||'对象级证据'} · {item.evidence} 条证据</small></span><ArrowRight size={14}/></button>)}{!preview.rows.some(row=>row.objects?.length)&&<Empty title="暂无关联工程对象" description="原始区域仍保留；没有对应字段证据不代表该区域已处理完成。"/>}</div>}
       <div className="source-pagination"><span>{view==='source'?'来源行':'工程对象'} {preview.total?preview.offset+1:0}–{Math.min(preview.offset+preview.limit,preview.total)} / {preview.total}</span><Button disabled={offset===0} onClick={()=>setOffset(Math.max(0,offset-20))}>上一页</Button><Button disabled={preview.offset+preview.limit>=preview.total} onClick={()=>setOffset(offset+20)}>下一页</Button></div>
      </>}
      <div className="mapping-strip"><span>来源区域</span><b>{region?.range}</b><ArrowRight size={15}/><span>工程类别</span><b>{region?.categories.join('、')||'待分类'}</b><span className="mapping-count">{region?.objectCount??0} 个对象</span></div>
      <div className="source-note"><span>{region?.coveredCells??0} / {region?.totalCells??0} 个单元格有证据 · {region?.fieldEvidenceCount??0} 条字段证据。对象级证据不等于字段已核对；标记 _candidate 的字段仅为精确值匹配候选，未经人工确认。</span></div>
      {!!region?.mappings.length&&<div className="source-mappings">{region.mappings.map((m,i)=><span key={i}>{m.field} · {m.kind==='field'?'字段':'对象'} {m.count}</span>)}</div>}
      {!!region?.issues.length&&<Notice tone="warning">{region.issues.map((issue,i)=><p key={i}>{issue}</p>)}</Notice>}
     </div></div>:<Empty title="暂无数据区域结果" description="开始分析后，在此查看端接区域、设备关联与字段来源。"/>}
    </>}
    <div className="source-footer"><span>SHA-256 <code>{source.hash.slice(0,28)}…</code></span>{source.state==='awaiting_analysis'&&<Button kind="ghost" disabled={!can('bom:import')} onClick={()=>{void api.removeSource(source.id).then(()=>setParams({})).catch(e=>setError(e.message));}}><Trash2 size={13}/>移除待分析资料</Button>}<button className="text-link" onClick={()=>notify('识别、采用与入库分别记录版本，确认后的数据保留来源。')}>来源版本说明</button></div>
   </>:<section className="source-empty-state"><div className="source-empty-icon"><FileSpreadsheet size={32}/></div><h2>从一份开局资料开始</h2><p>添加 XLSX，识别设备、连接与落位信息，再核对候选结果。</p><Button kind="primary" disabled={uploading||!can('bom:import')} onClick={()=>picker.current?.click()}><Plus size={15}/>{uploading?'正在上传…':'选择工程资料'}</Button><small>仅支持上传 XLSX 工程资料</small><div className="source-empty-guide"><span><b>01</b> 添加资料</span><ChevronRight size={14}/><span><b>02</b> Agent 识别</span><ChevronRight size={14}/><span><b>03</b> 核对并采用</span></div>{error&&<Notice tone="error">{error}</Notice>}</section>}</div>
  </div>
 </>;
}
