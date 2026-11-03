import { apiRequest } from '../request';
import { publishPreview } from './provider';
import type { KickoffAPI, Workspace, ModelRow, Publication, Calculation, ReviewPage, SourceUnderstanding, SourceRegionPreview, PublicationOptions, AnalysisDomain, SourceAnalysis } from './types';

export class HttpKickoffProvider implements KickoffAPI {
 private state:Workspace;
 private listeners=new Set<()=>void>();
 private controller=new AbortController();
 constructor(private projectId:string,projectName:string){this.state={schemaVersion:1,projectId,projectName,revision:0,devices:[],ports:[],connections:[],components:[],racks:[],sources:[],issues:[],models:[],calculations:[],publications:[],activity:[]};}
 private path=(suffix='')=>`projects/${encodeURIComponent(this.projectId)}/kickoff${suffix}`;
 subscribe=(fn:()=>void)=>{this.listeners.add(fn);return()=>{this.listeners.delete(fn);};};
 snapshot=()=>this.state;


 private emit=()=>this.listeners.forEach(fn=>fn());
 refresh=async()=>{if(this.controller.signal.aborted)this.controller=new AbortController();const state=await apiRequest<Workspace>(this.path('/workspace'),'GET',undefined,undefined,this.controller.signal);if(state.projectId!==this.projectId)throw Error('工作区项目与请求不一致');this.state=state;this.emit();};
 loadSection=async(section:string)=>{const revision=this.state.revision;const items:unknown[]=[];let offset=0;for(;;){const page=await apiRequest<Workspace>(this.path(`/workspace?section=${encodeURIComponent(section)}&offset=${offset}&limit=200&revision=${revision}`),'GET',undefined,undefined,this.controller.signal);items.push(...page[section as keyof Workspace] as unknown[]);if(!page.page?.hasMore)break;offset+=page.page.limit;}if(revision!==this.state.revision)throw Error('工作区版本已变化，请重新加载');this.state={...this.state,[section]:items};this.emit();};
 loadContext=async(id:string)=>{const revision=this.state.revision;const data=await apiRequest<Pick<Workspace,'devices'|'ports'|'connections'|'components'|'revision'>>(this.path(`/workspace?section=context&entity_id=${encodeURIComponent(id)}&revision=${revision}`),'GET',undefined,undefined,this.controller.signal);if(revision!==this.state.revision||data.revision!==revision)return;for(const key of ['devices','ports','connections','components'] as const){const map=new Map(this.state[key].map(v=>[v.id,v]));for(const v of data[key])map.set(v.id,v as never);this.state={...this.state,[key]:[...map.values()]};}this.emit();};
 publicationOptions=(revision:number)=>apiRequest<PublicationOptions>(this.path('/publication-options?revision='+revision),'GET',undefined,undefined,this.controller.signal);
 evaluatePublication=(deviceIds:string[],expectedRevision:number,facilityIds:string[]=[])=>apiRequest<{revision:number;counts:Record<string,number>}>(this.path('/preview'),'POST',{deviceIds,facilityIds,expectedRevision},undefined,this.controller.signal);
 reviewPage=(query:URLSearchParams)=>{const params=new URLSearchParams(query);params.set('revision',String(this.state.revision));return apiRequest<ReviewPage>(this.path('/review?'+params),'GET',undefined,undefined,this.controller.signal);};
 publicationDetail=(id:string)=>apiRequest<Publication>(this.path('/publications/'+encodeURIComponent(id)),'GET',undefined,undefined,this.controller.signal);
 dispose=()=>{this.controller.abort();this.listeners.clear();};
 exportReview=async(query:URLSearchParams)=>{const revision=this.state.revision;const rows:ReviewPage['rows']=[];let offset=0;for(;;){const params=new URLSearchParams(query);params.set('offset',String(offset));params.set('limit','200');const page=await this.reviewPage(params);if(page.revision!==revision)throw Error('工作区版本已变化，请重新导出');rows.push(...page.rows);if(!page.hasMore)break;offset+=200;}return rows;};
 private command=async<T>(path:string,body:unknown,method='POST')=>{try{const result=await apiRequest<T>(this.path(path),method,body,undefined,this.controller.signal);await this.refresh();return result;}catch(error){if((error as {status?:number}).status===409)await this.refresh();throw error;}};
 addSources=async(files:File[])=>{try{for(const file of files){const body=new FormData();body.append('file',file);await apiRequest(this.path('/sources'),'POST',body,undefined,this.controller.signal);}}finally{await this.refresh();}};
 startSourceAnalysis=(id:string,domains:AnalysisDomain[])=>apiRequest<{jobId:string;state:string}>(this.path('/sources/'+encodeURIComponent(id)+'/analyze'),'POST',{expectedRevision:this.state.revision,domains},undefined,this.controller.signal);
 sourceAnalysis=(id:string,jobId?:string)=>apiRequest<SourceAnalysis>(this.path('/sources/'+encodeURIComponent(id)+'/analysis'+(jobId?'?jobId='+encodeURIComponent(jobId):'')),'GET',undefined,undefined,this.controller.signal);
 adoptSourceAnalysis=(id:string,jobId:string,tableIds:string[])=>this.command<{revision:number;adoptedTableIds:string[];skippedTableIds:string[]}>('/sources/'+encodeURIComponent(id)+'/analysis/adopt',{jobId,tableIds,expectedRevision:this.state.revision});
 sourceUnderstanding=(id:string)=>apiRequest<SourceUnderstanding>(this.path('/sources/'+encodeURIComponent(id)+'/understanding'),'GET',undefined,undefined,this.controller.signal);
 reuseSourceHistory=async(id:string)=>{const report=await apiRequest<SourceUnderstanding>(this.path('/sources/'+encodeURIComponent(id)+'/reuse-history'),'POST',{},undefined,this.controller.signal);await this.refresh();return report;};
 sourceRegionPreview=(sourceId:string,regionId:string,offset:number,revisionId:string,view:'source'|'normalized')=>apiRequest<SourceRegionPreview>(this.path('/sources/'+encodeURIComponent(sourceId)+'/regions/'+encodeURIComponent(regionId)+'/preview?'+new URLSearchParams({offset:String(offset),limit:'20',revisionId,view})),'GET',undefined,undefined,this.controller.signal);
 removeSource=async(id:string)=>{await this.command('/sources/'+encodeURIComponent(id),undefined,'DELETE');};
 analyzeSource=async(id:string)=>{await this.command('/sources/'+encodeURIComponent(id)+'/analyze',{expectedRevision:this.state.revision});};
 resolveIssue=async(id:string,value:string)=>{await this.command('/issues/'+encodeURIComponent(id)+'/resolve',{value,expectedRevision:this.state.revision});};
 saveModel=async(tableId:string,rows:ModelRow[],source:string)=>{await this.command('/models/'+encodeURIComponent(tableId),{rows,source,expectedRevision:this.state.revision},'PUT');};
 previewPublication=(ids:string[])=>publishPreview(this.state,ids);
 publish=async(deviceIds:string[],reason:string,expectedRevision:number,facilityIds:string[]=[])=>{
  let result=await this.command<Publication>('/publications',{deviceIds,facilityIds,reason,expectedRevision});
  for(let i=0;result.state==='pending'&&i<60;i++){
   const signal=this.controller.signal;
   await new Promise<void>((resolve,reject)=>{const abort=()=>{clearTimeout(timer);reject(new DOMException('Aborted','AbortError'));};const timer=setTimeout(()=>{signal.removeEventListener('abort',abort);resolve();},1000);if(signal.aborted)abort();else signal.addEventListener('abort',abort,{once:true});});
   const receipt=await apiRequest<{id:string;state:Publication['state'];errorCode:string|null}>(this.path('/publications/'+encodeURIComponent(result.id)+'/status'),'GET',undefined,undefined,signal);
   result={...result,state:receipt.state,errorCode:receipt.errorCode??undefined};
   if(result.state==='published')result=await apiRequest<Publication>(this.path('/publications/'+encodeURIComponent(result.id)),'GET',undefined,undefined,signal);
  }
  await this.refresh();
  if(result.state==='published')window.dispatchEvent(new CustomEvent('keel-inventory-published',{detail:{projectId:this.projectId}}));
  if(result.state!=='published')throw Error(result.errorCode||'CMDB 投影仍在后台处理，请查看入库版本状态；无需重复提交。');
  return result;
 };
 calculate=async(domain:'cabling'|'power')=>{const revision=this.state.revision;const result=await apiRequest<Calculation>(this.path('/calculations'),'POST',{domain,expectedRevision:revision},undefined,this.controller.signal);this.state={...this.state,revision:revision+1,calculations:[...this.state.calculations,result]};this.emit();return result;};
}
