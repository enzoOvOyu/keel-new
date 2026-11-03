import { useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react';
import { Navigate, Route, Routes, useNavigate, useLocation } from 'react-router-dom';
import { Check, CircleHelp, X } from 'lucide-react';
import { useProject } from '../../app/ProjectContext';
import { HttpKickoffProvider } from '../../lib/api/kickoff/http';
import type { Evidence } from './lib/api';
import { WorkspaceContext } from './context';
import { EvidenceDrawer } from './components/EvidenceDrawer';
import Overview from './pages/Overview';
import Sources from './pages/Sources';
import Review from './pages/Review';
import Publish from './pages/Publish';
import Modeling from './pages/Modeling';
import './styles.css';

export default function KickoffWorkspace(){const project=useProject();if(!project.permissions.includes('provision:read'))return <div className="empty"><h2>403 · 当前身份无此页面权限</h2></div>;return <ProjectWorkspace key={project.projectId+':'+project.permissionRevision}/>;}
function ProjectWorkspace(){
 const project=useProject(),navigate=useNavigate(),location=useLocation();
 const api=useMemo(()=>new HttpKickoffProvider(project.projectId,project.projects.find(p=>p.id===project.projectId)?.name||project.projectId),[project.projectId]);
 const state=useSyncExternalStore(api.subscribe,api.snapshot);
 const [loading,setLoading]=useState(true),[error,setError]=useState(''),[toast,setToast]=useState<{message:string;error:boolean}>();
 const [drawer,setDrawer]=useState<{kind:'device'|'port'|'connection'|'component'|'evidence';id:string;evidence?:Evidence}>();
 const request=useRef(0);
 const load=()=>{const revision=++request.current;setLoading(true);setError('');void api.refresh().catch(e=>{if(request.current===revision&&e.name!=='AbortError')setError(e.message);}).finally(()=>{if(request.current===revision)setLoading(false);});};
 useEffect(()=>{load();return()=>{request.current++;api.dispose();};},[api]);
 useEffect(()=>{if(loading||!location.pathname.includes('/models/'))return;void api.loadSection('calculations').catch(e=>{if(e.name!=='AbortError')setToast({message:e.message,error:true});});},[api,loading,location.pathname,state.revision]);
 useEffect(()=>{if(!toast)return;const timer=setTimeout(()=>setToast(undefined),6500);return()=>clearTimeout(timer);},[toast]);
 const notify=(message:string,error=false)=>setToast({message,error});
 const run=async(message:string,action:()=>unknown)=>{try{await action();notify(message);return true;}catch(e){notify((e as Error).message,true);return false;}};
 return <div className="kickoff-workspace"><WorkspaceContext.Provider value={{api,state,can:permission=>project.permissions.includes(permission),notify,run,go:route=>{setDrawer(undefined);navigate(`/p/${project.projectId}/kickoff/${route}`);},inspect:(kind,id,evidence)=>setDrawer({kind,id,evidence})}}>
 {loading?<div className="empty" aria-busy="true">正在读取智能开局工程…</div>:error?<div className="notice error" role="alert">{error}<button onClick={load}>重新加载</button></div>:<div className="main-content"><Routes><Route index element={<Navigate to="overview" replace/>}/><Route path="overview" element={<Overview/>}/><Route path="sources" element={<Sources/>}/><Route path="review" element={<Review/>}/><Route path="publish" element={<Publish/>}/><Route path="models/:domain" element={<Modeling/>}/><Route path="*" element={<div className="empty">工作区不存在</div>}/></Routes></div>}
 {toast&&<div className={`toast ${toast.error?'error':''}`} role={toast.error?'alert':'status'}>{toast.error?<CircleHelp size={18}/>:<Check size={18}/>}<span>{toast.message}</span><button aria-label="关闭消息" onClick={()=>setToast(undefined)}><X size={15}/></button></div>}
 {drawer&&<EvidenceDrawer {...drawer} close={()=>setDrawer(undefined)}/>}<div id="kickoff-portals"/>
 </WorkspaceContext.Provider></div>;
}
