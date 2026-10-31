import {useState} from 'react';
import {ArrowDownUp, UnfoldHorizontal} from 'lucide-react';
import {api, data, devices, racks, name} from '../model';
import {Button, Tabs, SearchBox, Empty} from '../shared';
import {AssetRow, FigureHead, GoTo, Legend, MemberLedger, ObjectPanel, PodSelect, complete, incident, keyActivate, podOf, shortPod, sw, type ViewProps} from '../visual';
import type {InfraCable} from '@/lib/api/infrastructure';

type FlowNode = {id: string; label: string; sub: string; x: number; y: number; kind: string};
type Bundle = {id: string; from: string; to: string; members: InfraCable[]};
export default function Network({params, set, go}: ViewProps) {
  const pod = params.get('pod') || 'demo-pod-1', mode = params.get('mode') || 'Fabric Flow';
  const selected = params.get('asset') || '', bundleId = params.get('bundle') || '', q = params.get('q') || '';
  const [expanded, setExpanded] = useState(false), [biPage, setBiPage] = useState(0);
  const leaves = sw.filter(d => podOf(d.id) === pod && d.fields.role === 'leaf');
  const spines = sw.filter(d => d.fields.role === 'spine');
  const pr = racks.filter(r => r.fields.pod_id === pod);
  const roles = [...spines, ...leaves];
  const nodes: FlowNode[] = [
    ...spines.map((s, i) => ({id: s.id, label: s.name, sub: s.fields.model, x: 30, y: 74 + i * 112, kind: 'spine'})),
    ...leaves.map((s, i) => ({id: s.id, label: s.name, sub: api.ports(s.id).length + ' 个登记端口', x: 407, y: 74 + i * 112, kind: 'leaf'})),
    ...pr.map((r, i) => ({id: r.id, label: r.name, sub: devices.filter(d => d.type === 'server' && d.fields.rack_id === r.id).length + ' Servers / 端口分组', x: 783, y: 74 + i * 112, kind: 'group'})),
  ];
  const bundles: Bundle[] = [];
  for (const l of leaves) {
    for (const s of spines) {
      const members = data.cables.filter(c => complete(c) && incident(c, l.id) && incident(c, s.id));
      if (members.length) bundles.push({id: s.id + '|' + l.id, from: s.id, to: l.id, members});
    }
    for (const r of pr) {
      const members = data.cables.filter(c => complete(c) && incident(c, l.id) && [c.device_a, c.device_b].some(id => api.asset(id)?.type === 'server' && api.asset(id)?.fields.rack_id === r.id));
      if (members.length) bundles.push({id: l.id + '|' + r.id, from: l.id, to: r.id, members});
    }
  }
  const active = bundles.find(b => b.id === bundleId);
  const nodeMap = new Map(nodes.map(n => [n.id, n]));
  const memberIds = new Set(bundles.flatMap(b => b.members.map(c => c.id)));
  const missing = data.cables.filter(c => !complete(c) && [c.device_a, c.device_b].some(id => podOf(id) === pod));
  const focused = (b: Bundle) => !active && !q || b.id === active?.id || (!!q && [b.from, b.to].some(id => name(id).toLowerCase().includes(q.toLowerCase())));
  const bi = active || bundles.find(b => b.to.startsWith('rack-'));
  const currentPage = Math.min(biPage, Math.max(0, Math.ceil((bi?.members.length || 0) / 12) - 1));
  const biVisible = bi?.members.slice(currentPage * 12, currentPage * 12 + 12) || [];
  function pick(id: string) { set({bundle: id, asset: ''}); setExpanded(false); setBiPage(0); }
  return <div className="viz-v3">
    <div className="v-command"><span className="v-fabric-name"><i/>RoCE Fabric</span><PodSelect value={pod} all={false} onChange={pod => {set({pod, bundle: '', asset: ''}); setExpanded(false);}}/><SearchBox value={q} onChange={q => set({q})} placeholder="高亮交换机或机柜"/><span className="spacer"/><span className="v-outline-label">Ethernet · 规划物理互联</span></div>
    <div className="v-viewline"><Tabs items={['Fabric Flow', 'Bipartite', '交换机目录']} value={mode} onChange={mode => set({mode})}/><span className="spacer"/><span>{memberIds.size} 条完整记录 → {bundles.length} 束</span></div>
    <div className={'v-workspace ' + (selected ? 'with-detail' : '')}>
      <div className="v-main-panel">
        <FigureHead title={mode === 'Fabric Flow' ? shortPod(pod) + ' / 分层物理互联' : mode === 'Bipartite' ? 'Bipartite / 精确端点对照' : '交换机目录'}>
          {expanded && active && active.members.length > 24 && <span>画布展开 24 / {active.members.length} 条，余下见成员表</span>}{active && <><Button onClick={() => set({bundle: ''})}>清除选中束</Button>{mode === 'Fabric Flow' && <Button aria-pressed={expanded} onClick={() => setExpanded(!expanded)}><UnfoldHorizontal size={14}/>{expanded ? '收拢成员' : '展开成员'}</Button>}</>}
        </FigureHead>
        {mode === 'Fabric Flow' ? <div className="v-flow-canvas"><svg viewBox="0 0 1000 540" aria-label="Fabric Flow 束线图" role="img">
          <text x="30" y="28" className="column-title">01 / SPINE</text><text x="407" y="28" className="column-title">02 / LEAF</text><text x="783" y="28" className="column-title">03 / COMPUTE GROUP</text>
          {[230, 605].map(x => <rect key={x} x={x} y="48" width="150" height="462" rx="16" className="routing-channel"/>)}
          {bundles.map(b => {
            const a = nodeMap.get(b.from)!, z = nodeMap.get(b.to)!, x1 = a.x + 184, x2 = z.x, y1 = a.y + 30, y2 = z.y + 30;
            const path = `M${x1},${y1} C${x1 + 88},${y1} ${x2 - 88},${y2} ${x2},${y2}`;
            const chosen = active?.id === b.id, dim = !focused(b);
            return <g key={b.id} className={dim ? 'dimmed' : ''}>
              <path d={path} className={'flow-edge ' + (chosen ? 'selected' : '')} style={{strokeWidth: chosen ? 5 : Math.min(10, 2 + Math.sqrt(b.members.length))}}/>
              <g className="flow-hit" tabIndex={0} role="button" aria-label={name(b.from) + ' 到 ' + name(b.to) + ' ' + b.members.length + ' 条'} onClick={() => pick(b.id)} onKeyDown={keyActivate(() => pick(b.id))}><path d={path} className="flow-target"/><rect x={(x1+x2)/2-22} y={(y1+y2)/2-12} width="44" height="24" fill="transparent"/></g>
              {chosen && expanded && b.members.slice(0, 24).map((c, j) => <path key={c.id} d={`M${x1},${y1 + (j - (Math.min(24, b.members.length) - 1) / 2) * 3} C${x1 + 75},${y1 + (j - (Math.min(24, b.members.length) - 1) / 2) * 6} ${x2 - 75},${y2 + (j - (Math.min(24, b.members.length) - 1) / 2) * 6} ${x2},${y2 + (j - (Math.min(24, b.members.length) - 1) / 2) * 3}`} className="flow-strand" tabIndex={0} role="button" aria-label={'线缆 ' + c.id} onClick={() => set({asset: c.id})} onKeyDown={keyActivate(() => set({asset: c.id}))}><title>{c.id}</title></path>)}
              {(b.to.startsWith('rack-') || chosen) && <g className="bundle-label" pointerEvents="none"><rect x={(x1 + x2) / 2 - 17} y={(y1 + y2) / 2 - 10} width="34" height="20" rx="6"/><text x={(x1 + x2) / 2} y={(y1 + y2) / 2 + 4} textAnchor="middle">{b.members.length}</text></g>}
            </g>;
          })}
          {nodes.map(n => <g key={n.id} role="button" tabIndex={0} aria-label={'查看 ' + n.label} className={'flow-node ' + (selected === n.id ? 'selected' : '')} onClick={() => n.kind === 'group' ? go('physical-cabling', {pod, q: '', row: devices.find(d => d.fields.rack_id === n.id && d.type === 'server')?.id || ''}) : set({asset: n.id})} onKeyDown={keyActivate(() => set({asset: n.id}))}>
            <rect x={n.x} y={n.y} width="184" height="62" rx="7"/>
            <rect x={n.x + 12} y={n.y + 14} width="5" height="31" rx="2" className={'role-bar ' + n.kind}/>
            <text x={n.x + 28} y={n.y + 26} className="node-title">{n.label}</text><text x={n.x + 28} y={n.y + 45} className="svg-small">{n.sub}</text>
          </g>)}
        </svg></div> : mode === 'Bipartite' ? <>
          <div className="v-inline-control"><label>端点集合 <select aria-label="二分图连接束" value={bi?.id || ''} onChange={e => pick(e.target.value)}>{bundles.map(b => <option key={b.id} value={b.id}>{name(b.from)} ↔ {name(b.to)} / {b.members.length}</option>)}</select></label><span>仅显示 Cable 原始双端</span><span className="spacer"/><Button disabled={!currentPage} onClick={() => setBiPage(currentPage - 1)}>上一组</Button><span>{currentPage + 1}/{Math.max(1, Math.ceil((bi?.members.length || 0) / 12))}</span><Button disabled={(currentPage + 1) * 12 >= (bi?.members.length || 0)} onClick={() => setBiPage(currentPage + 1)}>下一组</Button></div>
          {bi ? <svg className="v-bipartite" viewBox={`0 0 1000 ${Math.max(280, biVisible.length * 48 + 64)}`} role="img" aria-label="端口二分图">
            <text x="32" y="26" className="column-title">A / 设备与端口</text><text x="720" y="26" className="column-title">B / 设备与端口</text>
            {biVisible.map((c, i) => { const y = 65 + i * 48; return <g key={c.id} role="button" tabIndex={0} className="bipartite-row" aria-label={'查看线缆 ' + c.id} onClick={() => set({asset: c.id})} onKeyDown={keyActivate(() => set({asset: c.id}))}>
              <rect x="24" y={y - 21} width="260" height="36" rx="5"/><rect x="716" y={y - 21} width="260" height="36" rx="5"/>
              <line x1="284" x2="716" y1={y - 3} y2={y - 3}/><circle cx="284" cy={y - 3} r="4"/><circle cx="716" cy={y - 3} r="4"/>
              <text x="36" y={y + 2} className="svg-label">{name(c.device_a)} / {name(c.a)}</text><text x="728" y={y + 2} className="svg-label">{name(c.device_b)} / {name(c.b)}</text><text x="500" y={y - 10} textAnchor="middle" className="svg-small">{c.id}</text>
            </g>;})}
          </svg> : <Empty>没有端点完整的连接束。</Empty>}
        </> : <div className="v-directory">{roles.filter(d => d.name.toLowerCase().includes(q.toLowerCase())).map(d => <AssetRow key={d.id} a={d} onClick={() => set({asset: d.id})}/>)}</div>}
        <Legend/>
        {missing.length > 0 && <button className="v-exception-line" onClick={() => go('physical-cabling', {pod, state: 'incomplete', mode: 'Cable Inventory'})}>! {missing.length} 条缺端记录未进入束线，去核查 <ArrowDownUp size={13}/></button>}
        {(active || mode === 'Bipartite') && <MemberLedger rows={(active || bi)?.members || []} title="选中连接束 / 原始线缆" onSelect={asset => set({asset})}/>}
      </div>
      <ObjectPanel id={selected} go={go} close={() => set({asset: ''})}/>
    </div>
    <div className="v-statusline"><span>束线按明确角色 / 端点分组；交叉处不构成连接</span><span className="spacer"/><GoTo go={go} page="physical-cabling" params={{pod}}>精确核线矩阵</GoTo></div>
  </div>;
}
