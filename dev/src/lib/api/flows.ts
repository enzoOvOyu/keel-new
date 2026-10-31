import type { components } from '../schemas/openapi';
import { identitySnapshot } from './identityBridge';
type S = components['schemas'];
export type Flow = S['FlowsDefinition'];
export type FlowSave = S['FlowsSave'] & {draft?:boolean};
export type Execution = S['FlowsExecution'];
export type Target = S['FlowsTarget'];
export type Agent = S['FlowsAgent'];
export type Run = S['FlowsRun'] & {
    inputs?: Record<string, string>;
    target_revisions?: Record<string, number>;
};
export type Decision = S['FlowsDecision'];
export type Audit = S['FlowsAudit'];
export type Engine = S['FlowsEngine'];
export type SSHTrust = S['FlowsSSHTrust'];
async function request<T>(path: string, project: string | null, method = 'GET', body?: unknown, key?: string, signal?: AbortSignal): Promise<T> { const q = new URLSearchParams(project ? { project_id: project } : {}); const r = await fetch('/api/v1/flows' + path + (q.size ? (path.includes('?') ? '&' : '?') + q : ''), { method, signal, credentials: 'same-origin', headers: { 'Content-Type': 'application/json', ...(method !== 'GET' ? { 'X-CSRF-Token': identitySnapshot()?.csrf_token || '', 'Idempotency-Key': key || crypto.randomUUID() } : {}) }, body: body === undefined ? undefined : JSON.stringify(body) }); const v = await r.json(); if (!r.ok) {
    if (r.status === 401)
        window.dispatchEvent(new Event('keel-session-expired'));
    throw Object.assign(Error(v.error?.code || 'FLOWS_API_ERROR'), { status: r.status });
} return v.data; }
export const flowsApi = { template: (p: string) => request<{
        name: string;
        kind: string;
        source: string;
    }>('/templates', p), file: (p: string, id: string, path: string) => request<{
        path: string;
        content: string;
    }>('/' + id + '/namespace-file?path=' + encodeURIComponent(path), p), saveFile: (p: string, id: string, path: string, content: string) => request<{
        path: string;
        content: string;
    }>('/' + id + '/namespace-file', p, 'PUT', { path, content }), list: (p: string | null) => request<Flow[]>('', p), flow: (p: string | null, id: string) => request<Flow>('/' + id, p), save: (p: string, id: string | null, input: FlowSave) => request<Flow>(id ? '/' + id : '', p, id ? 'PUT' : 'POST', input), capabilities: (p: string | null) => request<S['FlowsCapabilities']>('/capabilities', p), targets: (p: string | null) => request<Target[]>('/targets', p), preflight: (r: Run) => request<Decision[]>('/preflight', r.project_id, 'POST', r), run: (r: Run, key: string) => request<Execution>(r.flow_id ? '/' + r.flow_id + '/execute' : '/quick-run', r.project_id, 'POST', r, key), executionPage:(p:string|null,cursor:string,filters:Record<string,string>={})=>request<S['FlowsExecutionPage']>('/executions?'+new URLSearchParams({...filters,...(cursor?{cursor}:{})}),p),executions: (p: string | null) => request<S['FlowsExecutionPage']>('/executions', p).then(page=>page.items), execution: (p: string | null, id: string) => request<Execution>('/executions/' + id, p), action: (p: string | null, id: string, action: string, key: string) => request<Execution>('/executions/' + id + '/' + action, p, 'POST', {}, key), logs: (p: string | null, id: string) => request<Engine[]>('/executions/' + id + '/logs', p), audit: (p: string | null, id: string) => request<Audit[]>('/executions/' + id + '/audit', p), dependencies:(p:string|null,id:string)=>request<S['FlowsDependencyGraph']>('/'+id+'/dependencies',p),context: (p: string | null, id: string, tab: string) => request<Engine[]>('/' + id + '/' + tab, p), validate: (p: string | null, id: string, source: string) => request<Engine[]>('/' + id + '/validate', p, 'POST', { source }), bootstrap:(project:string,asset_id:string)=>request<{asset_id:string;state:string;version:string}>('/agents/bootstrap',project,'POST',{project_id:project,asset_id,version:'0.1.0'}), agents: (p: string | null) => request<Agent[]>('/agents', p), token: (project: string, asset: string) => request<S['FlowsToken']>('/agents/enrollment-tokens', project, 'POST', { project_id: project, asset_id: asset }), agentAction: (p: string | null, id: string, action: string, reason: string) => request<S['FlowsAction']>('/agents/' + id + '/' + action, p, 'POST', { reason }), agentHistory: (p: string | null, id: string, kind: string) => request<Audit[]>('/agents/' + id + '/' + kind, p), ssh: (p: string | null) => request<SSHTrust[]>('/ssh-trust', p), saveSSH: (p: string, input: S['FlowsSSHTrustInput']) => request<S['FlowsSSHTrustResult']>('/ssh-trust', p, 'POST', input) };
export function followExecution(p: string | null, id: string, onUpdate: () => void) { const q = new URLSearchParams(p ? { project_id: p } : {}); const source = new EventSource('/api/v1/flows/executions/' + id + '/events?' + q, { withCredentials: true }); source.onmessage = onUpdate; for(const event of ["progress","end","execution","updated"])source.addEventListener(event,onUpdate); source.onerror = () => onUpdate(); return () => source.close(); }

export function flowsKey(...parts:unknown[]){const identity=identitySnapshot();return ["flows",identity?.user.id||"",identity?.permission_revision||"",...parts]}

export function followLogs(project:string|null,id:string,onLog:(entry:Engine)=>void,onReconnect:()=>void){const query=new URLSearchParams({follow:'true',...(project?{project_id:project}:{})});const stream=new EventSource('/api/v1/flows/executions/'+id+'/logs?'+query,{withCredentials:true});stream.onmessage=event=>{try{const log=JSON.parse(event.data);if(typeof log.message==='string')onLog(log)}catch{onReconnect()}};stream.onerror=onReconnect;return()=>stream.close()}
export function mergeLogs(a:Engine[],b:Engine[]){return [...new Map([...a,...b].map(log=>[JSON.stringify(log),log])).values()].sort((x,y)=>String(x.timestamp||'').localeCompare(String(y.timestamp||''))).slice(-5000)}
