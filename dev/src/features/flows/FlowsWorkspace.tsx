import { useEffect, useMemo, useState } from 'react';
import { Link, Navigate, useLocation, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient, useInfiniteQuery } from '@tanstack/react-query';
import YAML from 'yaml';
import { Plus, Play, ArrowLeft, FileCode2, GitBranch, RefreshCw, MoreHorizontal } from 'lucide-react';
import { useProject } from '@/app/ProjectContext';
import { flowsKey, flowsApi, followExecution, followLogs, mergeLogs, type Flow, type Execution, type Agent, type Engine } from '@/lib/api/flows';
import { Builder, FlowGraph } from './Builder';
import { QuickRun } from './QuickRun';
import { NativeRun, isTargetFlow } from './NativeRun';
import { AgentsPage } from './Agents';
import { TaskRuns } from './TaskRuns';
import { RevisionView, TriggerView, NamespaceFiles, DependencyView } from './ContextViews';
import { Button, State, Notice, Table, Panel, Tabs, Search, Empty, Json } from './ui';
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem } from '@/components/shadcn/dropdown-menu';
import './flows.css';
const flowTabs = ['Overview', 'Builder', 'Executions', 'Revisions', 'Triggers', 'Dependencies', 'Files'];
const executionTabs = ['Overview', 'Targets', 'Task Runs', 'Logs', 'Outputs & Artifacts', 'Audit / Context'];
export function FlowsWorkspace() {
    const ctx = useProject(), location = useLocation(), nav = useNavigate(), params = useParams();
    const [url, setUrl] = useSearchParams();
    const project = ctx.filter || null;
    const can = (p: string) => ctx.permissions.includes('flows:' + p);
    const [run, setRun] = useState<Flow | true | null>(null);
    const [q, setQ] = useState(''), [filter, setFilter] = useState('all');
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
    const list = useQuery({ queryKey: flowsKey('list', project), queryFn: () => flowsApi.list(project), enabled: !isAgents && !executionId });
    const executionPages = useInfiniteQuery({ queryKey: flowsKey('executions', project, flowId || '', q, filter), initialPageParam: '', queryFn: ({ pageParam }) => flowsApi.executionPage(project, pageParam, { ...(q ? { q } : {}), ...(filter !== 'all' && isExecutions ? { state: filter } : {}), ...(flowId && flowId !== 'new' ? { flow_id: flowId } : {}) }), getNextPageParam: last => last.page.next_cursor || undefined, enabled: isExecutions || !!flowId && tab === 'Executions', refetchInterval: query => (query.state.data?.pages.length || 0) <= 1 ? 5000 : false });
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
    if (['/flows/blueprints', '/flows/media', '/flows/schedules'].includes(location.pathname))
        return <Navigate to={'/flows?context=' + location.pathname.split('/').at(-1)} replace/>;
    const actionHeader = <div className="fl-heading"><div><small>KEEL / FLOWS</small><h1>{isAgents ? 'Agents' : isExecutions ? 'Executions' : 'Flows'}</h1></div><span className="spacer"/>{!isAgents && !isExecutions && !flowId && <><Button variant="outline" disabled={!can('execute')} title={!ctx.filter ? '先选择本次执行项目' : undefined} onClick={() => setRun(true)}><Play size={14}/>Quick Run</Button><Button disabled={!can('create')} onClick={() => nav('/flows/definitions/new')}><Plus size={14}/>Create Flow</Button></>}</div>;
    const tableExec = (data: Execution[]) => <Table data={data} onSelect={e => nav('/flows/executions/' + e.id)} columns={[{ accessorKey: 'id', header: 'Execution', cell: ({ row }) => <><b>{row.original.id.slice(0, 12)}</b><small>{row.original.kestra_id || '引擎尚未启动'}</small></> }, { accessorKey: 'engine_flow_id', header: 'Flow / Source', cell: ({ row }) => <>{row.original.engine_flow_id}<small>{row.original.source_module}</small></> }, { accessorKey: 'project_id', header: 'Project' }, { id: 'targets', header: 'Targets / Result', cell: ({ row }) => <>{row.original.targets.length} targets<small>{row.original.targets.filter(t => t.state === 'SUCCESS').length} success / {row.original.targets.filter(t => t.state === 'FAILED').length} failed</small></> }, { accessorKey: 'mode', header: 'Mode / Route', cell: ({ row }) => <>{row.original.mode}<small>Agent {row.original.targets.filter(t => t.actual_transport === 'AGENT').length} / SSH {row.original.targets.filter(t => t.actual_transport === 'SSH').length}</small></> }, { accessorKey: 'created_at', header: 'Started', cell: ({ getValue }) => new Date(String(getValue())).toLocaleString() }, { id: 'state', header: 'State', cell: ({ row }) => <State value={state(row.original)}/> }]}/>;
    return <div className="fl-workspace">{!flowId && !executionId && actionHeader}<Notice error={capability.error}/>{capability.data && !capability.data.configured && <div className="fl-warning">Kestra 尚未配置。可检查 CMDB 和 Agent，执行与定义保存需要先启动本分支的执行底座。</div>}
 {isAgents ? <AgentsPage project={project} canEdit={can('edit')}/> : executionId ? ex.data ? <ExecutionDetail execution={ex.data} project={project} tab={tab} setTab={setTab} canExecute={can('execute')}/> : <Empty loading={ex.isLoading} error={ex.error}/> : flowId ? <FlowDetail id={flowId} flow={flow.data} loading={flow.isLoading} error={flow.error} project={ctx.filter || ctx.projectId} tab={tab} setTab={setTab} canEdit={can(flowId === 'new' ? 'create' : 'edit')} canExecute={can('execute')} onRun={f => setRun(f)} executions={executions.data || []} tableExec={tableExec}/> : isExecutions ? <><div className="fl-toolbar"><Search value={q} onChange={setQ} placeholder="Execution ID / Flow / 设备"/><select value={filter} onChange={e => setFilter(e.target.value)}><option value="all">所有状态</option>{['RUNNING', 'SUCCESS', 'FAILED', 'WARNING', 'PAUSED', 'KILLED', 'RECONCILING'].map(v => <option key={v}>{v}</option>)}</select><Button variant="outline" size="sm" onClick={() => void executions.refetch()}><RefreshCw size={14}/>刷新</Button></div><Notice error={executions.error}/>{executions.isLoading ? <Empty loading/> : tableExec((executions.data || []).filter(e => (filter === 'all' || state(e) === filter) && JSON.stringify([e.id, e.engine_flow_id, e.targets.map(t => t.snapshot.name)]).toLowerCase().includes(q.toLowerCase())))}</> : <><div className="fl-toolbar"><Search value={q} onChange={setQ} placeholder="搜索 Flow 名称 / ID / Namespace"/><select value={filter} onChange={e => setFilter(e.target.value)}><option value="all">所有目标类型</option><option value="server">Server</option><option value="switch">Switch</option><option value="both">Server & Switch</option></select><Button size="sm" variant="outline" onClick={() => void list.refetch()}><RefreshCw size={14}/>刷新</Button><span className="spacer"/><span>{list.data ? list.data.length + ' flows' : '加载中…'}</span></div><Notice error={list.error || prepareRun.error || manage.error}/>{list.isLoading ? <Empty loading/> : <Table data={(list.data || []).filter(f => (filter === 'all' || f.target_type === filter) && [f.name, f.flow_id, f.namespace].join(' ').toLowerCase().includes(q.toLowerCase()))} onSelect={f => nav('/flows/definitions/' + f.id)} columns={[{ accessorKey: 'name', header: 'Name / Flow ID', cell: ({ row }) => <><b>{row.original.name}</b><small>{row.original.flow_id}</small></> }, { accessorKey: 'description', header: 'Description' }, { accessorKey: 'namespace', header: 'Namespace' }, { accessorKey: 'target_type', header: 'Target' }, { accessorKey: 'revision', header: 'Revision', cell: ({ getValue }) => 'rev ' + getValue() }, { id: 'trigger', header: 'Trigger', cell: ({ row }) => row.original.engine?.triggers?.length ? row.original.engine.triggers.map((t: any) => t.id).join(', ') : 'Manual' }, { id: 'state', header: 'Enabled', cell: ({ row }) => <State value={row.original.engine && Object.keys(row.original.engine).length ? (row.original.draft ? 'DRAFT' : row.original.disabled ? 'DISABLED' : 'ENABLED') : 'unknown'}/> }, { id: 'actions', header: 'Actions', cell: ({ row }) => <div className="fl-row-actions"><Button size="sm" variant="outline" disabled={!can('execute') || row.original.disabled || row.original.draft} onClick={() => prepareRun.mutate(row.original)}>Run</Button><Button size="sm" variant="outline" onClick={() => nav('/flows/definitions/' + row.original.id + '?tab=Builder')}>Edit</Button><DropdownMenu><DropdownMenuTrigger asChild><Button size="sm" variant="outline" aria-label="Flow actions"><MoreHorizontal size={14}/></Button></DropdownMenuTrigger><DropdownMenuContent><DropdownMenuItem disabled={!can('edit') || manage.isPending} onSelect={() => manage.mutate({ flow: row.original, action: 'toggle' })}>{row.original.disabled ? 'Enable' : 'Disable'}</DropdownMenuItem><DropdownMenuItem disabled={!can('create') || manage.isPending} onSelect={() => manage.mutate({ flow: row.original, action: 'copy' })}>Copy as draft</DropdownMenuItem></DropdownMenuContent></DropdownMenu></div> }]}/>}</>}
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
    const [source, setSource] = useState(''), [name, setName] = useState(''), [description, setDescription] = useState(''), [target, setTarget] = useState('server'), [context, setContext] = useState(''), [validation, setValidation] = useState<unknown>(null);
    const isNew = id === 'new';
    const namespaceQuery = useQuery({ queryKey: flowsKey('capabilities', project), queryFn: () => flowsApi.capabilities(project), enabled: isNew });
    useEffect(() => {
        if (flow) {
            setSource(flow.source);
            setName(flow.name);
            setDescription(flow.description);
            setTarget(flow.target_type);
        }
        else if (isNew) {
            setSource(YAML.stringify({ id: 'new-flow', namespace: namespaceQuery.data?.namespace || '', description: '', tasks: [{ id: 'start', type: 'io.kestra.plugin.core.log.Log', message: 'Flow started' }] }));
        }
    }, [flow, isNew, project, namespaceQuery.data?.namespace]);
    const save = useMutation({ mutationFn: (draft: boolean) => flowsApi.save(project, isNew ? null : id, { project_id: project, name, description, target_type: target, source, expected_revision: flow?.revision || 0, draft }), onSuccess: f => { void cache.invalidateQueries({ queryKey: ['flows'] }); nav('/flows/definitions/' + f.id + '?tab=Builder'); } });
    const [templateKind, setTemplateKind] = useState('bash'), [templatePolicy, setTemplatePolicy] = useState('agent_preferred');
    const template = useMutation({ mutationFn: () => flowsApi.template(project), onSuccess: t => { const d = YAML.parse(t.source); d.inputs.push({ id: 'command', type: 'STRING', defaults: templateKind === 'file.upload' ? JSON.stringify({ path: 'uploads/file.txt', data_base64: '', sha256: '' }) : '' }, { id: 'kind', type: 'STRING', defaults: templateKind }, { id: 'transport_policy', type: 'STRING', defaults: templatePolicy }); setSource(YAML.stringify(d)); setName(t.name); setContext(''); } });
    const validate = useMutation({ mutationFn: () => flowsApi.validate(project, id, source), onSuccess: setValidation });
    const extra = useQuery({ queryKey: flowsKey('context', project, id, context || tab), queryFn: () => flowsApi.context(project, id, (context || tab).toLowerCase()), enabled: !isNew && ['Revisions', 'Files'].includes(context || tab) });
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
    const active = isNew ? 'Builder' : tab;
    return <><Link className="fl-back" to="/flows"><ArrowLeft size={14}/>Flows</Link><div className="fl-heading"><div><h1>{isNew ? 'Create Flow' : flow?.name}</h1><small>{flow?.namespace || 'Namespace 绑定当前项目'} · {flow ? 'Revision ' + flow.revision : 'New draft'} · {project}</small></div><span className="spacer"/>{flow && <><State value={flow.draft ? 'DRAFT' : flow.disabled ? 'DISABLED' : 'ENABLED'}/><Button variant="outline" onClick={() => setTab('Builder')}>Edit Flow</Button><Button disabled={!canExecute || flow.disabled || flow.draft} onClick={() => onRun(flow)}>Run</Button></>}</div><Tabs items={isNew ? ['Builder'] : flowTabs} current={active} onChange={setTab}/><Notice error={save.error || validate.error}/>{active === 'Builder' ? <><div className="fl-toolbar"><label>Name<input value={name} disabled={!canEdit} onChange={e => setName(e.target.value)}/></label><label>Description<input value={description} disabled={!canEdit} onChange={e => setDescription(e.target.value)}/></label><select value={target} disabled={!canEdit} onChange={e => setTarget(e.target.value)}><option value="server">Server</option><option value="switch">Switch</option><option value="both">Server & Switch</option></select></div><Builder filesEnabled={!isNew} source={source} onChange={setSource} onValidate={() => validate.mutate()} onSave={draft => save.mutate(draft ?? false)} onFiles={() => setContext('Files')} onTemplate={(kind = 'bash', policy = 'agent_preferred') => { setTemplateKind(kind); setTemplatePolicy(policy); setContext('Templates'); }} canEdit={canEdit} busy={save.isPending || validate.isPending}/>{validation !== null && <section className="fl-validation"><h3>Kestra validation</h3><Json value={validation}/></section>}</> : active === 'Overview' ? <div className="fl-flow-overview"><section><h3>FLOW OVERVIEW</h3><dl className="fl-properties"><dt>Description</dt><dd>{flow?.description || '—'}</dd><dt>Target type</dt><dd>{flow?.target_type}</dd><dt>Namespace</dt><dd>{flow?.namespace}</dd><dt>Revision</dt><dd>{flow?.revision}</dd><dt>Definition authority</dt><dd>Kestra</dd></dl><h3>FLOW TOPOLOGY</h3><div className="fl-overview-dag"><FlowGraph document={doc}/></div></section><aside><h3>FLOW CONTEXT</h3><Button variant="outline" onClick={() => setTab('Files')}><FileCode2 size={14}/>Namespace Files</Button><Button variant="outline" onClick={() => setTab('Triggers')}>Triggers</Button><Button variant="outline" onClick={() => setTab('Revisions')}><GitBranch size={14}/>Revisions</Button><p>定义、Revision、触发器和执行状态来自 Kestra；设备身份和目标来自 CMDB。</p></aside></div> : active === 'Executions' ? tableExec(executions.filter(e => e.flow_id === id)) : active === 'Triggers' ? <TriggerView source={source} onChange={setSource} onBuilder={() => setTab('Builder')} canEdit={canEdit}/> : <><Notice error={extra.error}/>{extra.isLoading ? <Empty loading/> : active === 'Revisions' ? <RevisionView values={extra.data || []} current={source} canEdit={canEdit} onRestore={s => { setSource(s); setTab('Builder'); }}/> : active === 'Dependencies' ? <DependencyView project={project} id={id}/> : active === 'Files' ? <NamespaceFiles project={project} id={id} values={extra.data || []} canEdit={canEdit}/> : <Json value={extra.data || []}/>}</>}

 <Panel open={!!context} title={context} description={context === 'Templates' ? '选择 Kestra 原生起点；设备执行使用受控 Quick Run。' : '当前 Namespace 文件，由 Kestra 提供。'} onClose={() => setContext('')}>{context === 'Templates' ? <div className="fl-template-list"><Notice error={template.error}/><Button variant="outline" disabled={!canEdit || template.isPending} onClick={() => template.mutate()}>CMDB Batch Execution · Agent / SSH</Button>{[['Log', 'io.kestra.plugin.core.log.Log'], ['Parallel', 'io.kestra.plugin.core.flow.Parallel'], ['Loop', 'io.kestra.plugin.core.flow.Loop']].map(([label, type]) => <Button key={type} variant="outline" disabled={!canEdit} onClick={() => { const d = doc || {}; d.tasks = [{ id: 'start', type, ...(label === 'Log' ? { message: 'Flow started' } : label === 'Loop' ? { values: '{{ inputs.targets }}', concurrencyLimit: 8, tasks: [] } : { tasks: [] }) }]; setSource(YAML.stringify(d)); setContext(''); }}>Start with {label}</Button>)}</div> : <><Notice error={extra.error}/><NamespaceFiles project={project} id={id} values={extra.data || []} canEdit={canEdit}/></>}</Panel></>;
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
                window.location.assign('/flows/executions/' + result.id);
        } });
    const failed = e.targets.filter(t => t.state === 'FAILED').length;
    const targetTable = <Table data={e.targets} onSelect={t => setSelected(t.id)} columns={[{ id: 'target', header: 'Target / Asset', cell: ({ row }) => <><b>{row.original.snapshot.name}</b><small>{row.original.id}</small></> }, { accessorKey: 'actual_transport', header: 'Transport' }, { accessorKey: 'state', header: 'State', cell: ({ getValue }) => <State value={getValue()}/> }, { accessorKey: 'attempt', header: 'Attempt' }, { accessorKey: 'exit_code', header: 'Exit Code', cell: ({ getValue }) => getValue() ?? '—' }, { id: 'duration', header: 'Duration', cell: ({ row }) => row.original.ended_at && row.original.started_at ? ((Date.parse(row.original.ended_at) - Date.parse(row.original.started_at)) / 1000).toFixed(1) + 's' : '—' }]}/>;
    return <><Link className="fl-back" to="/flows/executions"><ArrowLeft size={14}/>Executions</Link><div className="fl-heading"><div><h1>Execution · {e.id.slice(0, 12)}</h1><small>{e.engine_flow_id} · rev {e.flow_revision} · {e.project_id} · {e.mode}</small></div><State value={state}/><span className="spacer"/>{[['pause', 'Pause', ['RUNNING']], ['resume', 'Resume', ['PAUSED']], ['kill', 'Kill', ['RUNNING', 'PAUSED', 'CREATED']], ['restart', 'Restart All Targets', ['SUCCESS', 'FAILED', 'WARNING', 'KILLED']], ['retry-failed-targets', 'Retry Failed Targets', ['SUCCESS', 'FAILED', 'WARNING', 'KILLED']]].map(([id, label, states]) => <Button key={id as string} size="sm" variant="outline" disabled={!canExecute || !(states as string[]).includes(state) || id === 'retry-failed-targets' && !failed || action.isPending} onClick={() => setConfirm(id as string)}>{label as string}</Button>)}</div><div className="fl-ex-summary"><span>{e.targets.length} targets</span><span>{e.targets.filter(t => t.state === 'SUCCESS').length} success</span><span>{failed} failed</span><span>{e.targets.filter(t => t.state === 'RUNNING').length} running</span><span>Agent {e.targets.filter(t => t.actual_transport === 'AGENT').length} / SSH {e.targets.filter(t => t.actual_transport === 'SSH').length}</span><span>{new Date(e.created_at).toLocaleString()}</span></div><Tabs items={executionTabs} current={tab} onChange={setTab}/><Notice error={action.error || logs.error || audit.error}/>{tab === 'Overview' || tab === 'Targets' ? <div className="fl-ex-detail"><section>{targetTable}</section><aside><h3>SELECTED TARGET</h3>{target ? <><b>{target.snapshot.name}</b><dl className="fl-properties"><dt>Asset</dt><dd><Link to={'/cmdb?asset=' + encodeURIComponent(target.id)}>{target.id}</Link></dd><dt>Transport</dt><dd>{target.actual_transport || '—'}</dd><dt>SSH Trust</dt><dd>{target.snapshot.ssh_profile_id || '—'}</dd><dt>Agent</dt><dd>{target.snapshot.agent_id || '—'}</dd><dt>Exit code</dt><dd>{target.exit_code ?? '—'}</dd></dl><h4>stdout</h4><pre className="fl-code">{target.stdout || '尚无输出'}</pre><h4>stderr</h4><pre className="fl-code">{target.stderr || '尚无输出'}</pre></> : <p>选择目标查看状态、通道和输出。</p>}</aside></div> : tab === 'Task Runs' ? <TaskRuns engine={e.engine}/> : tab === 'Logs' ? <pre className="fl-log-stream">{(logs.data || []).map((l: any) => `${l.timestamp || ''} ${l.level || ''} ${l.message || ''}`).join('\n') || '暂无引擎日志'}</pre> : tab === 'Outputs & Artifacts' ? <><ArtifactLinks value={e.engine?.outputs} project={project} execution={e.id}/><Json value={e.engine?.outputs || {}}/></> : <><dl className="fl-properties"><dt>Source</dt><dd>{e.source_module}</dd><dt>Started by</dt><dd>{e.started_by}</dd><dt>Parent Execution</dt><dd>{e.parent_id ? <Link to={'/flows/executions/' + e.parent_id}>{e.parent_id}</Link> : '—'}</dd><dt>Kestra ID</dt><dd>{e.kestra_id || '—'}</dd></dl><Json value={audit.data || []}/></>}
 <Panel title="确认执行操作" description={confirm === 'restart' ? '对原目标集合重新预检，创建新 Execution 并重跑全部目标；原执行记录保留。' : confirm === 'retry-failed-targets' ? `仅冻结 ${failed} 台失败设备，创建新的 Execution；成功设备不重跑。` : '操作交由 Kestra；未知结果不启动第二份设备任务。'} open={!!confirm} onClose={() => setConfirm('')}><p>{confirm} · {e.id}</p><Notice error={action.error}/><Button disabled={action.isPending} onClick={() => action.mutate(confirm)}>确认</Button></Panel></>;
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
