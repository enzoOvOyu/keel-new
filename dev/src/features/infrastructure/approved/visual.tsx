import {useEffect, useState} from 'react';
import {ChevronRight, X, ArrowUpRight} from 'lucide-react';
import {useTopologyModel,type Go} from './model';
import {Button, Badge, Detail, Empty} from './shared';
import type {InfraAsset, InfraCable} from '@/lib/api/infrastructure';

export type ViewProps = {params: URLSearchParams; set: (p: Record<string, string>) => void; go: Go};
export function useVisualModel(){
 const {api,data,devices,racks,name,status,queries}=useTopologyModel();
const podIds = [...new Set([...racks.map(r => String(r.fields.pod_id||'__ungrouped__')),...(queries.groups||[]).filter(g=>g.type==='pod').map(g=>g.id),...devices.filter(d=>!racks.some(r=>r.id===d.fields.rack_id)).map(()=>'__ungrouped__')])];
const assetIndex = new Map([...data.assets, ...data.ports].map(a => [a.id, a]));
const podOf = (id: string) => {
  const a = assetIndex.get(id);
  return a?.type === 'rack' ? a.fields.pod_id||'__ungrouped__' : assetIndex.get(a?.fields.rack_id)?.fields.pod_id||'__ungrouped__';
};
const shortPod = (id: string) => id==='__ungrouped__'?'未分组机柜':queries.groups?.find(g=>g.id===id)?.name||name(id);
const complete = (c: InfraCable) => !!(assetIndex.get(c.a)?.type==='port' && assetIndex.get(c.b)?.type==='port' && assetIndex.has(c.device_a) && assetIndex.has(c.device_b) && assetIndex.get(c.a)?.fields.owner_device_id===c.device_a && assetIndex.get(c.b)?.fields.owner_device_id===c.device_b);
const incident = (c: InfraCable, id: string) => [c.device_a, c.device_b, c.a, c.b, c.id].includes(id);
const inPod = (c: InfraCable, pod: string) => !pod || [podOf(c.device_a), podOf(c.device_b)].includes(pod);
const cableSearch = (c: InfraCable, q: string) => [c.id, name(c.device_a), name(c.device_b), name(c.a), name(c.b),...[c.device_a,c.device_b,c.a,c.b].flatMap(id=>{const f=api.asset(id)?.fields;return [f?.management_ip,f?.management_address,f?.permanent_mac,f?.mac,f?.serial]})].join(' ').toLowerCase().includes(q.toLowerCase());
const special = (c: InfraCable) => status(c) !== 'unverified';
const peer = (c: InfraCable, id: string) => c.device_a === id ? c.device_b : c.device_a;
const sw = devices.filter(d => d.type === 'switch');
const servers = devices.filter(d => d.type === 'server');
const between = (rows: InfraCable[], a: string, b: string) => rows.filter(c => incident(c, a) && incident(c, b));
return {podIds,assetIndex,podOf,shortPod,complete,incident,inPod,cableSearch,special,peer,sw,servers,between};
}
export const shortLabel=(text:unknown,max=24)=>{const value=String(text??'');return value.length>max?value.slice(0,max-1)+'…':value};
export const keyActivate = (f: () => void) => (e: React.KeyboardEvent) => {
  if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); f(); }
};

