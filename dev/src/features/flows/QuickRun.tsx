import { lazy,Suspense,useState,useEffect,useMemo } from 'react';
import YAML from 'yaml';
import { runDefaults } from './definition';
import { useQuery, useMutation } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { flowsKey, flowsApi, type Flow, type Run, type Decision, type JobStep } from '@/lib/api/flows';
import { Button, Panel, Notice, State, Search, Table } from './ui';
const NativeQuickRunHost=lazy(()=>import('../jobs-native/NativeQuickRunHost').then(m=>({default:m.NativeQuickRunHost})));
export function QuickRun({ project, flow, onClose, allowed, canSave = false,seedRun,submitRun,nonce }: {
    project: string;
    flow: Flow | null;
    onClose: () => void;
    allowed: boolean;
    canSave?: boolean;seedRun?:Run;submitRun?:(run:Run,key:string)=>Promise<import('@/lib/api/flows').Execution>;nonce?:string;
}) {
    const nav = useNavigate();
    const [step, setStep] = useState(0), [query, setQuery] = useState(''), [type, setType] = useState('all'), [selected, select] = useState<string[]>(seedRun?.target_ids||[]), [decisions, setDecisions] = useState<Decision[] | null>(null), [key] = useState(() => nonce||crypto.randomUUID());
    const [request, setRequest] = useState<Run>(seedRun||{ project_id: project, flow_id: flow?.id || '', expected_revision: flow?.revision || 0, target_ids: [], kind: flow ? flowDefaults(flow).kind : 'bash', command: flow ? flowDefaults(flow).command : '', transport_policy: flow ? flowDefaults(flow).policy : 'agent_preferred', timeout_seconds: flow ? flowDefaults(flow).timeout : 600, target_concurrency: flow ? flowDefaults(flow).concurrency : 8, max_retries: 0, steps: flow ? flowDefaults(flow).steps : [], source_module: flow ? 'flows' : 'quick-run' });
    const targets = useQuery({ queryKey: flowsKey('targets', project), queryFn: () => flowsApi.targets(project) });
    const preflightSignature=JSON.stringify({...request,approval_id:undefined,approval_action:undefined});
    useEffect(() => { setDecisions(null); }, [selected, preflightSignature]);
    const records = useMemo(() => (targets.data || []).filter(t => (!seedRun||seedRun.target_ids.includes(t.id))&& (type === 'all' || t.type === type) && (t.name + ' ' + t.id).toLowerCase().includes(query.toLowerCase())), [targets.data, type, query]);
    const preflight = useMutation({ mutationFn: () => flowsApi.preflight({ ...request, target_ids: selected }), onSuccess: setDecisions });
    const execute = useMutation({ mutationFn: () => (submitRun||flowsApi.run)({ ...request, target_ids: selected, target_revisions: Object.fromEntries((decisions || []).map(d => [d.asset_id, d.revision])) }, key), onSuccess: ex => nav('/flows/executions/' + ex.id) });
    const [flowName, setFlowName] = useState('');
    const saveFlow = useMutation({ mutationFn: async () => { const template = await flowsApi.template(project); const doc = YAML.parse(template.source); doc.id = 'quickrun-' + crypto.randomUUID().slice(0, 8); doc.inputs.push({ id: 'command', type: 'STRING', defaults: request.command }, { id: 'kind', type: 'STRING', defaults: request.kind }, { id: 'transport_policy', type: 'STRING', defaults: request.transport_policy }, {id:'steps',type:'JSON',defaults:request.steps||[]}); const loop = findTask(doc.tasks, 'targets'); loop.concurrencyLimit = request.target_concurrency; for (const id of ['agent_task', 'ssh_task']) { const task = findTask(loop.tasks, id); if (task) task.timeout = 'PT' + (request.timeout_seconds + 10) + 'S'; } return flowsApi.save(project, null, { project_id: project, name: flowName, description: 'Saved from Quick Run', target_type: type === 'all' ? 'both' : type, source: YAML.stringify(doc), expected_revision: 0, draft: true }); }, onSuccess: f => nav('/flows/definitions/' + f.id + '?tab=Builder') });
    const [approvalReason,setApprovalReason]=useState('');
    const approval=useMutation({mutationFn:()=>flowsApi.approvalRequest({...request,target_ids:selected,target_revisions:Object.fromEntries((decisions||[]).map(d=>[d.asset_id,d.revision]))},approvalReason),onSuccess:value=>setRequest(v=>({...v,approval_id:value.id}))});
    const patchStep=(index:number,patch:Partial<JobStep>)=>update({steps:(request.steps||[]).map((value,i)=>i===index?{...value,...patch}:value)});
    const steps = ['Targets', 'Command / Script', 'Transport', 'Preflight', 'Confirm'];
    const update = (p: Partial<Run>) => setRequest(v => ({ ...v, ...p,approval_id:undefined }));
    return <Suspense fallback={null}><NativeQuickRunHost title={flow?'Run · '+flow.name:'Quick Run'} project={project} request={request} targets={(targets.data||[]).filter(t=>!seedRun||seedRun.target_ids.includes(t.id))} selected={selected} decisions={decisions} allowed={allowed} canSave={canSave} hasFlow={!!flow} revision={flow?.revision||0} loading={targets.isLoading} busy={execute.isPending} preflightBusy={preflight.isPending} saveBusy={saveFlow.isPending} approvalBusy={approval.isPending} approvalReason={approvalReason} flowName={flowName} error={String((targets.error||preflight.error||execute.error||saveFlow.error||approval.error)?.message||'')} onClose={onClose} onPatch={update} onSelect={ids=>{select(ids);setRequest(v=>({...v,approval_id:undefined}))}} onPreflight={()=>preflight.mutate()} onExecute={()=>execute.mutate()} onReason={setApprovalReason} onApproval={()=>approval.mutate()} onApprovalId={id=>setRequest(v=>({...v,approval_id:id}))} onFlowName={setFlowName} onSaveFlow={()=>saveFlow.mutate()}/></Suspense>;
}
function flowDefaults(flow: Flow) {
    let d: any = {};
    try {
        d = YAML.parse(flow.source) || {};
    }
    catch { }
    ;
    return runDefaults(d);
}

function findTask(tasks: any[] | undefined, id: string): any { for (const task of tasks || []) { if (task.id === id) return task; const found = findTask([...(task.tasks || []), ...(task.then || []), ...(task.else || [])], id); if (found) return found; } return null; }
