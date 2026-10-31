import {useMemo,useState,useEffect} from 'react';
import {ArrowUpRight,Box,Database,LayoutGrid,ScanLine,Search,Bookmark,RotateCcw} from 'lucide-react';
import {Dialog,DialogContent,DialogTitle,DialogDescription} from '@/components/shadcn/dialog';
import {Button,SearchBox} from '../shared';
import {type Go} from '../model';
import {useIdentity} from '@/features/auth/IdentityProvider';
import {useProject} from '@/app/ProjectContext';
import {useRackModel,type Overlay,overlayLabels,shortPod,gpuText,occupiedText,freeText} from '../rack/model';
import {RoomGallery} from '../rack/RoomGallery';
import {Elevation} from '../rack/Elevation';
import {SpatialWorkspace} from '../rack/SpatialWorkspace';
import '../rack/rack.css';
import '../rack/spatial.css';

type Props={params:URLSearchParams;set:(values:Record<string,string>)=>void;go:Go};
const views=[['room','3D 机房',Box],['gallery','机柜阵列',LayoutGrid],['spatial','单柜工作台',ScanLine]] as const;

export default function RackPage({params,set,go}:Props){
  const rackApi=useRackModel(),identity=useIdentity(),context=useProject();
  const savedKey='keel.visualization.rack-view:'+identity.user.id+':'+(context.filter||'authorized');
  const overlayOptions=(Object.keys(overlayLabels) as Overlay[]).filter(k=>rackApi.capabilities[k]);
  const [capabilities,showCapabilities]=useState(false),[viewNotice,setViewNotice]=useState('');
  const selectedId=params.get('asset')||rackApi.asset(params.get('port')||'')?.fields.owner_device_id||rackApi.cable(params.get('cable')||'')?.device_a||'';
  const asset=rackApi.asset(selectedId);
  const rackID=params.get('rack')||asset?.fields.rack_id||rackApi.list()[0]?.id||'';
  const view=params.get('view')==='gallery'?'gallery':'spatial';
  const room=view==='gallery'&&params.get('overlay')==='capacity';
  const activeView=room?'room':view;
  // No eligible confirmed planning/as-built pair: hide comparison, including old links.
  useEffect(()=>{if(params.get('view')==='compare'){set({view:'spatial'});setViewNotice('对比暂未开放：缺少已确认规划快照与实装记录。')}},[params,set]);
  const overlay=overlayOptions.includes(params.get('overlay') as Overlay)?params.get('overlay') as Overlay:'asset';
  const snapshot=rackApi.snapshot(rackID);
  const pod=params.get('pod')||'',q=params.get('q')||'';
  const pods=[...new Set(rackApi.list().map(r=>r.fields.pod_id as string))];
  function openRack(id:string,asset=''){set({view:'spatial',rack:id,asset,port:'',cable:'',tab:'asset',...(view==='gallery'&&overlay==='capacity'?{camera:'dual'}:{})})}
  return <div className="rack-viz">
    <div className="rv-commandbar"><h1 className="sr-page-title">Spatial Rack<span>机柜工程视图</span></h1>
      <div className="rv-segment" role="tablist" aria-label="机柜视图">{views.map(([key,label,Icon])=><button key={key} role="tab" aria-selected={activeView===key} onClick={()=>set(key==='room'?{view:'gallery',overlay:'capacity'}:key==='gallery'?{view:'gallery',overlay:'asset'}:{view:'spatial',rack:rackID})}><Icon size={14}/>{label}</button>)}</div>
      <span className="spacer"/>
      <span className="rv-source-tag"><Database size={12}/>CMDB 资产登记</span>
      <Button onClick={()=>showCapabilities(true)}><ScanLine size={14}/>数据能力</Button>
    </div>
    {view==='gallery'?<>
      <div className="rv-filterbar"><SearchBox value={q} onChange={q=>set({q})} placeholder="搜索机柜、设备、型号、SN"/><select aria-label="Pod 筛选" value={pod} onChange={e=>set({pod:e.target.value})}><option value="">所有 Pod</option>{pods.map(p=><option key={p} value={p}>{shortPod(p)}</option>)}</select><span className="spacer"/><OverlayControl value={overlay} onChange={overlay=>set({overlay})}/></div>
      {room?<RoomGallery q={q} pod={pod} current={rackID} params={params} set={set} onOpen={openRack}/>:<Gallery q={q} pod={pod} overlay={overlay} current={rackID} onOpen={openRack}/>}
    </>:!snapshot?<div className="rv-empty"><Database/><h2>找不到此机柜</h2><p>保留 URL 中的对象 ID，不替换成其他资产。</p><Button onClick={()=>set({view:'gallery',rack:'',asset:'',port:'',cable:''})}>返回机柜阵列</Button></div>:<><div className="sr-contextbar"><select aria-label="选择机柜" value={rackID} onChange={e=>openRack(e.target.value)}>{rackApi.list().map(r=><option key={r.id} value={r.id}>{r.name} · {shortPod(r.fields.pod_id)}</option>)}</select><span>{snapshot.height===null?'U 高未知':snapshot.height+'U'}</span><span>{snapshot.placementComplete?`${snapshot.items.length} 设备`:`已加载 ${snapshot.items.length} / ${snapshot.registeredDevices??'未知'} 设备`}</span><span>{snapshot.layout?occupiedText(snapshot)+'U 登记占位':'占位未核算'}</span><span className="spacer"/>{viewNotice&&<small role="status">{viewNotice}</small>}<OverlayControl value={overlay} onChange={overlay=>set({overlay})}/><Button aria-label="保存机柜视图" title="保存当前机柜、对象与视角" onClick={()=>{localStorage.setItem(savedKey,params.toString());setViewNotice('视图已保存')}}><Bookmark size={14}/></Button><Button aria-label="恢复机柜视图" title="恢复已保存的机柜视图" onClick={()=>{const saved=localStorage.getItem(savedKey);if(saved!==null){const restored:Record<string,string>=Object.fromEntries([...params.keys()].map(k=>[k,'']));const savedParams=new URLSearchParams(saved),savedProject=savedParams.get('project_id');if(savedProject&&!context.projects.some(p=>p.id===savedProject)){setViewNotice('保存视图的项目已无访问权限');return}savedParams.delete('device_id');savedParams.delete('drawer');savedParams.forEach((v,k)=>restored[k]=v);set(restored);setViewNotice('视图已恢复')}else setViewNotice('暂无保存视图')}}><RotateCcw size={14}/></Button></div>{!snapshot.placementComplete&&<div className="rv-inline-warning">已加载 {snapshot.loadedDevices} / {snapshot.registeredDevices??'未知'} 条设备位置；当前图不代表完整机柜，未占用空间不作结论。</div>}<SpatialWorkspace key={rackID} {...{params,set,go,snapshot,overlay,selectedId}} onOpen={openRack}/></>}
    <Capabilities open={capabilities} close={()=>showCapabilities(false)}/>
  </div>
}
function OverlayControl({value,onChange}:{value:Overlay;onChange:(v:Overlay)=>void}){
  const rackApi=useRackModel();const overlayOptions=(Object.keys(overlayLabels) as Overlay[]).filter(k=>rackApi.capabilities[k]);
  return <div className="rv-overlay"><span>叠加层</span><div className="rv-segment" role="tablist" aria-label="信息叠加层">{overlayOptions.map(o=><button key={o} role="tab" aria-selected={o===value} onClick={()=>onChange(o)}>{overlayLabels[o]}</button>)}</div></div>
}
function Gallery({q,pod,overlay,current,onOpen}:{q:string;pod:string;overlay:Overlay;current:string;onOpen:(id:string,asset?:string)=>void}){
  const rackApi=useRackModel();
  const all=useMemo(()=>rackApi.list().map(r=>rackApi.snapshot(r.id)!),[rackApi]);
  const rows=all.filter(s=>(!pod||s.rack.fields.pod_id===pod)&&[s.rack.id,s.rack.name,...s.items.flatMap(d=>[d.id,d.name,d.fields.model,d.fields.serial])].join(' ').toLowerCase().includes(q.toLowerCase()));
  const groups=[...new Set(rows.map(s=>s.rack.fields.pod_id as string))];
  const total=rows.reduce((t,s)=>({u:t.u+(s.height??0),occupied:t.occupied+(s.layout?.occupied??0),gpu:t.gpu+s.gpu,gpuCoverage:t.gpuCoverage+s.gpuCoverage,gpuTotal:t.gpuTotal+s.gpuTotal,gpuComplete:t.gpuComplete&&s.gpuComplete,servers:t.servers+s.servers,switches:t.switches+s.switches,known:t.known+(s.layout&&s.placementComplete?1:0)}),{u:0,occupied:0,gpu:0,gpuCoverage:0,gpuTotal:0,gpuComplete:true,servers:0,switches:0,known:0});
  return <><div className="rv-summary" aria-label="阵列汇总"><b>{rows.length} <span>机柜</span></b><i/><span>{total.known?`${total.known<rows.length?'≥ ':''}${total.occupied} / ${total.u} U 登记占位`:'U 占位未知'}{total.known<rows.length?` · ${rows.length-total.known} 柜未核算`:''}</span><span>{total.servers} 服务器</span><span>{gpuText(total)} 登记 GPU</span><span>{total.switches} 交换机</span><span className="spacer"/><span>按 Pod 分组 · 非机房坐标图</span></div>
    {!rows.length?<div className="rv-empty"><Search/><h2>没有匹配的机柜或设备</h2><p>调整搜索或 Pod 筛选。</p></div>:<div className="rv-pod-grid">{groups.map(p=><section key={p} className="rv-pod"><div className="rv-pod-heading"><span className="rv-pod-marker"/><b>{shortPod(p)}</b><span>{rows.filter(s=>s.rack.fields.pod_id===p).length} 柜</span></div><div className="rv-rack-grid">{rows.filter(s=>s.rack.fields.pod_id===p).map(s=><button key={s.rack.id} className={'rv-rack-card '+(s.rack.id===current?'current':'')} aria-current={s.rack.id===current?'true':undefined} aria-label={`打开 ${s.rack.name}`} onClick={()=>{const match=q?s.items.find(d=>[d.id,d.name,d.fields.model,d.fields.serial].join(' ').toLowerCase().includes(q.toLowerCase())):null;onOpen(s.rack.id,match?.id)}}>
      <div className="rv-rack-title"><b>{s.rack.name}</b><ArrowUpRight size={13}/></div><Elevation snapshot={s} overlay={overlay} mini/>
      <div className="rv-mini-capacity"><span style={{width:`${s.height?(s.layout?.occupied||0)/s.height*100:0}%`}}/></div>
      <div className="rv-rack-facts"><strong>{occupiedText(s)} / {s.height??'—'}U</strong><span>{freeText(s)}U 未占用</span><span>{s.servers} 服务器 · {s.switches} 交换机</span><span className={overlay==='compute'?'rv-compute-value':''}>{gpuText(s)} 登记 GPU</span><small>实装 / 健康未核验</small></div>
    </button>)}</div></section>)}</div>}
    <Legend overlay={overlay}/>
  </>
}
function Legend({overlay}:{overlay:Overlay}){return <div className="rv-legend">{overlay==='asset'?<><span><i className="server"/>服务器</span><span><i className="switch"/>交换机</span></>:overlay==='capacity'?<><span><i className="occupied"/>登记占位并集</span><span><i className="free"/>未占用 U 区间</span></>:<span><i className="compute"/>登记 GPU 数量 · 非利用率</span>}<span><i className="unknown"/>虚线：安装面未知</span><span className="spacer"/><span>未占用 ≠ 已确认可安装 · 深度与安装面待补齐</span></div>}
function Capabilities({open,close}:{open:boolean;close:()=>void}){const rackApi=useRackModel(),racks=rackApi.list(),devices=racks.flatMap(r=>rackApi.devices(r.id));const covered=(key:string)=>devices.filter(d=>d.fields[key]!=null).length;return <Dialog open={open} onOpenChange={v=>!v&&close()}><DialogContent className="rv-capability-dialog"><DialogTitle>当前 CMDB 数据能力</DialogTitle><DialogDescription>仅统计当前授权查询范围；没有字段证据的能力保持未知。</DialogDescription><table><thead><tr><th>字段 / 能力</th><th>当前覆盖</th><th>展示边界</th></tr></thead><tbody><tr><td>机柜 U 高</td><td>{racks.filter(r=>r.fields.u_height!=null).length} / {racks.length}</td><td>精确 U 刻度</td></tr>{[['rack_u','设备 U 位'],['u_height','设备 U 高'],['mounting_face','安装面'],['depth_mm','设备深度'],['gpu_count','GPU 数量']].map(([key,label])=><tr key={key}><td>{label}</td><td>{covered(key)} / {devices.length}</td><td>缺少数据不推断</td></tr>)}<tr><td>机房坐标</td><td>未验证测绘来源</td><td>3D 仅示意排列</td></tr><tr><td>规划与实装对比</td><td>无成对确认快照</td><td>保持隐藏</td></tr></tbody></table><Button onClick={close}>返回工作台</Button></DialogContent></Dialog>}
