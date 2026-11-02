import type { components } from '../schemas/openapi';
import { identitySnapshot } from './identityBridge';
type S = components['schemas'];
export type Flow = S['FlowsDefinition'];
export type FlowSave = S['FlowsSave'] & {draft?:boolean};
export type Execution = S['FlowsExecution'];
export type Target = S['FlowsTarget'];
export type Agent = S['FlowsAgent'];
export type Run = S['FlowsRun'] & {
 steps?:JobStep[];approval_id?:string;approval_action?:string;
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
async function collection<T extends {id:string;project_id:string}>(path:string,project:string|null):Promise<T[]> {
 const items:T[]=[],seen=new Set<string>();
 for(let offset=0;;offset+=200){
  const page=await request<T[]>(path+(path.includes('?')?'&':'?')+new URLSearchParams({offset:String(offset),limit:'200'}),project);
  if(page.length>200||page.some(item=>seen.has(item.project_id+':'+item.id)))throw Error('列表数据发生变化或 API 分页版本尚未更新，请刷新后重试。');
  for(const item of page)seen.add(item.project_id+':'+item.id);
  items.push(...page);if(page.length<200)return items;
 }
}
export const flowsApi = {
 artifactURL:(project:string|null,execution:string,uri:string)=>'/api/v1/flows/executions/'+encodeURIComponent(execution)+'/artifacts?'+new URLSearchParams({path:uri,...(project?{project_id:project}:{})}),
 artifactPreview:async(project:string|null,execution:string,uri:string)=>{const response=await fetch(flowsApi.artifactURL(project,execution,uri),{credentials:'same-origin'});if(!response.ok)throw Error('文件预览失败（'+response.status+'）');const blob=await response.blob();if(blob.size>1048576)throw Error('文件超过 1 MiB，请下载查看');return new TextDecoder('utf-8',{fatal:true}).decode(await blob.arrayBuffer())},
 approvals:async(project:string):Promise<S['IdentityApproval'][]>=>{const response=await fetch('/api/v1/projects/'+encodeURIComponent(project)+'/approvals',{credentials:'same-origin'});const result=await response.json();if(!response.ok){if(response.status===401)window.dispatchEvent(new Event('keel-session-expired'));throw Error(result.error?.code||'APPROVALS_UNAVAILABLE')}return result.data},
 scheduleStatus:(p:string,id:string)=>request<S['FlowsScheduleStatus']>('/'+id+'/schedule/status',p),
 replayRequest:(p:string|null,id:string,mode:'all'|'failed',nonce:string)=>request<Run>('/executions/'+id+'/replay-request?'+new URLSearchParams({mode,nonce}),p),
 replay:(p:string,id:string,mode:'all'|'failed',run:Run,key:string)=>request<Execution>('/executions/'+id+'/replay',p,'POST',{mode,run},key),
 backfillPreview:(p:string,id:string,value:S['FlowsBackfillRequest'])=>request<S['FlowsBackfillPreview']>('/'+id+'/backfill/preview',p,'POST',value),
 createBackfill:(p:string,id:string,value:S['FlowsBackfillRequest'],key:string)=>request<S['FlowsOperationJob']>('/'+id+'/backfill',p,'POST',value,key),
 backfills:(p:string,id:string)=>request<S['FlowsOperationJob'][]>('/'+id+'/backfill',p),
 operationItemRequest:(p:string,id:string,ordinal:number)=>request<Run>('/operation-jobs/'+id+'/items/'+ordinal+'/request',p),
 bootstrapJobs:(p:string|null)=>request<S['FlowsOperationJob'][]>('/agents/bootstrap-jobs',p),
 bootstrapBatch:(p:string,asset_ids:string[],version:string,key:string)=>request<S['FlowsOperationJob']>('/agents/bootstrap-jobs',p,'POST',{project_id:p,asset_ids,version},key),
 operationAction:(p:string,id:string,action:S['FlowsOperationAction']['action'],revision:number,extra:{ordinal?:number;approval_id?:string}={})=>request<S['FlowsOperationJob']>('/operation-jobs/'+id+'/actions',p,'POST',{action,expected_revision:revision,...extra}),
 variables:(p:string|null,id:string)=>request<S['FlowsExecutionVariables']>('/executions/'+id+'/variables',p),
 catalog:(p:string|null,filters:Record<string,string>,signal?:AbortSignal)=>request<S['FlowsCatalogPage']>('/catalog?'+new URLSearchParams(filters),p,'GET',undefined,undefined,signal),
 executionSummary:(p:string|null,filters:Record<string,string>={})=>request<S['FlowsExecutionSummary']>('/execution-summary?'+new URLSearchParams(filters),p),
 logSearch:(p:string|null,filters:Record<string,string>,signal?:AbortSignal)=>request<S['FlowsLogSearchPage']>('/log-search?'+new URLSearchParams(filters),p,'GET',undefined,undefined,signal),
 insights:(p:string|null,id:string,filters:Record<string,string>={})=>request<S['FlowsExecutionInsights']>('/executions/'+id+'/insights?'+new URLSearchParams(filters),p),
 taskDocumentation:(p:string|null,type:string)=>request<S['FlowsTaskDocumentation']>('/task-documentation?'+new URLSearchParams({type}),p),
 executionDefinition:(p:string|null,id:string)=>request<S['FlowsExecutionDefinition']>('/executions/'+id+'/definition',p),
 schedule:(p:string,id:string)=>request<BatchSchedule>('/'+id+'/schedule',p),saveSchedule:(p:string,id:string,value:BatchSchedule)=>request<BatchSchedule>('/'+id+'/schedule',p,'PUT',value),
 fileVersions:(p:string,id:string)=>request<FileRevision[]>('/'+id+'/namespace-artifacts',p),uploadFile:(p:string,id:string,value:{path:string;data_base64:string;expected_revision:number})=>request<FileRevision>('/'+id+'/namespace-artifacts',p,'POST',value),
 distributeFile:(p:string,id:string,revision:string,value:Run)=>request<Execution>('/'+id+'/namespace-artifacts/'+revision+'/distribute',p,'POST',value),
 fileText:async(p:string,id:string,revision:string)=>{const r=await fetch(flowsApi.fileDownloadURL(p,id,revision),{credentials:'same-origin'});if(!r.ok){if(r.status===401)window.dispatchEvent(new Event('keel-session-expired'));throw Error('文件读取失败（'+r.status+'）')}const data=await r.arrayBuffer();try{const text=new TextDecoder('utf-8',{fatal:true}).decode(data);if(text.includes('\0'))throw Error('BINARY_FILE');return text}catch{throw Error('该文件不是 UTF-8 文本，请通过文件分发面板下载。')}},
 fileDownloadURL:(p:string,id:string,revision:string)=>'/api/v1/flows/'+id+'/namespace-artifacts/'+revision+'?'+new URLSearchParams({project_id:p}),
 approvalRequest:(run:Run,reason:string)=>request<S['FlowsApprovalResult']>('/approval-request',run.project_id,'POST',{run,reason}),
 steps:(p:string|null,id:string)=>request<{asset_id:string;step_id:string;state:string;exit_code:number|null;stdout:string;stderr:string}[]>('/executions/'+id+'/steps',p),
 template: (p: string) => request<{
        name: string;
        kind: string;
        source: string;
    }>('/templates', p), file: (p: string, id: string, path: string) => request<{
        path: string;
        content: string;
    }>('/' + id + '/namespace-file?path=' + encodeURIComponent(path), p), saveFile: (p: string, id: string, path: string, content: string) => request<{
        path: string;
        content: string;
    }>('/' + id + '/namespace-file', p, 'PUT', { path, content }), list: (p: string | null) => collection<Flow>('', p), flow: (p: string | null, id: string) => request<Flow>('/' + id, p), save: (p: string, id: string | null, input: FlowSave) => request<Flow>(id ? '/' + id : '', p, id ? 'PUT' : 'POST', input), capabilities: (p: string | null) => request<S['FlowsCapabilities']>('/capabilities', p), targets: (p: string | null) => collection<Target>('/targets', p), preflight: (r: Run) => request<Decision[]>('/preflight', r.project_id, 'POST', r), run: (r: Run, key: string) => request<Execution>(r.flow_id ? '/' + r.flow_id + '/execute' : '/quick-run', r.project_id, 'POST', r, key), executionPage:(p:string|null,cursor:string,filters:Record<string,string>={})=>request<S['FlowsExecutionPage']>('/executions?'+new URLSearchParams({...filters,...(cursor?{cursor}:{})}),p),executions: (p: string | null) => request<S['FlowsExecutionPage']>('/executions', p).then(page=>page.items), execution: (p: string | null, id: string) => request<Execution>('/executions/' + id, p), action: (p: string | null, id: string, action: string, key: string) => request<Execution>('/executions/' + id + '/' + action, p, 'POST', {}, key), logs: (p: string | null, id: string) => request<Engine[]>('/executions/' + id + '/logs', p), audit: (p: string | null, id: string) => request<Audit[]>('/executions/' + id + '/audit', p), dependencies:(p:string|null,id:string)=>request<S['FlowsDependencyGraph']>('/'+id+'/dependencies',p),context: (p: string | null, id: string, tab: string) => request<Engine[]>('/' + id + '/' + tab, p), validate: (p: string | null, id: string, source: string) => request<Engine[]>('/' + id + '/validate', p, 'POST', { source }), bootstrap:(project:string,asset_id:string,version="0.2.0")=>request<{asset_id:string;state:string;version:string}>('/agents/bootstrap',project,'POST',{project_id:project,asset_id,version}), agents: (p: string | null) => collection<Agent>('/agents', p), token: (project: string, asset: string) => request<S['FlowsToken']>('/agents/enrollment-tokens', project, 'POST', { project_id: project, asset_id: asset }), agentAction: (p: string | null, id: string, action: string, reason: string) => request<S['FlowsAction']>('/agents/' + id + '/' + action, p, 'POST', { reason }), agentHistory: (p: string | null, id: string, kind: string) => request<Audit[]>('/agents/' + id + '/' + kind, p), ssh: (p: string | null) => request<SSHTrust[]>('/ssh-trust', p), saveSSH: (p: string, input: S['FlowsSSHTrustInput']) => request<S['FlowsSSHTrustResult']>('/ssh-trust', p, 'POST', input) };
export function followExecution(p: string | null, id: string, onUpdate: () => void) { const q = new URLSearchParams(p ? { project_id: p } : {}); const source = new EventSource('/api/v1/flows/executions/' + id + '/events?' + q, { withCredentials: true }); source.onmessage = onUpdate; for(const event of ["progress","end","execution","updated","target"])source.addEventListener(event,onUpdate); source.onerror = () => onUpdate(); return () => source.close(); }

export function flowsKey(...parts:unknown[]){const identity=identitySnapshot();return ["flows",identity?.user.id||"",identity?.permission_revision||"",...parts]}

export function followLogs(project:string|null,id:string,onLog:(entry:Engine)=>void,onReconnect:()=>void){const query=new URLSearchParams(project?{project_id:project}:{});const stream=new EventSource('/api/v1/flows/executions/'+id+'/events?'+query,{withCredentials:true});stream.addEventListener('log',event=>{try{onLog(JSON.parse((event as MessageEvent).data))}catch{onReconnect()}});stream.onerror=onReconnect;return()=>stream.close()}
export function mergeLogs(a:Engine[],b:Engine[]){return [...new Map([...a,...b].map(log=>[JSON.stringify(log),log])).values()].sort((x,y)=>String(x.timestamp||'').localeCompare(String(y.timestamp||''))).slice(-5000)}

export type JobStep=S['FlowsJobStep'];
export type BatchSchedule=S['FlowsSchedule'];
export type FileRevision=S['FlowsFileRevision'];
