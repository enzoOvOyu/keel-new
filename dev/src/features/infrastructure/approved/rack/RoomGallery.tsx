import {lazy,Suspense,useMemo,useState} from 'react';
import {ArrowUpRight,Columns2,Database,Layers3,Server} from 'lucide-react';
import {Button} from '../shared';
import {Elevation} from './Elevation';
import {roomModel} from './room-model';
import {shortPod,useRackModel} from './model';
import './room.css';
const RoomScene=lazy(()=>import('./room/RoomScene'));
type Props={q:string;pod:string;current:string;params:URLSearchParams;set:(v:Record<string,string>)=>void;onOpen:(id:string,asset?:string)=>void};
export function RoomGallery({q,pod,current,params,set,onOpen}:Props){
  const rackApi=useRackModel();
  const model=useMemo(()=>roomModel(rackApi),[rackApi]),[face,setFace]=useState<'front'|'rear'>('front');
  const visible=useMemo(()=>model.racks.filter(({snapshot:s})=>(!pod||s.rack.fields.pod_id===pod)&&[s.rack.id,s.rack.name,...s.items.flatMap(d=>[d.id,d.name,d.fields.model,d.fields.serial])].join(' ').toLowerCase().includes(q.toLowerCase())),[model,q,pod]);
  const visibleIds=useMemo(()=>visible.map(r=>r.snapshot.rack.id),[visible]);
  const selected=visible.find(r=>r.snapshot.rack.id===current)||visible[0],s=selected?.snapshot;
  const totals=visible.reduce((n,{snapshot:r})=>({u:n.u+(r.height||0),used:n.used+(r.layout?.occupied||0),gpu:n.gpu+r.gpu,servers:n.servers+r.servers,switches:n.switches+r.switches}),{u:0,used:0,gpu:0,servers:0,switches:0});
  const open=(id:string,asset='')=>{const match=q?model.racks.find(r=>r.snapshot.rack.id===id)?.snapshot.items.find(d=>[d.id,d.name,d.fields.model,d.fields.serial].join(' ').toLowerCase().includes(q.toLowerCase())):null;onOpen(id,asset||match?.id||'');};
  return <div className="room-gallery">
    <div className="room-summary"><b><Layers3 size={15}/>{visible.length} 机柜</b><span>{totals.used} / {totals.u} U 已登记</span><span>{totals.u-totals.used}U 未占用</span><i/><span>{totals.servers} 服务器 · {totals.gpu} GPU · {totals.switches} 交换机</span><span className="spacer"/><span className="room-scale"><i/>U 占用率 <small>0—100%</small></span></div>
    <div className="room-workspace">
      <Suspense fallback={<div className="room-loading">正在加载机房渲染器…</div>}><RoomScene racks={model.racks} visible={visibleIds} selected={s?.rack.id||''} angle={params.get('roomCamera')||'oblique'} onAngle={roomCamera=>set({roomCamera})} onSelect={rack=>set({rack,asset:'',port:'',cable:''})} onOpen={id=>open(id)}/></Suspense>
      <aside className="room-inspector">{s?<>
        <div className="room-inspector-heading"><span>{shortPod(s.rack.fields.pod_id)} / 机柜</span><h2>{s.rack.name}</h2><code>{s.rack.id}</code><Button aria-label={`打开 ${s.rack.name} 双面详情`} onClick={()=>open(s.rack.id)}><ArrowUpRight size={16}/></Button></div>
        <div className="room-rack-capacity"><b>{s.layout?.occupied??'—'}<small> / {s.height??'—'}U</small></b><span>{s.layout?.free??'—'}U 未占用</span><div><i style={{width:`${s.height?(s.layout?.occupied||0)/s.height*100:0}%`}}/></div></div>
        <div className="room-asset-counts"><span><b>{s.servers}</b>服务器</span><span><b>{s.gpu}</b>GPU</span><span><b>{s.switches}</b>交换机</span></div>
        <div className="room-elevation-header"><b>登记立面</b><div className="rv-segment" role="tablist" aria-label="机房预览安装面"><button role="tab" aria-selected={face==='front'} onClick={()=>setFace('front')}>Front</button><button role="tab" aria-selected={face==='rear'} onClick={()=>setFace('rear')}>Rear</button></div></div>
        <div className="room-elevation"><Elevation snapshot={s} face={face} overlay="asset" selected={params.get('asset')||''} onSelect={asset=>open(s.rack.id,asset)} onPort={(asset,port)=>set({view:'spatial',rack:s.rack.id,asset,port,camera:'rear',tab:'mapping'})}/></div>
        <Button className="room-open-rack" onClick={()=>open(s.rack.id)}><Columns2 size={13}/>打开机柜双面详情<ArrowUpRight size={13}/></Button>
        <div className="room-device-heading"><Server size={13}/><b>柜内设备</b><span>{s.items.length}</span></div>
        <div className="room-device-list">{s.items.map(d=><button key={d.id} onClick={()=>open(s.rack.id,d.id)}><span><b>{d.name}</b><small>{d.fields.model||'型号未知'}</small></span><code>U{d.fields.rack_u??'—'}</code></button>)}</div>
        <div className="room-evidence"><Database size={12}/><span>设备位置事实层：{[...new Set(s.items.map(d=>d.fields.position_fact_layer||'未注明'))].join(' / ')}<br/>机房 XY / 宽深：未采集<br/>实时功耗 / 温度 / 健康：未知</span></div>
      </>:<div className="room-inspector-empty">调整搜索或 Pod 筛选后，选择机柜查看设备与容量。</div>}</aside>
    </div>
  </div>;
}
