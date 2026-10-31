import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import YAML from 'yaml';
import { flowsApi, flowsKey } from '@/lib/api/flows';
import { Tabs, Table, State, Empty } from './ui';
import { FlowGraph } from './Builder';
import type { Engine } from '@/lib/api/flows';
export function TaskRuns({ engine, project, flowId, revision, executionId }: {
    executionId:string;engine: Engine;project:string|null;flowId:string|null;revision:number;
}) {
 const [selected,setSelected]=useState('');
 const capability=useQuery({queryKey:flowsKey('capabilities',project),queryFn:()=>flowsApi.capabilities(project)});
 const definitions=useQuery({queryKey:flowsKey('execution-definition',project,executionId,revision,!!capability.data?.execution_definition),queryFn:async()=>{
  if(capability.data?.execution_definition)return flowsApi.executionDefinition(project,executionId);
  if(!flowId)throw Error('当前后端尚未提供 Quick Run 执行版本查询');
  const versions=await flowsApi.context(project,flowId,'revisions');
  const frozen=versions.find(d=>d.revision===revision);if(!frozen?.source)throw Error('执行版本的定义不可用');
  return {source:String(frozen.source)};
 },enabled:!!capability.data});
 let doc:any=null;try{if(definitions.data?.source)doc=YAML.parse(definitions.data.source)}catch{}
 const states:Record<string,string>={};for(const run of (engine.taskRunList||[]) as any[]){const current=run.state?.current||'unknown';const old=states[run.taskId];states[run.taskId]=old&&old!==current?'MIXED':current;}
 const [view, setView] = useState('Gantt'); const tasks = (engine.taskRunList || []) as Record<string, any>[]; const times = tasks.flatMap(t => [Date.parse(t.state?.startDate), Date.parse(t.state?.endDate)]).filter(Number.isFinite); const start = times.length ? Math.min(...times) : 0; const end = times.length ? Math.max(...times, ...(tasks.some(t => !t.state?.endDate) ? [Date.now()] : [])) : 0; const span = Math.max(1, end - start); return <><Tabs items={['Gantt', 'Topology', 'Table']} current={view} onChange={setView}/>{selected&&<div className="fl-toolbar"><b>{selected}</b><button onClick={()=>setView('Table')}>查看迭代与 Attempt</button><button onClick={()=>setSelected('')}>清除选择</button></div>}{!tasks.length ? <Empty>引擎尚无 Task Run。</Empty> : view === 'Gantt' ? <div className="fl-gantt">{tasks.map(t => { const from = Date.parse(t.state?.startDate), to = t.state?.endDate ? Date.parse(t.state.endDate) : Date.now(); return <div className="fl-gantt-row" key={t.id}><span><b>{t.taskId}</b><small>{t.value ?? ''} · {t.attempts?.length || 1} attempt</small></span><div className="fl-gantt-track">{Number.isFinite(from) && <i className={'fl-gantt-bar ' + String(t.state?.current || '').toLowerCase()} style={{ left: Math.max(0, (from - start) / span * 100) + '%', width: Math.max(.5, (to - from) / span * 100) + '%' }} title={new Date(from).toLocaleTimeString() + ' → ' + (t.state?.endDate ? new Date(to).toLocaleTimeString() : 'running')}/>}</div><State value={t.state?.current}/></div>; })}<div className="fl-gantt-axis"><span>{new Date(start).toLocaleTimeString()}</span><span>{((end - start) / 1000).toFixed(1)}s</span></div></div> : view === 'Topology' ? <div className="fl-overview-dag">{doc?<FlowGraph document={doc} states={states} selected={selected} onSelect={setSelected}/>:<Empty loading={definitions.isLoading} error={definitions.error}>该执行版本的 Flow 定义不可用，无法确认依赖关系。</Empty>}</div> : <Table<any> data={selected?tasks.filter(t=>t.taskId===selected):tasks} columns={[{ accessorKey: 'taskId', header: 'Task' }, { accessorKey: 'value', header: 'Iteration' }, { accessorKey: 'id', header: 'Task Run' }, { id: 'state', header: 'State', cell: ({ row }) => <State value={row.original.state?.current}/> }, { id: 'attempts', header: 'Attempts', cell: ({ row }) => row.original.attempts?.length || 1 }, { id: 'start', header: 'Started', cell: ({ row }) => row.original.state?.startDate || '—' }, { id: 'end', header: 'Ended', cell: ({ row }) => row.original.state?.endDate || '—' }]}/>}</>; }
