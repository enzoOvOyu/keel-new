import {useState} from 'react';
import {ArrowRight, ChevronDown, ChevronRight, GitBranch, Play} from 'lucide-react';
import {useTopologyModel} from '../model';
import {Button, Badge, Empty} from '../shared';
import {FigureHead,GoTo,MemberLedger,ObjectPanel,keyActivate,type ViewProps,useVisualModel,shortLabel,QueryPager} from '../visual';

export default function Trace({params, set, go}: ViewProps) {
  const {api,data,devices,name,status,queries}=useTopologyModel();
  const {complete}=useVisualModel();
  const port=params.has('port')?params.get('port')!:params.get('asset')?api.ports(params.get('asset')!)[0]?.id||'':data.ports[0]?.id||'';
  const portAsset=api.asset(port),validPort=portAsset?.type==='port';
  const owner = validPort?portAsset.fields.owner_device_id||'':params.get('asset')||'';
  const mode = params.get('mode') || 'Physical Trace', run = params.get('run') !== '0';
  const selected = params.get('detail') || '';
  const candidates = queries.trace?.segments||[];
  const requested = candidates.find(c => c.id === params.get('cable'));
  const invalidCable=!!params.get('cable')&&!requested;
  const cable = params.get('cable')?requested:(candidates.length === 1 ? candidates[0] : undefined);
  const rawEnd = cable ? (cable.a === port ? cable.b : cable.a) : '';
  const end = api.asset(rawEnd)?.type==='port'?rawEnd:'';
  const endOwner = api.asset(end)?.fields.owner_device_id;
  const [expanded, setExpanded] = useState(true);
  const local = (queries.connections?.cables||[]).filter(c=>c.device_a===owner||c.device_b===owner);
  const logical = mode === 'Logical Trace';
  function start(p: string, c = '') { set({port: p, cable: c, run: '1', detail: ''}); }
  const station = (x: number, title: string, subtitle: string, id: string, kind: string) => <g className={'metro-station ' + (!id ? 'missing' : '')} tabIndex={0} role="button" aria-label={title + ' ' + subtitle} onClick={() => set({detail: id || cable?.id || port})} onKeyDown={keyActivate(() => set({detail: id || cable?.id || port}))}>
    <rect x={x - 130} y="59" width="260" height="155" fill="transparent"/>
    <circle className="station-ring" cx={x} cy="134" r={kind === 'cable' ? 10 : 14}/>{id && <circle cx={x} cy="134" r="4" className="station-core"/>}
    <text x={x} y="83" textAnchor="middle" className="column-title">{title}</text><text x={x} y="183" textAnchor="middle" className="node-title" style={{pointerEvents:'auto'}}><title>{subtitle} · {id}</title>{shortLabel(subtitle,30)}</text><text x={x} y="204" textAnchor="middle" className="svg-small">{id ? kind === 'cable' ? cable?.fields.cable_type + ' · ' + (cable?.fields.length_m ?? '未知') + ' m' : name(api.asset(id)?.fields.owner_device_id) : '未登记 · 停止追踪'}</text>
  </g>;
  return <div className="viz-v3">
    <div className="v-command"><select aria-label="追踪模式" value={mode} onChange={e => set({mode: e.target.value, detail: ''})}><option>Physical Trace</option><option>Logical Trace</option></select><select aria-label="起始设备" value={owner} onChange={e => set({asset:e.target.value,port: api.ports(e.target.value)[0]?.id || '', cable: '', run: '0', detail: ''})}>{(!owner||!devices.some(d=>d.id===owner))&&<option value={owner}>{owner?'目标设备不可用':'请选择设备'}</option>}{devices.map(d => <option key={d.id} value={d.id}>{d.name}</option>)}</select><select aria-label="起始端口" value={port} onChange={e => set({port: e.target.value, cable: '', run: '0', detail: ''})}>{(!port||!validPort)&&<option value={port}>{port?'目标端口不可用':'请选择端口'}</option>}{api.ports(owner).map(p => <option key={p.id} value={p.id}>{p.name}</option>)}</select><Button variant="default" disabled={logical || !validPort} onClick={() => start(port, params.get('cable') || '')}><Play size={13}/>开始追踪</Button><Button onClick={() => set({run: '0', detail: '', cable: ''})}>清空</Button><span className="spacer"/><span className="v-outline-label">按端口追踪 · 不穿越交换芯片</span></div>
    <div className={'v-workspace ' + (selected ? 'with-detail' : '')}>
      <div className="v-main-panel">
        {!logical&&queries.loading&&!queries.trace?<Empty>正在读取端口与路径证据…</Empty>:!logical&&(!validPort||invalidCable)?<Empty>{invalidCable?'所选线缆不存在、未加载、无权访问或不属于当前端口；未自动替换为其他连接。':'目标端口不存在、未加载或无权访问；请选择有效端口。'}<Button onClick={()=>set({port:'',cable:'',run:'0'})}>重新选择</Button></Empty>:logical ? <div className="v-capability"><GitBranch size={24}/><div><h2>逻辑路径：缺少转发证据</h2><p>需要同一时间窗口的 VRF、FIB、方向及目标地址。当前物理记录不能替代它们。</p><GoTo go={go} page="network-architecture">查看已有网络架构</GoTo></div><Badge value="unsupported"/></div> : !run ? <Empty><h2>选择端口后开始追踪</h2><p>只查询已登记端接，不执行设备操作。</p></Empty> : <>
          <FigureHead title="Metro Route / 物理段">{queries.trace&&<span>{queries.trace.integrity} / {queries.trace.verification}</span>}<span>{cable ? '1 个已记录物理段' : '待定位物理段'}</span><span className="spacer"/>{cable && <><span className={'v-integrity ' + (!complete(cable) ? 'issue' : '')}>{complete(cable) ? '记录完整' : '记录有缺口'}</span><Badge value={status(cable)}/></>}</FigureHead>
          {candidates.length > 1 && !requested ? <div className="v-branch-choice"><h3>同一端口存在 {candidates.length} 条记录，请选择要核查的分支。</h3><MemberLedger rows={candidates} onSelect={cable => set({cable})}/></div> : !cable ? <Empty>该端口没有线缆记录。不能确认路径，也不能据此判为故障。</Empty> : <>
            <div className="v-metro-wrap"><svg viewBox="0 0 1000 260" role="img" aria-label="Metro Route 端口线缆路径">
              <path d="M150 134 H500" className="metro-track"/><path d="M500 134 H850" className={'metro-track ' + (!end ? 'missing' : '')}/>
              <path d="M335 128 l8 6 -8 6 M660 128 l8 6 -8 6" className="metro-arrow"/>
              {station(150, '01 / 起始端口', name(port), port, 'port')}{station(500, '02 / 物理线缆', cable.id, cable.id, 'cable')}{station(850, '03 / 对端端口', end ? name(end) : rawEnd?'端口引用不可用':'对端缺失', end, 'port')}
              {!end && <text x="850" y="140" textAnchor="middle" className="break-mark">?</text>}
              <text x="500" y="239" textAnchor="middle" className="svg-small">{end ? '物理段在交换机端口终止；没有贯通或转发证据，不继续绘制下一跳。' : '缺端记录保留原身份，不自动补齐设备、端口或中间段。'}</text>
            </svg></div>
            <div className="v-trace-context"><span><b>{name(owner)}</b> / {name(api.asset(owner)?.fields.rack_id)}</span><ArrowRight size={14}/><span>{end ? <><b>{name(endOwner)}</b> / {name(api.asset(endOwner)?.fields.rack_id)}</> : '待核查终点'}</span><span className="spacer"/><GoTo go={go} page="physical-cabling" params={{cable: cable.id, asset: owner}}>定位布线记录</GoTo></div>
            <div className="v-trace-bottom">
              <section className="v-tree"><FigureHead title="Tree / 设备关联支路"><button aria-label="折叠关联支路" onClick={() => setExpanded(!expanded)}>{expanded ? <ChevronDown size={15}/> : <ChevronRight size={15}/>}</button></FigureHead><div className="tree-root"><GitBranch size={15}/><b>{name(owner)}</b><small>{local.length} 条独立端接</small></div>
                {expanded && local.map(c => { const p = c.device_a === owner ? c.a : c.b, target = c.device_a === owner ? c.device_b : c.device_a; return <button key={c.id} className={'tree-branch ' + (c.id === cable.id ? 'selected' : '')} onClick={() => start(p, c.id)}><span className="tree-elbow"/><strong>{name(p)}</strong><span className="spacer"/><small>{name(target)}</small>{!complete(c) && <span className="issue-dot">!</span>}</button>; })}
                <QueryPager kind="connections" set={set}/><div className="v-tree-note">关联支路不等于备用路径。冗余与故障影响需要额外证据。</div>
              </section>
              <section className="v-evidence-table"><FigureHead title="Table / 路径证据"><span>规划 / 观测 / 验收分层</span></FigureHead><table><thead><tr><th>检查对象</th><th>登记事实</th><th>当前观测 / 判定</th></tr></thead><tbody>
                <tr><td>物理双端</td><td><button className="text-link" onClick={() => set({detail: cable.id})}>{cable.id}</button></td><td>{complete(cable) ? '完整 / 尚未验证' : '缺端 / 无法验证'}</td></tr>
                <tr><td>起始端引用</td><td><button className="text-link" onClick={() => set({detail: port})}>{port}</button></td><td>当前双端 LLDP 未提供</td></tr>
                <tr><td>对端引用</td><td>{end || '缺失'}</td><td>{end ? '端口存在 / 运行状态待采集' : '停止追踪'}</td></tr>
                <tr><td>介质 / 长度</td><td>{cable.fields.cable_type} / {cable.fields.length_m ?? '未知'} m</td><td>{cable.fact||'事实层未登记'} · 非实测衰减</td></tr>
                <tr><td>配置 / 转发</td><td>本次物理追踪不推导</td><td>FIB / VRF 证据缺失</td></tr>
                <tr><td>冗余 / 影响</td><td>仅直接关联的双端设备</td><td>未证明业务影响与高可用</td></tr>
              </tbody></table><div className="v-source-strip">{cable.source||'来源未登记'} · {cable.fields.source_ref||'来源引用未登记'} · {cable.fields.observed_at||'采集时间未提供'} · {cable.fact||'事实层未登记'}</div></section>
            </div>
          </>}
        </>}
      </div>
      <ObjectPanel id={selected} go={go} close={() => set({detail: ''})}/>
    </div>
    <div className="v-statusline"><span>路线箭头仅表示阅读顺序，非流量方向；关联支路不等于备用路径。</span></div>
  </div>;
}
