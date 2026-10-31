import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useProject } from '@/app/ProjectContext';
import { flowsKey, flowsApi, type SSHTrust } from '@/lib/api/flows';
import { Button, Panel, Notice, Table, State, Search } from './ui';
import './flows.css';
export function SSHTrustWorkspace() {
    const ctx = useProject(), cache = useQueryClient();
    const [open, setOpen] = useState(false), [q, setQ] = useState(''), [error, setError] = useState('');
    const [form, setForm] = useState({ project_id: ctx.filter || ctx.projectId, asset_id: '', host: '', port: 22, username: '', secret_ref: '', known_hosts_ref: '', status: 'approved', expected_revision: 0 });
    const list = useQuery({ queryKey: flowsKey('ssh-trust', ctx.filter), queryFn: () => flowsApi.ssh(ctx.filter) });
    const targets = useQuery({ queryKey: flowsKey('targets', form.project_id), queryFn: () => flowsApi.targets(form.project_id), enabled: open });
    const save = useMutation({ mutationFn: () => flowsApi.saveSSH(form.project_id, form), onSuccess: () => { setOpen(false); void cache.invalidateQueries({ queryKey: ['flows'] }); } });
    const can = ctx.permissions.includes('admin:edit');
    function edit(row?: SSHTrust) { setForm(row ? { project_id: row.project_id, asset_id: row.asset_id, host: row.host, port: row.port, username: row.username, secret_ref: row.secret_ref, known_hosts_ref: row.known_hosts_ref, status: row.status, expected_revision: row.revision } : { project_id: ctx.filter || ctx.projectId, asset_id: '', host: '', port: 22, username: '', secret_ref: '', known_hosts_ref: '', status: 'approved', expected_revision: 0 }); setOpen(true); setError(''); save.reset(); }
    ;
    return <div className="fl-workspace"><div className="fl-heading"><div><h1>SSH 信任管理</h1><small>受控密钥引用与严格 Known Hosts；不向浏览器读取私钥。</small></div><span className="spacer"/><Button disabled={!can} onClick={() => edit()}>配置 SSH Trust</Button></div><div className="fl-toolbar"><Search value={q} onChange={setQ} placeholder="Asset / Host / Username"/></div><Notice error={list.error}/><Table data={(list.data || []).filter(t => JSON.stringify([t.asset_id, t.host, t.username]).toLowerCase().includes(q.toLowerCase()))} onSelect={edit} columns={[{ accessorKey: 'asset_id', header: 'CMDB Asset' }, { accessorKey: 'host', header: 'Host' }, { accessorKey: 'port', header: 'Port' }, { accessorKey: 'username', header: 'Execution identity' }, { accessorKey: 'secret_ref', header: 'Secret Reference' }, { accessorKey: 'known_hosts_ref', header: 'Known Hosts Reference' }, { accessorKey: 'status', header: 'State', cell: ({ getValue }) => <State value={getValue()}/> }, { accessorKey: 'revision', header: 'Revision' }]}/><Panel open={open} title="配置 SSH Trust" description="主机地址必须对应当前项目 CMDB；维护态禁止配置带内访问。" onClose={() => setOpen(false)}><Notice error={save.error || targets.error || error}/><label>CMDB 设备<select value={form.asset_id} disabled={!can || !!form.expected_revision} onChange={e => {
            const target = targets.data?.find(t => t.id === e.target.value);
            if (!target)
                return;
            const host = String(target.fields.management_ip || target.fields.mgmt_ip || target.fields.ip || '');
            if (!host)
                setError('该设备没有已确认的管理地址，请先在原数据来源中补齐。');
            else
                setError('');
            setForm(v => ({ ...v, asset_id: target.id, host }));
        }}><option value="">选择设备</option>{targets.data?.map(t => <option key={t.id} value={t.id}>{t.name} · {t.type}</option>)}</select></label><div className="fl-fields"><label>Host<input readOnly value={form.host}/></label><label>Port<input type="number" min={1} max={65535} disabled={!can} value={form.port} onChange={e => setForm(v => ({ ...v, port: Number(e.target.value) }))}/></label></div>{(['username', 'secret_ref', 'known_hosts_ref'] as const).map(k => <label key={k}>{k}<input disabled={!can} value={form[k]} autoComplete="off" onChange={e => setForm(v => ({ ...v, [k]: e.target.value }))}/></label>)}<label>Status<select disabled={!can} value={form.status} onChange={e => setForm(v => ({ ...v, status: e.target.value }))}><option value="approved">approved</option><option value="revoked">revoked</option></select></label><p className="fl-note">秘密引用是服务端 KEEL_SSH_SECRET_DIR 内权限0600的文件名；Known Hosts 必须事先经过人工可信校验。这里不运行 ssh-keyscan 自动批准指纹。</p><Button disabled={!can || !form.asset_id || !form.host || !form.username || !form.secret_ref || !form.known_hosts_ref || save.isPending} onClick={() => save.mutate()}>保存并审计</Button></Panel></div>;
}
