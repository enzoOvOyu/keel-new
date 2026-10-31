import {ChevronRight, Layers, Maximize2, Network, Server} from 'lucide-react';
import {api, data, devices, racks, name} from '../model';
import {Button, SearchBox, Empty} from '../shared';
import {AssetRow, ConnectionMatrix, FigureHead, GoTo, Legend, MemberLedger, ObjectPanel, between, complete, incident, keyActivate, podIds, podOf, shortPod, sw, type ViewProps} from '../visual';

export default function Global({params, set, go}: ViewProps) {
  const scope = params.get('scope') || '', q = params.get('q') || '', selected = params.get('asset') || '';
  const bundle = params.get('bundle') || '';
  const cell = params.get('row') && params.get('col') ? [params.get('row')!, params.get('col')!] : [];
  const setBundle = (bundle: string) => set({bundle});
  const setCell = (ids: string[]) => set({row: ids[0] || '', col: ids[1] || ''});
  const pods = podIds.includes(scope) ? [scope] : podIds;
  const results = q ? devices.filter(d => [d.id, d.name, d.fields.serial].join(' ').toLowerCase().includes(q.toLowerCase())).slice(0, 10) : [];
  const scopeDevices = devices.filter(d => !scope || podOf(d.id) === scope);
  const spines = sw.filter(s => s.fields.role === 'spine');
  const upstream = (pod: string) => data.cables.filter(c => complete(c) && [c.device_a, c.device_b].some(id => spines.some(s => s.id === id)) && [c.device_a, c.device_b].some(id => podOf(id) === pod && api.asset(id)?.fields.role === 'leaf'));
  const leaves = sw.filter(s => podOf(s.id) === scope && s.fields.role === 'leaf');
  function selectPod(pod: string) { set({scope: pod, asset: '', q: '', bundle: '', row: '', col: ''}); }
  return <div className="viz-v3">
    <div className="v-command"><div className="v-breadcrumb"><button onClick={() => selectPod('')}>Demo DC</button><ChevronRight size={13}/><span>4 Pod</span>{scope && <><ChevronRight size={13}/><b>{shortPod(scope)}</b></>}</div><span className="spacer"/><SearchBox value={q} onChange={q => set({q})} placeholder="搜索服务器、交换机或 SN"/>{scope && <Button onClick={() => selectPod('')}><Maximize2 size={14}/>全局范围</Button>}</div>
    {q && <div className="v-search-results">{results.length ? results.map(d => <button key={d.id} onClick={() => set({asset: d.id, scope: String(podOf(d.id)), q: ''})}>{d.type === 'server' ? <Server size={14}/> : <Network size={14}/>}<b>{d.name}</b><small>{name(d.fields.rack_id)} · U{d.fields.rack_u}</small></button>) : <Empty>没有匹配资产</Empty>}</div>}
    <div className={'v-workspace ' + (selected ? 'with-detail' : '')}>
      <div className="v-main-panel">
        <FigureHead title={scope ? shortPod(scope) + ' / 资产与连接' : 'Cluster Map / 数据中心结构'}><span className="v-mini-stat">{scopeDevices.filter(d => d.type === 'server').length} Servers</span><span className="v-mini-stat">{scopeDevices.filter(d => d.type === 'switch').length} Switches</span><span className="v-outline-label">位置分组 · 非地理坐标</span></FigureHead>
        {!scope ? <div className="v-cluster-canvas"><svg viewBox="0 0 1040 578" role="img" aria-label="Pod 聚合图，内部机柜到 Leaf 连接矩阵">
          <rect x="350" y="267" width="340" height="44" rx="22" className="core-bar"/>
          <text x="520" y="286" textAnchor="middle" className="core-title">RoCE / SPINE BACKBONE</text>
          <text x="520" y="301" textAnchor="middle" className="core-sub">{spines.length} 个 Spine · 规划记录 / 未验证</text>
          {pods.map((pod, i) => {
            const x = i % 2 ? 550 : 30, y = i < 2 ? 16 : 338, pr = racks.filter(r => r.fields.pod_id === pod);
            const members = devices.filter(d => podOf(d.id) === pod), ls = members.filter(d => d.fields.role === 'leaf');
            const links = upstream(pod), down = i > 1, sx = x + 230;
            return <g key={pod}>
              <path d={`M${sx},${down ? y : y + 224} V${down ? 324 : 254} H520 V${down ? 311 : 267}`} className={'cluster-link ' + (bundle === pod ? 'chosen' : '')} role="button" tabIndex={0} aria-label={shortPod(pod) + ' 上联 ' + links.length + ' 条'} onClick={() => setBundle(pod)} onKeyDown={keyActivate(() => setBundle(pod))}/>
              <rect x={sx - 25} y={down ? 315 : 246} width="50" height="19" rx="8" fill="#f7f8fc"/>
              <text x={sx} y={down ? 328 : 259} textAnchor="middle" className="bundle-count">{links.length} 条</text>
              <rect x={x} y={y} width="460" height="224" rx="9" className="cluster-container"/>
              <g className="svg-button" role="button" tabIndex={0} aria-label={'展开 ' + shortPod(pod)} onClick={() => selectPod(pod)} onKeyDown={keyActivate(() => selectPod(pod))}>
                <rect x={x + 1} y={y + 1} width="458" height="57" rx="8" fill="transparent"/>
                <text x={x + 18} y={y + 29} className="cluster-title">{shortPod(pod)}</text><text x={x + 443} y={y + 28} textAnchor="end" className="svg-link">展开 ↗</text>
                <text x={x + 18} y={y + 47} className="svg-muted">{members.filter(d => d.type === 'server').length} Servers · {members.filter(d => d.type === 'switch').length} Switches · {pr.length} Racks</text>
              </g>
              <text x={x + 18} y={y + 72} className="svg-small">机柜 / 服务器</text><text x={x + 287} y={y + 72} className="svg-small">→ Leaf 端接矩阵</text>
              {ls.map((l, k) => <text key={l.id} x={x + 295 + k * 40} y={y + 91} className="svg-small" textAnchor="middle">{l.name.replace('leaf-', 'L')}</text>)}
              {pr.map((r, j) => {
                const ds = members.filter(d => d.fields.rack_id === r.id && d.type === 'server');
                return <g key={r.id}>
                  <text x={x + 18} y={y + 113 + j * 27} className="svg-label">{r.name}</text>
                  {ds.map((d, k) => <g key={d.id} role="button" tabIndex={0} aria-label={'定位 ' + d.name} className="svg-button" onClick={() => set({asset: d.id})} onKeyDown={keyActivate(() => set({asset: d.id}))}>
                    <rect x={x + 115 + k * 30} y={y + 101 + j * 27} width="23" height="16" rx="3" className={selected === d.id ? 'mini-server selected' : 'mini-server'}/><title>{d.name} · {d.fields.gpu_count} GPU</title>
                  </g>)}
                  {ls.map((l, k) => {
                    const count = data.cables.filter(c => complete(c) && incident(c, l.id) && ds.some(d => incident(c, d.id))).length;
                    return <g key={l.id} role="button" tabIndex={0} aria-label={r.name + ' 到 ' + l.name + ' ' + count + ' 条'} className="svg-button" onClick={() => {set({scope: pod, row: r.id, col: l.id, asset: '', bundle: ''});}} onKeyDown={keyActivate(() => {set({scope: pod, row: r.id, col: l.id, asset: '', bundle: ''});})}>
                      <rect x={x + 281 + k * 40} y={y + 99 + j * 27} width="29" height="21" rx="3" className={count ? 'mini-cell has-record' : 'mini-cell'}/><text x={x + 295 + k * 40} y={y + 114 + j * 27} textAnchor="middle" className="svg-small">{count || '·'}</text>
                    </g>;
                  })}
                </g>;
              })}
              <text x={x + 18} y={y + 213} className="svg-small">□ 服务器 · 单元格为已登记完整线缆数</text>
            </g>;
          })}
        </svg></div> : <div className="v-pod-content">
          <div className="v-local-assets"><FigureHead title="服务器 / 交换机"/>{scopeDevices.map(d => <AssetRow key={d.id} a={d} onClick={() => set({asset: d.id})}/>)}</div>
          <section><FigureHead title="局部邻接矩阵"><GoTo go={go} page="network-architecture" params={{pod: scope}}>展开网络架构</GoTo></FigureHead>
            <ConnectionMatrix rows={scopeDevices.filter(d => d.type === 'server')} cols={leaves} cables={data.cables} selected={cell.join(':')} onCell={(a, b) => setCell([a, b])}/>
          </section>
        </div>}
        <Legend/>
        {bundle && <MemberLedger rows={upstream(bundle)} title={shortPod(bundle) + ' 上联束 / 原始成员'} selected={selected} onSelect={asset => set({asset})} onClose={() => setBundle('')}/>}
        {scope && cell.length > 0 && <MemberLedger rows={cell[0].startsWith('rack-') ? data.cables.filter(c => complete(c) && incident(c, cell[1]) && [c.device_a, c.device_b].some(id => podOf(id) === scope && api.asset(id)?.fields.rack_id === cell[0] && api.asset(id)?.type === 'server')) : between(data.cables, cell[0], cell[1])} title={name(cell[0]) + ' → ' + name(cell[1])} onSelect={asset => set({asset})} onClose={() => setCell([])}/>}
      </div>
      <ObjectPanel id={selected} go={go} close={() => set({asset: ''})}/>
    </div>
    <div className="v-statusline"><Layers size={13}/><span>位置层级 × 物理关联 · 聚合不生成新资产</span><span className="spacer"/><span>{data.cables.length} 条规划记录 · 当前运行健康未知</span></div>
  </div>;
}
