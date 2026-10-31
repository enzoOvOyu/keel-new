import { cmdbFetch } from "./cmdb-http";
import type {components} from '../schemas/openapi';
export type InfraAsset=Omit<components['schemas']['InfrastructureAsset'],'fields'>&{fields:Record<string,any>};
export type InfraCable=Omit<components['schemas']['InfrastructureCable'],'fields'>&{fields:Record<string,any>};
export type InfraSnapshot=Omit<components['schemas']['InfrastructureSnapshot'],'assets'|'cables'>&{assets:InfraAsset[];cables:InfraCable[]};
export type InfraPage=components['schemas']['InfrastructureQueryPage'];
export type InfraGroup=components['schemas']['InfrastructureGroup'];
export type InfraQuerySnapshot=Omit<components['schemas']['InfrastructureSubgraph'],'assets'|'cables'>&{assets:InfraAsset[];cables:InfraCable[]};
export type InfraMatrix=Omit<components['schemas']['InfrastructureMatrix'],'rows'|'columns'>&{rows:InfraAsset[];columns:InfraAsset[]};
export type InfraTrace=Omit<components['schemas']['InfrastructureTrace'],'assets'|'segments'>&{assets:InfraAsset[];segments:InfraCable[]};
export type InfraTraceRequest=Omit<components['schemas']['InfrastructureTraceRequest'],'project_id'>;
export type InfraQuery={unplaced?:boolean;root_id?:string;view?:'hierarchy'|'fabric'|'physical';fabric_id?:string;depth?:number;cursor?:string;limit?:number;snapshot_id?:string;q?:string;axes?:string;asset_id?:string;port_id?:string;member_query_ref?:string;status?:string};
function queryUrl(path:string,project:string|null,query:InfraQuery){return 'visualization/'+path+'?'+new URLSearchParams(Object.entries({...query,...(project?{project_id:project}:{})}).filter(([,v])=>v!==undefined&&v!=='').map(([k,v])=>[k,String(v)]))}
export const infrastructureApi = {
  subgraph(project:string|null,query:InfraQuery,signal?:AbortSignal){return cmdbFetch<{data:InfraQuerySnapshot}>(queryUrl('subgraph',project,query),'GET',undefined,signal).then(r=>r.data)},
  matrix(project:string|null,query:InfraQuery,signal?:AbortSignal){return cmdbFetch<{data:InfraMatrix}>(queryUrl('matrix',project,query),'GET',undefined,signal).then(r=>r.data)},
  connections(project:string|null,query:InfraQuery,signal?:AbortSignal){return cmdbFetch<{data:InfraQuerySnapshot}>(queryUrl('connections',project,query),'GET',undefined,signal).then(r=>r.data)},
  trace(project:string|null,query:InfraTraceRequest,signal?:AbortSignal){return cmdbFetch<{data:InfraTrace}>('visualization/trace','POST',{...query,...(project?{project_id:project}:{})},signal).then(r=>r.data)},
  fullSnapshot(project: string | null, signal?: AbortSignal, scope: {rack_id?:string;focus?:string;limit?:number} = {}) {
    return cmdbFetch<{data:InfraSnapshot}>("visualization?"+new URLSearchParams({detail:"full",limit:String(scope.limit??5000),...(project?{project_id:project}:{}),...(scope.rack_id?{rack_id:scope.rack_id}:{}),...(scope.focus?{focus:scope.focus}:{})}),"GET",undefined,signal).then(r=>r.data);
  },
  snapshot(
    project: string | null,
    rack: string,
    focus: string,
    signal?: AbortSignal,
  ) {
    return cmdbFetch<{ data: InfraSnapshot }>(
      "visualization?" +
        new URLSearchParams({
          ...(project ? { project_id: project } : {}),
          ...(rack ? { rack_id: rack } : {}),
          ...(focus ? { focus } : {}),
        }),
      "GET",
      undefined,
      signal,
    ).then((r) => r.data);
  },
};
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
// Physical continuity stops at a device port. A switch is not evidence of a
// patch-through connection or a forwarding path between arbitrary ports.
export function physicalPath(cables: InfraCable[], from: string, to: string) {
  if (!from || !to || from === to) return null;
  const cable = cables.find(c=>c.a && c.b && ((c.a===from && c.b===to)||(c.b===from && c.a===to)));
  return cable ? [cable] : null;
}
