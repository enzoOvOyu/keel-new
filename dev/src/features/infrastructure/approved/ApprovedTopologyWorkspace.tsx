import {useCallback,useEffect,useRef,useState,useMemo,lazy,Suspense} from 'react';
import {useNavigate,useParams,useSearchParams,useLocation} from 'react-router-dom';
import {useQuery,useQueryClient} from '@tanstack/react-query';
import {useIdentity} from '@/features/auth/IdentityProvider';
import {Bookmark,RotateCcw,ArrowLeft} from 'lucide-react';
import {useProject} from '@/app/ProjectContext';
import {infrastructureApi,type InfraAsset,type InfraSnapshot} from '@/lib/api/infrastructure';
import {realCmdbApi} from '@/lib/api/cmdb-http';
import {DrawerDetail,openDevice} from '@/components/DeviceDrawers';
import {TopologyModelProvider,type Go} from './model';
import {Button,Empty} from './shared';
import Global from './pages/global-topology';
import Network from './pages/network-architecture';
import Cabling from './pages/physical-cabling';
import Trace from './pages/link-tracing';
import './approved.css';
const Rack=lazy(()=>import('./pages/rack-detail'));
const pageViews:Record<string,string>={'global-topology':'global','network-architecture':'network','physical-cabling':'cabling','link-tracing':'trace'};
export function ApprovedTopologyWorkspace({view,rackOnly=false}:{view?:string;rackOnly?:boolean}){
 const identity=useIdentity(),location=useLocation();
 const [notice,setNotice]=useState(''),[reloadRevision,setReloadRevision]=useState(0);
 const ctx=useProject(),navigate=useNavigate(),route=useParams(),[url,setUrl]=useSearchParams(),cache=useQueryClient();
 const [error,setError]=useState('');
 const scopeSync=useRef<{initialized:boolean;url:string|null;pending:string|null|undefined}>({initialized:false,url:null,pending:undefined});
 useEffect(()=>{
 const requested=url.get('project_id'),sync=scopeSync.current;
 if(requested&&!ctx.projects.some(p=>p.id===requested))return;
 if(!sync.initialized||sync.url!==requested){sync.initialized=true;sync.url=requested;sync.pending=undefined;if(url.has('project_id')&&(!requested||ctx.projects.some(p=>p.id===requested))&&ctx.filter!==(requested||null)){sync.pending=requested||null;ctx.setFilter(sync.pending)}return;}
 if(sync.pending!==undefined){if(ctx.filter!==sync.pending)return;sync.pending=undefined}
 if(url.has('project_id')&&requested!==(ctx.filter||'')){const next=new URLSearchParams(url);sync.url=ctx.filter||'';next.set('project_id',sync.url);['cursor','matrix_cursor','connection_cursor','member_query_ref','bundle'].forEach(k=>next.delete(k));setUrl(next,{replace:true})}
 },[ctx.filter,url.get('project_id')]);
 const active=rackOnly?'rack':view||url.get('view')||'global';
 const requestedProject=url.get('project_id'),invalidScope=!!requestedProject&&!ctx.projects.some(p=>p.id===requestedProject);
 const allowed=!invalidScope&&(!url.has('project_id')||requestedProject===(ctx.filter||''));
 const root=(rackOnly?(url.get('view')==='gallery'?url.get('pod')||'':route.rackId||url.get('rack')||''):active==='global'?url.get('scope'):url.get('pod'))||'';
 const rootId=root==='__ungrouped__'?'':root;
 const visualView=active==='network'?'fabric':active==='cabling'||active==='trace'?'physical':'hierarchy';
 const baseParams={q:rackOnly&&url.get('view')==='gallery'?url.get('q')||undefined:undefined,unplaced:root==='__ungrouped__'||undefined,view:visualView as 'hierarchy'|'fabric'|'physical',root_id:rootId||undefined,fabric_id:url.get('fabric')||undefined,depth:rackOnly&&rootId&&url.get('view')!=='gallery'?2:1,limit:200,cursor:url.get('cursor')||undefined};
 const query=useQuery({enabled:allowed,queryKey:['infrastructure-approved',identity.user.id,ctx.filter,ctx.permissionRevision,active,reloadRevision,baseParams],queryFn:({signal})=>infrastructureApi.subgraph(ctx.filter,baseParams,signal),staleTime:60000});
 const snapshotId=query.data?.snapshot_id;
 const networkScopeReady=active!=='network'||!!url.get('pod');
 const directoryParams={view:'hierarchy' as const,depth:1,limit:200,snapshot_id:snapshotId};
 const directory=useQuery({enabled:allowed&&!!snapshotId&&!!rootId,queryKey:['infrastructure-approved',identity.user.id,ctx.filter,ctx.permissionRevision,'directory',directoryParams],queryFn:({signal})=>infrastructureApi.subgraph(ctx.filter,directoryParams,signal),staleTime:60000});
 const axes=active==='cabling'?(url.get('axis')==='switches'?'leaf_spine':'server_switch'):active==='global'&&root?'server_switch':'rack_leaf';
 const matrixParams={unplaced:root==='__ungrouped__'||undefined,root_id:rootId||undefined,fabric_id:url.get('fabric')||undefined,axes,q:active==='cabling'?url.get('q')||undefined:undefined,limit:100,cursor:url.get('matrix_cursor')||undefined,snapshot_id:snapshotId};
 const matrix=useQuery({enabled:allowed&&!!snapshotId&&networkScopeReady&&['global','network','cabling'].includes(active),queryKey:['infrastructure-approved',identity.user.id,ctx.filter,ctx.permissionRevision,'matrix',active,matrixParams],queryFn:({signal})=>infrastructureApi.matrix(ctx.filter,matrixParams,signal),staleTime:60000});
 const backboneParams={...matrixParams,axes:'leaf_spine',cursor:undefined};
 const backbone=useQuery({enabled:allowed&&!!snapshotId&&networkScopeReady&&['global','network'].includes(active),queryKey:['infrastructure-approved',identity.user.id,ctx.filter,ctx.permissionRevision,'matrix',active,backboneParams],queryFn:({signal})=>infrastructureApi.matrix(ctx.filter,backboneParams,signal),staleTime:60000});
 const selectedCell=matrix.data?.cells.find(c=>c.row_id===url.get('row')&&c.column_id===url.get('col'));
 const memberRef=url.get('member_query_ref')||selectedCell?.member_query_ref||'';
 const fromPort=url.has('port')?url.get('port')||'':query.data?.assets.find(a=>a.type==='port')?.id||'';
 const connectionParams={unplaced:root==='__ungrouped__'||url.get('bundle')==='__ungrouped__'||undefined,root_id:rootId||(active==='global'&&url.get('bundle')!=='__ungrouped__'?url.get('bundle'):null)||undefined,asset_id:active==='trace'?(query.data?.assets.find(a=>a.id===fromPort)?.fields.owner_device_id||url.get('asset')||undefined):!memberRef&&!url.get('port')?url.get('asset')||undefined:undefined,port_id:active!=='trace'&&!memberRef?url.get('port')||undefined:undefined,member_query_ref:memberRef||undefined,q:active==='cabling'?url.get('q')||undefined:undefined,status:active==='cabling'&&url.get('state')==='incomplete'?'incomplete':undefined,cursor:url.get('connection_cursor')||undefined,limit:100,snapshot_id:snapshotId};
 const connections=useQuery({enabled:allowed&&!!snapshotId&&(active==='cabling'||active==='trace'||rackOnly&&!!rootId&&url.get('view')!=='gallery'||active==='network'&&networkScopeReady&&!!memberRef||active==='global'&&(!!memberRef||!!url.get('bundle'))),queryKey:['infrastructure-approved',identity.user.id,ctx.filter,ctx.permissionRevision,'connections',active,connectionParams],queryFn:({signal})=>infrastructureApi.connections(ctx.filter,connectionParams,signal),staleTime:60000});
 const focusId=url.get('detail')||url.get('cable')||url.get('asset')||'';
 const detailParams={asset_id:focusId,limit:100,snapshot_id:snapshotId};
 const detail=useQuery({enabled:allowed&&!!snapshotId&&!!focusId&&!rackOnly,queryKey:['infrastructure-approved',identity.user.id,ctx.filter,ctx.permissionRevision,'detail',active,detailParams],queryFn:({signal})=>infrastructureApi.connections(ctx.filter,detailParams,signal),staleTime:60000});
 const focusParams={view:'physical' as const,root_id:url.get('asset')||url.get('port')||url.get('cable')||undefined,depth:2,limit:200,snapshot_id:snapshotId};
 const focused=useQuery({enabled:allowed&&!!snapshotId&&!!focusParams.root_id&&(rackOnly&&url.get('view')!=='gallery'||active==='cabling'&&url.get('mode')==='Port Panel'),queryKey:['infrastructure-approved',identity.user.id,ctx.filter,ctx.permissionRevision,'focus',active,focusParams],queryFn:({signal})=>infrastructureApi.subgraph(ctx.filter,focusParams,signal),staleTime:60000});
 const searchText=active==='global'?url.get('q')||'':'';
 const search=useQuery({enabled:allowed&&!!snapshotId&&!!searchText,queryKey:['infrastructure-approved',identity.user.id,ctx.filter,ctx.permissionRevision,'search',searchText,snapshotId],queryFn:({signal})=>infrastructureApi.subgraph(ctx.filter,{view:'hierarchy',q:searchText,limit:30,snapshot_id:snapshotId},signal),staleTime:60000});
 const requestedCable=[...(detail.data?.cables||[]),...(connections.data?.cables||[])].find(c=>c.id===url.get('cable'));
 const traceParams={candidate_cable_id:url.get('cable')||undefined,to_port_id:requestedCable?(requestedCable.a===fromPort?requestedCable.b:requestedCable.a)||undefined:undefined,mode:(url.get('mode')==='Logical Trace'?'logical':'physical') as 'logical'|'physical',from_port_id:fromPort,snapshot_id:snapshotId};
 const trace=useQuery({enabled:allowed&&!!snapshotId&&active==='trace'&&!!fromPort&&url.get('run')!=='0'&&traceParams.mode==='physical',queryKey:['infrastructure-approved',identity.user.id,ctx.filter,ctx.permissionRevision,'trace',traceParams],queryFn:({signal})=>infrastructureApi.trace(ctx.filter,traceParams,signal),staleTime:60000});
 const snapshot=useMemo(()=>{
  if(!query.data)return undefined;
  const fragments=[...(rootId?[directory.data]:[]),query.data,connections.data,detail.data,focused.data,search.data].filter(Boolean) as InfraSnapshot[];
  const assets=new Map<string,InfraAsset>(),cables=new Map(),relationships=new Map(),findings=new Map();
  const putAsset=(a:InfraAsset)=>{const old=assets.get(a.id),oldWins=old&&a.fields.visualization_summary===true&&old.fields.visualization_summary!==true;assets.set(a.id,{...old,...a,fields:oldWins?{...a.fields,...old.fields}:{...old?.fields,...a.fields}})};
  for(const f of fragments){for(const a of f.assets)putAsset(a);for(const c of f.cables)cables.set(c.id,c);for(const r of f.relationships)relationships.set(r.id,r);for(const fnd of f.findings)findings.set(fnd.asset_id+fnd.code,fnd)}
  for(const m of [matrix.data,backbone.data])for(const a of [...(m?.rows||[]),...(m?.columns||[])])putAsset(a);
  if(trace.data){for(const a of trace.data.assets)putAsset(a);for(const c of trace.data.segments)cables.set(c.id,c);for(const r of trace.data.relationships)relationships.set(r.id,r)}
  const rootAsset=query.data.assets.find(a=>a.id===rootId);if(rootAsset){const merged=assets.get(rootAsset.id);assets.set(rootAsset.id,{...merged,...rootAsset,fields:{...merged?.fields,...rootAsset.fields}})}
  return {...query.data,assets:[...assets.values()],cables:[...cables.values()],relationships:[...relationships.values()],findings:[...findings.values()]};
 },[query.data,directory.data,connections.data,detail.data,focused.data,search.data,matrix.data,backbone.data,trace.data]);
 const groups=useMemo(()=>[...new Map([...(rootId?directory.data?.groups||[]:[]),...(query.data?.groups||[])].map(g=>[g.id,g])).values()],[rootId,directory.data?.groups,query.data?.groups]);
 const queries=useMemo(()=>({groups,matrix:matrix.data,backbone:backbone.data,connections:connections.data,trace:trace.data,searchResults:search.data?.assets,loading:directory.isFetching||search.isFetching||backbone.isFetching||matrix.isFetching||connections.isFetching||detail.isFetching||focused.isFetching||trace.isFetching,error:(rootId?directory.error:null)||matrix.error||connections.error||detail.error||focused.error||trace.error||backbone.error||search.error||undefined}),[groups,query.data,directory.data,directory.error,directory.isFetching,search.isFetching,backbone.isFetching,matrix.data,backbone.data,connections.data,trace.data,search.data,matrix.isFetching,connections.isFetching,detail.isFetching,focused.isFetching,trace.isFetching,matrix.error,connections.error,detail.error,focused.error,trace.error,backbone.error,search.error]);
 useEffect(()=>realCmdbApi.subscribe(()=>{void cache.invalidateQueries({queryKey:['infrastructure-approved']})}),[cache]);
 const set=(patch:Record<string,string>)=>{const next=new URLSearchParams(window.location.search);if(Object.keys(patch).some(k=>['scope','pod','fabric','axis','q','state'].includes(k)))['member_query_ref','bundle'].forEach(k=>next.delete(k));if(Object.keys(patch).some(k=>['scope','pod','fabric','asset','port','row','col','axis','member_query_ref','q','state'].includes(k))){['connection_cursor','matrix_cursor','cursor'].forEach(k=>next.delete(k))}for(const [k,v]of Object.entries(patch)){const key=rackOnly&&k==='tab'?'rack_tab':k;v?next.set(key,v):next.delete(key);if(k==='asset')next.delete('selected');if(rackOnly&&k==='tab'&&!next.has('device_id'))next.delete('tab');}if(rackOnly&&patch.rack){next.delete('rack');navigate({pathname:'/racks/'+encodeURIComponent(patch.rack),search:next.toString()})}else setUrl(next);};
 useEffect(()=>{
  if(!rackOnly||route.rackId||url.get('rack')||url.get('view')==='gallery'||!query.data)return;
  const selected=url.get('asset')||url.get('port')||url.get('cable');
  const selectedAsset=selected?focused.data?.assets.find(a=>a.id===selected):undefined;
  const owner=selectedAsset?.fields.owner_device_id||focused.data?.cables.find(c=>c.id===selected)?.device_a;
  const target=selected?selectedAsset?.fields.rack_id||focused.data?.assets.find(a=>a.id===owner)?.fields.rack_id:query.data.assets.find(a=>a.type==='rack')?.id;
  if(target)set({rack:target});
 },[rackOnly,route.rackId,url.get('rack'),url.get('view'),url.get('asset'),url.get('port'),query.data,focused.data]);
 const reloadRange=async()=>{await cache.cancelQueries({queryKey:['infrastructure-approved']});const next=new URLSearchParams(window.location.search);['cursor','matrix_cursor','connection_cursor','member_query_ref','bundle'].forEach(k=>next.delete(k));setUrl(next,{replace:true});cache.removeQueries({queryKey:['infrastructure-approved']});setReloadRevision(v=>v+1);};
 const go:Go=(page,params={})=>{const next=new URLSearchParams(Object.entries(params).filter(([,v])=>v!=null&&v!==''));const origin=new URLSearchParams(url);origin.delete('return_to');next.set('return_to',location.pathname+'?'+origin.toString());next.set('project_id',ctx.filter||'');if(page==='rack-detail'){const rack=params.rack;next.delete('rack');if(next.has('tab')){next.set('rack_tab',next.get('tab')!);next.delete('tab')}navigate({pathname:rack?'/racks/'+encodeURIComponent(rack):'/racks',search:next.toString()})}else{next.set('view',pageViews[page]||'global');navigate({pathname:'/topology',search:next.toString()})}};
 const requestIdentity=useRef('');requestIdentity.current=identity.user.id+'|'+ctx.filter+'|'+ctx.permissionRevision+'|'+location.pathname+location.search;
 const openDeviceDrawer=useCallback(async(a:InfraAsset)=>{const identity=requestIdentity.current;await realCmdbApi.asset(a.id,a.project_id);const d=realCmdbApi.device(a.id);if(identity!==requestIdentity.current)return;if(d){const q=openDevice(new URLSearchParams(window.location.search),d);navigate({search:q.toString()})}},[navigate]);
 const handleOpenDevice=useCallback((a:InfraAsset)=>{void openDeviceDrawer(a).catch(e=>setError(e.message))},[openDeviceDrawer]);
 const savedKey='keel.visualization.view:'+identity.user.id+':'+(ctx.filter||'authorized')+':'+active;
 const validLocal=(path:string)=>{if(!path.startsWith('/')||path.startsWith('//'))return false;const u=new URL(path,window.location.origin);if(!/^\/(topology|racks)(\/|$)/.test(u.pathname))return false;const p=u.searchParams.get('project_id');return !p||ctx.projects.some(x=>x.id===p)};
 const params=new URLSearchParams(url);if(route.rackId)params.set('rack',route.rackId);if(!params.has('asset')&&params.has('selected'))params.set('asset',params.get('selected')!);if(params.get('face')==='back')params.set('face','rear');if(rackOnly&&params.has('rack_tab'))params.set('tab',params.get('rack_tab')!);
 if(invalidScope)return <div className="approved-visualization"><Empty>403 · 无权访问所请求的项目范围。</Empty></div>;
 if(query.isPending)return <div className="approved-visualization"><Empty>正在读取授权范围内的 CMDB 资产与连接…</Empty></div>;
 if(query.error)return <div className="approved-visualization"><Empty><p role="alert">{query.error.message}</p><Button onClick={reloadRange}>重新加载当前范围</Button></Empty></div>;
 if(!snapshot)return <Empty>没有可用的 CMDB 查询结果。</Empty>;
 const devices=snapshot.assets.filter(a=>['server','switch'].includes(a.type)).map(a=>({...a.fields,id:a.id,project_id:a.project_id,category:a.type,hostname:a.name}));
 const views:Record<string,React.ComponentType<any>>={global:Global,network:Network,cabling:Cabling,trace:Trace,rack:Rack};
 const Page=views[active]||Global;
 return <div className="approved-visualization">{active!=='rack'&&<div className="page-title"><h1>{({global:'全局拓扑',network:'网络架构',cabling:'物理布线',trace:'链路追踪',quality:'数据质量'} as Record<string,string>)[active]||'全局拓扑'}</h1><span className="spacer"/>{notice&&<small role="status">{notice}</small>}<Button onClick={()=>{localStorage.setItem(savedKey,location.pathname+location.search);setNotice('视图已保存至当前浏览器')}}><Bookmark size={14}/>保存视图</Button><Button onClick={()=>{const saved=localStorage.getItem(savedKey);if(saved&&validLocal(saved)){navigate(saved);setNotice('已恢复视图')}else setNotice('暂无可恢复的授权视图')}}><RotateCcw size={14}/>恢复视图</Button></div>}{url.get('return_to')&&validLocal(url.get('return_to')!)&&<div className="toolbar"><Button onClick={()=>navigate(url.get('return_to')!)}><ArrowLeft size={14}/>返回来源视图</Button></div>}<TopologyModelProvider snapshot={snapshot} queries={queries} onOpenDevice={handleOpenDevice}>{error&&<div role="alert" className="approved-partial">{error}<Button onClick={()=>setError('')}>关闭</Button></div>}{queries.error&&<div role="alert" className="approved-partial">局部查询失败：{queries.error.message}<Button onClick={reloadRange}>重新加载当前范围</Button></div>}{queries.loading&&<small aria-busy="true">正在加载当前范围…</small>}{(snapshot.truncated||snapshot.coverage?.complete===false)&&<div className="approved-partial" role="status">当前画布按范围分页加载，未显示全部资产；汇总以服务端计数为准。</div>}{query.data?.page?.has_more&&<div className="toolbar"><span>当前资产窗口 {query.data.assets.length} 项</span><Button onClick={()=>set({cursor:query.data!.page.next_cursor||''})}>下一批资产</Button><Button disabled={!url.get('cursor')} onClick={()=>set({cursor:''})}>首批资产</Button></div>}{active==='quality'?<section className="viz-v3"><div className="v-main-panel"><div className="v-figure-head"><strong>CMDB 数据质量</strong><span>{snapshot.findings.length} 项</span></div><table><thead><tr><th>对象</th><th>检查项</th><th>问题</th><th>定位</th></tr></thead><tbody>{snapshot.findings.map((f,i)=><tr key={f.asset_id+f.code+i}><td>{snapshot.assets.find(a=>a.id===f.asset_id)?.name||f.asset_id}</td><td>{f.code}</td><td>{f.message}</td><td><Button onClick={()=>go('physical-cabling',{cable:snapshot.cables.some(c=>c.id===f.asset_id)?f.asset_id:'',asset:f.asset_id})}>查看对象</Button></td></tr>)}</tbody></table>{!snapshot.findings.length&&<Empty>当前范围未报告数据质量问题；不代表运行状态已验证。</Empty>}</div></section>:snapshot.assets.length?<Suspense fallback={<Empty>加载机柜工作区…</Empty>}><Page params={params} set={set} go={go}/></Suspense>:<Empty>当前授权项目没有 CMDB 资产。可切换项目或前往 CMDB 导入资产。</Empty>}</TopologyModelProvider><DrawerDetail devices={devices}/></div>;
}
