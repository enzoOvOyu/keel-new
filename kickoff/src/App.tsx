import { useEffect, useState, useSyncExternalStore } from 'react';
import { NavLink, Navigate, Route, Routes, useLocation, useNavigate, useParams } from 'react-router-dom';
import { Activity, ArrowUpRight, BookOpen, Box, Cable, Check, ChevronDown, ChevronRight, CircleHelp, Database, FileStack, FolderKanban, LayoutDashboard, ListChecks, Menu, Network, PanelLeftClose, PanelLeftOpen, Search, Server, Settings2, Sparkles, X, Zap } from 'lucide-react';
import { LocalKickoffProvider, type Evidence } from './lib/api';
import { WorkspaceContext } from './context';
import { Button, Empty, Modal } from './components/ui';
import { EvidenceDrawer } from './components/EvidenceDrawer';
import Overview from './pages/Overview';
import Sources from './pages/Sources';
import Review from './pages/Review';
import Publish from './pages/Publish';
import Modeling from './pages/Modeling';

const entries=[
  {path:'overview',label:'启动总览',icon:LayoutDashboard,group:'工作区'},
  {path:'sources',label:'资料与识别',icon:FileStack},
  {path:'review',label:'数据核对',icon:ListChecks},
  {path:'publish',label:'入库与版本',icon:Database},
  {path:'models/facility',label:'机房与机柜',icon:Server,group:'工程建模'},
  {path:'models/network',label:'网络与接入',icon:Network},
  {path:'models/provisioning',label:'装机准备',icon:Box},
  {path:'models/cabling',label:'布线与计算',icon:Cable},
  {path:'models/power',label:'电力与冷却',icon:Zap},
];
export default function App() {
  const [api]=useState(()=>new LocalKickoffProvider());
  const state=useSyncExternalStore(api.subscribe,api.snapshot);
  const navigate=useNavigate(),location=useLocation();
  const base=`/p/${state.projectId}/kickoff`;
  const [collapsed,setCollapsed]=useState(false),[toast,setToast]=useState<{message:string;error:boolean}>(),[about,setAbout]=useState(false);
  const [search,setSearch]=useState(false),[query,setQuery]=useState('');
  const [drawer,setDrawer]=useState<{kind:'device'|'port'|'connection'|'component'|'evidence';id:string;evidence?:Evidence}>();
  const page=entries.find(e=>location.pathname===base+'/'+e.path);
  const notify=(message:string,error=false)=>setToast({message,error});
  const go=(route:string)=>{navigate(base+'/'+route);setDrawer(undefined);};
  const run=(message:string,action:()=>unknown)=>{try{action();notify(message);return true;}catch(e){notify((e as Error).message,true);return false;}};
  useEffect(()=>{if(!toast)return;const t=setTimeout(()=>setToast(undefined),6500);return()=>clearTimeout(t);},[toast]);
  useEffect(()=>{const key=(e:KeyboardEvent)=>{if((e.metaKey||e.ctrlKey)&&e.key==='k'){e.preventDefault();setSearch(v=>!v);}};window.addEventListener('keydown',key);return()=>window.removeEventListener('keydown',key);},[]);
  return <WorkspaceContext.Provider value={{api,state,go,run,notify,inspect:(kind,id,evidence)=>setDrawer({kind,id,evidence})}}>
    <div className={`app-shell ${collapsed?'collapsed':''}`}>
      <aside className="sidebar">
        <NavLink className="brand" to={base+'/overview'}><span className="brand-symbol">k</span><strong>keel<span>工程交付平台</span></strong></NavLink>
        <button className="project-switch" onClick={()=>setAbout(true)} title="当前项目"><span className="project-symbol"><FolderKanban size={18}/></span><span><b>{state.projectName}</b><small>智能开局工程</small></span><ChevronDown size={14}/></button>
        <nav>{entries.map(e=><div key={e.path}>{e.group&&<div className="nav-group">{e.group}</div>}<NavLink to={base+'/'+e.path} title={e.label} className={({isActive})=>'nav-item'+(isActive?' active':'')}><e.icon size={17}/><span>{e.label}</span>{e.path==='review'&&state.issues.some(i=>!i.resolved&&i.scope!=='configuration')&&<em>{state.issues.filter(i=>!i.resolved&&i.scope!=='configuration').length}</em>}</NavLink></div>)}</nav>
        <div className="sidebar-links"><div className="nav-group">关联工作区</div><a href="http://localhost:5194/p/proj-b300-demo/fabric/overview" target="_blank" rel="noreferrer"><Network size={16}/><span>Fabric 网络交付</span><ArrowUpRight size={13}/></a><a href="http://localhost:5180/topology?project_id=proj-b300-demo" target="_blank" rel="noreferrer"><Activity size={16}/><span>数据中心拓扑</span><ArrowUpRight size={13}/></a></div>
        <div className="sidebar-bottom"><button onClick={()=>setAbout(true)} title="工作区说明"><CircleHelp size={16}/><span>工作区说明</span></button><button aria-label={collapsed?'展开侧栏':'折叠侧栏'} onClick={()=>setCollapsed(!collapsed)}>{collapsed?<PanelLeftOpen size={17}/>:<PanelLeftClose size={17}/>}</button></div>
      </aside>
      <div className="main-shell"><header className="topbar"><div className="breadcrumb"><FolderKanban size={14}/><span>{state.projectName}</span><ChevronRight size={13}/><span>智能开局</span><ChevronRight size={13}/><b>{page?.label??'工作区'}</b></div><div className="topbar-right"><button className="command" onClick={()=>setSearch(true)}><Search size={15}/><span>查找设备、功能</span><kbd>⌘ K</kbd></button><span className="workspace-saved"><Check size={12}/>已保存</span><button className="avatar" onClick={()=>setAbout(true)} aria-label="当前用户与工作区">陈</button></div></header>
      <main className="main-content"><Routes>
        <Route path="/" element={<Navigate to={base+'/overview'} replace/>}/>
        <Route path="/p/:projectId/kickoff" element={<ProjectGuard projectId={state.projectId}/> }>
          <Route index element={<Navigate to="overview" replace/>}/>
          <Route path="overview" element={<Overview/>}/><Route path="sources" element={<Sources/>}/><Route path="review" element={<Review/>}/><Route path="publish" element={<Publish/>}/><Route path="models/:domain" element={<Modeling/>}/>
        </Route>
        <Route path="*" element={<Empty title="工作区不存在" description="请使用左侧导航进入智能开局。" action={<Button onClick={()=>go('overview')}>返回总览</Button>}/>}/>
      </Routes></main></div>
    </div>
    {toast&&<div className={`toast ${toast.error?'error':''}`} role={toast.error?'alert':'status'}><span>{toast.error?<CircleHelp size={18}/>:<Check size={18}/>}</span>{toast.message}<button aria-label="关闭消息" onClick={()=>setToast(undefined)}><X size={15}/></button></div>}
    {drawer&&<EvidenceDrawer {...drawer} close={()=>setDrawer(undefined)}/>}
    {about&&<Modal title="智能开局工程工作区" description="统一资料输入，按能力逐步就绪。" close={()=>setAbout(false)}><div className="about-mark"><Sparkles size={25}/><div><b>Keel Intelligent Kickoff</b><p>{state.projectName}</p></div></div><p className="body-copy">设备与关系先形成最小开局，机房、网络、装机、布线和电力可以持续补充。每项缺口只影响对应功能。</p><div className="about-boundary"><b>当前连接方式</b><p>本地交互数据 · 变更保存在当前浏览器。AI 分析、真实 CMDB 写入及设备执行尚未连接。关联工作区使用各自已有数据。</p></div><p className="muted">前端与方案：kickoff/ · 后续通过统一 Provider 接入服务。</p><div className="modal-actions"><Button onClick={()=>setAbout(false)}>关闭</Button></div></Modal>}
    {search&&<Modal title="快速定位" description="查找工程工作区或已识别设备。" close={()=>setSearch(false)}><label className="search-input large"><Search size={18}/><input autoFocus placeholder="输入设备、SN或功能名称" value={query} onChange={e=>setQuery(e.target.value)}/></label><div className="search-results">{entries.filter(e=>e.label.includes(query)).map(e=><button key={e.path} onClick={()=>{go(e.path);setSearch(false);}}><e.icon size={17}/><span>{e.label}</span><small>工作区</small><ChevronRight size={14}/></button>)}{state.devices.filter(d=>(d.name+d.serial+d.model).toLowerCase().includes(query.toLowerCase())).slice(0,8).map(d=><button key={d.id} onClick={()=>{setDrawer({kind:'device',id:d.id});setSearch(false);}}><Server size={17}/><span>{d.name}</span><small>{d.model}</small><ChevronRight size={14}/></button>)}</div></Modal>}
  </WorkspaceContext.Provider>;
}
import { Outlet } from 'react-router-dom';
function ProjectGuard({projectId}:{projectId:string}){const params=useParams();return params.projectId===projectId?<Outlet/>:<Empty title="该项目尚未接入此工作区" description="请返回当前项目；不会将当前数据展示为其他项目的数据。"/>;}
