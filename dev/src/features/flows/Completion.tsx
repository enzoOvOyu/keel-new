import { lazy, Suspense, useEffect, useState, useRef } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import YAML from 'yaml';
import { runDefaults } from './definition';
import { flowsApi, flowsKey, type Run } from '@/lib/api/flows';
import { Button, Notice, Table, State, Panel } from './ui';
import { useProject } from '@/app/ProjectContext';
const ScheduleView=lazy(()=>import('../jobs-native/NativeScheduleHost').then(m=>({default:m.NativeScheduleHost})));
export function TargetSchedule({project,id,source,canEdit}:{project:string;id:string;source:string;canEdit:boolean}){
 const cache=useQueryClient(),[cron,setCron]=useState(''),[zone,setZone]=useState('UTC'),[enabled,setEnabled]=useState(false),[selected,setSelected]=useState<string[]>([]),[command,setCommand]=useState(''),[policy,setPolicy]=useState('agent_preferred');
 const nav=useNavigate(),runtime=useQuery({queryKey:flowsKey('schedule-status',project,id),queryFn:()=>flowsApi.scheduleStatus(project,id),staleTime:60000});
 const query=useQuery({queryKey:flowsKey('schedule',project,id),queryFn:()=>flowsApi.schedule(project,id)}),targets=useQuery({queryKey:flowsKey('targets',project),queryFn:()=>flowsApi.targets(project)});
 let defaults:any={};try{defaults=YAML.parse(source)||{}}catch{}
 const defaultsRun=runDefaults(defaults);
 const [useSteps,setUseSteps]=useState(true),[planSteps,setPlanSteps]=useState(defaultsRun.steps);
 const [kind,setKind]=useState(defaultsRun.kind),[timeout,setTimeoutValue]=useState(defaultsRun.timeout),[concurrency,setConcurrency]=useState(defaultsRun.concurrency);
 const hydrated=useRef('');
 useEffect(()=>{if(query.data){
  const key=project+':'+id+':'+query.data.revision;if(hydrated.current===key)return;hydrated.current=key;
  const r=query.data.run,existing=!!query.data.revision;
  setCron(query.data.cron||'');setZone(query.data.timezone||'UTC');setEnabled(query.data.enabled);setSelected(r.target_ids||[]);
  setKind(existing?r.kind:defaultsRun.kind);setTimeoutValue(existing?r.timeout_seconds:defaultsRun.timeout);setConcurrency(existing?r.target_concurrency:defaultsRun.concurrency);
  setCommand(existing?r.command:defaultsRun.command);setPolicy(existing?r.transport_policy:defaultsRun.policy);
  setPlanSteps(existing?r.steps||[]:defaultsRun.steps);setUseSteps(existing?!!r.steps?.length:!!defaultsRun.steps.length);
 }},[query.data,project,id]);
 const save=useMutation({mutationFn:async()=>{const flow=await flowsApi.flow(project,id);return flowsApi.saveSchedule(project,id,{cron,timezone:zone,enabled,revision:query.data?.revision||0,last_error:null,run:{project_id:project,flow_id:id,expected_revision:flow.revision,target_ids:selected,kind,command,transport_policy:policy as Run['transport_policy'],timeout_seconds:timeout,target_concurrency:concurrency,max_retries:0,steps:useSteps?planSteps:[],source_module:'schedule'}})},onSuccess:()=>{void cache.invalidateQueries({queryKey:flowsKey('schedule-status',project,id)});return cache.invalidateQueries({queryKey:flowsKey('schedule',project,id)})}});
 const patch=(key:string,value:any)=>{const setters:Record<string,(v:any)=>void>={cron:setCron,zone:setZone,enabled:setEnabled,policy:setPolicy,kind:setKind,timeout:setTimeoutValue,concurrency:setConcurrency,useSteps:setUseSteps,command:setCommand};setters[key]?.(value)};
 return <Suspense fallback={null}><ScheduleView form={{cron,zone,enabled,policy,kind,timeout,concurrency,useSteps,command}} revision={query.data?.revision||0} selected={selected} targets={targets.data||[]} steps={planSteps} runtime={runtime.data} canEdit={canEdit} busy={save.isPending} error={String((query.error||targets.error||runtime.error||save.error)?.message||query.data?.last_error||'')} onPatch={patch} onSelect={setSelected} onSave={()=>save.mutate()} onRefresh={()=>void runtime.refetch()} onReloadSteps={()=>setPlanSteps(defaultsRun.steps)} onReloadScript={()=>setCommand(defaultsRun.command)} onNavigate={nav}/></Suspense>;
}
export function BinaryFiles({project,id,canEdit}:{project:string;id:string;canEdit:boolean}){
 const nav=useNavigate(),cache=useQueryClient(),ctx=useProject(),[selected,setSelected]=useState(''),[targetIDs,setTargets]=useState<string[]>([]),[path,setPath]=useState('');
 const files=useQuery({queryKey:flowsKey('binary-files',project,id),queryFn:()=>flowsApi.fileVersions(project,id)}),targets=useQuery({queryKey:flowsKey('targets',project),queryFn:()=>flowsApi.targets(project),enabled:!!selected});
 const upload=useMutation({mutationFn:async(file:File)=>{if(file.size>1048576)throw Error('资源文件上限1MiB');const bytes=new Uint8Array(await file.arrayBuffer());let raw='';for(const x of bytes)raw+=String.fromCharCode(x);const name=path||file.name;const current=(files.data||[]).filter(f=>f.path==='/'+name.replace(/^\//,'')).reduce((n,f)=>Math.max(n,f.revision),0);return flowsApi.uploadFile(project,id,{path:name,data_base64:btoa(raw),expected_revision:current})},onSuccess:()=>{void cache.invalidateQueries({queryKey:flowsKey('file-versions',project,id)});return cache.invalidateQueries({queryKey:flowsKey('binary-files',project,id)})}});
 const distribute=useMutation({mutationFn:()=>flowsApi.distributeFile(project,id,selected,{project_id:project,target_ids:targetIDs,kind:'file.upload',command:'',transport_policy:'agent_only',timeout_seconds:600,target_concurrency:8,max_retries:0}),onSuccess:ex=>nav('/flows/executions/'+ex.id)});
 return <section><div className="fl-toolbar"><h3>Binary Resources & Revisions</h3><span className="spacer"/><input placeholder="目标Namespace路径（默认文件名）" aria-label="Binary resource path" value={path} onChange={e=>setPath(e.target.value)}/><label>上传文件 ≤1MiB<input aria-label="Upload binary resource" type="file" disabled={!canEdit||upload.isPending} onChange={e=>{const file=e.target.files?.[0];if(file)upload.mutate(file);e.target.value=''}}/></label></div><Notice error={files.error||upload.error||distribute.error}/><Table data={files.data||[]} columns={[{accessorKey:'path',header:'Resource'},{accessorKey:'revision',header:'Revision'},{accessorKey:'size',header:'Bytes'},{accessorKey:'sha256',header:'SHA256'},{id:'actions',header:'Actions',cell:({row})=><div className="fl-row-actions"><a href={flowsApi.fileDownloadURL(project,id,row.original.id)} download>Download</a><Button variant="outline" size="sm" disabled={!ctx.permissions.includes('flows:execute')} onClick={()=>{setSelected(row.original.id);setTargets([])}}>Distribute</Button></div>}]}/><Panel open={!!selected} onClose={()=>setSelected('')} title="分发资源版本" description="冻结所选文件版本与SHA256，使用Agent写入受控uploads目录，不覆盖系统路径。"><Notice error={targets.error||distribute.error}/><div className="fl-target-list">{(targets.data||[]).filter(t=>t.agent_online).map(t=><label key={t.id}><input type="checkbox" checked={targetIDs.includes(t.id)} onChange={e=>setTargets(e.target.checked?[...targetIDs,t.id]:targetIDs.filter(x=>x!==t.id))}/>{t.name}</label>)}</div><Button disabled={!targetIDs.length||distribute.isPending} onClick={()=>distribute.mutate()}>确认分发至 {targetIDs.length} 台设备</Button></Panel></section>
}
export function DeviceSteps({project,id}:{project:string|null;id:string}){
 const query=useQuery({queryKey:flowsKey('device-steps',project,id),queryFn:()=>flowsApi.steps(project,id),refetchInterval:5000});
 return <section><h3>Device Steps</h3><Notice error={query.error}/><Table data={(query.data||[]).map(v=>({...v,id:v.asset_id+':'+v.step_id}))} columns={[{accessorKey:'asset_id',header:'Asset'},{accessorKey:'step_id',header:'Step'},{accessorKey:'state',header:'State',cell:({row})=><State value={row.original.state}/>},{accessorKey:'exit_code',header:'Exit'},{accessorKey:'stdout',header:'Output'},{accessorKey:'stderr',header:'Error'}]}/></section>
}
