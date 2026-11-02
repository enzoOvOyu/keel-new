import {useEffect,useMemo,useState,useSyncExternalStore} from 'react';
import {useLocation,useNavigate,useSearchParams} from 'react-router-dom';
import {CheckCircle2,X} from 'lucide-react';
import {useQuery} from '@tanstack/react-query';
import {useProject} from '../../app/ProjectContext';
import {useIdentity} from '../auth/IdentityProvider';
import {loadFabricProvider,type StudioProvider} from '../../lib/api/fabric/provider';
import {WorkspaceContext,type DialogRequest} from './context';
import {Overview,Blueprint,Onboarding,Configuration,Validation,Deployments,Assurance,Settings} from './Pages';
import {ActionDialog,EvidenceDrawer} from './Dialogs';
import {Button,Empty,Note} from './ui';
import './styles.css';
import './host.css';
const pages:Record<string,()=>React.ReactNode>={overview:Overview,blueprint:Blueprint,onboarding:Onboarding,configuration:Configuration,validation:Validation,deployments:Deployments,assurance:Assurance,settings:Settings};
export function FabricWorkspace({view='overview'}:{view?:string}){
 const project=useProject(),identity=useIdentity();
 const query=useQuery({queryKey:['fabric-workspace',project.projectId,project.permissionRevision,identity.user.id,import.meta.env.VITE_FABRIC_PROVIDER||'real'],queryFn:({signal})=>loadFabricProvider(project.projectId,identity,signal),staleTime:60000});
 const provider=query.data;
 if(query.error)return <div className="fabric-workspace"><Note tone="danger">{query.error.message}</Note><Button onClick={()=>void query.refetch()}>重新加载</Button></div>;
 if(!provider)return <div className="fabric-workspace" aria-busy="true"><Empty title="打开 Fabric Workspace">读取发布输入、资产引用与历史状态…</Empty></div>;
 return <Workspace key={project.projectId+':'+identity.user.id} p={provider} page={view}/>;
}
function Workspace({p,page}:{p:StudioProvider;page:string}){
 const state=useSyncExternalStore(p.subscribe,p.snapshot),project=useProject(),location=useLocation(),navigate=useNavigate();const [params,setParams]=useSearchParams();
 const [dialog,setDialog]=useState<DialogRequest>(),[drawer,setDrawer]=useState<{kind:string;id:string}>(),[toast,setToast]=useState<{message:string;error:boolean}>();
 const plane=params.get('plane')||'',mode=(['physical','logical','planes'].includes(params.get('mode')||'')?params.get('mode'):'physical') as 'physical'|'logical'|'planes';
 const base='/p/'+project.projectId+'/deploy/fabric';
 useEffect(()=>{setDrawer(undefined);setDialog(undefined)},[location.pathname]);
 useEffect(()=>{if(!toast)return;const id=setTimeout(()=>setToast(undefined),6000);return()=>clearTimeout(id)},[toast]);
 const go=(id:string)=>{navigate(base+'/'+id+(params.toString()?'?'+params.toString():''));setDrawer(undefined)};
 const setQueryParam=(key:string,value:string)=>{const next=new URLSearchParams(params);if(value)next.set(key,value);else next.delete(key);setParams(next)};
 const run=async(message:string,fn:()=>unknown)=>{try{await fn();setToast({message,error:false});return true}catch(e){setToast({message:(e as Error).message,error:true});return false}};
 const value=useMemo(()=>({p,s:state,run,open:(kind:string,id?:string,ids?:string[])=>setDialog({kind,id,ids}),inspect:(kind:string,id:string)=>setDrawer({kind,id}),go,plane,setPlane:(id:string)=>setQueryParam('plane',id),mode,setMode:(m:'physical'|'logical'|'planes')=>setQueryParam('mode',m)}),[p,state,plane,mode,params.toString()]);
 const Page=pages[page];
 return <div className="fabric-workspace"><WorkspaceContext.Provider value={value}><div className="main-content">{Page?<Page/>:<Empty title="工作区不存在" action={<Button onClick={()=>go('overview')}>返回总览</Button>}/>}
 <footer className="page-footer"><span>Keel Fabric Studio <i/> Source of truth → Intent → Evidence</span><span>{p.mode==='mock'?'SIMULATION_ONLY · 无真实设备执行':'已发布 CMDB 输入 · 配置执行尚未接入'}</span></footer></div>
 {toast&&<div className={'toast '+(toast.error?'error':'')} role={toast.error?'alert':'status'}><CheckCircle2 size={18}/><span>{toast.message}</span><button onClick={()=>setToast(undefined)} aria-label="关闭提示"><X size={15}/></button></div>}
 {dialog&&<ActionDialog key={dialog.kind+dialog.id} request={dialog} close={()=>setDialog(undefined)}/>}{drawer&&!dialog&&<EvidenceDrawer key={drawer.kind+drawer.id} kind={drawer.kind} id={drawer.id} close={()=>setDrawer(undefined)}/>}<div id="fabric-portals"/>
 </WorkspaceContext.Provider></div>;
}
