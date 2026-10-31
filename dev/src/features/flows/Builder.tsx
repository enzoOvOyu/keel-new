import { useMemo, useState } from 'react';
import { ReactFlow, Background, Controls, MarkerType, Panel as CanvasPanel, type Node, type Edge } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import YAML from 'yaml';
import { Button, Notice, Tabs } from './ui';
const library = [['KEEL · ADAPTER', 'CMDB Target Resolver', 'keel.cmdb.resolve'], ['KEEL · ADAPTER', 'Transport Selector', 'keel.transport.select'], ['KEEL · ADAPTER', 'Agent Task', 'keel.agent.task'], ['KEEL · ADAPTER', 'SSH Command', 'keel.ssh.command'], ['KEEL · ADAPTER', 'File Transfer', 'keel.file.transfer'], ['CONTROL FLOW', 'Loop', 'io.kestra.plugin.core.flow.Loop'], ['CONTROL FLOW', 'Parallel', 'io.kestra.plugin.core.flow.Parallel'], ['CONTROL FLOW', 'Condition', 'io.kestra.plugin.core.flow.If'], ['CONTROL FLOW', 'Subflow', 'io.kestra.plugin.core.flow.Subflow'], ['KEEL · ADAPTER', 'Bash Script', 'keel.script.bash'], ['KEEL · ADAPTER', 'Python Script', 'keel.script.python'], ['KESTRA TASKS', 'Log / Output', 'io.kestra.plugin.core.log.Log']] as const;
export function FlowGraph({ document, onSelect, readonly = true }: {
    document: any;
    onSelect?: (id: string) => void;
    readonly?: boolean;
}) {
    const [offsets, setOffsets] = useState<Record<string, {
        x: number;
        y: number;
    }>>({});
    const [nodes, edges] = useMemo(() => {
        const nodes: Node[] = [], edges: Edge[] = [];
        function sequence(tasks: any[], incoming: string[], column: number): string[] {
            let previous = incoming;
            for (const task of tasks || []) {
                previous = place(task, previous, column);
            }
            return previous;
        }
        function place(t: any, incoming: string[], column: number): string[] {
            if (!t?.id)
                return incoming;
            const id = String(t.id);
            nodes.push({ id, position: offsets[id] || { x: column * 260, y: nodes.length * 110 }, data: { label: <><strong>{id}</strong><small>{String(t.type || '').split('.').slice(-2).join('.')}</small></> }, style: { width: 220, borderRadius: 6, border: '1px solid var(--border,#d7dce5)', background: 'var(--card,#fff)', fontSize: 12 }, draggable: !readonly });
            for (const from of incoming)
                edges.push({ id: from + '-' + id, source: from, target: id, markerEnd: { type: MarkerType.ArrowClosed }, type: 'smoothstep' });
            if (String(t.type).endsWith('.Parallel'))
                return (t.tasks || []).flatMap((child: any, i: number) => place(child, [id], column + i));
            if (String(t.type).endsWith('.If'))
                return [...sequence(t.then || [], [id], column + 1), ...sequence(t.else || [], [id], column - 1)];
            return Array.isArray(t.tasks) && t.tasks.length ? sequence(t.tasks, [id], column + 1) : [id];
        }
        sequence(document?.tasks || [], [], 0);
        for (const task of document?.finally || []) {
            const before = nodes.length;
            place(task, [], 0);
            if (nodes[before])
                nodes[before].data.label = <><strong>{task.id}</strong><small>Finally · 任何终态</small></>;
        }
        return [nodes, edges] as const;
    }, [document, offsets, readonly]);
    return <ReactFlow nodes={nodes} edges={edges} fitView fitViewOptions={{ maxZoom: 1, padding: .3 }} onPaneClick={() => onSelect?.('')} onNodeClick={(_, node) => onSelect?.(node.id)} onNodeDragStop={(_, node) => setOffsets(v => ({ ...v, [node.id]: node.position }))} nodesConnectable={false} nodesDraggable={!readonly} minZoom={.25} maxZoom={2}><Background /><Controls /></ReactFlow>;
}
export function Builder({ source, onChange, onValidate, onSave, onFiles, onTemplate, canEdit, busy, filesEnabled = true }: {
    source: string;
    onChange: (s: string) => void;
    onValidate: () => void;
    onSave: (draft?: boolean) => void;
    onFiles: () => void;
    onTemplate: (kind?: string, policy?: string) => void;
    canEdit: boolean;
    busy: boolean;
    filesEnabled?: boolean;
}) {
    const [view, setView] = useState('Visual'), [selected, setSelected] = useState(''), [search, setSearch] = useState(''), [editError, setEditError] = useState('');
    let doc: any = null, parseError = '';
    try {
        doc = YAML.parse(source);
        if (!doc || typeof doc !== 'object')
            parseError = '需要有效的 Kestra YAML';
    }
    catch (e) {
        parseError = String(e);
    }
    const find = (tasks: any[]): any => {
        for (const t of tasks || []) {
            if (t.id === selected)
                return t;
            const nested = find([...(t.tasks || []), ...(t.then || []), ...(t.else || [])]);
            if (nested)
                return nested;
        }
        return null;
    };
    const task = doc ? find([...(doc.tasks || []), ...(doc.finally || []), ...(doc.errors || [])]) : null;
    function changeTask(patch: any) {
        if (!canEdit || !task)
            return;
        Object.assign(task, patch);
        onChange(YAML.stringify(doc));
    }
    function add(type: string) {
        if (!canEdit || !doc)
            return;
        if (type.startsWith('keel.')) {
            const kind = type === 'keel.script.python' ? 'python' : type === 'keel.ssh.command' ? 'command' : type === 'keel.file.transfer' ? 'file.upload' : 'bash';
            const policy = type === 'keel.agent.task' || type === 'keel.file.transfer' ? 'agent_only' : type === 'keel.ssh.command' ? 'ssh_only' : 'agent_preferred';
            onTemplate(kind, policy);
            return;
        }
        const id = 'task_' + crypto.randomUUID().slice(0, 6);
        const t: any = { id, type };
        if (type.endsWith('.Log'))
            t.message = '';
        if (type.endsWith('.Loop'))
            Object.assign(t, { values: '{{ inputs.targets }}', concurrencyLimit: 8, tasks: [] });
        if (type.endsWith('.Parallel'))
            t.tasks = [];
        if (type.endsWith('.If'))
            Object.assign(t, { condition: '', then: [], else: [] });
        if (type.endsWith('.Subflow'))
            Object.assign(t, { namespace: doc.namespace, flowId: '' });
        if (type.endsWith('.Commands'))
            t.commands = [''];
        if (type.endsWith('.Script'))
            t.script = '';
        if (task && Array.isArray(task.tasks))
            task.tasks.push(t);
        else
            doc.tasks = [...(doc.tasks || []), t];
        onChange(YAML.stringify(doc));
        setSelected(id);
        setEditError('');
    }
    return <div className="fl-builder"><div className="fl-builder-toolbar"><Tabs items={['Visual', 'YAML', 'Topology']} current={view} onChange={setView}/><span className="spacer"/><Button size="sm" variant="outline" onClick={() => onTemplate()}>Start from Template</Button><Button size="sm" variant="outline" disabled={!filesEnabled} title={!filesEnabled ? '保存 Flow 后管理 Namespace Files' : undefined} onClick={onFiles}>Files</Button><Button size="sm" variant="outline" disabled={busy} onClick={onValidate}>Validate</Button><Button size="sm" variant="outline" disabled={!canEdit || busy || !!parseError} onClick={() => onSave(true)}>Save Draft</Button><Button size="sm" disabled={!canEdit || busy || !!parseError} onClick={() => onSave(false)}>Publish</Button></div><Notice error={parseError || editError}/>{view === 'YAML' ? <textarea aria-label="Kestra Flow YAML" className="fl-yaml" readOnly={!canEdit} value={source} onChange={e => onChange(e.target.value)}/> : <div className="fl-builder-columns"><aside className="fl-node-library"><h3>NODE LIBRARY</h3><button onClick={() => setSelected('')}>Flow Settings / Inputs</button><input placeholder="Search nodes…" value={search} onChange={e => setSearch(e.target.value)}/>{library.filter(t => t.join(' ').toLowerCase().includes(search.toLowerCase())).map(([group, label, type], i, a) => <div key={type}>{i === 0 || group !== a[i - 1][0] ? <h4>{group}</h4> : null}<button disabled={!canEdit || view === 'Topology'} title={type} onClick={() => add(type)}>{label}<small>{type.startsWith('keel.') ? 'Keel 接入点 · 非原生插件' : type.split('.').slice(-1)}</small></button></div>)}<h4>ERROR HANDLING</h4><button disabled={!canEdit} onClick={() => { doc.finally = [...(doc.finally || []), { id: 'finalize_' + crypto.randomUUID().slice(0, 6), type: 'io.kestra.plugin.core.log.Log', message: 'Execution finished' }]; onChange(YAML.stringify(doc)); }}>Finally</button></aside><div className="fl-dag"><FlowGraph document={doc} readonly={view === 'Topology' || !canEdit} onSelect={setSelected}/></div><aside className="fl-node-inspector"><h3>NODE INSPECTOR</h3>{task ? <><label>Task ID<input disabled={!canEdit} value={task.id} onChange={e => { const id = e.target.value; changeTask({ id }); setSelected(id); }}/></label><label>Type<code>{task.type}</code></label>{['message', 'script', 'values', 'condition', 'namespace', 'flowId', 'body'].filter(k => k in task).map(k => <label key={k}>{k}<textarea disabled={!canEdit} value={String(task[k])} onChange={e => changeTask({ [k]: e.target.value })}/></label>)}{'commands' in task && <label>Commands<textarea disabled={!canEdit} value={task.commands.join('\n')} onChange={e => changeTask({ commands: e.target.value.split('\n') })}/></label>}{'concurrencyLimit' in task && <label>Target concurrency<input disabled={!canEdit} type="number" min={1} max={64} value={task.concurrencyLimit} onChange={e => changeTask({ concurrencyLimit: Number(e.target.value) })}/></label>}<label>最大重试次数<input disabled={!canEdit || task.type === 'io.kestra.plugin.core.http.Request'} type="number" min={0} max={10} value={Math.max(0, (task.retry?.maxAttempts || 1) - 1)} onChange={e => {
                    const retries = Number(e.target.value);
                    if (retries === 0) {
                        delete task.retry;
                        onChange(YAML.stringify(doc));
                    }
                    else
                        changeTask({ retry: { type: 'constant', maxAttempts: retries + 1, interval: 'PT1S' } });
                }}/></label><small>目标端副作用不自动重放；使用失败目标定向重试。</small><label>Timeout<input disabled={!canEdit} placeholder="PT10M" value={task.timeout || ''} onChange={e => changeTask({ timeout: e.target.value })}/></label><p>Flow 并发与单次目标并发分开配置。插件和字段支持由 Kestra 校验。</p><Button variant="outline" disabled={!canEdit} onClick={() => { const remove = (tasks: any[]): any[] => tasks.filter(t => t.id !== selected).map(t => ({ ...t, ...(t.tasks ? { tasks: remove(t.tasks) } : {}) })); doc.tasks = remove(doc.tasks || []); onChange(YAML.stringify(doc)); setSelected(''); }}>删除节点</Button></> : <FlowInputs document={doc} onChange={onChange} canEdit={canEdit}/>}</aside></div>}</div>;
}
function FlowInputs({ document: d, onChange, canEdit }: {
    document: any;
    onChange: (s: string) => void;
    canEdit: boolean;
}) {
    if (!d)
        return <p>先修正 YAML。</p>;
    return <><label>Flow concurrency<input disabled={!canEdit} type="number" min={1} max={64} value={d.concurrency?.limit || 1} onChange={e => { d.concurrency = { limit: Number(e.target.value), behavior: 'QUEUE' }; onChange(YAML.stringify(d)); }}/></label><p>同一 Flow 的 Execution 并发；与 Loop 内的目标并发分别配置。</p><h4>Inputs</h4>{(d.inputs || []).map((input: any, i: number) => <div className="fl-validation" key={i}><label>Name<input disabled={!canEdit || input.id === 'ticket' || input.id === 'targets'} value={input.id} onChange={e => { input.id = e.target.value; onChange(YAML.stringify(d)); }}/></label><label>Type<select disabled={!canEdit || input.id === 'ticket' || input.id === 'targets'} value={input.type} onChange={e => { input.type = e.target.value; onChange(YAML.stringify(d)); }}>{['STRING', 'INT', 'BOOLEAN', 'JSON', 'SECRET'].map(t => <option key={t}>{t}</option>)}</select></label>{input.id !== 'ticket' && input.id !== 'targets' && input.type !== 'SECRET' && <label>Default<textarea disabled={!canEdit} value={typeof input.defaults === 'string' ? input.defaults : input.defaults === undefined ? '' : JSON.stringify(input.defaults)} onChange={e => { input.defaults = e.target.value; onChange(YAML.stringify(d)); }}/></label>}<small>{input.id === 'ticket' ? '受控 Job Service 注入，不填写明文。' : input.id === 'targets' ? '由 CMDB 目标预检冻结。' : '原生 Kestra 输入参数'}</small></div>)}<Button size="sm" variant="outline" disabled={!canEdit} onClick={() => { d.inputs = [...(d.inputs || []), { id: 'input_' + crypto.randomUUID().slice(0, 5), type: 'STRING' }]; onChange(YAML.stringify(d)); }}>Add Input</Button></>;
}
