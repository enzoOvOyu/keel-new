import fixture from './fixture.json';
import type {InfraAsset,InfraCable} from '@/lib/api/infrastructure';
export const data=fixture as unknown as {meta:{synthetic:boolean;source:string;observed_at:string};assets:InfraAsset[];ports:InfraAsset[];cables:InfraCable[]};
export const api={snapshot:()=>data,asset:(id:string)=>[...data.assets,...data.ports].find(x=>x.id===id),cable:(id:string)=>data.cables.find(x=>x.id===id),ports:(id:string)=>data.ports.filter(x=>x.fields.owner_device_id===id),connections:(id:string)=>data.cables.filter(x=>[x.id,x.a,x.b,x.device_a,x.device_b].includes(id))};
export const racks=data.assets.filter(x=>x.type==='rack'),devices=data.assets.filter(x=>x.type!=='rack');
export const status=(c:InfraCable)=>c.fields.example||'unverified';
export const labels:Record<string,string>={unverified:'记录待验证',stale:'观测过期',conflict:'证据冲突',incomplete:'端点缺失',unknown:'未知',maintained:'维护态',healthy:'历史正常'};
export const name=(id:string)=>api.asset(id)?.name||id||'缺失';
export function exportFile(filename:string,text:string,type='text/csv;charset=utf-8'){const u=URL.createObjectURL(new Blob([text],{type}));const a=document.createElement('a');a.href=u;a.download=filename;a.click();setTimeout(()=>URL.revokeObjectURL(u),1000)}
export type Go=(page:string,params?:Record<string,string>)=>void;

// Same pure U validation as current implementation; isolated to avoid importing live API initialization.
export function locationIssues(
  devices: InfraAsset[],
  height: number,
  positions: Record<string, number> = {},
  rack?: InfraAsset,
) {
  const issues: { asset_id: string; code: string; message: string }[] = [];
  const slots = new Map<number, string>();
  for (const d of devices) {
    const u = positions[d.id] ?? Number(d.fields.u_start ?? d.fields.rack_u),
      h = Number(d.fields.u_height);
    const add = (code: string, message: string) =>
      issues.push({ asset_id: d.id, code, message });
    for (const dimension of ["width_mm", "depth_mm"])
      if (
        rack &&
        Number(rack.fields[dimension]) > 0 &&
        Number(d.fields[dimension]) > Number(rack.fields[dimension])
      )
        add("DIMENSION_CONFLICT", dimension + " 超过机柜规格");
    if (
      d.fields.mounting_face &&
      !["front", "rear", "back", "both"].includes(d.fields.mounting_face)
    )
      add("MOUNTING_FACE", "安装面记录无效");
    if (!Number.isInteger(u) || !Number.isInteger(h) || h < 1) {
      add("MISSING_U", "U 位或设备高度未确认");
      continue;
    }
    if (u < 1 || u + h - 1 > height) add("U_RANGE", "设备超出机柜 U 位范围");
    for (let i = u; i < u + h; i++) {
      if (slots.has(i))
        add("U_OVERLAP", "与 " + slots.get(i) + " 在 U" + i + " 重叠");
      slots.set(i, d.name);
    }
  }
  return issues;
}
