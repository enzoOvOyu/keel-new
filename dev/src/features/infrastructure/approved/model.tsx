import {createContext,useContext,useMemo,type ReactNode} from 'react';
import type {InfraAsset,InfraCable,InfraSnapshot} from '@/lib/api/infrastructure';
export type Go=(page:string,params?:Record<string,string>)=>void;
export const labels:Record<string,string>={unverified:'记录待验证',stale:'观测过期',conflict:'证据冲突',incomplete:'端点缺失',unknown:'未知',maintained:'维护态',unsupported:'不支持',healthy:'证据有效'};
export function createTopologyModel(snapshot:InfraSnapshot,onOpenDevice?:(asset:InfraAsset)=>void){
 const all=snapshot.assets,ports=all.filter(a=>a.type==='port'),assets=all.filter(a=>a.type!=='port'),racks=assets.filter(a=>a.type==='rack'),devices=assets.filter(a=>['server','switch','patch_panel','pdu'].includes(a.type));
 const index=new Map(all.map(a=>[a.id,a]));
 const cables=snapshot.cables;
 const cableIndex=new Map(cables.map(c=>[c.id,c]));
 const byOwner=new Map<string,InfraAsset[]>(),byObject=new Map<string,InfraCable[]>();
 for(const p of ports){const id=p.fields.owner_device_id;if(id)byOwner.set(id,[...(byOwner.get(id)||[]),p])}
 for(const c of cables)for(const id of new Set([c.id,c.a,c.b,c.device_a,c.device_b].filter(Boolean)))byObject.set(id,[...(byObject.get(id)||[]),c]);
 const synthetic=all.some(a=>/fixture|synthetic/i.test(a.source||'')||/fixture|synthetic/i.test(a.fields.source_ref||''));
 const data={assets,ports,cables,meta:{synthetic,source:synthetic?'CMDB · 含合成来源记录':'CMDB',observed_at:'',read_at:snapshot.read_at||'',snapshot_id:snapshot.snapshot_id||''}};
 const api={snapshot:()=>data,asset:(id:string)=>index.get(id),cable:(id:string)=>cableIndex.get(id),ports:(id:string)=>byOwner.get(id)||[],connections:(id:string)=>byObject.get(id)||[]};
 const name=(id:string)=>index.get(id)?.name||id||'缺失';
 const status=(c:InfraCable)=>{
  if(!c.a||!c.b||index.get(c.a)?.type!=='port'||index.get(c.b)?.type!=='port'||!index.has(c.device_a)||!index.has(c.device_b)||index.get(c.a)?.fields.owner_device_id!==c.device_a||index.get(c.b)?.fields.owner_device_id!==c.device_b)return 'incomplete';
  const findings=snapshot.findings.filter(f=>f.asset_id===c.id||[c.a,c.b].includes(f.asset_id));
  if(findings.some(f=>/CONFLICT|DUPLICATE|MISMATCH|OCCUPIED/.test(f.code)))return 'conflict';
  if(c.fields.freshness==='stale')return 'stale';
  return 'unverified';
 };
 return {api,data,racks,devices,name,status,labels,openDevice:onOpenDevice,snapshot};
}
export type TopologyModel=ReturnType<typeof createTopologyModel>;
const Context=createContext<TopologyModel|null>(null);
export function TopologyModelProvider({snapshot,children,onOpenDevice}:{snapshot:InfraSnapshot;children:ReactNode;onOpenDevice?:(asset:InfraAsset)=>void}){const model=useMemo(()=>createTopologyModel(snapshot,onOpenDevice),[snapshot,onOpenDevice]);return <Context.Provider value={model}>{children}</Context.Provider>}
export function useTopologyModel(){const m=useContext(Context);if(!m)throw Error('缺少 CMDB 可视化数据范围');return m;}
export function exportFile(filename:string,text:string,type='text/csv;charset=utf-8'){const u=URL.createObjectURL(new Blob([text],{type}));const a=document.createElement('a');a.href=u;a.download=filename;a.click();setTimeout(()=>URL.revokeObjectURL(u),1000)}
export {locationIssues} from '@/lib/api/infrastructure';
