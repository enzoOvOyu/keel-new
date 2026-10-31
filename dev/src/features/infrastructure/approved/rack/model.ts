import {useMemo} from 'react';
import {useTopologyModel} from '../model';
import {layoutRack,type Face,type GeometryInput} from './geometry';
import type {InfraAsset} from '@/lib/api/infrastructure';
export type Overlay='asset'|'capacity'|'compute';
export const overlayLabels:Record<Overlay,string>={asset:'资产',capacity:'空间',compute:'计算资源'};
const number=(v:unknown):number|null=>v===null||v===undefined||v===''||!Number.isFinite(Number(v))?null:Number(v);
export function geometry(d:InfraAsset,positions:Record<string,number>={}):GeometryInput{
  const f=d.fields,m=f.mounting_face??f.face;
  const face:Face=m==='back'?'rear':['front','rear','both'].includes(m)?m:'unknown';
  return {id:d.id,u:positions[d.id]??number(f.u_start??f.rack_u),height:number(f.u_height),face,fullDepth:typeof f.is_full_depth==='boolean'?f.is_full_depth:null,depth:number(f.depth_mm)};
}
export function useRackModel(){
  const {api,data,racks,devices}=useTopologyModel();
  return useMemo(()=>{
    const inRack=(id:string)=>devices.filter(d=>d.fields.rack_id===id);
    return {
      list:()=>racks,devices:inRack,
      snapshot(id:string,positions:Record<string,number>={}){
        const rack=racks.find(r=>r.id===id);if(!rack)return null;
        const items=inRack(id),height=number(rack.fields.u_height);
        const layout=height!==null&&height>0?layoutRack(height,items.map(d=>geometry(d,positions)),number(rack.fields.depth_mm)):null;
        const servers=items.filter(d=>d.type==='server'),gpuKnown=servers.filter(d=>number(d.fields.gpu_count)!==null);
        return {rack,items,height,layout,servers:servers.length,switches:items.filter(d=>d.type==='switch').length,gpu:gpuKnown.reduce((n,d)=>n+Number(d.fields.gpu_count),0),gpuCoverage:gpuKnown.length,gpuTotal:servers.length,missingFaces:items.filter(d=>geometry(d).face==='unknown').length,ports:data.ports.filter(p=>items.some(d=>d.id===p.fields.owner_device_id)),connections:data.cables.filter(c=>items.some(d=>d.id===c.device_a||d.id===c.device_b))};
      },
      capabilities:{asset:devices.length>0,capacity:racks.some(r=>number(r.fields.u_height)!==null),compute:devices.some(d=>d.type==='server'&&number(d.fields.gpu_count)!==null),power:false,health:false,thermal:false},
      source:{layer:'recorded',label:'CMDB 资产登记',synthetic:data.meta.synthetic,referenceAt:data.meta.observed_at||'未提供',description:'来源和实装确认以各资产证据为准。'},
      asset:api.asset,ports:api.ports,connections:api.connections,cable:api.cable,
    };
  },[api,data,racks,devices]);
}
export type RackModel=ReturnType<typeof useRackModel>;
export type RackSnapshot=NonNullable<ReturnType<RackModel['snapshot']>>;
export const typeLabel=(d:InfraAsset)=>d.type==='server'?(Number(d.fields.gpu_count)>0?'GPU 服务器':'服务器'):d.type==='switch'?`${d.fields.role||'网络'} 交换机`:d.type;
export const faceLabel=(face:Face)=>({front:'正面',rear:'背面',both:'双面',unknown:'未记录'}[face]);
export const shortPod=(id:string)=>id?String(id).replace(/^(demo-)?pod-0*/, 'Pod '):'未分组';
