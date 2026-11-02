import type { Calculation, CalculationRow, ModelRow, Workspace } from './types';

export function rowsFor(s: Workspace, id: string): ModelRow[] {
  return s.models.find(m => m.tableId === id)?.rows ?? [];
}
export function calculationDigest(s: Workspace, domain: 'cabling' | 'power') {
  const ids = domain === 'cabling'
    ? ['route_nodes', 'route_edges', 'rack_access', 'cable_specs', 'cabling_rules', 'placements']
    : ['power_profiles', 'rack_capacity', 'placements'];
  return JSON.stringify([s.models.filter(m => ids.includes(m.tableId)), s.devices.map(d => [d.id, d.rackId]), domain === 'cabling' ? s.connections : []]);
}
function shortestPath(edges: ModelRow[], from: string, to: string) {
  const distance = new Map<string, number>([[from, 0]]);
  const paths = new Map<string, string[]>([[from, [from]]]);
  const visited = new Set<string>();
  for (;;) {
    const next = [...distance].filter(([id]) => !visited.has(id)).sort((a,b) => a[1] - b[1] || a[0].localeCompare(b[0]))[0];
    if (!next) return null;
    const [node, cost] = next;
    if (node === to) return { distance: cost, nodes: paths.get(node)! };
    visited.add(node);
    for (const edge of edges) {
      const other = edge.from === node ? edge.to : edge.to === node ? edge.from : '';
      const length = Number(edge.distance_m);
      if (!other || !(length > 0)) continue;
      if (cost + length < (distance.get(other) ?? Infinity)) {
        distance.set(other, cost + length);
        paths.set(other, [...paths.get(node)!, other]);
      }
    }
  }
}
const round = (n: number) => Math.round(n * 1000) / 1000;
export function calculateWorkspace(s: Workspace, domain: 'cabling' | 'power'): Calculation {
  const result: Calculation = { id: crypto.randomUUID(), domain, inputDigest: calculationDigest(s, domain),
    createdAt: new Date().toISOString(), status: 'needs_input', rows: [], issues: [], power: [] };
  if (domain === 'power') {
    for (const rack of s.racks) {
      const devices = s.devices.filter(d => d.rackId === rack.id);
      if (!devices.length) continue;
      const profiles = rowsFor(s, 'power_profiles');
      const powers = devices.map(d => profiles.find(p => p.device_id === d.id && p.basis === 'planned'));
      const capacity = rowsFor(s, 'rack_capacity').find(r => r.rack_id === rack.id);
      const loadKW = powers.every(p => p?.power_kw && Number(p.power_kw) >= 0) ? round(powers.reduce((sum,p) => sum + Number(p!.power_kw), 0)) : null;
      const capacityKW = capacity?.capacity_kw ? Number(capacity.capacity_kw) : null;
      result.power.push({ rackId: rack.id, loadKW, capacityKW, remainingKW: loadKW !== null && capacityKW !== null ? round(capacityKW-loadKW) : null });
      if (loadKW === null) result.issues.push(`${rack.name}：${powers.filter(p => !p?.power_kw).length} 台设备缺少规划功率`);
      if (capacityKW === null) result.issues.push(`${rack.name}：容量待补充`);
      if (loadKW !== null && capacityKW !== null && loadKW > capacityKW) result.issues.push(`${rack.name}：规划负荷超过容量`);
    }
    result.status = !result.power.length ? 'needs_input' : result.issues.length ? 'partial' : 'ready';
    return result;
  }
  const rule = rowsFor(s,'cabling_rules')[0];
  if (!rule?.code || !rule.version || rule.strategy !== 'shortest_path' || !rule.source || !rule.cable_spec || rule.slack_m === '' || rule.slack_percent === '') {
    result.issues.push('先明确路径规则、规则依据、线缆规格及余量参数。');
    return result;
  }
  const specs = rowsFor(s,'cable_specs').filter(r => r.code === rule.cable_spec && Number(r.length_m) > 0);
  if (!specs.length) { result.issues.push('当前规则没有匹配的标准长度规格。'); return result; }
  const nodes = new Set(rowsFor(s,'route_nodes').map(r => r.code));
  const edges = rowsFor(s,'route_edges').filter(e => nodes.has(e.from) && nodes.has(e.to));
  const access = rowsFor(s,'rack_access');
  const missingRacks = new Set<string>();
  for (const link of s.connections) {
    const aPort = s.ports.find(p => p.id === link.a), bPort = s.ports.find(p => p.id === link.b);
    const a = s.devices.find(d => d.id === aPort?.deviceId), b = s.devices.find(d => d.id === bPort?.deviceId);
    if (!a || !b) { result.issues.push(`${link.id}：端点设备缺失`); continue; }
    const start = access.find(r => r.rack_id === a.rackId), end = access.find(r => r.rack_id === b.rackId);
    if (!start?.node || !nodes.has(start.node) || start.lead_m === '') missingRacks.add(a.rackId);
    if (!end?.node || !nodes.has(end.node) || end.lead_m === '') missingRacks.add(b.rackId);
    if (!start || !end || !nodes.has(start.node) || !nodes.has(end.node) || start.lead_m === '' || end.lead_m === '') continue;
    const path = shortestPath(edges, start.node, end.node);
    if (!path) { result.issues.push(`${a.name} → ${b.name}：没有允许的连通路径`); continue; }
    const leadM = Number(start.lead_m) + Number(end.lead_m);
    const slackM = round(Number(rule.slack_m) + (path.distance + leadM) * Number(rule.slack_percent) / 100);
    const calculatedM = round(path.distance + leadM + slackM);
    const stockM = specs.filter(r => r.medium && r.medium === link.medium).map(r => Number(r.length_m)).sort((a,b) => a-b).find(n => n >= calculatedM) ?? null;
    const row: CalculationRow = { connectionId: link.id, from: a.name, to: b.name, route: path.nodes, pathM: round(path.distance), leadM, slackM, calculatedM, stockM };
    result.rows.push(row);
    if (stockM === null) result.issues.push(`${link.id}：没有满足 ${calculatedM}m 的同规格标准长度`);
  }
  if (missingRacks.size) result.issues.push(`${[...missingRacks].join('、')}：接入节点或引线长度待补充`);
  result.status = !result.rows.length ? 'needs_input' : result.issues.length ? 'partial' : 'ready';
  return result;
}
