import { bomRequest } from '../realBom';
import { publishPreview } from './provider';
import type { KickoffAPI, Workspace, ModelRow, Publication, Calculation } from './types';

export class HttpKickoffProvider implements KickoffAPI {
 private state:Workspace;
 private listeners=new Set<()=>void>();
 private controller=new AbortController();
 constructor(private projectId:string,projectName:string){this.state={schemaVersion:1,projectId,projectName,revision:0,devices:[],ports:[],connections:[],components:[],racks:[],sources:[],issues:[],models:[],calculations:[],publications:[],activity:[]};}
 private path=(suffix='')=>`projects/${encodeURIComponent(this.projectId)}/kickoff${suffix}`;
 subscribe=(fn:()=>void)=>{this.listeners.add(fn);return()=>{this.listeners.delete(fn);};};
 snapshot=()=>this.state;
 refresh=async()=>{if(this.controller.signal.aborted)this.controller=new AbortController();const state=await bomRequest<Workspace>(this.path('/workspace'),'GET',undefined,undefined,this.controller.signal);if(state.projectId!==this.projectId)throw Error('工作区项目与请求不一致');const prior=new Set(this.state.publications.filter(p=>p.state==='published').map(p=>p.id));this.state=state;this.listeners.forEach(fn=>fn());if(state.publications.some(p=>p.state==='published'&&!prior.has(p.id)))window.dispatchEvent(new Event('keel-inventory-published'));};
 dispose=()=>{this.controller.abort();this.listeners.clear();};
 private command=async<T>(path:string,body:unknown,method='POST')=>{try{const result=await bomRequest<T>(this.path(path),method,body,undefined,this.controller.signal);await this.refresh();return result;}catch(error){if((error as {status?:number}).status===409)await this.refresh();throw error;}};
 addSources=async(files:File[])=>{for(const file of files){const body=new FormData();body.append('file',file);await this.command('/sources',body);}};
 removeSource=async(id:string)=>{await this.command('/sources/'+encodeURIComponent(id),undefined,'DELETE');};
 analyzeSource=async(id:string)=>{await this.command('/sources/'+encodeURIComponent(id)+'/analyze',{expectedRevision:this.state.revision});};
 resolveIssue=async(id:string,value:string)=>{await this.command('/issues/'+encodeURIComponent(id)+'/resolve',{value,expectedRevision:this.state.revision});};
 saveModel=async(tableId:string,rows:ModelRow[],source:string)=>{await this.command('/models/'+encodeURIComponent(tableId),{rows,source,expectedRevision:this.state.revision},'PUT');};
 previewPublication=(ids:string[])=>publishPreview(this.state,ids);
 publish=async(deviceIds:string[],reason:string,expectedRevision:number)=>{
  let result=await this.command<Publication>('/publications',{deviceIds,reason,expectedRevision});
  for(let i=0;result.state==='pending'&&i<60;i++){
   const signal=this.controller.signal;
   await new Promise<void>((resolve,reject)=>{const abort=()=>{clearTimeout(timer);reject(new DOMException('Aborted','AbortError'));};const timer=setTimeout(()=>{signal.removeEventListener('abort',abort);resolve();},1000);if(signal.aborted)abort();else signal.addEventListener('abort',abort,{once:true});});
   const receipt=await bomRequest<{id:string;state:Publication['state'];errorCode:string|null}>(this.path('/publications/'+encodeURIComponent(result.id)+'/status'),'GET',undefined,undefined,signal);
   result={...result,state:receipt.state,errorCode:receipt.errorCode??undefined};
   if(result.state==='published')result=await bomRequest<Publication>(this.path('/publications/'+encodeURIComponent(result.id)),'GET',undefined,undefined,signal);
  }
  await this.refresh();
  if(result.state!=='published')throw Error(result.errorCode||'CMDB 投影仍在后台处理，请查看入库版本状态；无需重复提交。');
  return result;
 };
 calculate=(domain:'cabling'|'power')=>this.command<Calculation>('/calculations',{domain,expectedRevision:this.state.revision});
}
