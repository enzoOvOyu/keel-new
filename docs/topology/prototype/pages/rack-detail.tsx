import {useMemo,useState} from 'react';
import {ArrowRight,ArrowUpRight,Box,Columns2,Database,Layers3,LayoutGrid,ScanLine,Search,Bookmark,RotateCcw} from 'lucide-react';
import {Dialog,DialogContent,DialogTitle,DialogDescription} from '@/components/shadcn/dialog';
import {Button,SearchBox} from '../shared';
import {type Go} from '../model';
import {rackApi,type RackSnapshot,type Overlay,overlayLabels,shortPod} from '../rack/model';
import {Elevation} from '../rack/Elevation';
import {SpatialWorkspace} from '../rack/SpatialWorkspace';
import '../rack/rack.css';
import '../rack/spatial.css';

type Props={params:URLSearchParams;set:(values:Record<string,string>)=>void;go:Go};
const views=[['spatial','空间工作台',Box],['gallery','机柜阵列',LayoutGrid],['compare','规划 / 实装',Layers3]] as const;
const overlayOptions=(Object.keys(overlayLabels) as Overlay[]).filter(k=>rackApi.capabilities[k]);
export default function RackPage({params,set,go}:Props){
  const [capabilities,showCapabilities]=useState(false),[viewNotice,setViewNotice]=useState('');
  const asset=rackApi.asset(params.get('asset')||'');
  const rackID=params.get('rack')||asset?.fields.rack_id||rackApi.list()[0].id;
  const view=params.get('view')==='dual'?'spatial':params.get('view')||'spatial';
  const overlay=overlayOptions.includes(params.get('overlay') as Overlay)?params.get('overlay') as Overlay:'asset';
  const snapshot=rackApi.snapshot(rackID);
  const pod=params.get('pod')||'',q=params.get('q')||'';
  const pods=[...new Set(rackApi.list().map(r=>r.fields.pod_id as string))];
  function openRack(id:string,asset=''){set({view:'spatial',rack:id,asset,port:'',cable:'',tab:'asset'})}
  return <div className="rack-viz">
    <div className="rv-commandbar"><h1 className="sr-page-title">Spatial Rack<span>机柜工程视图</span></h1>
      <div className="rv-segment" role="tablist" aria-label="机柜视图">{views.map(([key,label,Icon])=><button key={key} role="tab" aria-selected={view===key} onClick={()=>set({view:key,...(key==='spatial'?{rack:rackID}:{})})}><Icon size={14}/>{label}</button>)}</div>
      <span className="spacer"/>
      <span className="rv-source-tag"><Database size={12}/>规划登记 · 合成数据</span>
      <Button onClick={()=>showCapabilities(true)}><ScanLine size={14}/>数据能力</Button>
    </div>
    {view==='gallery'?<>
      <div className="rv-filterbar"><SearchBox value={q} onChange={q=>set({q})} placeholder="搜索机柜、设备、型号、SN"/><select aria-label="Pod 筛选" value={pod} onChange={e=>set({pod:e.target.value})}><option value="">所有 Pod</option>{pods.map(p=><option key={p} value={p}>{shortPod(p)}</option>)}</select><span className="spacer"/><OverlayControl value={overlay} onChange={overlay=>set({overlay})}/></div>
      <Gallery q={q} pod={pod} overlay={overlay} onOpen={openRack}/>
    </>:!snapshot?<div className="rv-empty"><Database/><h2>找不到此机柜</h2><p>保留 URL 中的对象 ID，不替换成其他资产。</p><Button onClick={()=>set({view:'gallery',rack:'',asset:'',port:'',cable:''})}>返回机柜阵列</Button></div>:view==='compare'?<Compare snapshot={snapshot} onSource={()=>showCapabilities(true)} onBack={()=>set({view:'spatial'})}/>:<><div className="sr-contextbar"><select aria-label="选择机柜" value={rackID} onChange={e=>openRack(e.target.value)}>{rackApi.list().map(r=><option key={r.id} value={r.id}>{r.name} · {shortPod(r.fields.pod_id)}</option>)}</select><span>{snapshot.height}U</span><span>{snapshot.items.length} 设备</span><span>{snapshot.layout?.occupied}U 登记占位</span><span className="spacer"/>{viewNotice&&<small role="status">{viewNotice}</small>}<OverlayControl value={overlay} onChange={overlay=>set({overlay})}/><Button aria-label="保存机柜视图" title="保存当前机柜、对象与视角" onClick={()=>{localStorage.setItem('keel.topology.prototype.rack-view',window.location.search);setViewNotice('视图已保存')}}><Bookmark size={14}/></Button><Button aria-label="恢复机柜视图" title="恢复已保存的机柜视图" onClick={()=>{const saved=localStorage.getItem('keel.topology.prototype.rack-view');if(saved!==null){const restored:Record<string,string>=Object.fromEntries([...params.keys()].map(k=>[k,'']));new URLSearchParams(saved).forEach((v,k)=>restored[k]=v);set(restored);setViewNotice('视图已恢复')}else setViewNotice('暂无保存视图')}}><RotateCcw size={14}/></Button></div><SpatialWorkspace key={rackID} {...{params,set,go,snapshot,overlay}} onOpen={openRack}/></>}
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
function Compare({snapshot,onSource,onBack}:{snapshot:RackSnapshot;onSource:()=>void;onBack:()=>void}){return <section className="rv-compare"><div className="rv-compare-head"><div><span className="rv-eyebrow">PLANNED / AS-BUILT</span><h2>{snapshot.rack.name} · 规划与实装</h2></div><Button onClick={onBack}><Columns2 size={14}/>查看登记立面</Button></div>
  <div className="rv-compare-grid"><div className="rv-compare-column"><span className="rv-step-number">01</span><h3>规划基准</h3><b className="rv-compare-status">有规划登记，缺已批准比较快照</b><dl className="rv-properties"><dt>可用数据</dt><dd>{snapshot.items.length} 台设备与 U 位</dd><dt>事实层</dt><dd>planned / BOM</dd><dt>确认性质</dt><dd>synthetic_confirmed</dd><dt>批准快照</dt><dd>未提供</dd><dt>基线版本</dt><dd>未选定</dd></dl></div><div className="rv-compare-arrow"><ArrowRight size={24}/></div><div className="rv-compare-column"><span className="rv-step-number">02</span><h3>现场实装</h3><b className="rv-compare-status">等待安装确认记录</b><dl className="rv-properties"><dt>可用数据</dt><dd>{rackApi.audit().additional_observations.confirmed_as_built_locations} 条已确认实装位置</dd><dt>必要证据</dt><dd>安装确认 / 现场验收</dd><dt>比较主键</dt><dd>同一稳定资产 ID</dd><dt>数据时间</dt><dd>未提供</dd><dt>实装版本</dt><dd>未选定</dd></dl></div></div>
  <div className="rv-compare-foot"><ScanLine size={22}/><div><b>暂不能生成差异</b><p>补齐批准规划版本与实装快照后，按设备、型号、机柜和 U 位比较缺失、新增及移位。展示草稿不参与实装对比。</p></div><Button onClick={onSource}>查看数据依据</Button><Button disabled title="缺少批准规划与已确认实装快照">生成差异</Button></div>
</section>}
function Capabilities({open,close}:{open:boolean;close:()=>void}){const audit=rackApi.audit(),c=audit.counts;const covered=(type:string,key:string)=>audit.field_coverage.find(f=>f.asset_type===type&&f.key===key)?.present||0;const deviceTotal=c.server+c.switch;return <Dialog open={open} onOpenChange={v=>!v&&close()}><DialogContent className="rv-capability-dialog"><DialogTitle>数据能力与可视化边界</DialogTitle><DialogDescription>本地 PostgreSQL 只读审计 · {audit.audited_at.slice(0,10)}。原型使用独立 64 台合成切片，不读取或修改业务 API。</DialogDescription><table><thead><tr><th>字段 / 能力</th><th>本地 CMDB 覆盖</th><th>原型处理</th></tr></thead><tbody><tr><td>机柜 U 高</td><td>{covered('rack','u_height')} / {c.rack}</td><td>精确 U 刻度</td></tr><tr><td>设备位置 / U 高</td><td>{covered('server','rack_u')+covered('switch','rack_u')} / {deviceTotal}</td><td>规划登记立面</td></tr><tr><td>GPU 组成关联</td><td>{audit.additional_observations.server_gpu_contains_relations}</td><td>登记数量叠加</td></tr><tr><td>安装面 / 深度 / 面板</td><td>{covered('server','mounting_face')+covered('switch','mounting_face')} / {deviceTotal}</td><td>虚线占位，不伪造硬件细节</td></tr><tr><td>已确认实装位置</td><td>{audit.additional_observations.confirmed_as_built_locations}</td><td>不生成实装差异</td></tr><tr><td>可靠实时 Power / Health / Thermal</td><td>未验证</td><td>不启用这些覆盖层</td></tr><tr><td>PDU / 供电端接</td><td>{audit.additional_observations.pdu_entities}</td><td>不生成纵向 PDU 或电路</td></tr></tbody></table><div className="rv-capability-note">本地库包含 {c.server} 台服务器、{c.switch} 台交换机；另有测试资产缺少完整位置。原型保留已有合成演示切片，没有删除或补造正式资产。历史 fixture 的 front 默认值和功耗样例不用于填补 CMDB 缺测。</div><Button onClick={close}>返回工作台</Button></DialogContent></Dialog>}
