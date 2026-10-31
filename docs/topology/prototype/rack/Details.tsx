import {useState} from 'react';
import {ArrowRight,ArrowUpRight,ChevronLeft,ChevronRight} from 'lucide-react';
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
export function Mapping({id,params,set,go}:Props&{id:string}){
  const [page,setPage]=useState(0),[q,setQ]=useState('');const ports=rackApi.ports(id),port=params.get('port')||'';
  const filtered=ports.filter(p=>[p.name,p.id].join(' ').toLowerCase().includes(q.toLowerCase())),pages=Math.max(1,Math.ceil(filtered.length/16)),current=Math.min(page,pages-1);
  const cables=rackApi.connections(port||id),cable=rackApi.cable(params.get('cable')||'');
  return <><div className="rv-mapping-path"><span>{name(id)}</span><ArrowRight size={12}/><span>端口</span><ArrowRight size={12}/><span>线缆</span><ArrowRight size={12}/><span>对端</span></div><SearchBox value={q} onChange={q=>{setQ(q);setPage(0)}} placeholder="搜索登记端口"/>
    <div className="rv-port-index">{filtered.slice(current*16,current*16+16).map(p=><button key={p.id} className={port===p.id?'selected':''} onClick={()=>set({port:port===p.id?'':p.id,cable:''})} title={p.id}>{p.name}<small>{p.fields.speed||'速率未知'}</small></button>)}</div>
    <div className="rv-port-pager"><span>{ports.length} 个登记端口 · 清单非面板坐标</span><button aria-label="上一组端口" disabled={!current} onClick={()=>setPage(current-1)}><ChevronLeft size={14}/></button><span>{current+1}/{pages}</span><button aria-label="下一组端口" disabled={current+1>=pages} onClick={()=>setPage(current+1)}><ChevronRight size={14}/></button></div>
    <div className="rv-connected-heading"><b>{port?name(port):'全部关联'} · {cables.length} 条端接</b>{port&&<button onClick={()=>set({port:'',cable:''})}>清除</button>}</div>
    <div className="rv-connection-list">{cables.map(c=><button key={c.id} className={cable?.id===c.id?'selected':''} onClick={()=>set({cable:c.id})}><code>{c.id}</code><span>{name(c.device_a)} / {name(c.a)}</span><span className="rv-wire-label">↓ {c.fields.cable_type||'介质未知'} · {c.fields.length_m??'—'} m</span><span>{name(c.device_b)} / {name(c.b)}</span><Badge value={c.fields.example||'unverified'}/></button>)}</div>
    {!cables.length&&<p>没有登记端接；缺失关系不代表物理断开。</p>}
    {cable&&<div className="rv-cable-action"><b>{cable.id}</b><p>来源：{cable.source} · 规划物理关系</p><Button onClick={()=>go('link-tracing',{cable:cable.id,port:port||cable.a})}>追踪线缆<ArrowUpRight size={14}/></Button><Button onClick={()=>go('physical-cabling',{cable:cable.id,asset:id,port})}>定位布线矩阵<ArrowUpRight size={14}/></Button></div>}
  </>
}
