import {useMemo,useRef,useState} from 'react';
import {ArrowLeft,ArrowRight,ArrowUpRight,Box,ChevronLeft,ChevronRight,Columns2,Database,Download,Layers3,LayoutGrid,Link2,Move,ScanLine,Search,Undo2,Redo2,X} from 'lucide-react';
import {Dialog,DialogContent,DialogTitle,DialogDescription} from '@/components/shadcn/dialog';
import {Button,SearchBox,Badge} from '../shared';
import {name,exportFile,type Go} from '../model';
import {rackApi,type RackSnapshot,type Overlay,overlayLabels,geometry,faceLabel,typeLabel,shortPod} from '../rack/model';
import {Elevation} from '../rack/Elevation';
import '../rack/rack.css';

type Props={params:URLSearchParams;set:(values:Record<string,string>)=>void;go:Go};
const views=[['gallery','机柜阵列',LayoutGrid],['dual','双面立面',Columns2],['compare','规划 / 实装',Layers3]] as const;
const overlayOptions=(Object.keys(overlayLabels) as Overlay[]).filter(k=>rackApi.capabilities[k]);
export default function RackPage({params,set,go}:Props){
  const [capabilities,showCapabilities]=useState(false);
  const asset=rackApi.asset(params.get('asset')||'');
  const rackID=params.get('rack')||asset?.fields.rack_id||rackApi.list()[0].id;
  const view=params.get('view')||(params.has('rack')||params.has('asset')||params.has('port')?'dual':'gallery');
  const overlay=overlayOptions.includes(params.get('overlay') as Overlay)?params.get('overlay') as Overlay:'asset';
  const snapshot=rackApi.snapshot(rackID);
  const pod=params.get('pod')||'',q=params.get('q')||'';
  const pods=[...new Set(rackApi.list().map(r=>r.fields.pod_id as string))];
  function openRack(id:string,asset=''){set({view:'dual',rack:id,asset,port:'',cable:'',tab:asset?'asset':'devices'})}
  return <div className="rack-viz">
    <div className="rv-commandbar">
      <div className="rv-segment" role="tablist" aria-label="机柜视图">{views.map(([key,label,Icon])=><button key={key} role="tab" aria-selected={view===key} onClick={()=>set({view:key,...(key==='dual'?{rack:rackID}:{})})}><Icon size={14}/>{label}</button>)}</div>
      <span className="spacer"/>
      <span className="rv-source-tag"><Database size={12}/>规划登记 · 合成数据</span>
      <Button onClick={()=>showCapabilities(true)}><ScanLine size={14}/>数据能力</Button>
    </div>
    {view==='gallery'?<>
      <div className="rv-filterbar"><SearchBox value={q} onChange={q=>set({q})} placeholder="搜索机柜、设备、型号、SN"/><select aria-label="Pod 筛选" value={pod} onChange={e=>set({pod:e.target.value})}><option value="">所有 Pod</option>{pods.map(p=><option key={p} value={p}>{shortPod(p)}</option>)}</select><span className="spacer"/><OverlayControl value={overlay} onChange={overlay=>set({overlay})}/></div>
      <Gallery q={q} pod={pod} overlay={overlay} onOpen={openRack}/>
    </>:!snapshot?<div className="rv-empty"><Database/><h2>找不到此机柜</h2><p>保留 URL 中的对象 ID，不替换成其他资产。</p><Button onClick={()=>set({view:'gallery',rack:'',asset:'',port:'',cable:''})}>返回机柜阵列</Button></div>:view==='compare'?<Compare snapshot={snapshot} onSource={()=>showCapabilities(true)} onBack={()=>set({view:'dual'})}/>:<RackWorkbench key={rackID} {...{params,set,go,snapshot,overlay}} onOpen={openRack}/>}
    <Capabilities open={capabilities} close={()=>showCapabilities(false)}/>
  </div>
}
function OverlayControl({value,onChange}:{value:Overlay;onChange:(v:Overlay)=>void}){
  return <div className="rv-overlay"><span>叠加层</span><div className="rv-segment" role="tablist" aria-label="信息叠加层">{overlayOptions.map(o=><button key={o} role="tab" aria-selected={o===value} onClick={()=>onChange(o)}>{overlayLabels[o]}</button>)}</div></div>
}
function Gallery({q,pod,overlay,onOpen}:{q:string;pod:string;overlay:Overlay;onOpen:(id:string,asset?:string)=>void}){
  const all=useMemo(()=>rackApi.list().map(r=>rackApi.snapshot(r.id)!),[]);
  const rows=all.filter(s=>(!pod||s.rack.fields.pod_id===pod)&&[s.rack.id,s.rack.name,...s.items.flatMap(d=>[d.id,d.name,d.fields.model,d.fields.serial])].join(' ').toLowerCase().includes(q.toLowerCase()));
  const groups=[...new Set(rows.map(s=>s.rack.fields.pod_id as string))];
  const total=rows.reduce((t,s)=>({u:t.u+(s.height||0),occupied:t.occupied+(s.layout?.occupied||0),gpu:t.gpu+s.gpu,servers:t.servers+s.servers,switches:t.switches+s.switches}),{u:0,occupied:0,gpu:0,servers:0,switches:0});
  return <><div className="rv-summary" aria-label="阵列汇总"><b>{rows.length} <span>机柜</span></b><i/><span>{total.occupied} / {total.u} U 登记占位</span><span>{total.servers} 服务器</span><span>{total.gpu} 登记 GPU</span><span>{total.switches} 交换机</span><span className="spacer"/><span>按 Pod 分组 · 非机房坐标图</span></div>
    {!rows.length?<div className="rv-empty"><Search/><h2>没有匹配的机柜或设备</h2><p>调整搜索或 Pod 筛选。</p></div>:<div className="rv-pod-grid">{groups.map(p=><section key={p} className="rv-pod"><div className="rv-pod-heading"><span className="rv-pod-marker"/><b>{shortPod(p)}</b><span>{rows.filter(s=>s.rack.fields.pod_id===p).length} 柜</span></div><div className="rv-rack-grid">{rows.filter(s=>s.rack.fields.pod_id===p).map(s=><button key={s.rack.id} className="rv-rack-card" aria-label={`打开 ${s.rack.name}`} onClick={()=>{const match=q?s.items.find(d=>[d.id,d.name,d.fields.model,d.fields.serial].join(' ').toLowerCase().includes(q.toLowerCase())):null;onOpen(s.rack.id,match?.id)}}>
      <div className="rv-rack-title"><b>{s.rack.name}</b><ArrowUpRight size={13}/></div><Elevation snapshot={s} overlay={overlay} mini/>
      <div className="rv-mini-capacity"><span style={{width:`${s.height?(s.layout?.occupied||0)/s.height*100:0}%`}}/></div>
      <div className="rv-rack-facts"><strong>{s.layout?.occupied??'—'} / {s.height??'—'}U</strong><span>{s.layout?.free??'—'}U 未占用</span><span>{s.servers} 服务器 · {s.switches} 交换机</span><span className={overlay==='compute'?'rv-compute-value':''}>{s.gpuCoverage===s.gpuTotal?s.gpu:`≥ ${s.gpu}`} 登记 GPU</span><small>实装 / 健康未核验</small></div>
    </button>)}</div></section>)}</div>}
    <Legend overlay={overlay}/>
  </>
}
function Legend({overlay}:{overlay:Overlay}){return <div className="rv-legend">{overlay==='asset'?<><span><i className="server"/>服务器</span><span><i className="switch"/>交换机</span></>:overlay==='capacity'?<><span><i className="occupied"/>登记占位并集</span><span><i className="free"/>未占用 U 区间</span></>:<span><i className="compute"/>登记 GPU 数量 · 非利用率</span>}<span><i className="unknown"/>虚线：安装面未知</span><span className="spacer"/><span>未占用 ≠ 已确认可安装 · 深度与安装面待补齐</span></div>}
function RackWorkbench({params,set,go,snapshot:original,overlay,onOpen}:Props&{snapshot:RackSnapshot;overlay:Overlay;onOpen:(id:string,asset?:string)=>void}){
  const [edit,setEdit]=useState(false),[history,setHistory]=useState<Record<string,number>[]>([{}]),[index,setIndex]=useState(0),[confirm,setConfirm]=useState(false),[notice,setNotice]=useState(''),[zoom,setZoom]=useState(1);
  const positions=edit?history[index]:{};
  const snapshot=rackApi.snapshot(original.rack.id,positions)!;
  const selected=snapshot.items.find(d=>d.id===params.get('asset'));
  const requested=params.get('asset'),tab=params.get('tab')||(selected?'asset':'devices');
  const dual=useRef<HTMLDivElement>(null);
  const knownRackIndex=rackApi.list().findIndex(r=>r.id===snapshot.rack.id);
  function select(asset:string){set({asset,port:'',cable:'',tab:'asset'})}
  function move(id:string,u:number){const next={...positions,[id]:u};setHistory([...history.slice(0,index+1),next]);setIndex(index+1);setNotice('展示草稿已调整，尚未保存。')}
  function restore(){const raw=localStorage.getItem('keel.topology.prototype.rack-v2.'+snapshot.rack.id);if(raw){try{const p=JSON.parse(raw);setHistory([{},p]);setIndex(1);setEdit(true);setNotice('已恢复本地展示草稿。')}catch{setNotice('保存的草稿不可读，请重新编辑。')}}else setNotice('当前机柜没有保存的展示草稿。')}
  function exportSVG(){const elements=dual.current?.querySelectorAll<SVGSVGElement>('svg.rv-elevation');if(!elements?.length)return;const h=Number(elements[0].viewBox.baseVal.height);const bodies=Array.from(elements).map((el,i)=>`<g transform="translate(${i*320},38)">${el.innerHTML}</g>`).join('');exportFile(snapshot.rack.name+'-dual.svg',`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 620 ${h+72}"><style>${Array.from(document.styleSheets).flatMap(sheet=>{try{return Array.from(sheet.cssRules).map(r=>r.cssText)}catch{return[]}}).filter(s=>s.includes('rv-svg')).join('\n')}</style><rect width="100%" height="100%" fill="white"/><text x="24" y="22" font-size="12">${snapshot.rack.name} · SYNTHETIC · ${edit?'展示草稿':'规划登记'} · 安装面未知</text>${bodies}</svg>`,'image/svg+xml')}
  const isBad=!!snapshot.layout?.issues.length;
  return <>
    <div className="rv-rack-toolbar"><Button aria-label="返回机柜阵列" onClick={()=>set({view:'gallery'})}><ArrowLeft size={14}/></Button><select aria-label="选择机柜" value={snapshot.rack.id} onChange={e=>onOpen(e.target.value)}>{rackApi.list().map(r=><option key={r.id} value={r.id}>{r.name} · {shortPod(r.fields.pod_id)}</option>)}</select><Button aria-label="上一个机柜" disabled={knownRackIndex===0} onClick={()=>onOpen(rackApi.list()[knownRackIndex-1].id)}><ChevronLeft size={14}/></Button><Button aria-label="下一个机柜" disabled={knownRackIndex===rackApi.list().length-1} onClick={()=>onOpen(rackApi.list()[knownRackIndex+1].id)}><ChevronRight size={14}/></Button><span className="spacer"/><OverlayControl value={overlay} onChange={overlay=>set({overlay})}/><Button aria-pressed={edit} onClick={()=>{setEdit(!edit);setNotice('')}}><Move size={14}/>{edit?'退出草稿':'展示草稿'}</Button><Button onClick={exportSVG}><Download size={14}/>SVG</Button></div>
    <div className="rv-detail-summary"><b>{snapshot.rack.name}</b><span>{shortPod(snapshot.rack.fields.pod_id)}</span><span>{snapshot.height??'—'}U</span><span><strong>{snapshot.layout?.occupied??'—'}</strong>U 登记占位</span><span><strong>{snapshot.layout?.free??'—'}</strong>U 未占用</span><span>{snapshot.items.length} 设备</span><span>{snapshot.gpu} GPU</span><span className="spacer"/><span className="rv-muted">{edit?'展示草稿 · 不改 CMDB':'BOM 登记位置 · 实装待核验'}</span></div>
    <div className="rv-workbench"><section className="rv-dual-surface">
      <div className="rv-dual-labels"><span>FRONT <b>正面</b></span><span>REAR <b>背面</b></span></div>
      <div className="rv-geometry-note"><ScanLine size={13}/>{snapshot.missingFaces} 台设备安装面未记录，两面显示相同占位参考；不代表双面安装。</div>
      <div className="rv-dual-scroll"><div className="rv-dual" ref={dual} data-zoomed={zoom>1} style={{width:`${zoom*100}%`,maxWidth:zoom>1?'none':undefined}}>{(['front','rear'] as const).map(face=><Elevation key={face} {...{snapshot,face,overlay}} selected={selected?.id} onSelect={select} onMove={edit?move:undefined}/>)}</div></div>
      <div className="rv-canvas-controls"><span>同一资产 · 同一 U 坐标</span><span className="spacer"/><button aria-label="缩小立面" disabled={zoom<=1} onClick={()=>setZoom(Math.max(1,zoom-.25))}>−</button><span>{Math.round(zoom*100)}%</span><button aria-label="放大立面" disabled={zoom>=1.5} onClick={()=>setZoom(Math.min(1.5,zoom+.25))}>+</button><button onClick={()=>setZoom(1)}>适配</button></div>
      {snapshot.layout&&[...snapshot.layout.zeroU,...snapshot.layout.unplaced].length>0&&<div className="rv-tray"><b>未定位 / 0U 附件</b>{[...snapshot.layout.zeroU,...snapshot.layout.unplaced].map(d=><button key={d.id} onClick={()=>select(d.id)}>{name(d.id)} · {d.height===0?'0U，侧挂位置未记录':'位置缺失'}</button>)}</div>}
      {edit&&<div className="rv-draftbar"><span>拖动吸附 0.5U，或选中后按 ↑ ↓</span><Button aria-label="撤销草稿" disabled={!index} onClick={()=>setIndex(index-1)}><Undo2 size={14}/></Button><Button aria-label="重做草稿" disabled={index>=history.length-1} onClick={()=>setIndex(index+1)}><Redo2 size={14}/></Button><Button onClick={restore}>恢复草稿</Button><Button disabled={!Object.keys(positions).length||isBad} onClick={()=>setConfirm(true)}>保存草稿</Button></div>}
      {snapshot.layout?.issues.length?<div className="rv-inline-warning" role="alert">{snapshot.layout.issues.map((i,n)=><div key={n}>{i.ids.map(name).join(' / ')}：{i.message}</div>)}</div>:null}
      {notice&&<div role="status" className="rv-notice">{notice}</div>}
    </section>
    <aside className="rv-inspector"><div className="rv-inspector-heading"><span>{selected?typeLabel(selected):'机柜工作台'}</span>{selected&&<Button aria-label="取消设备选择" variant="ghost" onClick={()=>set({asset:'',port:'',cable:'',tab:'devices'})}><X size={14}/></Button>}<h2>{selected?.name||snapshot.rack.name}</h2><code>{selected?.id||snapshot.rack.id}</code></div>
      {requested&&!selected&&<div className="rv-inline-warning">目标资产不属于当前机柜，请按资产所属机柜定位。</div>}
      <div className="rv-detail-tabs" role="tablist" aria-label="详情分区">{[['devices','设备'],['asset','资产'],['mapping','Mapping'],['source','来源']].map(([v,label])=><button key={v} role="tab" aria-selected={tab===v} onClick={()=>set({tab:v})}>{label}</button>)}</div>
      <div className="rv-inspector-body">
        {tab==='devices'?<DeviceList snapshot={snapshot} selected={selected?.id||''} onSelect={select}/>:tab==='mapping'?selected?<Mapping key={selected.id} id={selected.id} params={params} set={set} go={go}/>:<PickDevice snapshot={snapshot} onSelect={id=>set({asset:id,tab:'mapping'})}/>:tab==='source'?<Source selected={selected?.id} snapshot={snapshot}/>:selected?<>
          <dl className="rv-properties"><dt>型号</dt><dd>{selected.fields.model||'未记录'}</dd><dt>资产编号</dt><dd>{selected.fields.serial||'未记录'}</dd><dt>登记 U 位</dt><dd>U{selected.fields.rack_u} · {selected.fields.u_height}U</dd><dt>安装面</dt><dd className="rv-unknown">{faceLabel(geometry(selected).face)}</dd><dt>设备深度</dt><dd className="rv-unknown">{selected.fields.depth_mm?selected.fields.depth_mm+' mm':'未记录'}</dd><dt>全深度设备</dt><dd className="rv-unknown">{geometry(selected).fullDepth===null?'未记录':geometry(selected).fullDepth?'是':'否'}</dd><dt>登记 GPU</dt><dd>{selected.type==='server'?(selected.fields.gpu_count??'未记录'):'不适用'}</dd><dt>登记 NIC</dt><dd>{selected.type==='server'?(selected.fields.nic_count??'未记录'):'不适用'}</dd><dt>已登记端口</dt><dd>{rackApi.ports(selected.id).length}</dd><dt>当前健康</dt><dd className="rv-unknown">未采集</dd></dl>
          <div className="rv-panel-fallback"><Box size={24}/><div><b>标准化资产轮廓</b><p>未绑定精确型号面板。端口以清单展示，不猜测插槽数量或物理位置。</p></div></div>
          <Button className="rv-wide-button" onClick={()=>set({tab:'mapping'})}><Link2 size={14}/>查看端口与关联设备<ArrowRight size={14}/></Button>
          <Button className="rv-wide-button" onClick={()=>go('global-topology',{asset:selected.id,scope:snapshot.rack.fields.pod_id})}>定位全局拓扑<ArrowUpRight size={14}/></Button>
        </>:<PickDevice snapshot={snapshot} onSelect={select}/>}
      </div>
    </aside></div><Legend overlay={overlay}/>
    <Dialog open={confirm} onOpenChange={setConfirm}><DialogContent><DialogTitle>保存 {snapshot.rack.name} 展示草稿</DialogTitle><DialogDescription>仅保存在此浏览器中，不修改正式 CMDB 位置，也不形成实装或审批版本。</DialogDescription><table><thead><tr><th>设备</th><th>登记位置</th><th>草稿位置</th></tr></thead><tbody>{Object.entries(positions).map(([id,u])=><tr key={id}><td>{name(id)}</td><td>U{rackApi.asset(id)?.fields.rack_u}</td><td>U{u}</td></tr>)}</tbody></table><Button variant="default" onClick={()=>{localStorage.setItem('keel.topology.prototype.rack-v2.'+snapshot.rack.id,JSON.stringify(positions));setConfirm(false);setNotice('展示草稿已保存，正式资产位置未改变。')}}>确认保存展示草稿</Button></DialogContent></Dialog>
  </>
}
function DeviceList({snapshot,selected,onSelect}:{snapshot:RackSnapshot;selected:string;onSelect:(id:string)=>void}){const [q,setQ]=useState('');return <><SearchBox value={q} onChange={setQ} placeholder="搜索柜内设备"/><div className="rv-device-list">{snapshot.items.filter(d=>[d.id,d.name,d.fields.serial,d.fields.model].join(' ').toLowerCase().includes(q.toLowerCase())).sort((a,b)=>Number(b.fields.rack_u)-Number(a.fields.rack_u)).map(d=><button className={selected===d.id?'selected':''} key={d.id} onClick={()=>onSelect(d.id)}><span className={'rv-type-dot '+d.type}/><div><b>{d.name}</b><small>{d.fields.model}</small></div><span>U{d.fields.rack_u}<small>{d.fields.u_height}U</small></span></button>)}</div><div className="rv-inspector-note">设备使用相同 CMDB 身份；点选同时定位前后立面。</div></>}
function PickDevice({snapshot,onSelect}:{snapshot:RackSnapshot;onSelect:(id:string)=>void}){return <><p className="rv-pick-note">选择设备，查看资产和关联信息。</p><DeviceList snapshot={snapshot} selected="" onSelect={onSelect}/></>}
function Source({selected,snapshot}:{selected?:string;snapshot:RackSnapshot}){const d=rackApi.asset(selected||snapshot.rack.id)!;return <>
  <div className="rv-source-layer"><span>01</span><div><b>登记事实 · Planned</b><p>{d.source||'BOM / CMDB 合成投影'}</p><code>{d.id}</code></div></div>
  <div className="rv-source-layer"><span>02</span><div><b>位置与几何来源</b><p>{selected?'rack_id / rack_u / u_height':'u_height / pod_id'} 已登记。安装面、深度及面板素材未记录。</p></div></div>
  <div className="rv-source-layer missing"><span>03</span><div><b>现场安装确认 · 未提供</b><p>没有可匹配的 As-built 位置证据，不使用规划记录代替。</p></div></div>
  <div className="rv-source-layer missing"><span>04</span><div><b>实时观测 · 未提供</b><p>历史合成健康、功耗和温度字段不用于当前运行态判定。</p></div></div>
  <dl className="rv-properties"><dt>演示参考时间</dt><dd>{rackApi.source.referenceAt}</dd><dt>证据性质</dt><dd>SYNTHETIC</dd><dt>位置核验</dt><dd>未核验实装</dd><dt>实时保鲜</dt><dd>unknown</dd></dl>
</>}
function Mapping({id,params,set,go}:Props&{id:string}){
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
function Compare({snapshot,onSource,onBack}:{snapshot:RackSnapshot;onSource:()=>void;onBack:()=>void}){return <section className="rv-compare"><div className="rv-compare-head"><div><span className="rv-eyebrow">PLANNED / AS-BUILT</span><h2>{snapshot.rack.name} · 规划与实装</h2></div><Button onClick={onBack}><Columns2 size={14}/>查看登记立面</Button></div>
  <div className="rv-compare-grid"><div className="rv-compare-column"><span className="rv-step-number">01</span><h3>规划基准</h3><b className="rv-compare-status">有规划登记，缺已批准比较快照</b><dl className="rv-properties"><dt>可用数据</dt><dd>{snapshot.items.length} 台设备与 U 位</dd><dt>事实层</dt><dd>planned / BOM</dd><dt>确认性质</dt><dd>synthetic_confirmed</dd><dt>批准快照</dt><dd>未提供</dd><dt>基线版本</dt><dd>未选定</dd></dl></div><div className="rv-compare-arrow"><ArrowRight size={24}/></div><div className="rv-compare-column"><span className="rv-step-number">02</span><h3>现场实装</h3><b className="rv-compare-status">等待安装确认记录</b><dl className="rv-properties"><dt>可用数据</dt><dd>{rackApi.audit().additional_observations.confirmed_as_built_locations} 条已确认实装位置</dd><dt>必要证据</dt><dd>安装确认 / 现场验收</dd><dt>比较主键</dt><dd>同一稳定资产 ID</dd><dt>数据时间</dt><dd>未提供</dd><dt>实装版本</dt><dd>未选定</dd></dl></div></div>
  <div className="rv-compare-foot"><ScanLine size={22}/><div><b>暂不能生成差异</b><p>补齐批准规划版本与实装快照后，按设备、型号、机柜和 U 位比较缺失、新增及移位。展示草稿不参与实装对比。</p></div><Button onClick={onSource}>查看数据依据</Button><Button disabled title="缺少批准规划与已确认实装快照">生成差异</Button></div>
</section>}
function Capabilities({open,close}:{open:boolean;close:()=>void}){const audit=rackApi.audit(),c=audit.counts;const covered=(type:string,key:string)=>audit.field_coverage.find(f=>f.asset_type===type&&f.key===key)?.present||0;const deviceTotal=c.server+c.switch;return <Dialog open={open} onOpenChange={v=>!v&&close()}><DialogContent className="rv-capability-dialog"><DialogTitle>数据能力与可视化边界</DialogTitle><DialogDescription>本地 PostgreSQL 只读审计 · {audit.audited_at.slice(0,10)}。原型使用独立 64 台合成切片，不读取或修改业务 API。</DialogDescription><table><thead><tr><th>字段 / 能力</th><th>本地 CMDB 覆盖</th><th>原型处理</th></tr></thead><tbody><tr><td>机柜 U 高</td><td>{covered('rack','u_height')} / {c.rack}</td><td>精确 U 刻度</td></tr><tr><td>设备位置 / U 高</td><td>{covered('server','rack_u')+covered('switch','rack_u')} / {deviceTotal}</td><td>规划登记立面</td></tr><tr><td>GPU 组成关联</td><td>{audit.additional_observations.server_gpu_contains_relations}</td><td>登记数量叠加</td></tr><tr><td>安装面 / 深度 / 面板</td><td>{covered('server','mounting_face')+covered('switch','mounting_face')} / {deviceTotal}</td><td>虚线占位，不伪造硬件细节</td></tr><tr><td>已确认实装位置</td><td>{audit.additional_observations.confirmed_as_built_locations}</td><td>不生成实装差异</td></tr><tr><td>可靠实时 Power / Health / Thermal</td><td>未验证</td><td>不启用这些覆盖层</td></tr><tr><td>PDU / 供电端接</td><td>{audit.additional_observations.pdu_entities}</td><td>不生成纵向 PDU 或电路</td></tr></tbody></table><div className="rv-capability-note">本地库包含 {c.server} 台服务器、{c.switch} 台交换机；另有测试资产缺少完整位置。原型保留已有合成演示切片，没有删除或补造正式资产。历史 fixture 的 front 默认值和功耗样例不用于填补 CMDB 缺测。</div><Button onClick={close}>返回工作台</Button></DialogContent></Dialog>}
