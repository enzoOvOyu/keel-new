import {useState} from 'react';
import {ArrowRight,ArrowUpRight,ChevronLeft,ChevronRight,X} from 'lucide-react';
import {Button,SearchBox,Badge} from '../shared';
import {name,type Go} from '../model';
import {rackApi,type RackSnapshot} from './model';
type Props={params:URLSearchParams;set:(v:Record<string,string>)=>void;go:Go};
export function Source({selected,snapshot}:{selected?:string;snapshot:RackSnapshot}){const d=rackApi.asset(selected||snapshot.rack.id)!;return <>
  <div className="rv-source-layer"><span>01</span><div><b>登记事实 · Planned</b><p>{d.source||'BOM / CMDB 合成投影'}</p><code>{d.id}</code></div></div>
  <div className="rv-source-layer"><span>02</span><div><b>位置与几何来源</b><p>{selected?'rack_id / rack_u / u_height':'u_height / pod_id'} 已登记。安装面、深度及面板素材未记录。</p></div></div>
  <div className="rv-source-layer missing"><span>03</span><div><b>现场安装确认 · 未提供</b><p>没有可匹配的 As-built 位置证据，不使用规划记录代替。</p></div></div>
  <div className="rv-source-layer missing"><span>04</span><div><b>实时观测 · 未提供</b><p>历史合成健康、功耗和温度字段不用于当前运行态判定。</p></div></div>
  <dl className="rv-properties"><dt>演示参考时间</dt><dd>{rackApi.source.referenceAt}</dd><dt>证据性质</dt><dd>SYNTHETIC</dd><dt>位置核验</dt><dd>未核验实装</dd><dt>实时保鲜</dt><dd>unknown</dd></dl>
</>}
export function Mapping({id,params,set,go,onEndpoint}:Props&{id:string;onEndpoint:(asset:string,port:string)=>void}){
  const [page,setPage]=useState(0),[q,setQ]=useState('');const ports=rackApi.ports(id),requestedPort=params.get('port')||'',port=ports.some(p=>p.id===requestedPort)?requestedPort:'';
  const filtered=ports.filter(p=>[p.name,p.id].join(' ').toLowerCase().includes(q.toLowerCase())),pages=Math.max(1,Math.ceil(filtered.length/16)),current=Math.min(page,pages-1);
  const selectedPort=ports.find(p=>p.id===port);
  const cables=rackApi.connections(port||id),cable=cables.find(c=>c.id===params.get('cable'));
  return <><div className="rv-mapping-path"><span>{name(id)}</span><ArrowRight size={12}/><span>端口</span><ArrowRight size={12}/><span>线缆</span><ArrowRight size={12}/><span>对端</span></div><SearchBox value={q} onChange={q=>{setQ(q);setPage(0)}} placeholder="搜索登记端口"/>
    {requestedPort&&!port&&<div className="sr-alert">此端口不属于当前设备。<Button onClick={()=>set({port:'',cable:''})}>清除端口定位</Button></div>}<div className="rv-port-index">{filtered.slice(current*16,current*16+16).map(p=><button key={p.id} className={port===p.id?'selected':''} onClick={()=>set({port:port===p.id?'':p.id,cable:port===p.id?'':rackApi.connections(p.id)[0]?.id||''})} title={p.id}>{p.name}<small>{p.fields.speed||'速率未知'}</small></button>)}</div>
    <div className="rv-port-pager"><span>{ports.length} 个登记端口 · 清单非面板坐标</span><button aria-label="上一组端口" disabled={!current} onClick={()=>setPage(current-1)}><ChevronLeft size={14}/></button><span>{current+1}/{pages}</span><button aria-label="下一组端口" disabled={current+1>=pages} onClick={()=>setPage(current+1)}><ChevronRight size={14}/></button></div>
    {selectedPort&&<dl className="sr-port-facts"><dt>端口 ID</dt><dd>{selectedPort.id}</dd><dt>速率 / 用途</dt><dd>{selectedPort.fields.speed||'未知'} / {selectedPort.fields.network_role||'未登记'}</dd><dt>采集状态</dt><dd>未核验实际链路 · unknown</dd><dt>来源</dt><dd>{selectedPort.source}</dd></dl>}
    <div className="rv-connected-heading"><b>{port?name(port):'全部关联'} · {cables.length} 条端接</b>{port&&<button onClick={()=>set({port:'',cable:''})}>清除</button>}</div>
    {cable&&<section className="sr-cable-detail" aria-label="选中线缆详情"><div className="sr-cable-heading"><code>{cable.id}</code><button aria-label="取消线缆选择" onClick={()=>set({cable:''})}><X size={13}/></button></div><div className="sr-endpoint-pair">{([{side:'A',asset:cable.device_a,port:cable.a},{side:'B',asset:cable.device_b,port:cable.b}]).map(e=>{const device=rackApi.asset(e.asset),p=rackApi.asset(e.port),valid=!!device&&!!p&&p.fields.owner_device_id===device.id;return <div key={e.side} className={valid?'':'missing'}><small>{e.side} 端{!valid?' · 引用不完整':''}</small><b>{device?.name||'设备未登记'}</b><code>{p?.name||'端口未登记'}</code><span>{device?.fields.rack_id?name(device.fields.rack_id):'机柜未知'}{device?.fields.rack_u!=null?` · U${device.fields.rack_u}`:''}</span><button disabled={!valid||!device?.fields.rack_id} onClick={()=>onEndpoint(e.asset,e.port)} aria-label={`定位 ${device?.name||e.side+'端'}`}>定位设备<ArrowUpRight size={12}/></button></div>})}</div><div className="sr-cable-medium"><span>{cable.fields.cable_type||'介质未知'} · {cable.fields.length_m??'—'} m</span><Badge value={cable.fields.example||'unverified'}/></div><div className="sr-cable-links"><Button onClick={()=>go('link-tracing',{cable:cable.id,port:port||cable.a})}>追踪线缆<ArrowUpRight size={12}/></Button><Button onClick={()=>go('physical-cabling',{cable:cable.id,asset:id,port})}>布线矩阵<ArrowUpRight size={12}/></Button></div><details className="sr-cable-source"><summary>来源与证据</summary><p>{cable.source} · 规划物理关系；不代表已验证实际链路。</p></details></section>}
    <div className="rv-connection-list">{cables.map(c=><button key={c.id} className={cable?.id===c.id?'selected':''} onClick={()=>set({cable:c.id})}><code>{c.id}</code><span>{name(c.device_a)} / {name(c.a)}</span><span className="rv-wire-label">↓ {c.fields.cable_type||'介质未知'} · {c.fields.length_m??'—'} m</span><span>{name(c.device_b)} / {name(c.b)}</span><Badge value={c.fields.example||'unverified'}/></button>)}</div>
    {!cables.length&&<p>没有登记端接；缺失关系不代表物理断开。</p>}

  </>
}
