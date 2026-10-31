import { useState } from 'react';
import YAML from 'yaml';
import { useMutation } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { flowsApi, type Flow } from '@/lib/api/flows';
import { Panel, Button, Notice } from './ui';
export function isTargetFlow(flow: Flow) {
    try {
        return (YAML.parse(flow.source)?.inputs || []).some((i: any) => i.id === 'ticket');
    }
    catch {
        return false;
    }
}
export function NativeRun({ flow, onClose }: {
    flow: Flow;
    onClose: () => void;
}) {
    const nav = useNavigate();
    const [values, setValues] = useState<Record<string, string>>({}), [key] = useState(() => crypto.randomUUID());
    let inputs: any[] = [];
    try {
        inputs = YAML.parse(flow.source)?.inputs || [];
    }
    catch { }
    ;
    const run = useMutation({ mutationFn: () => flowsApi.run({ project_id: flow.project_id, flow_id: flow.id, expected_revision: flow.revision, target_ids: [], kind: 'command', command: '', transport_policy: 'agent_preferred', timeout_seconds: 600, target_concurrency: 1, max_retries: 0, inputs: values }, key), onSuccess: ex => nav('/flows/executions/' + ex.id) });
    return <Panel open title={'Run · ' + flow.name} description="执行当前 Kestra Revision。该定义不包含 Keel 设备目标任务，不会伪造设备执行记录。" onClose={onClose}>{inputs.map(i => <label key={i.id}>{i.id} · {i.type}<input type={i.type === 'SECRET' ? 'password' : 'text'} value={values[i.id] ?? ''} placeholder={i.defaults === undefined ? '' : String(i.defaults)} autoComplete="off" onChange={e => setValues(v => ({ ...v, [i.id]: e.target.value }))}/></label>)}{!inputs.length && <p>该 Flow 不需要输入参数。</p>}<Notice error={run.error}/><Button disabled={run.isPending} onClick={() => run.mutate()}>Create Execution</Button></Panel>;
}
