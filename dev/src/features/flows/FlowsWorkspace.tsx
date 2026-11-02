import { lazy, Suspense, useEffect, useMemo, useState, useRef } from 'react';
import { Link, Navigate, useLocation, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient, useInfiniteQuery } from '@tanstack/react-query';
import YAML from 'yaml';
import { Plus, Play, ArrowLeft, FileCode2, GitBranch, RefreshCw, MoreHorizontal } from 'lucide-react';
import { useProject } from '@/app/ProjectContext';
import { flowsKey, flowsApi, followExecution, followLogs, mergeLogs, type Flow, type Execution, type Agent, type Engine } from '@/lib/api/flows';
import { inlineScriptTask } from './script-task';
import { isControlled, taskList, nativeTemplate, inputDefault, setDefault } from './definition';
import { Builder, FlowGraph } from './Builder';
import { QuickRun } from './QuickRun';
import { NativeRun, isTargetFlow } from './NativeRun';
import { AgentsPage } from './Agents';
import { TaskRuns } from './TaskRuns';
import { ExecutionLogs, AuditView, OutputView } from './ExecutionPanels';
import { RevisionView, TriggerView, NamespaceFiles, DependencyView } from './ContextViews';
import { Button, State, Notice, Table, Panel, Tabs, Search, Empty, Json } from './ui';
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem } from '@/components/shadcn/dropdown-menu';
import { TargetSchedule, DeviceSteps } from './Completion';
import {BackfillWorkspace} from './BackfillWorkspace';
import {WorkspaceQueries} from './WorkspaceQueries';
import './flows.css';
const NativeFlowHeaderHost=lazy(()=>import('../jobs-native/NativeFlowHeaderHost').then(m=>({default:m.NativeFlowHeaderHost})));
const NativeExecutionHost=lazy(()=>import('../jobs-native/NativeExecutionHost').then(m=>({default:m.NativeExecutionHost})));
const NativeOutputsHost=lazy(()=>import('../jobs-native/NativeOutputsHost').then(m=>({default:m.NativeOutputsHost})));
const NativeTimelineHost=lazy(()=>import('../jobs-native/NativeTimelineHost').then(m=>({default:m.NativeTimelineHost})));
const NativeCatalogHost=lazy(()=>import('../jobs-native/NativeCatalogHost').then(m=>({default:m.NativeCatalogHost})));
function targetOutcome(e:Execution){if(!e.targets.length)return '';if(e.targets.some(t=>t.state==='RECONCILING'))return 'RECONCILING';if(e.targets.some(t=>t.state==='FAILED'))return e.targets.some(t=>t.state==='SUCCESS')?'PARTIAL_FAILURE':'FAILED';if(e.targets.every(t=>t.state==='SUCCESS'))return 'SUCCESS';return 'RUNNING';}
const flowTabs = ['Overview','Builder','Executions','Revisions','Triggers','Dependencies','Files','Logs','Metrics','Concurrency'];
const executionTabs = ['Overview', 'Targets', 'Task Runs', 'Logs', 'Outputs & Artifacts', 'Audit / Context'];
export function FlowsWorkspace() {
    const ctx = useProject(), location = useLocation(), nav = useNavigate(), params = useParams();
    const [url, setUrl] = useSearchParams();
    const project = ctx.filter || null;
    const can = (p: string) => ctx.permissions.includes('flows:' + p);
    const [run, setRun] = useState<Flow | true | null>(null);
    const q=url.get('q')||'',filter=url.get('filter')||'all';
    const setFilterValue=(key:string,value:string)=>setUrl(previous=>{const next=new URLSearchParams(previous);if(value&&value!=='all')next.set(key,value);else next.delete(key);return next},{replace:true});
    const setQ=(v:string)=>setFilterValue('q',v),setFilter=(v:string)=>setFilterValue('filter',v);
    const executionFilters=Object.fromEntries(['source','transport','started_by','since','until','mode','flow_id'].filter(k=>url.get(k)).map(k=>[k,['since','until'].includes(k)&&!Number.isNaN(Date.parse(url.get(k)!))?new Date(url.get(k)!).toISOString():url.get(k)!]));
    const flowState=url.get('flow_state')||'all', namespace=url.get('namespace')||'',trigger=url.get('trigger')||'all';
    const prepareRun = useMutation({ mutationFn: (f: Flow) => flowsApi.flow(f.project_id, f.id), onSuccess: setRun });
    const manage = useMutation({ mutationFn: async ({ flow, action }: { flow: Flow; action: 'toggle' | 'copy' }) => {
        const full = await flowsApi.flow(flow.project_id, flow.id), doc = YAML.parse(full.source);
        if (action === 'toggle') doc.disabled = !full.disabled;
        else {
            const caps = await flowsApi.capabilities(full.project_id);
            doc.namespace = caps.namespace;
            doc.id = String(doc.id).slice(0, 75) + '-copy-' + crypto.randomUUID().slice(0, 8);
            doc.disabled = true;
            if (doc.triggers) doc.triggers = doc.triggers.map((t: any) => ({ ...t, disabled: true }));
            delete doc.revision;
        }
        return flowsApi.save(full.project_id, action === 'copy' ? null : full.id, { project_id: full.project_id, name: action === 'copy' ? full.name + ' (copy)' : full.name, description: full.description, target_type: full.target_type, source: YAML.stringify(doc), expected_revision: action === 'copy' ? 0 : full.revision, draft: action === 'copy' || full.draft });
    }, onSuccess: () => { void cache.invalidateQueries({ queryKey: ['flows'] }); } });
    const flowId = params.flowId;
    const executionId = params.executionId;
    const tab = url.get('tab') || 'Overview';
    const isExecutions = location.pathname.startsWith('/flows/executions');
    const isAgents = location.pathname.startsWith('/flows/agents');
    const cache = useQueryClient();
    const catalogFilters={q,source_search:url.get('source_search')||'',state:url.get('native_state')||'',namespace:url.get('namespace')||'',target_type:url.get('target_type')||'',limit:url.get('limit')||'25',offset:url.get('offset')||'0',sort:url.get('sort')||'',direction:url.get('direction')||''};
    const catalog=useQuery({queryKey:flowsKey('catalog',project,catalogFilters),queryFn:({signal})=>flowsApi.catalog(project,catalogFilters,signal),enabled:!isAgents&&!executionId&&!isExecutions&&!flowId});
    const [batchResult,setBatchResult]=useState('');
    const batch=useMutation({mutationFn:async({rows,action}:{rows:Flow[];action:string})=>{const result:string[]=[];for(const row of rows){try{const full=await flowsApi.flow(row.project_id,row.id);const doc=YAML.parse(full.source);doc.disabled=action==='disable';await flowsApi.save(full.project_id,full.id,{project_id:full.project_id,name:full.name,description:full.description,target_type:full.target_type,source:YAML.stringify(doc),expected_revision:full.revision,draft:full.draft});result.push(row.name+'：成功')}catch(e){result.push(row.name+'：'+String(e))}}return result.join('；')},onSuccess:result=>{setBatchResult(result);void cache.invalidateQueries({queryKey:['flows']})}});
    const list = useQuery({ queryKey: flowsKey('list', project), queryFn: () => flowsApi.list(project), enabled: !isAgents && !executionId && !isExecutions && !!flowId });
    const executionPages = useInfiniteQuery({ queryKey: flowsKey('executions', project, flowId || '', q, filter, executionFilters), initialPageParam: '', queryFn: ({ pageParam }) => flowsApi.executionPage(project, pageParam, { ...executionFilters, ...(q ? { q } : {}), ...(filter !== 'all' && isExecutions ? { state: filter } : {}), ...(flowId && flowId !== 'new' ? { flow_id: flowId } : {}) }), getNextPageParam: last => last.page.next_cursor || undefined, enabled: !isAgents && !executionId && (isExecutions || !!flowId && ['Executions','Overview'].includes(tab)), refetchInterval: query => (query.state.data?.pages.length || 0) <= 1 ? 5000 : false });
    const executions = { ...executionPages, data: executionPages.data?.pages.flatMap(page => page.items) };
    const flow = useQuery({ queryKey: flowsKey('definition', project, flowId), queryFn: () => flowsApi.flow(project, flowId!), enabled: !!flowId && flowId !== 'new' });
    const ex = useQuery({ queryKey: flowsKey('execution', project, executionId), queryFn: () => flowsApi.execution(project, executionId!), enabled: !!executionId, refetchInterval: q => ['RUNNING', 'CREATED', 'RESTARTED', 'PAUSED'].includes(String(q.state.data?.engine?.state?.current)) ? 5000 : false });
    const capability = useQuery({ queryKey: flowsKey('capabilities', project), queryFn: () => flowsApi.capabilities(project), staleTime: 60000 });
    useEffect(() => {
        if (!executionId)
            return;
        return followExecution(project, executionId, () => { void cache.invalidateQueries({ queryKey: flowsKey('execution', project, executionId) }); });
    }, [project, executionId, cache]);
    function setTab(t: string) { const next = new URLSearchParams(url); next.set('tab', t); setUrl(next); }
    const state = (e: Execution) => String(e.engine?.state?.current || e.error_code || 'NOT_STARTED');
    if (location.pathname === '/flows/flows')
        return <Navigate to="/flows" replace/>;
    if(isExecutions&&!executionId&&['logs','overview'].includes(url.get('view')||''))return <><div className="fl-toolbar"><Button variant="ghost" onClick={()=>setFilterValue('view','')}>← 执行记录</Button></div><WorkspaceQueries project={project} kind={url.get('view')==='logs'?'logs':'metrics'}/></>;
    if (['/flows/blueprints', '/flows/media', '/flows/schedules'].includes(location.pathname))
        return <Navigate to={'/flows?context=' + location.pathname.split('/').at(-1)} replace/>;
    if(!isAgents&&!flowId&&!executionId)return <><Notice error={batchResult?Error(batchResult):null}/><Suspense fallback={<Empty loading/>}><NativeCatalogHost sourceSearch={catalogFilters.source_search==='true'} onView={view=>setFilterValue('view',view)} catalog={!isExecutions?catalog.data:undefined} canEdit={can('edit')} namespaceFilter={catalogFilters.namespace} targetFilter={catalogFilters.target_type} onExtraFilter={(key,value)=>setUrl(previous=>{const next=new URLSearchParams(previous);if(value)next.set(key,value);else next.delete(key);next.set('offset','0');return next},{replace:true})} onSort={(field,direction)=>setUrl(previous=>{const next=new URLSearchParams(previous);next.set('sort',field);next.set('direction',direction);next.set('offset','0');return next},{replace:true})} onPage={(page,size)=>setUrl(previous=>{const next=new URLSearchParams(previous);next.set('limit',String(size));next.set('offset',String((page-1)*size));return next},{replace:true})} onBatch={(rows,action)=>batch.mutate({rows,action})} onExecution={id=>nav('/flows/executions/'+id)} rows={isExecutions?executions.data||[]:catalog.data?.items||[]} executionMode={isExecutions} query={q} stateFilter={isExecutions?(url.get('filter')||''):url.get('native_state')||''} onFilter={(key,value)=>setUrl(previous=>{const next=new URLSearchParams(previous),field=key==='state'?(isExecutions?'filter':'native_state'):'q';if(value)next.set(field,value);else next.delete(field);next.set('offset','0');return next},{replace:true})} loading={isExecutions?executions.isFetching:catalog.isFetching||batch.isPending} error={String((isExecutions?executions.error:catalog.error)?.message||prepareRun.error?.message||'')} canCreate={can('create')} canExecute={can('execute')} hasMore={!!executionPages.hasNextPage} onRefresh={()=>{void(isExecutions?executions.refetch():catalog.refetch())}} onQuickRun={()=>setRun(true)} onCreate={()=>nav('/flows/definitions/new')} onOpen={row=>nav(isExecutions?'/flows/executions/'+row.id:'/flows/definitions/'+row.id)} onRun={row=>prepareRun.mutate(row)} onEdit={row=>nav('/flows/definitions/'+row.id+'?tab=Builder')} onLoadMore={()=>void executionPages.fetchNextPage()}/></Suspense>{run&&run!==true&&!isTargetFlow(run)?<NativeRun flow={run} onClose={()=>setRun(null)}/>:run&&<QuickRun key={run===true?ctx.filter||ctx.projectId:run.project_id} project={run===true?ctx.filter||ctx.projectId:run.project_id} flow={run===true?null:run} allowed={can('execute')} canSave={can('create')} onClose={()=>setRun(null)}/>}</>;
    const actionHeader = <div className="fl-heading"><div><small>KEEL / 作业平台</small><h1>{isAgents ? 'Agents' : isExecutions ? 'Executions' : '作业平台'}</h1></div><span className="spacer"/>{!isAgents && !isExecutions && !flowId && <><Button variant="outline" disabled={!can('execute')} title={!ctx.filter ? '先选择本次执行项目' : undefined} onClick={() => setRun(true)}><Play size={14}/>Quick Run</Button><Button disabled={!can('create')} onClick={() => nav('/flows/definitions/new')}><Plus size={14}/>Create Flow</Button></>}</div>;
    const tableExec = (data: Execution[]) => <Table data={data} onSelect={e => nav('/flows/executions/' + e.id)} columns={[{ accessorKey: 'id', header: 'Execution', cell: ({ row }) => <><b>{row.original.id.slice(0, 12)}</b><small>{row.original.kestra_id || '引擎尚未启动'}</small></> }, { accessorKey: 'engine_flow_id', header: 'Flow / Source', cell: ({ row }) => <>{row.original.engine_flow_id}<small>{row.original.source_module}</small></> }, { accessorKey: 'project_id', header: 'Project' }, { id: 'targets', header: 'Targets / Result', cell: ({ row }) => <>{row.original.targets.length} targets<small>{row.original.targets.filter(t => t.state === 'SUCCESS').length} success / {row.original.targets.filter(t => t.state === 'FAILED').length} failed</small></> }, { accessorKey: 'mode', header: 'Mode / Route', cell: ({ row }) => <>{row.original.mode}<small>Agent {row.original.targets.filter(t => t.actual_transport === 'AGENT').length} / SSH {row.original.targets.filter(t => t.actual_transport === 'SSH').length}</small></> }, {id:'duration',header:'Duration',cell:({row})=>elapsed(row.original)}, { accessorKey:'started_by',header:'Started by' }, { accessorKey: 'created_at', header: 'Started', cell: ({ getValue }) => new Date(String(getValue())).toLocaleString() }, { id: 'state', header: 'Target Result / Engine', cell: ({ row }) => <><State value={targetOutcome(row.original)||state(row.original)}/><small>Engine · {state(row.original)}</small></> }]}/>;
    return <div className="fl-workspace">{!flowId && !executionId && actionHeader}<Notice error={capability.error}/>{capability.data && !capability.data.configured && <div className="fl-warning">Kestra 尚未配置。可检查 CMDB 和 Agent，执行与定义保存需要先启动本分支的执行底座。</div>}
 {isAgents ? <AgentsPage project={project} canEdit={can('edit')}/> : executionId ? ex.data ? <ExecutionDetail execution={ex.data} project={project} tab={tab} setTab={setTab} canExecute={can('execute')}/> : <Empty loading={ex.isLoading} error={ex.error}/> : flowId ? <FlowDetail id={flowId} flow={flow.data} loading={flow.isLoading} error={flow.error} project={flow.data?.project_id || ctx.filter || ctx.projectId} tab={tab} setTab={setTab} canEdit={can(flowId === 'new' ? 'create' : 'edit')} canExecute={can('execute')} onRun={f => setRun(f)} executions={executions.data || []} tableExec={tableExec}/> : isExecutions ? <><div className="fl-toolbar"><Search value={q} onChange={setQ} placeholder="Execution ID / Flow / 设备"/><select value={filter} onChange={e => setFilter(e.target.value)}><option value="all">所有引擎状态</option>{['RUNNING', 'SUCCESS', 'FAILED', 'WARNING', 'PAUSED', 'KILLED'].map(v => <option key={v}>{v}</option>)}</select><Button variant="outline" size="sm" onClick={() => void executions.refetch()}><RefreshCw size={14}/>刷新</Button></div><div className="fl-toolbar"><select aria-label="Execution Flow" value={url.get('flow_id')||''} onChange={e=>setFilterValue('flow_id',e.target.value)}><option value="">所有 Flow</option>{(list.data||[]).map(f=><option key={f.id} value={f.id}>{f.name}</option>)}</select><select aria-label="Execution mode" value={url.get('mode')||''} onChange={e=>setFilterValue('mode',e.target.value)}><option value="">所有执行模式</option><option value="REAL">Real</option></select><select aria-label="Execution source" value={url.get('source')||''} onChange={e=>setFilterValue('source',e.target.value)}><option value="">所有来源</option>{['flows','quick-run','schedule','ztp','retry','restart'].map(v=><option key={v}>{v}</option>)}</select><select aria-label="Execution transport" value={url.get('transport')||''} onChange={e=>setFilterValue('transport',e.target.value)}><option value="">所有执行通道</option><option>AGENT</option><option>SSH</option></select><input aria-label="发起人ID" placeholder="发起人 ID" value={url.get('started_by')||''} onChange={e=>setFilterValue('started_by',e.target.value)}/><label>开始<input type="datetime-local" value={url.get('since')||''} onChange={e=>setFilterValue('since',e.target.value)}/></label><label>结束<input type="datetime-local" value={url.get('until')||''} onChange={e=>setFilterValue('until',e.target.value)}/></label><Button variant="ghost" size="sm" onClick={()=>setUrl({})}>清除筛选</Button></div><Notice error={executions.error}/>{executions.isLoading ? <Empty loading/> : tableExec((executions.data || []).filter(e => (filter === 'all' || state(e) === filter) && JSON.stringify([e.id, e.engine_flow_id, e.targets.map(t => t.snapshot.name)]).toLowerCase().includes(q.toLowerCase())))}</> : <><div className="fl-toolbar"><Search value={q} onChange={setQ} placeholder="搜索 Flow 名称 / ID / Namespace"/><select value={filter} onChange={e => setFilter(e.target.value)}><option value="all">所有目标类型</option><option value="server">Server</option><option value="switch">Switch</option><option value="both">Server & Switch</option></select><Button size="sm" variant="outline" onClick={() => void list.refetch()}><RefreshCw size={14}/>刷新</Button><span className="spacer"/><span>{list.data ? list.data.length + ' flows' : '加载中…'}</span></div><div className="fl-toolbar"><select aria-label="Flow status" value={flowState} onChange={e=>setFilterValue('flow_state',e.target.value)}><option value="all">所有状态</option><option value="enabled">Enabled</option><option value="disabled">Disabled</option><option value="draft">Draft</option></select><select aria-label="Namespace" value={namespace} onChange={e=>setFilterValue('namespace',e.target.value)}><option value="">所有 Namespace</option>{[...new Set((list.data||[]).map(f=>f.namespace))].map(n=><option key={n}>{n}</option>)}</select><select aria-label="Trigger type" value={trigger} onChange={e=>setFilterValue('trigger',e.target.value)}><option value="all">所有触发方式</option><option value="manual">Manual</option><option value="scheduled">Triggered</option></select><Button variant="ghost" size="sm" onClick={()=>setUrl({})}>清除筛选</Button></div><Notice error={list.error || prepareRun.error || manage.error}/>{list.isLoading ? <Empty loading/> : <Table data={(list.data || []).filter(f => (filter === 'all' || f.target_type === filter) && (!namespace||f.namespace===namespace) && (flowState==='all'||flowState==='draft'&&f.draft||flowState==='disabled'&&f.disabled&&!f.draft||flowState==='enabled'&&!f.disabled&&!f.draft) && (trigger==='all'||trigger==='scheduled'&&f.engine?.triggers?.length||trigger==='manual'&&!f.engine?.triggers?.length) && [f.name, f.flow_id, f.namespace].join(' ').toLowerCase().includes(q.toLowerCase()))} onSelect={f => nav('/flows/definitions/' + f.id)} columns={[{ accessorKey: 'name', header: 'Name / Flow ID', cell: ({ row }) => <><b>{row.original.name}</b><small>{row.original.flow_id}</small></> }, { accessorKey: 'description', header: 'Description' }, { accessorKey: 'namespace', header: 'Namespace' }, { accessorKey: 'target_type', header: 'Target' }, { accessorKey: 'revision', header: 'Revision', cell: ({ getValue }) => 'rev ' + getValue() }, { id: 'trigger', header: 'Trigger', cell: ({ row }) => row.original.engine?.triggers?.length ? row.original.engine.triggers.map((t: any) => t.id).join(', ') : 'Manual' }, {id:'latest',header:'Recent execution',cell:({row})=>{const e=(executions.data||[]).find(e=>e.flow_id===row.original.id);return e?<Link to={'/flows/executions/'+e.id}><State value={targetOutcome(e)||state(e)}/><small>{new Date(e.created_at).toLocaleString()}</small></Link>:<span title="当前已加载的近期执行中无记录；历史记录见 Flow 详情">未在近期记录中</span>}}, { id: 'state', header: 'Enabled', cell: ({ row }) => <State value={row.original.engine && Object.keys(row.original.engine).length ? (row.original.draft ? 'DRAFT' : row.original.disabled ? 'DISABLED' : 'ENABLED') : 'unknown'}/> }, { id: 'actions', header: 'Actions', cell: ({ row }) => <div className="fl-row-actions"><Button size="sm" variant="outline" disabled={!can('execute') || row.original.disabled || row.original.draft} onClick={() => prepareRun.mutate(row.original)}>Run</Button><Button size="sm" variant="outline" onClick={() => nav('/flows/definitions/' + row.original.id + '?tab=Builder')}>Edit</Button><DropdownMenu><DropdownMenuTrigger asChild><Button size="sm" variant="outline" aria-label="Flow actions"><MoreHorizontal size={14}/></Button></DropdownMenuTrigger><DropdownMenuContent><DropdownMenuItem disabled={!can('edit') || manage.isPending} onSelect={() => manage.mutate({ flow: row.original, action: 'toggle' })}>{row.original.disabled ? 'Enable' : 'Disable'}</DropdownMenuItem><DropdownMenuItem disabled={!can('create') || manage.isPending} onSelect={() => manage.mutate({ flow: row.original, action: 'copy' })}>Copy as draft</DropdownMenuItem></DropdownMenuContent></DropdownMenu></div> }]}/>}</>}
 {(isExecutions && !executionId || !!flowId && tab === 'Executions') && executionPages.hasNextPage && <div className="fl-toolbar"><Button variant="outline" disabled={executionPages.isFetchingNextPage} onClick={() => void executionPages.fetchNextPage()}>加载更早的执行记录</Button><small>历史浏览按需加载，回到最新页后恢复5秒刷新。</small></div>}
 {run && run !== true && !isTargetFlow(run) ? <NativeRun flow={run} onClose={() => setRun(null)}/> : run && <QuickRun key={run === true ? ctx.filter || ctx.projectId : run.project_id} project={run === true ? ctx.filter || ctx.projectId : run.project_id} flow={run === true ? null : run} allowed={can('execute')} canSave={can('create')} onClose={() => setRun(null)}/>}
 </div>;
}
function FlowDetail({ id, flow, loading, error, project, tab, setTab, canEdit, canExecute, onRun, executions, tableExec }: {
    id: string;
    flow?: Flow;
    loading: boolean;
    error: unknown;
    project: string;
    tab: string;
    setTab: (v: string) => void;
    canEdit: boolean;
    canExecute: boolean;
    onRun: (f: Flow) => void;
    executions: Execution[];
    tableExec: (e: Execution[]) => React.ReactNode;
}) {
    const nav = useNavigate(), cache = useQueryClient();
    const [source, putSource] = useState(''), [name, putName] = useState(''), [description, putDescription] = useState(''), [target, putTarget] = useState('server'), [context, setContext] = useState(''), [validation, setValidation] = useState<unknown>(null);
    type DefinitionDraft={source:string;name:string;description:string;target:string;revision:number};
    const draftKey=flowsKey('definition-draft',project,id);
    const remember=(patch:Partial<DefinitionDraft>)=>cache.setQueryData<DefinitionDraft>(draftKey,previous=>({source,name,description,target,...previous,revision:flow?.revision||0,...patch}));
    const setSource=(value:string)=>{putSource(value);remember({source:value})},setName=(value:string)=>{putName(value);remember({name:value})},setDescription=(value:string)=>{putDescription(value);remember({description:value})},setTarget=(value:string)=>{putTarget(value);remember({target:value})};
    const [scriptTarget,setScriptTarget]=useState('');
    const isNew = id === 'new';
    const loaded=useRef('');
    const batchTemplate=useQuery({queryKey:flowsKey('new-template',project),queryFn:()=>flowsApi.template(project),enabled:isNew});
    const namespaceQuery = useQuery({ queryKey: flowsKey('capabilities', project), queryFn: () => flowsApi.capabilities(project), enabled: isNew });
    useEffect(() => {
        if (loaded.current===project+':'+id) return;
        if (flow) {
            loaded.current=project+':'+id;
            const draft=cache.getQueryData<DefinitionDraft>(draftKey);
            const saved=draft?.revision===flow.revision?draft:null;
            putSource(saved?.source??flow.source);
            putName(saved?.name??flow.name);
            putDescription(saved?.description??flow.description);
            putTarget(saved?.target??flow.target_type);
        }
        else if (isNew && batchTemplate.data) {
            loaded.current=project+':'+id;
            const template=YAML.parse(batchTemplate.data.source);template.id='device-job-'+crypto.randomUUID().slice(0,6);
            template.inputs.push({id:'kind',type:'STRING',defaults:'bash'},{id:'command',type:'STRING',defaults:''},{id:'transport_policy',type:'STRING',defaults:'agent_preferred'});
            const draft=cache.getQueryData<DefinitionDraft>(draftKey);
            putSource(draft?.source??YAML.stringify(template));if(draft){putName(draft.name);putDescription(draft.description);putTarget(draft.target)}
        }
    }, [flow, isNew, project, id, batchTemplate.data]);
    useEffect(()=>{const warn=(e:BeforeUnloadEvent)=>{if(source&&source!==flow?.source){e.preventDefault();e.returnValue=''}};window.addEventListener('beforeunload',warn);return()=>window.removeEventListener('beforeunload',warn)},[source,flow?.source]);

    const save = useMutation({ mutationFn: (draft: boolean) => flowsApi.save(project, isNew ? null : id, { project_id: project, name, description, target_type: target, source, expected_revision: flow?.revision || 0, draft }), onSuccess: f => { cache.removeQueries({queryKey:draftKey,exact:true});void cache.invalidateQueries({ queryKey: ['flows'] }); nav('/flows/definitions/' + f.id + '?tab=Builder'); } });
    const [templateKind, setTemplateKind] = useState('bash'), [templatePolicy, setTemplatePolicy] = useState('agent_preferred');
    const template = useMutation({ mutationFn: () => flowsApi.template(project), onSuccess: t => { const d = YAML.parse(t.source); d.inputs.push({ id: 'command', type: 'STRING', defaults: templateKind === 'file.upload' ? JSON.stringify({ path: 'uploads/file.txt', data_base64: '', sha256: '' }) : '' }, { id: 'kind', type: 'STRING', defaults: templateKind }, { id: 'transport_policy', type: 'STRING', defaults: templatePolicy }); if(!isNew&&doc?.id)d.id=doc.id; if(source.trim()&&!window.confirm('使用模板替换当前未保存的定义？已发布版本保持不变。'))return; setSource(YAML.stringify(d)); setName(t.name); setContext(''); } });
    const validate = useMutation({ mutationFn: () => flowsApi.validate(project, id, source), onSuccess: setValidation });
    const extra = useQuery({ queryKey: flowsKey('context', project, id, context || tab), queryFn: () => flowsApi.context(project, id, ((context||tab)==='Builder'?'files':context||tab).toLowerCase()), enabled: !isNew && ['Revisions', 'Files','Builder'].includes(context || tab) });
    let doc: any;
    try {
        doc = YAML.parse(source);
    }
    catch {
        doc = null;
    }
    ;
    if (!isNew && !flow)
        return <Empty loading={loading} error={error}/>;
    const addScript=canEdit&&doc&&isControlled(doc)?(path:string,content:string)=>{
        const next=YAML.parse(source),steps=inputDefault(next,'steps',[]),candidate=inlineScriptTask(path,content,steps.map((s:any)=>s.id));
        steps.push({id:candidate.id,kind:/\.py$/i.test(path)?'python':'bash',command:content,depends_on:steps.length?[steps.at(-1).id]:[],inputs:{}});setDefault(next,'steps',steps,'JSON');
        setSource(YAML.stringify(next));setContext('');setTab('Builder');
    }:undefined;
    const active = isNew ? 'Builder' : tab;
    return <><Suspense fallback={<Empty loading/>}><NativeFlowHeaderHost name={name} flow={flow} project={project} tab={active} canEdit={canEdit} canExecute={canExecute} dirty={!!source&&source!==flow?.source} onBack={()=>nav('/flows')} onTab={setTab} onRun={()=>flow&&onRun(flow)}/></Suspense><Notice error={save.error || validate.error || batchTemplate.error}/>{active === 'Builder' ? <><div className="fl-toolbar"><label>Name<input value={name} disabled={!canEdit} onChange={e => setName(e.target.value)}/></label><label>Description<input value={description} disabled={!canEdit} onChange={e => setDescription(e.target.value)}/></label><select value={target} disabled={!canEdit} onChange={e => setTarget(e.target.value)}><option value="server">Server</option><option value="switch">Switch</option><option value="both">Server & Switch</option></select></div><Builder project={project} scopeKey={project+":"+id} filesEnabled={!isNew} source={source} onChange={setSource} onValidate={() => validate.mutate()} onSave={draft => save.mutate(draft ?? false)} filesPanel={!isNew?<NamespaceFiles project={project} id={id} values={extra.data||[]} canEdit={canEdit} useLabel={scriptTarget?undefined:'新增设备脚本步骤'} onUse={scriptTarget&&canEdit?(_path,content)=>{const next=YAML.parse(source),steps=inputDefault(next,'steps',[]);const step=steps.find((s:any)=>s.id===scriptTarget);if(!step)return;step.command=content;setDefault(next,'steps',steps,'JSON');setSource(YAML.stringify(next));setScriptTarget('')}:addScript}/>:undefined} onFiles={stepId => {setScriptTarget(stepId||'')}} onTemplate={(kind = 'bash', policy = 'agent_preferred') => { setTemplateKind(kind); setTemplatePolicy(policy); setContext('Templates'); }} canEdit={canEdit} busy={save.isPending || validate.isPending}/>{validation !== null && <section className="fl-validation"><h3>Kestra validation</h3><Json value={validation}/></section>}</> : ['Logs','Metrics','Concurrency'].includes(active)?<WorkspaceQueries project={project} flowId={id} kind={active==='Logs'?'logs':active==='Metrics'?'metrics':'concurrency'} limit={doc?.concurrency?.limit} onEdit={()=>setTab('Builder')}/>: active === 'Overview' ? <div className="fl-flow-overview"><section><h3>FLOW OVERVIEW</h3><dl className="fl-properties"><dt>Description</dt><dd>{flow?.description || '—'}</dd><dt>Target type</dt><dd>{flow?.target_type}</dd><dt>Namespace</dt><dd>{flow?.namespace}</dd><dt>Revision</dt><dd>{flow?.revision}</dd><dt>Definition authority</dt><dd>Kestra</dd></dl><h3>FLOW TOPOLOGY</h3><div className="fl-overview-dag"><FlowGraph document={doc}/></div></section><aside><h3>RECENT EXECUTIONS</h3>{executions.length?executions.slice(0,5).map(e=><Link className="fl-recent-run" key={e.id} to={'/flows/executions/'+e.id}><span><b>{e.id.slice(0,12)}</b><small>{new Date(e.created_at).toLocaleString()} · {elapsed(e)}</small></span><State value={targetOutcome(e)||e.engine?.state?.current}/></Link>):<Empty>尚无执行记录</Empty>}<Button size="sm" variant="ghost" onClick={()=>setTab('Executions')}>查看全部执行 →</Button><h3>FLOW CONTEXT</h3><Button variant="outline" onClick={() => setTab('Files')}><FileCode2 size={14}/>Namespace Files</Button><Button variant="outline" onClick={() => setTab('Triggers')}>Triggers</Button><Button variant="outline" onClick={() => setTab('Revisions')}><GitBranch size={14}/>Revisions</Button><dl className="fl-properties"><dt>Inputs</dt><dd>{(doc?.inputs||[]).filter((i:any)=>i.id!=='ticket').map((i:any)=>i.id).join(', ')||'—'}</dd><dt>Triggers</dt><dd>{doc?.triggers?.map((t:any)=>t.id).join(', ')||'Manual'}</dd></dl></aside></div> : active === 'Executions' ? tableExec(executions.filter(e => e.flow_id === id)) : active === 'Triggers' ? (flow && isTargetFlow(flow) ? <><TargetSchedule project={project} id={id} source={source} canEdit={canEdit&&canExecute}/><BackfillWorkspace project={project} flowId={id} revision={flow.revision} canExecute={canExecute}/></> : <TriggerView source={source} onChange={setSource} onBuilder={() => setTab('Builder')} canEdit={canEdit}/>) : <><Notice error={extra.error}/>{extra.isLoading ? <Empty loading/> : active === 'Revisions' ? <RevisionView values={extra.data || []} current={source} canEdit={canEdit} onRestore={s => { setSource(s); setTab('Builder'); }}/> : active === 'Dependencies' ? <DependencyView project={project} id={id}/> : active === 'Files' ? <NamespaceFiles project={project} id={id} values={extra.data || []} canEdit={canEdit} onUse={addScript} useLabel="新增设备脚本步骤"/> : <Json value={extra.data || []}/>}</>}

 <Panel open={!!context} title={context} description={context === 'Templates' ? '选择 Kestra 原生起点；设备执行使用受控 Quick Run。' : '当前 Namespace 文件，由 Kestra 提供。'} onClose={() => setContext('')}>{context === 'Templates' ? <div className="fl-template-list"><Notice error={template.error}/><Button variant="outline" disabled={!canEdit || template.isPending} onClick={() => template.mutate()}>CMDB Batch Execution · Agent / SSH</Button>{[['Log', 'io.kestra.plugin.core.log.Log'], ['Parallel', 'io.kestra.plugin.core.flow.Parallel'], ['Loop', 'io.kestra.plugin.core.flow.Loop']].map(([label, type]) => <Button key={type} variant="outline" disabled={!canEdit} onClick={() => { if(source.trim()&&!window.confirm('使用原生模板替换当前未保存定义？设备目标和通道配置将移除。'))return; const d = nativeTemplate(doc || {},label); setSource(YAML.stringify(d)); setContext(''); }}>Start with {label}</Button>)}</div> : <><Notice error={extra.error}/><NamespaceFiles project={project} id={id} values={extra.data || []} canEdit={canEdit} useLabel={scriptTarget?undefined:"新增设备脚本步骤"} onUse={scriptTarget&&canEdit?(path,content)=>{const next=YAML.parse(source),steps=inputDefault(next,'steps',[]);const step=steps.find((s:any)=>s.id===scriptTarget);if(!step)return;step.command=content;setDefault(next,'steps',steps,'JSON');setSource(YAML.stringify(next));setContext('');setScriptTarget('')}:addScript}/></>}</Panel></>;
}
function ExecutionDetail({ execution: e, project, tab, setTab, canExecute }: {
    execution: Execution;
    project: string | null;
    tab: string;
    setTab: (v: string) => void;
    canExecute: boolean;
}) {
    const cache = useQueryClient();
    const [selected, setSelected] = useState(''), [confirm, setConfirm] = useState('');
    const state = String(e.engine?.state?.current || e.error_code || 'NOT_STARTED');
    const target = e.targets.find(t => t.id === selected);
    const logs = useQuery({ queryKey: flowsKey('logs', project, e.id), queryFn: async () => mergeLogs((cache.getQueryData(flowsKey('logs', project, e.id)) as Engine[] | undefined) || [], await flowsApi.logs(project, e.id)), enabled: tab === 'Logs' });
    useEffect(() => {
        if (tab !== "Logs")
            return;
        return followLogs(project, e.id, log => cache.setQueryData(flowsKey('logs', project, e.id), (previous: Engine[] | undefined) => mergeLogs(previous || [], [log])), () => { void cache.invalidateQueries({ queryKey: flowsKey('logs', project, e.id) }); });
    }, [project, e.id, tab, cache]);
    const audit = useQuery({ queryKey: flowsKey('audit', project, e.id), queryFn: () => flowsApi.audit(project, e.id), enabled: tab === 'Audit / Context' });
    const action = useMutation({ mutationFn: (name: string) => flowsApi.action(project, e.id, name, crypto.randomUUID()), onSuccess: result => {
            setConfirm('');
            void cache.invalidateQueries({ queryKey: ['flows'] });
            if (result.id !== e.id)
                nav('/flows/executions/' + result.id);
        } });
    const nav=useNavigate(),[detailUrl,setDetailUrl]=useSearchParams();
    const [replay,setReplay]=useState<{run:import('@/lib/api/flows').Run;flow:Flow|null;mode:'all'|'failed';key:string}|null>(null);
    const prepareReplay=useMutation({mutationFn:async(mode:'all'|'failed')=>{const key=crypto.randomUUID(),run=await flowsApi.replayRequest(project,e.id,mode,key);const flow=run.flow_id?await flowsApi.flow(run.project_id,run.flow_id):null;if(flow&&flow.revision!==run.expected_revision){if(!window.confirm(`源执行 Revision ${run.expected_revision}，当前 Flow Revision ${flow.revision}。是否基于当前定义重新预检？`))return null;run.expected_revision=flow.revision}return {run,flow,mode,key}},onSuccess:setReplay});

    const context=Object.fromEntries(['task_run','attempt','target','metric_page'].map(key=>[key,detailUrl.get(key)||'']));
    const insights=useQuery({queryKey:flowsKey('insights',project,e.id,context.metric_page,tab==='Metrics'),queryFn:()=>flowsApi.insights(project,e.id,{page:context.metric_page||'1',...(tab==='Metrics'?{}:{metrics:'false'})})});
    const variables=useQuery({queryKey:flowsKey('variables',project,e.id),queryFn:()=>flowsApi.variables(project,e.id),enabled:tab==='Outputs & Artifacts'});
    const failed=e.targets.filter(t=>t.state==='FAILED').length;
    return <><Suspense fallback={<Empty loading/>}><NativeExecutionHost key={e.id} execution={e} logs={logs.data||[]} tab={tab} context={context} insights={insights.data} busy={action.isPending||prepareReplay.isPending} canExecute={canExecute} error={String((action.error||prepareReplay.error||logs.error||audit.error||insights.error||variables.error)?.message||'')} onTab={setTab} onAction={name=>{if(name==='restart'||name==='retry-failed-targets')prepareReplay.mutate(name==='restart'?'all':'failed');else setConfirm(name)}} onNavigate={nav} onContext={(key,value)=>setDetailUrl(previous=>{const next=new URLSearchParams(previous);if(value&&value!=='all')next.set(key,value);else next.delete(key);if(key==='task_run')next.delete('attempt');return next},{replace:true})} panels={{topology:<TaskRuns initialView="Topology" topologyOnly executionId={e.id} engine={e.engine} project={project} flowId={e.flow_id} revision={e.flow_revision}/>,outputs:variables.isLoading?<Empty loading/>:<><ArtifactLinks value={variables.data?.outputs} project={project} execution={e.id}/><Suspense fallback={<Empty loading/>}><NativeOutputsHost inputs={variables.data?.inputs||{}} outputs={variables.data?.outputs||{}}/></Suspense></>,audit:<><dl className="fl-properties"><dt>Source</dt><dd>{e.source_module}</dd><dt>Started by</dt><dd>{e.started_by}</dd><dt>Parent Execution</dt><dd>{e.parent_id?<Link to={'/flows/executions/'+e.parent_id}>{e.parent_id}</Link>:'—'}</dd><dt>Kestra ID</dt><dd>{e.kestra_id||'—'}</dd></dl><AuditView records={audit.data||[]}/></>}}/></Suspense>
    {replay&&(!replay.run.target_ids.length&&replay.flow?<NativeRun key={replay.key} flow={replay.flow} nonce={replay.key} seedInputs={replay.run.inputs} submitRun={(run,key)=>flowsApi.replay(run.project_id,e.id,replay.mode,run,key)} onClose={()=>setReplay(null)}/>:<QuickRun key={replay.key} nonce={replay.key} seedRun={replay.run} submitRun={(run,key)=>flowsApi.replay(run.project_id,e.id,replay.mode,run,key)} project={replay.run.project_id} flow={replay.flow} allowed={canExecute} canSave={false} onClose={()=>setReplay(null)}/>)}
    <Panel title="确认执行操作" description={confirm==='restart'?'对原目标集合重新预检，创建新执行；原记录保留。':confirm==='retry-failed-targets'?`仅冻结 ${failed} 台失败设备，成功设备不重跑。`:'状态由 Kestra 与设备实际结果提供；未知结果不会自动重发。'} open={!!confirm} onClose={()=>setConfirm('')}><dl className="fl-properties"><dt>项目</dt><dd>{e.project_id}</dd><dt>Flow Revision</dt><dd>{e.flow_revision}</dd><dt>执行</dt><dd>{e.id}</dd><dt>范围</dt><dd>{confirm==='retry-failed-targets'?e.targets.filter(t=>t.state==='FAILED').map(t=>t.id).join(', '):e.targets.map(t=>t.id).join(', ')||'无设备目标的引擎执行'}</dd></dl><Notice error={action.error}/><Button disabled={action.isPending} onClick={()=>action.mutate(confirm)}>确认 {confirm}</Button></Panel></>;
}

function ArtifactLinks({ value, project, execution }: {
    value: unknown;
    project: string | null;
    execution: string;
}) {
    const files: string[] = [];
    function walk(v: unknown) {
        if (typeof v === 'string' && v.startsWith('kestra://'))
            files.push(v);
        else if (Array.isArray(v))
            v.forEach(walk);
        else if (v && typeof v === 'object')
            Object.values(v).forEach(walk);
    }
    walk(value);
    return <div className="fl-toolbar">{[...new Set(files)].map(uri => <a className="btn" key={uri} href={'/api/v1/flows/executions/' + execution + '/artifacts?' + new URLSearchParams({ path: uri, ...(project ? { project_id: project } : {}) })} download>Download {uri.split('/').at(-1)}</a>)}</div>;
}

function elapsed(e:Execution){const state=e.engine?.state as any;const a=String(state?.startDate||e.created_at),b=state?.endDate?String(state.endDate):'';if(!b)return ['RUNNING','CREATED'].includes(String(state?.current))?'运行中':'—';return Math.max(0,(Date.parse(b)-Date.parse(a))/1000).toFixed(1)+'s'}
