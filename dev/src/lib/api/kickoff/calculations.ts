import type { Calculation, CalculationRow, ModelRow, Workspace } from './types';

export function rowsFor(s: Workspace, id: string): ModelRow[] {
  return s.models.find(m => m.tableId === id)?.rows ?? [];
}
export function calculationDigest(s: Workspace, domain: 'cabling' | 'power') {
 const ids=domain==='cabling'?['route_nodes','route_edges','rack_access','cable_specs','cabling_rules','placements']:['power_profiles','rack_capacity','placements'];
 const inputs=s.models.filter(m=>ids.includes(m.tableId)).map(m=>[m.tableId,m.version]).sort((a,b)=>String(a[0]).localeCompare(String(b[0])));
 const devices=s.devices.map(d=>[d.id,d.rackId]).sort((a,b)=>a[0].localeCompare(b[0]));
 const links=domain==='cabling'?s.connections.map(c=>[c.id,c.a,c.b,c.medium]).sort((a,b)=>a[0].localeCompare(b[0])):[];
 return JSON.stringify([inputs,devices,links]);
}
