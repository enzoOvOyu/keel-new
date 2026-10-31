import {useCallback,useEffect,useRef,useState,lazy,Suspense} from 'react';
import {useNavigate,useParams,useSearchParams,useLocation} from 'react-router-dom';
import {useQuery,useQueryClient} from '@tanstack/react-query';
import {useIdentity} from '@/features/auth/IdentityProvider';
import {Bookmark,RotateCcw,ArrowLeft} from 'lucide-react';
import {useProject} from '@/app/ProjectContext';
import {infrastructureApi,type InfraAsset} from '@/lib/api/infrastructure';
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
 const [notice,setNotice]=useState('');
 const ctx=useProject(),navigate=useNavigate(),route=useParams(),[url,setUrl]=useSearchParams(),cache=useQueryClient();
 const [error,setError]=useState('');
 const scopeSync=useRef<{initialized:boolean;url:string|null;pending:string|null|undefined}>({initialized:false,url:null,pending:undefined});
 useEffect(()=>{
 const requested=url.get('project_id'),sync=scopeSync.current;
 if(!sync.initialized||sync.url!==requested){sync.initialized=true;sync.url=requested;sync.pending=undefined;if(url.has('project_id')&&(!requested||ctx.projects.some(p=>p.id===requested))&&ctx.filter!==(requested||null)){sync.pending=requested||null;ctx.setFilter(sync.pending)}return;}
 if(sync.pending!==undefined){if(ctx.filter!==sync.pending)return;sync.pending=undefined}
 if(url.has('project_id')&&requested!==(ctx.filter||'')){const next=new URLSearchParams(url);sync.url=ctx.filter||'';next.set('project_id',sync.url);setUrl(next,{replace:true})}
 },[ctx.filter,url.get('project_id')]);
 const active=rackOnly?'rack':view||url.get('view')||'global';
 const requestedProject=url.get('project_id'),invalidScope=!!requestedProject&&!ctx.projects.some(p=>p.id===requestedProject);
 const query=useQuery({enabled:!invalidScope&&(!url.has('project_id')||requestedProject===(ctx.filter||'')),queryKey:['infrastructure-approved',ctx.filter,ctx.permissionRevision],queryFn:({signal})=>infrastructureApi.fullSnapshot(ctx.filter,signal),staleTime:60000});
 useEffect(()=>realCmdbApi.subscribe(()=>{void cache.invalidateQueries({queryKey:['infrastructure-approved']})}),[cache]);
 const set=(patch:Record<string,string>)=>{const next=new URLSearchParams(url);for(const [k,v]of Object.entries(patch)){const key=rackOnly&&k==='tab'?'rack_tab':k;v?next.set(key,v):next.delete(key);if(k==='asset')next.delete('selected');if(rackOnly&&k==='tab'&&!next.has('device_id'))next.delete('tab');}if(rackOnly&&patch.rack){next.delete('rack');navigate({pathname:'/racks/'+encodeURIComponent(patch.rack),search:next.toString()})}else setUrl(next);};
 const go:Go=(page,params={})=>{const next=new URLSearchParams(Object.entries(params).filter(([,v])=>v!=null&&v!==''));const origin=new URLSearchParams(url);origin.delete('return_to');next.set('return_to',location.pathname+'?'+origin.toString());next.set('project_id',ctx.filter||'');if(page==='rack-detail'){const rack=params.rack;next.delete('rack');if(next.has('tab')){next.set('rack_tab',next.get('tab')!);next.delete('tab')}navigate({pathname:rack?'/racks/'+encodeURIComponent(rack):'/racks',search:next.toString()})}else{next.set('view',pageViews[page]||'global');navigate({pathname:'/topology',search:next.toString()})}};
 const openDeviceDrawer=useCallback(async(a:InfraAsset)=>{await realCmdbApi.asset(a.id,a.project_id);const d=realCmdbApi.device(a.id);if(d){const q=openDevice(new URLSearchParams(window.location.search),d);navigate({search:q.toString()})}},[navigate]);
 const handleOpenDevice=useCallback((a:InfraAsset)=>{void openDeviceDrawer(a).catch(e=>setError(e.message))},[openDeviceDrawer]);
 const savedKey='keel.visualization.view:'+identity.user.id+':'+(ctx.filter||'authorized')+':'+active;
 const validLocal=(path:string)=>{if(!path.startsWith('/')||path.startsWith('//'))return false;const u=new URL(path,window.location.origin);if(!/^\/(topology|racks)(\/|$)/.test(u.pathname))return false;const p=u.searchParams.get('project_id');return !p||ctx.projects.some(x=>x.id===p)};
 const params=new URLSearchParams(url);if(route.rackId)params.set('rack',route.rackId);if(!params.has('asset')&&params.has('selected'))params.set('asset',params.get('selected')!);if(params.get('face')==='back')params.set('face','rear');if(rackOnly&&params.has('rack_tab'))params.set('tab',params.get('rack_tab')!);
 if(invalidScope)return <div className="approved-visualization"><Empty>403 · 无权访问所请求的项目范围。</Empty></div>;
 if(query.isPending)return <div className="approved-visualization"><Empty>正在读取授权范围内的 CMDB 资产与连接…</Empty></div>;
 if(query.error)return <div className="approved-visualization"><Empty><p role="alert">{query.error.message}</p><Button onClick={()=>query.refetch()}>重新加载</Button></Empty></div>;
 const snapshot=query.data!;
 const devices=snapshot.assets.filter(a=>['server','switch'].includes(a.type)).map(a=>({...a.fields,id:a.id,project_id:a.project_id,category:a.type,hostname:a.name}));
 const views:Record<string,React.ComponentType<any>>={global:Global,network:Network,cabling:Cabling,trace:Trace,rack:Rack};
 const Page=views[active]||Global;
 return <div className="approved-visualization">{active!=='rack'&&<div className="page-title"><h1>{({global:'全局拓扑',network:'网络架构',cabling:'物理布线',trace:'链路追踪',quality:'数据质量'} as Record<string,string>)[active]||'全局拓扑'}</h1><span className="spacer"/>{notice&&<small role="status">{notice}</small>}<Button onClick={()=>{localStorage.setItem(savedKey,location.pathname+location.search);setNotice('视图已保存至当前浏览器')}}><Bookmark size={14}/>保存视图</Button><Button onClick={()=>{const saved=localStorage.getItem(savedKey);if(saved&&validLocal(saved)){navigate(saved);setNotice('已恢复视图')}else setNotice('暂无可恢复的授权视图')}}><RotateCcw size={14}/>恢复视图</Button></div>}{url.get('return_to')&&validLocal(url.get('return_to')!)&&<div className="toolbar"><Button onClick={()=>navigate(url.get('return_to')!)}><ArrowLeft size={14}/>返回来源视图</Button></div>}<TopologyModelProvider snapshot={snapshot} onOpenDevice={handleOpenDevice}>{error&&<div role="alert" className="approved-partial">{error}<Button onClick={()=>setError('')}>关闭</Button></div>}{(snapshot.truncated||snapshot.coverage?.complete===false)&&<div className="approved-partial" role="status">当前查询结果不完整，请缩小项目范围；图中数量仅覆盖已加载记录。</div>}{active==='quality'?<section className="viz-v3"><div className="v-main-panel"><div className="v-figure-head"><strong>CMDB 数据质量</strong><span>{snapshot.findings.length} 项</span></div><table><thead><tr><th>对象</th><th>检查项</th><th>问题</th><th>定位</th></tr></thead><tbody>{snapshot.findings.map((f,i)=><tr key={f.asset_id+f.code+i}><td>{snapshot.assets.find(a=>a.id===f.asset_id)?.name||f.asset_id}</td><td>{f.code}</td><td>{f.message}</td><td><Button onClick={()=>go('physical-cabling',{cable:snapshot.cables.some(c=>c.id===f.asset_id)?f.asset_id:'',asset:f.asset_id})}>查看对象</Button></td></tr>)}</tbody></table>{!snapshot.findings.length&&<Empty>当前范围未报告数据质量问题；不代表运行状态已验证。</Empty>}</div></section>:snapshot.assets.length?<Suspense fallback={<Empty>加载机柜工作区…</Empty>}><Page params={params} set={set} go={go}/></Suspense>:<Empty>当前授权项目没有 CMDB 资产。可切换项目或前往 CMDB 导入资产。</Empty>}</TopologyModelProvider><DrawerDetail devices={devices}/></div>;
}
