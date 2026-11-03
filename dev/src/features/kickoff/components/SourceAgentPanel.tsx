import {useMutation,useMutationState} from '@tanstack/react-query';
import {LoaderCircle,Bot,CheckCircle2} from 'lucide-react';
import {useEffect,useState,type ReactNode} from 'react';
import {useWorkspace} from '../context';
import {tableById,type AnalysisDomain,type SourceAnalysis} from '../lib/api';
import {apiError} from '../../../lib/api/request';
import {Button,DataTable,Modal,Notice} from './ui';
const domains:{id:AnalysisDomain;label:string}[]=[{id:'equipment',label:'设备、组件与连接'},{id:'facility',label:'机房与落位'},{id:'cabling',label:'布线规则与物料'},{id:'network',label:'网络规划'},{id:'power',label:'电力与冷却'},{id:'provisioning',label:'装机准备'}];
const states={idle:'尚未分析',pending:'已排队',running:'正在分析',succeeded:'候选已生成',failed:'分析失败'};
export function SourceAgentPanel({sourceId,sourceName,onResult,children}:{sourceId:string;sourceName:string;onResult:()=>void;children?:ReactNode}){
 const {api,state:workspace,can,notify,inspect}=useWorkspace();
 const [selected,setSelected]=useState<AnalysisDomain[]>(domains.map(d=>d.id));
 const [report,setReport]=useState<SourceAnalysis>(),[error,setError]=useState(''),[busy,setBusy]=useState(false),[retry,setRetry]=useState(0);
 const [tables,setTables]=useState<string[]>([]),[preview,setPreview]=useState<string>();
 const [scopeOpen,setScopeOpen]=useState(false);
 const adoptionKey=['kickoff-adoption',workspace.projectId,sourceId];
 const pending=useMutationState({filters:{mutationKey:adoptionKey,status:'pending'},select:m=>m.state.submittedAt});
 const adopting=pending.length>0;
 const adoption=useMutation({mutationKey:adoptionKey,mutationFn:({jobId,tableIds}:{jobId:string;tableIds:string[]})=>api.adoptSourceAnalysis!(sourceId,jobId,tableIds),onSuccess:result=>{notify(`已采用 ${result.adoptedTableIds.length} 类候选；${result.skippedTableIds.length} 类保持原值`);setTables([]);},onError:e=>setError(e.message)});

 const completed=useMutationState({filters:{mutationKey:adoptionKey,status:'success'},select:m=>m.state.submittedAt});
 const completedAt=Math.max(0,...completed);
 useEffect(()=>{if(!completedAt)return;let active=true;void Promise.resolve(api.refresh?.()).then(()=>{if(active){setRetry(v=>v+1);onResult();}}).catch(e=>{if(active)setError(e.message)});return()=>{active=false};},[api,sourceId,completedAt]);
 const failures=useMutationState({filters:{mutationKey:adoptionKey,status:'error'},select:m=>({at:m.state.submittedAt,message:(m.state.error as Error)?.message})});
 const lastFailure=failures.filter(f=>f.at>completedAt).at(-1);
 const supported=/\.xlsx$/i.test(sourceName);
 const running=report?.state==='pending'||report?.state==='running';
 useEffect(()=>{let live=true;let timer:ReturnType<typeof setTimeout>|undefined;let terminalNotified=false;
  setReport(undefined);setTables([]);setError('');setPreview(undefined);
  const read=async()=>{try{const value=await api.sourceAnalysis!(sourceId);if(!live)return;setReport(value);if(value.state==='pending'||value.state==='running'){timer=setTimeout(read,2000);}else if(value.state==='succeeded'&&!terminalNotified){terminalNotified=true;const needs=new Set(value.coverage.filter(c=>c.state==='needs_input').map(c=>c.step));setSelected(domains.filter((_,i)=>needs.has(i+1)).map(d=>d.id));setTables(value.drafts.filter(d=>d.rowCount>0).map(d=>d.tableId));onResult();}else if(value.state==='failed'&&!terminalNotified){terminalNotified=true;onResult();}}catch(e){if(live&&(e as Error).name!=='AbortError')setError((e as Error).message);}};
  if(api.sourceAnalysis)void read();return()=>{live=false;if(timer)clearTimeout(timer)};
 },[api,sourceId,retry]);
 const start=async(scope:AnalysisDomain[])=>{setBusy(true);setError('');try{await api.startSourceAnalysis!(sourceId,scope);setScopeOpen(false);setRetry(v=>v+1);onResult();notify('分析任务已排队；候选生成后仍需确认采用。');}catch(e){setError((e as Error).message);}finally{setBusy(false)}};
 const draft=report?.drafts.find(d=>d.tableId===preview);
 if(!api.sourceAnalysis||!api.startSourceAnalysis||!api.adoptSourceAnalysis)return <>{children}</>;
 return <div className="source-analysis">
  <div className="source-task-heading"><Bot size={18}/><div><b>Agent 识别任务</b><small>当前资料 · 分域提取与候选核对</small></div></div>
  <div className="source-analysis-flow">
   {children}
  <div className="source-analysis-toolbar">
   <span className="source-analysis-status" role="status">{report?states[report.state]:'正在读取分析状态…'}{report?.state==='failed'&&report.errorCode?' · '+apiError(report.errorCode):''}</span>
   <div className="actions">
    <Button disabled={!supported||!report||busy||running||adopting||!can('bom:import')} onClick={()=>setScopeOpen(true)}>分析范围</Button>
    <Button kind="primary" disabled={!supported||!report||busy||running||adopting||!can('bom:import')} onClick={()=>void start(domains.map(d=>d.id))}>{busy?'正在排队…':running?states[report!.state]:report?.state==='succeeded'?'重新分析资料':'开始分析'}</Button>
   </div>
  </div>
  </div>
  {(running||adopting)&&<div className="agent-execution-status" role="status"><LoaderCircle size={17} className="agent-spinner"/><div><b>{adopting?'正在采用候选':'Agent 正在识别资料'}</b><p>{adopting?'正在合并候选并保存；可切换页面，返回后继续查看。':'任务在后台执行，结果就绪后展示候选与来源。'}</p><div className="agent-progress" role="progressbar" aria-label={adopting?'候选采用处理中':'资料分析执行中'}><span/></div><small>等待服务端完成，当前未返回百分比</small></div></div>}
  {report?.state==='succeeded'&&!adopting&&<p className="agent-completion"><CheckCircle2 size={15}/>识别任务已完成 · {report.drafts.length?`${report.drafts.length} 类候选待核对采用`:'当前无待采用候选'}</p>}
  {!supported&&<Notice tone="warning">当前自动分析支持 XLSX；其他格式保留为附件。</Notice>}
  {(error||lastFailure)&&<Notice tone="error">{error||lastFailure?.message}<Button kind="ghost" onClick={()=>setRetry(v=>v+1)}>重新读取</Button></Notice>}
  {scopeOpen&&<Modal title="选择分析范围" description="只重新识别所选内容，其他结果保留；已有人工确认值不会被自动覆盖。" close={()=>setScopeOpen(false)}>
   <div className="agent-domain-options">{domains.map(d=><label key={d.id}><input type="checkbox" checked={selected.includes(d.id)} onChange={e=>setSelected(v=>e.target.checked?[...v,d.id]:v.filter(x=>x!==d.id))}/>{d.label}</label>)}</div>
   <div className="actions"><Button onClick={()=>setScopeOpen(false)}>取消</Button><Button kind="primary" disabled={!selected.length||busy||running} onClick={()=>void start(selected)}>分析所选内容</Button></div>
  </Modal>}
  {report&&report.state!=='idle'&&!!(report.missing.length||report.coverage.length||report.drafts.length)&&<details className="source-analysis-results" open={report.state==='succeeded'&&report.drafts.length>0?true:undefined}>
   <summary>识别结果 <span>{report.drafts.length} 类待核对候选 · {report.missingTotal??report.missing.length} 项待补充</span></summary>
  {!!report?.missing.length&&<Notice tone="warning"><details><summary>{report.missingTotal??report.missing.length} 项缺失或未覆盖信息（当前显示 {report.missing.length} 项）</summary>{report.missing.map((message,i)=><p key={i}>{message}</p>)}</details></Notice>}
  {!!report?.coverage.length&&<details className="agent-region-coverage"><summary>查看区域覆盖结果与原因（当前显示 {report.coverage.length}/{report.coverageTotal??report.coverage.length} 项）</summary><DataTable data={report.coverage} pageSize={5} columns={[{accessorKey:'region',header:'区域'},{id:'state',header:'处理结果',cell:({row})=>({complete:'已覆盖',ready:'已就绪',needs_input:'待补充',unsupported:'暂不支持',not_applicable:'不适用'}[row.original.state]??row.original.state)},{accessorKey:'rows',header:'候选行数'},{accessorKey:'reason',header:'依据或未完成原因'}]}/></details>}
  {!!report?.drafts.length&&<><DataTable data={report.drafts} pageSize={6} columns={[
   {id:'select',header:'采用',enableSorting:false,cell:({row})=><input type="checkbox" aria-label={'采用 '+row.original.tableId} disabled={adopting||!row.original.rowCount||report.state!=='succeeded'} checked={tables.includes(row.original.tableId)} onChange={e=>setTables(v=>e.target.checked?[...v,row.original.tableId]:v.filter(x=>x!==row.original.tableId))}/>},
   {id:'table',header:'候选内容',cell:({row})=>tableById(row.original.tableId)?.name||({devices:'设备',ports:'端口',connections:'双端连接',components:'组件'}[row.original.tableId]??row.original.tableId)},
   {accessorKey:'rowCount',header:'候选数量'},{id:'issues',header:'核对事项',cell:({row})=>row.original.issues.length+' 项（预览范围）'},
   {id:'view',header:'来源与内容',cell:({row})=><Button kind="ghost" onClick={()=>setPreview(row.original.tableId)}>核对候选</Button>},
  ]}/><div className="actions"><span className="muted small-copy">仅补充新增和空字段，已有值保留；采用不等于CMDB发布。</span><Button kind="primary" disabled={adopting||running||!tables.length||report.state!=='succeeded'||!can('bom:edit')} onClick={()=>{if(report.jobId&&!adopting)adoption.mutate({jobId:report.jobId,tableIds:tables})}}>{adopting?'正在采用…':'确认采用所选候选'}</Button></div></>}
  </details>}
  {draft&&<Modal title={'核对候选 · '+(tableById(draft.tableId)?.name??draft.tableId)} description={`共 ${draft.rowCount} 行；显示最多 ${draft.previewLimit} 行。完整候选与来源保存在服务端。`} close={()=>setPreview(undefined)} wide><DataTable data={draft.rows} pageSize={5} columns={[...new Set(draft.rows.flatMap(Object.keys))].map(key=>({accessorKey:key,header:tableById(draft.tableId)?.fields.find(f=>f.key===key)?.label??key}))}/>{draft.evidence.map((e,i)=><div className="source-agent-evidence" key={i}><b>候选第 {e.row} 行</b>{Object.entries(e.fields).map(([field,cell])=><button className="text-link" key={field} onClick={()=>inspect('evidence',`${sourceId}:${cell.sheet}:${cell.cell}`,{sourceId,sheet:cell.sheet,cell:cell.cell,method:'source',note:`字段 ${field}${cell.normalization?' · '+cell.normalization:''}${cell.formula?' · 公式来源':''}${cell.normalizationEvidence?' · '+cell.normalizationEvidence.rule+' · 结果 '+(cell.normalizationEvidence.normalized_value??'')+' · '+(cell.normalizationEvidence.ticks?.length??0)+'个刻度来源':''}`} )}>{field} · {cell.sheet}!{cell.cell}{cell.normalizationEvidence&&<small> · 推导 {cell.normalizationEvidence.normalized_value} {cell.normalizationEvidence.ticks?.length?'('+cell.normalizationEvidence.ticks.length+'个U刻度)':''}</small>}</button>)}</div>)}{draft.issues.map((issue,i)=><Notice tone="warning" key={i}>第{issue.row}行 {issue.field}：{issue.message}</Notice>)}</Modal>}
 </div>;
}