export function PodSelect({value, onChange, all = true}: {value: string; onChange: (s: string) => void; all?: boolean}) {
  const {podIds,shortPod}=useVisualModel();
  return <select aria-label="Pod 范围" value={value} onChange={e => onChange(e.target.value)}>
    {all && <option value="">全部 Pod</option>}{value&&!podIds.includes(value)&&<option value={value}>范围不存在或无权访问：{value}</option>}
    {podIds.map(p => <option key={p} value={p}>{shortPod(p)}</option>)}
  </select>;
}
export function FigureHead({title, children}: {title: string; children?: React.ReactNode}) {
  return <div className="v-figure-head"><strong>{title}</strong><span className="spacer"/>{children}</div>;
}
export function Legend() {
  return <div className="v-legend"><span><i className="mark-record"/>物理端接记录</span><span><i className="mark-issue"/>证据缺口 / 冲突</span><span>连线数 ≠ 带宽 · 无记录 ≠ 已确认断线</span></div>;
}
export function ObjectPanel({id, go, close}: {id: string; go: Go; close: () => void}) {
  useEffect(() => {
    if (!id) return;
    const trigger = document.activeElement;
    const key = (e: KeyboardEvent) => { if (e.key === 'Escape') close(); };
    window.addEventListener('keydown', key);
    return () => { window.removeEventListener('keydown', key); if (trigger instanceof HTMLElement && trigger.isConnected) trigger.focus(); };
  }, [id]);
  return id ? <Detail id={id} go={go} onClose={close}/> : null;
}
export function MemberLedger({rows, selected, onSelect, title = '连接明细', onClose}: {
  rows: InfraCable[]; selected?: string; onSelect: (id: string) => void; title?: string; onClose?: () => void;
}) {
  const [page, setPage] = useState(0);
  const current = Math.min(page, Math.max(0, Math.ceil(rows.length / 6) - 1));
  const {name,status}=useTopologyModel();
  return <section className="v-ledger">
    <FigureHead title={title}><span>{rows.length} 条</span>
      {rows.length > 6 && <><Button disabled={!current} onClick={() => setPage(current - 1)}>上一组</Button><span>{current + 1}/{Math.ceil(rows.length / 6)}</span><Button disabled={(current + 1) * 6 >= rows.length} onClick={() => setPage(current + 1)}>下一组</Button></>}
      {onClose && <Button variant="ghost" aria-label="关闭成员明细" onClick={onClose}><X size={14}/></Button>}
    </FigureHead>
    <div className="v-ledger-scroll"><table><thead><tr><th>线缆 ID</th><th>A 端 / 端口</th><th>B 端 / 端口</th><th>介质</th><th>证据</th></tr></thead>
      <tbody>{rows.slice(current * 6, current * 6 + 6).map(c => <tr key={c.id} className={selected === c.id ? 'selected' : ''}>
        <td><button className="text-link" onClick={() => onSelect(c.id)}>{c.id}</button></td>
        <td>{name(c.device_a)} <small>{name(c.a)}</small></td><td>{name(c.device_b)} <small>{name(c.b)}</small></td>
        <td>{c.fields.cable_type || '未知'} · {c.fields.length_m ?? '—'} m</td><td><Badge value={status(c)}/></td>
      </tr>)}</tbody></table>{!rows.length && <Empty>当前范围没有端接记录。</Empty>}</div>
  </section>;
}
export function AssetRow({a, onClick}: {a: InfraAsset; onClick: () => void}) {
  const {name}=useTopologyModel();
  return <button className="v-asset-row" onClick={onClick}><span className={'v-device-dot ' + a.type}/><strong>{a.name}</strong><small>{a.fields.role || a.fields.model}</small><span className="spacer"/><small>{name(a.fields.rack_id)}</small><ChevronRight size={14}/></button>;
}
export function ConnectionMatrix({rows, cols, cables, selected, onCell, rowLabel = '设备 / 对端'}: {
  rows: InfraAsset[]; cols: InfraAsset[]; cables: InfraCable[]; selected: string; onCell: (a: string, b: string) => void; rowLabel?: string;
}) {
  const {complete,special}=useVisualModel();
  const {name,queries}=useTopologyModel();
  const cells = new Map<string, InfraCable[]>();
  const rowSet = new Set(rows.map(a => a.id)), colSet = new Set(cols.map(a => a.id));
  for (const c of cables.filter(complete)) {
    const idA = rowSet.has(c.device_a) && colSet.has(c.device_b) ? c.device_a : c.device_b;
    const idB = idA === c.device_a ? c.device_b : c.device_a;
    const key = idA + ':' + idB;
    if (rowSet.has(idA) && colSet.has(idB)) cells.set(key, [...(cells.get(key) || []), c]);
  }
  return <div className="v-matrix-scroll"><table className="v-matrix"><thead><tr><th>{rowLabel}</th>{cols.map(c => <th key={c.id}><span>{c.name}</span><small>{c.fields.role || c.type}</small></th>)}</tr></thead>
    <tbody>{rows.map(r => <tr key={r.id}><th><strong>{r.name}</strong><small>{name(r.fields.rack_id)}</small></th>{cols.map(c => {
      const key = r.id + ':' + c.id, members = cells.get(key) || [], serverCell=queries.matrix?.cells.find(x=>x.row_id===r.id&&x.column_id===c.id),count=queries.matrix?(serverCell?.count||0):members.length,issue = members.some(special);
      return <td key={c.id}><button aria-label={`${r.name} × ${c.name}：${count} 条记录`} aria-pressed={selected === key}
        className={`v-cell ${count ? 'recorded' : ''} ${issue ? 'attention' : ''} ${selected === key ? 'chosen' : ''}`}
        onClick={() => onCell(r.id, c.id)}>{count || '·'}{issue && <span className="cell-flag">!</span>}</button></td>;
    })}</tr>)}</tbody></table>{!rows.length && <Empty>没有匹配的设备。请调整范围或搜索。</Empty>}</div>;
}
export function QueryPager({kind,set}:{kind:'connections'|'matrix';set:(p:Record<string,string>)=>void}){
 const {queries}=useTopologyModel(),page=queries[kind]?.page;
 if(!page)return null;
 const key=kind==='matrix'?'matrix_cursor':'connection_cursor',hasCursor=new URLSearchParams(window.location.search).has(key);
 if(!page.has_more&&!hasCursor)return null;
 return <div className="pager"><span>当前服务端窗口 · 每批最多 {page.limit} 条{page.has_more?' · 还有后续记录':' · 已到末批'}</span><Button disabled={!hasCursor} onClick={()=>set({[key]:''})}>首批</Button><Button disabled={!page.has_more} onClick={()=>set({[key]:page.next_cursor||''})}>下一批</Button></div>;
}
export function GoTo({go, page, params, children}: {go: Go; page: string; params?: Record<string, string>; children: React.ReactNode}) {
  return <Button onClick={() => go(page, params)}>{children}<ArrowUpRight size={13}/></Button>;
}
