import { cmdbFetch } from "./cmdb-http";
import type {components} from '../schemas/openapi';
export type InfraAsset=Omit<components['schemas']['InfrastructureAsset'],'fields'>&{fields:Record<string,any>};
export type InfraCable=Omit<components['schemas']['InfrastructureCable'],'fields'>&{fields:Record<string,any>};
export type InfraSnapshot=Omit<components['schemas']['InfrastructureSnapshot'],'assets'|'cables'>&{assets:InfraAsset[];cables:InfraCable[]};
export const infrastructureApi = {
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
export function physicalPath(cables: InfraCable[], from: string, to: string) {
  const endpoints = (id: string) => {
    const port = cables.find((c) => c.a === id || c.b === id);
    return port ? (port.a === id ? port.device_a : port.device_b) : id;
  };
  const start = endpoints(from),
    target = endpoints(to);
  if (!start || !target || !from || !to) return null;
  const adjacency = new Map<string, InfraCable[]>();
  for (const c of cables) {
    if (!c.device_a || !c.device_b) continue;
    for (const id of [c.device_a, c.device_b]) {
      if (!adjacency.has(id)) adjacency.set(id, []);
      adjacency.get(id)!.push(c);
    }
  }
  const queue = [start],
    seen = new Set([start]),
    prev = new Map<string, { node: string; cable: InfraCable }>();
  for (let i = 0; i < queue.length && i < 10000; i++) {
    const node = queue[i];
    if (node === target) break;
    for (const c of adjacency.get(node) || []) {
      if (node === start && from !== start && c.a !== from && c.b !== from)
        continue;
      const next = c.device_a === node ? c.device_b : c.device_a;
      if (next === target && to !== target && c.a !== to && c.b !== to)
        continue;
      if (seen.has(next)) continue;
      seen.add(next);
      prev.set(next, { node, cable: c });
      queue.push(next);
    }
  }
  if (!seen.has(target)) return null;
  const path: InfraCable[] = [];
  let current = target;
  while (current !== start) {
    const item = prev.get(current)!;
    path.push(item.cable);
    current = item.node;
  }
  return path.reverse();
}
