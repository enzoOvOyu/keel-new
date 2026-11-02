import { ReactFlow, Background, Controls, MarkerType } from '@xyflow/react';
import { useNavigate } from 'react-router-dom';
import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import YAML from 'yaml';
import { flowsKey, flowsApi, type Engine } from '@/lib/api/flows';
import { Button, Table, Notice, Empty, State } from './ui';
export function RevisionView({ values, current, onRestore, canEdit }: {
    values: Engine[];
    current: string;
    onRestore: (source: string) => void;
    canEdit: boolean;
}) { const [selected, setSelected] = useState<Engine | null>(null); const lines = (selected?.source || '').split('\n'), now = current.split('\n'); return <div className="fl-context-columns"><section><Table<any> data={values.map((v, i) => ({ ...v, id: String(v.revision ?? i) }))} onSelect={setSelected} columns={[{ accessorKey: 'revision', header: 'Revision' }, { accessorKey: 'description', header: 'Description' }, { id: 'state', header: 'State', cell: ({ row }) => <State value={row.original.disabled ? 'DISABLED' : 'ENABLED'}/> }]}/></section><aside><div className="fl-toolbar"><h3>Revision diff</h3><span className="spacer"/><Button variant="outline" disabled={!canEdit || !selected?.source} onClick={() => onRestore(selected!.source!)}>作为新草稿恢复</Button></div>{selected ? <div className="fl-diff">{Array.from({ length: Math.max(lines.length, now.length) }, (_, i) => <div key={i} className={lines[i] === now[i] ? '' : 'changed'}><code>{i + 1}</code><pre>{lines[i] ?? ''}</pre><pre>{now[i] ?? ''}</pre></div>)}</div> : <Empty>选择版本与当前定义比较</Empty>}</aside></div>; }
export function TriggerView({ source, onChange, onBuilder, canEdit }: {
    source: string;
    onChange: (s: string) => void;
    onBuilder: () => void;
    canEdit: boolean;
}) {
    const [cron, setCron] = useState(''), [id, setId] = useState('schedule'), [zone, setZone] = useState('UTC');
    let doc: any = {};
    try {
        doc = YAML.parse(source) || {};
    }
    catch { }
    ;
    const triggers = doc.triggers || [];
    function update(i: number) { doc.triggers = triggers.map((t: any, n: number) => n === i ? { ...t, disabled: !t.disabled } : t); onChange(YAML.stringify(doc)); onBuilder(); }
    ;
    return <><div className="fl-toolbar"><h3>Triggers</h3><span className="spacer"/><span>修改进入同一 Flow 草稿，校验保存后生效。</span></div><Table<any> data={triggers} columns={[{ accessorKey: 'id', header: 'Trigger' }, { accessorKey: 'type', header: 'Type' }, { accessorKey: 'cron', header: 'Cron' }, { accessorKey: 'timezone', header: 'Timezone' }, { id: 'status', header: 'State', cell: ({ row }) => <State value={row.original.disabled ? 'DISABLED' : 'ENABLED'}/> }, { id: 'actions', header: 'Actions', cell: ({ row }) => <Button variant="outline" size="sm" disabled={!canEdit} onClick={() => update(row.index)}>{row.original.disabled ? '启用草稿' : '停用草稿'}</Button> }]}/><div className="fl-toolbar"><label>Trigger ID<input value={id} onChange={e => setId(e.target.value)}/></label><label>Cron<input placeholder="0 */6 * * *" value={cron} onChange={e => setCron(e.target.value)}/></label><label>Timezone<input value={zone} onChange={e => setZone(e.target.value)}/></label><Button disabled={!canEdit || !id || !cron || triggers.some((t: any) => t.id === id)} onClick={() => { doc.triggers = [...triggers, { id, type: 'io.kestra.plugin.core.trigger.Schedule', cron, timezone: zone, disabled: true }]; onChange(YAML.stringify(doc)); onBuilder(); }}>添加禁用的 Schedule 草稿</Button></div><p className="fl-note">调度属于 Kestra Flow；设备执行仍需有效项目授权、受控目标及通道，不会从任意计划绕过预检。Webhook 与事件触发可在 YAML 中配置并交由引擎验证。</p></>;
}
export { NamespaceFiles } from './NamespaceFiles';
export function DependencyView({ project, id }: {
    project: string;
    id: string;
}) {
    const nav = useNavigate();
    const query = useQuery({ queryKey: flowsKey('dependencies', project, id), queryFn: () => flowsApi.dependencies(project, id) });
    const graph = query.data;
    return <><Notice error={query.error}/>{graph?.partial && <p className="fl-note">仅展示已授权且已登记的当前项目依赖。</p>}{graph?.nodes.length ? <div className="fl-overview-dag"><ReactFlow nodes={graph.nodes.map((n, i) => ({ id: n.uid, position: { x: (i % 3) * 250, y: Math.floor(i / 3) * 140 }, data: { label: n.flow_id }, style: { width: 200 } }))} edges={graph.edges.map((e, i) => ({ id: String(i), source: e.source, target: e.target, markerEnd: { type: MarkerType.ArrowClosed } }))} fitView fitViewOptions={{ maxZoom: 1 }} onNodeClick={(_, n) => {
                const b = graph.nodes.find(v => v.uid === n.id)?.binding_id;
                if (b)
                    nav('/flows/definitions/' + b);
            }}><Background /><Controls /></ReactFlow></div> : <Empty loading={query.isLoading}>当前没有已登记的 Flow 依赖。</Empty>}</>;
}
