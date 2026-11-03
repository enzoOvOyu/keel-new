import {useMutation,useMutationState} from '@tanstack/react-query';
import {LoaderCircle} from 'lucide-react';
import {useEffect,useState,type ReactNode} from 'react';
import {useWorkspace} from '../context';
import {tableById,type AnalysisDomain,type SourceAnalysis,type SourceTaskPage,type SourceTaskDetail} from '../lib/api';
import {apiError} from '../../../lib/api/request';
import {Button,DataTable,Modal,Notice,formatDate} from './ui';
const domains:{id:AnalysisDomain;label:string}[]=[{id:'equipment',label:'设备、组件与连接'},{id:'facility',label:'机房与落位'},{id:'cabling',label:'布线规则与物料'},{id:'network',label:'网络规划'},{id:'power',label:'电力与冷却'},{id:'provisioning',label:'装机准备'}];
const states={idle:'尚未分析',pending:'已排队',running:'正在分析',succeeded:'候选已生成',failed:'分析失败'};
export function SourceAgentPanel({sourceId,sourceName,onResult,children}:{sourceId:string;sourceName:string;onResult:()=>void;children?:ReactNode}){
 const {api,state:workspace,can,notify,inspect}=useWorkspace();
 const [selected,setSelected]=useState<AnalysisDomain[]>(domains.map(d=>d.id));
 const [report,setReport]=useState<SourceAnalysis>(),[error,setError]=useState(''),[busy,setBusy]=useState(false),[retry,setRetry]=useState(0);
 const [tables,setTables]=useState<string[]>([]),[preview,setPreview]=useState<string>();
 const [scopeOpen,setScopeOpen]=useState(false);
 const [tasks,setTasks]=useState<SourceTaskPage>(),[taskOffset,setTaskOffset]=useState(0),[taskRetry,setTaskRetry]=useState(0);
 const [tasksLoading,setTasksLoading]=useState(false),[tasksError,setTasksError]=useState('');
 const [selectedTask,setSelectedTask]=useState<string>(),[taskDetail,setTaskDetail]=useState<SourceTaskDetail>();
 const [taskLoading,setTaskLoading]=useState(false),[taskError,setTaskError]=useState(''),[historyReport,setHistoryReport]=useState<SourceAnalysis>();
 const [previewAnalysis,setPreviewAnalysis]=useState<SourceAnalysis>();
 const adoptionKey=['kickoff-adoption',workspace.projectId,sourceId];
 const pending=useMutationState({filters:{mutationKey:adoptionKey,status:'pending'},select:m=>m.state.submittedAt});
 const adopting=pending.length>0||report?.adoption?.state==='pending'||report?.adoption?.state==='running';
 const adoption=useMutation({mutationKey:adoptionKey,mutationFn:({jobId,tableIds}:{jobId:string;tableIds:string[]})=>api.adoptSourceAnalysis!(sourceId,jobId,tableIds),onSuccess:task=>{notify('采用任务已提交，正在后台执行');setReport(v=>v?{...v,adoption:task}:v);setRetry(v=>v+1);},onError:e=>setError(e.message)});
 useEffect(()=>{if(report?.adoption?.state!=='succeeded')return;let active=true;void Promise.resolve(api.refresh?.()).then(()=>{if(active){onResult();}}).catch(e=>{if(active)setError(e.message)});return()=>{active=false};},[api,sourceId,report?.adoption?.jobId,report?.adoption?.state]);
 const supported=/\.xlsx$/i.test(sourceName);
 const running=report?.state==='pending'||report?.state==='running';
 useEffect(()=>{let live=true;let timer:ReturnType<typeof setTimeout>|undefined;let terminalNotified=false;
  setTables([]);setError('');setPreview(undefined);
  const readAdoption=async()=>{try{const task=await api.adoptionStatus!(sourceId);if(!live)return;if(task)setReport(v=>v?{...v,adoption:task}:v);if(task?.state==='pending'||task?.state==='running')timer=setTimeout(readAdoption,2000);else void read();}catch(e){if(live)setError((e as Error).message)}};
  const read=async()=>{try{const value=await api.sourceAnalysis!(sourceId);if(!live)return;setReport(value);if(value.state==='pending'||value.state==='running'||value.adoption?.state==='pending'||value.adoption?.state==='running'){timer=setTimeout((value.adoption?.state==='pending'||value.adoption?.state==='running')&&api.adoptionStatus?readAdoption:read,2000);}else if(value.state==='succeeded'&&!terminalNotified){terminalNotified=true;const needs=new Set(value.coverage.filter(c=>c.state==='needs_input').map(c=>c.step));setSelected(domains.filter((_,i)=>needs.has(i+1)).map(d=>d.id));setTables(value.drafts.filter(d=>d.rowCount>0).map(d=>d.tableId));onResult();}else if(value.state==='failed'&&!terminalNotified){terminalNotified=true;onResult();}}catch(e){if(live&&(e as Error).name!=='AbortError')setError((e as Error).message);}};
  if(api.sourceAnalysis)void read();return()=>{live=false;if(timer)clearTimeout(timer)};
 },[api,sourceId,retry]);
 const start=async(scope:AnalysisDomain[])=>{setBusy(true);setError('');try{await api.startSourceAnalysis!(sourceId,scope);setScopeOpen(false);setRetry(v=>v+1);onResult();notify('分析任务已排队；候选生成后仍需确认采用。');}catch(e){setError((e as Error).message);}finally{setBusy(false)}};
 // History is server-owned; refresh only while real tasks are active.
 useEffect(()=>{if(!api.sourceTasks)return;let live=true;let timer:ReturnType<typeof setTimeout>|undefined;
  setTasksLoading(true);setTasksError('');
  const read=async()=>{try{const value=await api.sourceTasks!(sourceId,taskOffset,5);if(!live)return;setTasks(value);setTasksLoading(false);if(value.items.some(t=>t.state==='pending'||t.state==='running')||running||adopting)timer=setTimeout(read,2000);}catch(e){if(live){setTasksError((e as Error).message);setTasksLoading(false);}}};
  void read();return()=>{live=false;if(timer)clearTimeout(timer)};
 },[api,sourceId,taskOffset,taskRetry,retry,running,adopting,report?.jobId,report?.adoption?.jobId]);
 useEffect(()=>{if(!selectedTask||!api.sourceTask)return;let live=true;let timer:ReturnType<typeof setTimeout>|undefined;let loadedAnalysis:string|undefined;
  setTaskLoading(true);setTaskError('');setTaskDetail(undefined);setHistoryReport(undefined);
  const read=async()=>{try{const value=await api.sourceTask!(sourceId,selectedTask);if(!live)return;setTaskDetail(value);
   const analysisId=value.task.kind==='analysis'?value.task.id:value.task.analysisJobId;
   if(analysisId&&api.sourceAnalysis&&loadedAnalysis!==analysisId){const analysis=await api.sourceAnalysis(sourceId,analysisId);if(!live)return;setHistoryReport(analysis);if(analysis.state!=='pending'&&analysis.state!=='running')loadedAnalysis=analysisId;}
   if(!live)return;setTaskLoading(false);if(value.task.state==='pending'||value.task.state==='running')timer=setTimeout(read,2000);
  }catch(e){if(live){setTaskError((e as Error).message);setTaskLoading(false);}}};
  void read();return()=>{live=false;if(timer)clearTimeout(timer)};
 },[api,sourceId,selectedTask,taskRetry]);
 const draft=(previewAnalysis??report)?.drafts.find(d=>d.tableId===preview);
 const taskState=(state:string)=>(({pending:'已排队',running:'执行中',succeeded:'已完成',failed:'失败',cancelled:'已取消'} as Record<string,string>)[state]??state);
 if(!api.sourceAnalysis||!api.startSourceAnalysis||!api.adoptSourceAnalysis)return <>{children}</>;
 return <div className={'source-analysis'+(running?' is-analyzing':'')}>
  <div className="source-analysis-flow">
   {children}
  <aside className="source-agent-actions"><div className="source-analysis-toolbar"><b>Agent 任务</b>
   <span className="source-analysis-status" role="status" aria-busy={running||adopting||busy}>{(running||adopting||busy)&&<LoaderCircle size={14} className="source-task-spinner" aria-hidden="true"/>}{adopting?'候选采用任务处理中':report?states[report.state]:'正在读取分析状态…'}{report?.state==='failed'&&report.errorCode?' · '+apiError(report.errorCode):''}{report?.adoption?.state==='failed'?' · 采用失败：'+apiError(report.adoption.errorCode??'ENGINEERING_ANALYSIS_FAILED'):''}{report?.adoption?.state==='succeeded'?' · 候选已采用，可进入数据核对':''}</span>
   <div className="actions">
    <Button disabled={!supported||!report||busy||running||adopting||!can('bom:import')} onClick={()=>setScopeOpen(true)}>分析范围</Button>
    <Button kind="primary" disabled={!supported||!report||busy||running||adopting||!can('bom:import')} onClick={()=>void start(domains.map(d=>d.id))}>{adopting?'正在采用…':busy?'正在排队…':running?states[report!.state]:report?.jobId?'重新分析资料':'开始分析'}</Button>
   </div>
  </div>

  </aside>
  </div>
  {!supported&&<Notice tone="warning">当前自动分析支持 XLSX；其他格式保留为附件。</Notice>}
  {error&&<Notice tone="error">{error}<Button kind="ghost" onClick={()=>setRetry(v=>v+1)}>重新读取</Button></Notice>}
  {scopeOpen&&<Modal title="选择分析范围" description="只重新识别所选内容，其他结果保留；已有人工确认值不会被自动覆盖。" close={()=>setScopeOpen(false)}>
   <div className="agent-domain-options">{domains.map(d=><label key={d.id}><input type="checkbox" checked={selected.includes(d.id)} onChange={e=>setSelected(v=>e.target.checked?[...v,d.id]:v.filter(x=>x!==d.id))}/>{d.label}</label>)}</div>
   <div className="actions"><Button onClick={()=>setScopeOpen(false)}>取消</Button><Button kind="primary" disabled={!selected.length||busy||running||adopting||!can('bom:import')} onClick={()=>void start(selected)}>分析所选内容</Button></div>
  </Modal>}
  {report&&report.state!=='idle'&&!!(report.missing.length||report.coverage.length||report.drafts.length)&&<details className="source-analysis-results" open={report.state==='succeeded'&&report.drafts.length>0?true:undefined}>
   <summary>识别结果 <span>{report.drafts.length} 类待核对候选 · {report.missingTotal??report.missing.length} 项待补充</span></summary>
  {!!report?.missing.length&&<Notice tone="warning"><details><summary>{report.missingTotal??report.missing.length} 项缺失或未覆盖信息（当前显示 {report.missing.length} 项）</summary>{report.missing.map((message,i)=><p key={i}>{message}</p>)}</details></Notice>}
  {!!report?.coverage.length&&<details className="agent-region-coverage"><summary>查看区域覆盖结果与原因（当前显示 {report.coverage.length}/{report.coverageTotal??report.coverage.length} 项）</summary><DataTable data={report.coverage} pageSize={5} columns={[{accessorKey:'region',header:'区域'},{id:'state',header:'处理结果',cell:({row})=>({complete:'已覆盖',ready:'已就绪',needs_input:'待补充',unsupported:'暂不支持',not_applicable:'不适用'}[row.original.state]??row.original.state)},{accessorKey:'rows',header:'候选行数'},{accessorKey:'reason',header:'依据或未完成原因'}]}/></details>}
  {!!report?.drafts.length&&<><DataTable data={report.drafts} pageSize={6} columns={[
   {id:'select',header:'采用',enableSorting:false,cell:({row})=><input type="checkbox" aria-label={'采用 '+row.original.tableId} disabled={adopting||!row.original.rowCount||report.state!=='succeeded'} checked={tables.includes(row.original.tableId)} onChange={e=>setTables(v=>e.target.checked?[...v,row.original.tableId]:v.filter(x=>x!==row.original.tableId))}/>},
   {id:'table',header:'候选内容',cell:({row})=>tableById(row.original.tableId)?.name||({devices:'设备',ports:'端口',connections:'双端连接',components:'组件'}[row.original.tableId]??row.original.tableId)},
   {accessorKey:'rowCount',header:'候选数量'},{id:'issues',header:'核对事项',cell:({row})=>row.original.issues.length+' 项（预览范围）'},
   {id:'view',header:'来源与内容',cell:({row})=><Button kind="ghost" onClick={()=>{setPreviewAnalysis(undefined);setPreview(row.original.tableId)}}>核对候选</Button>},
  ]}/><div className="actions"><span className="muted small-copy">仅补充新增和空字段，已有值保留；采用不等于CMDB发布。</span><Button kind="primary" disabled={adopting||running||!tables.length||report.state!=='succeeded'||!can('bom:edit')} onClick={()=>{if(report.jobId&&!adopting)adoption.mutate({jobId:report.jobId,tableIds:tables})}}>{adopting&&<LoaderCircle size={14} className="source-task-spinner" aria-hidden="true"/>}{adopting?'正在采用…':'确认采用所选候选'}</Button></div></>}
  </details>}
   <div className="source-task-history"><div className="source-task-history-heading"><b>任务记录</b><Button kind="ghost" onClick={()=>setTaskRetry(v=>v+1)}>刷新</Button></div>
    {!api.sourceTasks?<p className="muted">任务记录接口尚未接入</p>:tasksError?<Notice tone="error">{tasksError}</Notice>:tasksLoading?<p role="status">正在读取任务记录…</p>:tasks?.items.length?tasks.items.map(task=><div key={task.id} title={task.id} className={'source-task-row '+(selectedTask===task.id?'selected':'')}>
     <div><b>{task.kind==='analysis'?'资料分析':'候选采用'}</b><span>{(task.state==='pending'||task.state==='running')&&<LoaderCircle size={12} className="source-task-spinner" aria-hidden="true"/>}{taskState(task.state)}</span></div><small>{formatDate(task.createdAt)}</small>
     {task.errorCode&&<p>{apiError(task.errorCode)}</p>}
     <button className="text-link" onClick={()=>setSelectedTask(task.id)}>资料理解与候选证据</button>
    </div>):<p className="muted">暂无任务记录</p>}
    {tasks&&tasks.total>0&&<div className="source-task-pagination"><span>共 {tasks.total} 次任务</span><Button kind="ghost" disabled={tasksLoading||taskOffset===0} onClick={()=>setTaskOffset(v=>Math.max(0,v-5))}>上一页</Button><Button kind="ghost" disabled={tasksLoading||taskOffset+tasks.limit>=tasks.total} onClick={()=>setTaskOffset(v=>v+5)}>下一页</Button></div>}
   </div>
  {selectedTask&&<Modal drawer title="资料理解与候选证据" description={`所选任务 ${selectedTask} · ${sourceName}`} close={()=>setSelectedTask(undefined)}>
   {taskLoading&&<p role="status">正在读取任务日志…</p>}
   {taskError&&<Notice tone="error">{taskError}<Button onClick={()=>setTaskRetry(v=>v+1)}>重新读取</Button></Notice>}
   {taskDetail&&<div className="source-task-detail">
    <div className="source-task-summary"><b>{taskDetail.task.kind==='analysis'?'资料分析':'候选采用'} · {taskState(taskDetail.task.state)}</b><p>创建：{formatDate(taskDetail.task.createdAt)}</p><p>更新：{formatDate(taskDetail.task.updatedAt)}</p>{taskDetail.task.errorCode&&<Notice tone="error">{apiError(taskDetail.task.errorCode)}</Notice>}</div>
    <h3>任务日志</h3>{taskDetail.steps.length?<ol className="source-task-logs">{taskDetail.steps.map(step=><li key={step.sequence}><div><b>{step.tool}</b><span>{taskState(step.state)}</span></div><small>{formatDate(step.createdAt)}</small><p>{step.detail}</p></li>)}</ol>:<p className="muted">此任务暂无步骤日志</p>}
    {taskDetail.understanding&&<><h3>所选任务的资料理解</h3>{taskDetail.understanding.steps.map(step=><div className="source-task-understanding" key={step.id}><b>{{structure:'工作簿结构',regions:'数据区域',semantics:'语义分类',relationships:'跨表关联',fields:'字段核对'}[step.id]}</b><span>{{complete:'已完成',partial:'部分完成',pending:'待处理',unsupported:'暂无结果',not_applicable:'不适用'}[step.state]}{step.total>0?` ${step.completed}/${step.total}`:''}</span><p>{step.note}</p></div>)}{taskDetail.understanding.limitations.map((note,i)=><p key={i}>{note}</p>)}</>}
    {historyReport&&<><h3>该次分析的候选证据</h3><p className="muted">{states[historyReport.state]} · 历史查看不改变当前采用范围</p>{historyReport.errorCode&&<Notice tone="error">{apiError(historyReport.errorCode)}</Notice>}{historyReport.drafts.map(item=><div className="source-task-candidate" key={item.tableId}><span>{tableById(item.tableId)?.name??item.tableId} · {item.rowCount} 行</span><Button kind="ghost" onClick={()=>{setPreviewAnalysis(historyReport);setPreview(item.tableId)}}>查看来源与内容</Button></div>)}{!historyReport.drafts.length&&<p className="muted">该次任务暂无候选结果</p>}{historyReport.missing.map((note,i)=><p key={i}>{note}</p>)}</>}
   </div>}
  </Modal>}
  {draft&&<Modal title={'核对候选 · '+(tableById(draft.tableId)?.name??draft.tableId)} description={`共 ${draft.rowCount} 行；显示最多 ${draft.previewLimit} 行。完整候选与来源保存在服务端。`} close={()=>setPreview(undefined)} wide><DataTable data={draft.rows} pageSize={5} columns={[...new Set(draft.rows.flatMap(Object.keys))].map(key=>({accessorKey:key,header:tableById(draft.tableId)?.fields.find(f=>f.key===key)?.label??key}))}/>{draft.evidence.map((e,i)=><div className="source-agent-evidence" key={i}><b>候选第 {e.row} 行</b>{Object.entries(e.fields).map(([field,cell])=><button className="text-link" key={field} onClick={()=>inspect('evidence',`${sourceId}:${cell.sheet}:${cell.cell}`,{sourceId,sheet:cell.sheet,cell:cell.cell,method:'source',note:`字段 ${field}${cell.normalization?' · '+cell.normalization:''}${cell.formula?' · 公式来源':''}${cell.normalizationEvidence?' · '+cell.normalizationEvidence.rule+' · 结果 '+(cell.normalizationEvidence.normalized_value??'')+' · '+(cell.normalizationEvidence.ticks?.length??0)+'个刻度来源':''}`} )}>{field} · {cell.sheet}!{cell.cell}{cell.normalizationEvidence&&<small> · 推导 {cell.normalizationEvidence.normalized_value} {cell.normalizationEvidence.ticks?.length?'('+cell.normalizationEvidence.ticks.length+'个U刻度)':''}</small>}</button>)}</div>)}{draft.issues.map((issue,i)=><Notice tone="warning" key={i}>第{issue.row}行 {issue.field}：{issue.message}</Notice>)}</Modal>}
 </div>;
}
