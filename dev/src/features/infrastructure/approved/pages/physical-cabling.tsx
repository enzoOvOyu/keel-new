import {useState} from 'react';
import {Download, Filter, ArrowRight} from 'lucide-react';
import {useTopologyModel,exportFile} from '../model';
import {Button, Tabs, SearchBox, Empty, Badge, CableTable} from '../shared';
import {ConnectionMatrix,FigureHead,Legend,MemberLedger,ObjectPanel,PodSelect,keyActivate,type ViewProps,useVisualModel,shortLabel} from '../visual';

export default function Cabling({params, set, go}: ViewProps) {
  const {api,data,devices,name,status}=useTopologyModel();
  const {between,cableSearch,complete,incident,podOf,servers,sw}=useVisualModel();
  const mode = params.get('mode') || 'Connection Matrix';
  const q = params.get('q') || '', state = params.get('state') || '';
  const selection = params.get('cable') || params.get('port') || '';
  const target = api.cable(params.get('cable') || '');
  const requestedPort=api.asset(params.get('port')||'');
  const asset = params.has('asset')?params.get('asset')!:target?.device_a||requestedPort?.fields.owner_device_id||(!params.has('cable')&&!params.has('port')?devices[0]?.id:'')||'';
  const invalidAsset=!!asset&&!devices.some(d=>d.id===asset),invalidCable=!!params.get('cable')&&!target,invalidPort=!!params.get('port')&&requestedPort?.type!=='port';
  const pod = params.has('pod') ? params.get('pod')! : String(podOf(asset) || '');
  const axis = params.get('axis') || 'servers';
  const row = params.get('row') || '', col = params.get('col') || '';
  const [bioPage, setBioPage] = useState(0);
  const rows = data.cables.filter(c => (!pod || [podOf(c.device_a), podOf(c.device_b)].includes(pod)) && (!state || status(c) === state) && cableSearch(c, q));
  const matrixRows = (axis === 'servers' ? servers : sw.filter(s => s.fields.role === 'leaf')).filter(d => (!pod || podOf(d.id) === pod) && (!q || [d.id,d.name,d.fields.management_ip,d.fields.permanent_mac,d.fields.mac,d.fields.serial,...api.connections(d.id).map(c => [c.id, name(c.device_a), name(c.device_b)].join(' '))].join(' ').toLowerCase().includes(q.toLowerCase())));
  const ordering = params.get('order') || 'rack';
  matrixRows.sort((a, b) => ordering === 'connections' ? api.connections(b.id).length - api.connections(a.id).length || a.id.localeCompare(b.id) : ordering === 'name' ? a.name.localeCompare(b.name) : String(a.fields.rack_id).localeCompare(String(b.fields.rack_id)) || a.id.localeCompare(b.id));
  const matrixCols = sw.filter(s => (axis === 'servers' ? s.fields.role !== 'spine' : s.fields.role === 'spine') && rows.some(c => complete(c) && incident(c, s.id) && matrixRows.some(d => incident(c, d.id))));
  const matrixIds = new Set(rows.filter(c => complete(c) && matrixRows.some(d => incident(c, d.id)) && matrixCols.some(d => incident(c, d.id))).map(c => c.id));
  const incomplete = rows.filter(c => !complete(c));
  const members = row && col ? between(rows.filter(complete), row, col) : [];
  const local = rows.filter(c => incident(c, asset));
  const bioAll = row && col ? members : local;
  const page = Math.min(bioPage, Math.max(0, Math.ceil(bioAll.length / 24) - 1));
  const bio = bioAll.slice(page * 24, page * 24 + 24);
  const portIds = [...new Set(bio.flatMap(c => [c.a, c.b]).filter(id=>api.asset(id)?.type==='port'))].sort((a, b) => {
    const ownerA = api.asset(a)?.fields.owner_device_id, ownerB = api.asset(b)?.fields.owner_device_id;
    return name(ownerA).localeCompare(name(ownerB)) || name(a).localeCompare(name(b), undefined, {numeric: true});
  });
  function pick(id: string) { set({cable: id, port: ''}); }
  function clearCell() { set({row: '', col: ''}); setBioPage(0); }
  const inventory = params.get('coverage') === 'outside' ? rows.filter(c => !matrixIds.has(c.id)) : rows;
  return <div className="viz-v3">{(invalidAsset||invalidCable||invalidPort)&&<div className="v-exception-line" role="status">目标对象不存在、未加载或无权访问；未自动替换为其他设备。<Button onClick={()=>set({asset:'',cable:'',port:''})}>清除定位</Button></div>}
    <div className="v-command"><PodSelect value={pod} onChange={pod => {set({pod, row: '', col: '', cable: '', port: ''}); setBioPage(0);}}/><SearchBox value={q} onChange={q => {set({q}); setBioPage(0);}}/><select aria-label="证据筛选" value={state} onChange={e => {set({state: e.target.value}); setBioPage(0);}}><option value="">全部证据</option><option value="incomplete">端点缺失</option><option value="conflict">证据冲突</option><option value="stale">观测过期</option><option value="unverified">待验证</option></select><span className="spacer"/><Button onClick={() => exportFile('cmdb-connections.csv', 'id,a_port,b_port,evidence\n' + rows.map(c => [c.id, c.a, c.b, status(c)].join(',')).join('\n'))}><Download size={14}/>导出 {rows.length} 条</Button></div>
    <div className="v-viewline"><Tabs items={['Connection Matrix', 'BioFabric', 'Port Panel', 'Cable Inventory']} value={mode} onChange={mode => set({mode})}/><span className="spacer"/><span>{mode === 'Connection Matrix' ? matrixIds.size + ' 条进入矩阵 / ' : ''}{rows.length} 条范围记录</span></div>
    <div className={'v-workspace ' + (selection ? 'with-detail' : '')}>
      <div className="v-main-panel">
        <FigureHead title={mode === 'Connection Matrix' ? 'Connection Matrix / 端点核对' : mode === 'BioFabric' ? 'BioFabric / 端口—线缆索引' : mode === 'Port Panel' ? 'Port Panel / 登记端口' : 'Cable Inventory / 物理记录'}>
          {mode === 'Connection Matrix' && <><select aria-label="矩阵排序" value={ordering} onChange={e => set({order: e.target.value})}><option value="rack">按位置排序</option><option value="name">按设备名称</option><option value="connections">按登记连接数</option></select><select aria-label="矩阵层级" value={axis} onChange={e => set({axis: e.target.value, row: '', col: ''})}><option value="servers">Server × Switch</option><option value="switches">Leaf × Spine</option></select></>}
          {(mode === 'BioFabric' || mode === 'Port Panel') && <select aria-label="布线设备" value={asset} onChange={e => {set({asset: e.target.value, row: '', col: '', port: '', cable: ''}); setBioPage(0);}}>{(!asset||invalidAsset)&&<option value={asset}>{asset?'目标设备不可用':'请选择设备'}</option>}{devices.filter(d => !pod || podOf(d.id) === pod || d.fields.role === 'spine').map(d => <option key={d.id} value={d.id}>{d.name}</option>)}</select>}
        </FigureHead>
        {mode === 'Connection Matrix' ? <>
          <ConnectionMatrix rows={matrixRows} cols={matrixCols} cables={rows} selected={row + ':' + col} onCell={(row, col) => {set({row, col, cable: '', port: ''}); setBioPage(0);}}/>
          <div className="v-matrix-key"><i/>数字 = 完整线缆条数<span>· = 无登记记录</span><span>! = 存在证据异常</span><span className="spacer"/><button className="text-link" onClick={() => set({mode: 'Cable Inventory', coverage: 'outside'})}>其他类型 / 缺端 {rows.length - matrixIds.size} 条</button><span>{matrixRows.length} 行 × {matrixCols.length} 列</span></div>
          {incomplete.length > 0 && <div className="v-incomplete-tray"><strong>未落入矩阵</strong>{incomplete.map(c => <button key={c.id} onClick={() => pick(c.id)}>{name(c.device_a)} / {name(c.a)} <ArrowRight size={12}/> 端点缺失 <code>{c.id}</code></button>)}</div>}
          {row && col && <><div className="v-selection-actions"><span>{name(row)} ↔ {name(col)}</span><span className="spacer"/><Button onClick={() => set({mode: 'BioFabric'})}>展开端口关系</Button><Button variant="ghost" onClick={clearCell}>清除单元格</Button></div><MemberLedger rows={members} selected={selection} onSelect={pick}/></>}
        </> : mode === 'BioFabric' ? <>
          <div className="v-inline-control"><span>横线 = 唯一端口 · 竖线 = 唯一线缆 · 方点 = 真实端接</span>{row && col && <Button onClick={clearCell}>清除矩阵范围</Button>}<span className="spacer"/><Button disabled={!page} onClick={() => setBioPage(page - 1)}>上一组</Button><span>{page + 1}/{Math.max(1, Math.ceil(bioAll.length / 24))}</span><Button disabled={(page + 1) * 24 >= bioAll.length} onClick={() => setBioPage(page + 1)}>下一组</Button></div>
          {!bio.length ? <Empty>该设备在当前筛选下没有记录，请切换设备或清除筛选。</Empty> : <div className="v-bio-scroll"><svg width="100%" style={{minWidth: Math.max(750, 300 + bio.length * 27)}} height={92 + portIds.length * 25} role="img" aria-label="BioFabric 端口线缆矩阵">
            {portIds.map((id, i) => {
              const y = 76 + i * 25, owner = api.asset(id)?.fields.owner_device_id;
              return <g key={id} role="button" tabIndex={0} className="bio-port" aria-label={'查看端口 ' + id} onClick={() => set({port: id, cable: ''})} onKeyDown={keyActivate(() => set({port: id, cable: ''}))}>
                <text x="16" y={y + 4} className="svg-label">{name(owner)} <tspan className="svg-small">/ {name(id)}</tspan></text><line x1="264" x2="98%" y1={y} y2={y} className="bio-row"/>
              </g>;
            })}
            {bio.map((c, i) => {
              const x = 289 + i * 27, a = portIds.indexOf(c.a), b = portIds.indexOf(c.b);
              const y1 = a<0?43:76+a*25, y2 = b < 0 ? 43 : 76 + b * 25;
              return <g key={c.id} className={'bio-cable ' + (selection === c.id ? 'selected' : selection ? 'dimmed' : '')} role="button" tabIndex={0} aria-label={'线缆列 ' + c.id} onClick={() => pick(c.id)} onKeyDown={keyActivate(() => pick(c.id))}>
                <title>{c.id}: {name(c.a)} → {name(c.b)}</title><text x={x} y="29" textAnchor="middle" className="svg-small">{String(page * 24 + i + 1).padStart(2, '0')}</text>
                <line x1={x} x2={x} y1="38" y2={Math.max(y1, y2)} className="bio-guide"/><line x1={x} x2={x} y1={y1} y2={y2} className={'bio-edge ' + (!complete(c) ? 'broken' : '')}/>
                {a>=0?<rect x={x - 4} y={y1 - 4} width="8" height="8" className="bio-terminal" data-port-id={c.a}/>:<text x={x} y="47" textAnchor="middle" className="break-mark">?</text>}{b >= 0 ? <rect x={x - 4} y={y2 - 4} width="8" height="8" className="bio-terminal" data-port-id={c.b}/> : <text x={x} y="47" textAnchor="middle" className="break-mark">?</text>}
                <rect x={x - 10} y="16" width="20" height={Math.max(y1, y2)} fill="transparent"/>
              </g>;
            })}
          </svg></div>}
          <div className="v-matrix-key">穿过横线不代表端接；只有两个方点才是该线缆端点。<span className="spacer"/>{bio.length} / {bioAll.length} 条</div>
          <MemberLedger rows={bio} selected={selection} onSelect={pick} title="当前线缆列 / 可追溯原记录"/>
        </> : mode === 'Port Panel' ? <>
          <div className="v-port-header"><b>{name(asset)}</b><span>{api.ports(asset).length} 个登记端口 · 编号示意，非设备面板几何</span></div>
          <div className="v-port-grid">{api.ports(asset).map(p => {
            const cs = api.connections(p.id); return <button key={p.id} className={selection === p.id ? 'selected' : ''} aria-label={'端口 ' + p.name} onClick={() => set({port: p.id, cable: ''})}><span className="port-socket"/><b title={p.name}>{shortLabel(p.name,25)}</b><small>{p.fields.speed || '速率未知'}</small><span>{cs.length ? cs.length + ' 条记录' : '未登记端接'}</span>{cs.some(c => status(c) !== 'unverified') && <Badge value={status(cs.find(c => status(c) !== 'unverified')!)}/>}</button>;
          })}</div>
          <MemberLedger rows={params.get('port') ? rows.filter(c => incident(c, params.get('port')!)) : local} selected={selection} onSelect={pick}/>
        </> : <>{params.get('coverage') === 'outside' && <div className="v-selection-actions">仅显示未进入当前矩阵的记录<span className="spacer"/><Button onClick={() => set({coverage: ''})}>显示全部范围</Button></div>}<CableTable rows={inventory} selected={selection} onSelect={pick}/></>}
        <Legend/>
      </div>
      <ObjectPanel id={selection} go={go} close={() => set({cable: '', port: ''})}/>
    </div>
    <div className="v-statusline"><Filter size={13}/><span>所有数值来自当前筛选的 Cable 集合，未登记关系不会补成正常链路。</span></div>
  </div>;
}
