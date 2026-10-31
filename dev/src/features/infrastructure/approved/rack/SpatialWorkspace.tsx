import {useEffect,useRef,useState} from 'react';
import {useIdentity} from '@/features/auth/IdentityProvider';
import {useProject} from '@/app/ProjectContext';
import {realCmdbApi} from '@/lib/api/cmdb-http';
import {ArrowUpRight,Box,ChevronLeft,ChevronRight,Cpu,Download,Expand,Eye,Focus,Layers3,Link2,Move,Network,RotateCcw,Save,ScanLine,Server,Undo2,Redo2} from 'lucide-react';
import {Dialog,DialogContent,DialogTitle,DialogDescription} from '@/components/shadcn/dialog';
import {Button,SearchBox} from '../shared';
import {useTopologyModel,exportFile,type Go} from '../model';
import {useRackModel,geometry,faceLabel,shortPod,gpuText,occupiedText,freeText,type RackSnapshot,type Overlay} from './model';
import {SpatialScene,type Camera} from './SpatialScene';
import {Elevation} from './Elevation';
import {Mapping,Source} from './Details';
import './spatial.css';

type Props={selectedId:string;snapshot:RackSnapshot;overlay:Overlay;params:URLSearchParams;set:(v:Record<string,string>)=>void;go:Go;onOpen:(id:string,asset?:string)=>void};
type ViewCamera=Camera|'dual';
const cameras:[ViewCamera,string][]=[['spatial','空间'],['dual','双面'],['front','正视'],['rear','背视'],['section','剖面']];
export function SpatialWorkspace({snapshot:base,overlay,params,set,go,onOpen,selectedId}:Props){
  const rackApi=useRackModel(),{name,openDevice}=useTopologyModel();
  const identity=useIdentity(),project=useProject();
  const draftKey='keel.visualization.rack-draft.'+identity.user.id+':'+base.rack.project_id+':'+base.rack.id;
  const fingerprint=base.rack.id+':'+base.rack.revision+'|'+base.items.map(d=>d.id+':'+d.revision).sort().join('|');
  const positionRequest=useRef<{project:string;fingerprint:string;changes:{op:string;asset_id:string;expected_revision:number;attributes:{u_start:number}}[]}|null>(null);
  const [requesting,setRequesting]=useState(false),[reason,setReason]=useState(''),[busy,setBusy]=useState(false);
  const [q,setQ]=useState(''),[edit,setEdit]=useState(false),[history,setHistory]=useState<Record<string,number>[]>([{}]),[cursor,setCursor]=useState(0),[save,setSave]=useState(false),[message,setMessage]=useState('');
  const [pan,setPan]=useState({x:0,y:0}),[zoom,setZoom]=useState(1);
  const [exportFormat,setExportFormat]=useState<'svg'|'png'|'pdf'>('svg');
  const snapshot=rackApi.snapshot(base.rack.id,edit?history[cursor]:{})!;
  const id=selectedId||snapshot.items.find(d=>d.type==='server')?.id||'';
  const device=snapshot.items.find(d=>d.id===id),g=device?geometry(device,edit?history[cursor]:{}):null;
  const tab=params.get('tab')||(params.get('port')||params.get('cable')||params.get('camera')==='rear'?'mapping':'asset');
  const baselineGeometry=device?geometry(device):null;
  const positionChanged=!!g&&g.u!==baselineGeometry?.u;
  const rawCamera=params.get('camera')||(params.get('view')==='dual'?'dual':'spatial');
  const camera=cameras.some(([v])=>v===rawCamera)?rawCamera as ViewCamera:'spatial';
  const selectedCable=rackApi.cable(params.get('cable')||'');
  const cable=selectedCable&&[selectedCable.device_a,selectedCable.device_b].includes(id)?selectedCable:undefined,focusIds=cable?[cable.device_a,cable.device_b]:[id];
  const explode=params.get('explode')==='1',isolate=params.get('isolate')==='1',cutaway=params.get('cutaway')!=='0';
  const scene=useRef<SVGSVGElement>(null),dual=useRef<HTMLDivElement>(null),rackIndex=rackApi.list().findIndex(r=>r.id===snapshot.rack.id);
  const sorted=[...snapshot.items].sort((a,b)=>Number(b.fields.rack_u)-Number(a.fields.rack_u));
  const previousFingerprint=useRef(fingerprint);
  useEffect(()=>{if(previousFingerprint.current!==fingerprint){previousFingerprint.current=fingerprint;setHistory([{}]);setCursor(0);setEdit(false);setSave(false);setRequesting(false);setMessage('CMDB 资产版本已变化，已退出旧展示草稿。')}},[fingerprint]);
  function chooseFace(asset:string,face:'front'|'rear'){set({asset,face,tab:face==='rear'?'mapping':'asset',...(asset!==id?{port:'',cable:''}:{})})}
  function choosePort(asset:string,port:string){const choices=rackApi.connections(port),existing=choices.find(c=>c.id===params.get('cable'));set({asset,port,cable:existing?.id||(choices.length===1?choices[0].id:''),face:'rear',tab:'mapping',camera:camera==='dual'?'dual':'rear'})}
  function select(asset:string){set({asset,port:'',cable:'',tab})}
  function locateEndpoint(asset:string,port:string){const peer=rackApi.asset(asset);if(!peer?.fields.rack_id)return;set({rack:peer.fields.rack_id,asset,port,cable:cable?.id||'',tab:'mapping',camera:'rear',face:'rear'})}
  function move(asset:string,u:number){setHistory([...history.slice(0,cursor+1),{...history[cursor],[asset]:u}]);setCursor(cursor+1);setMessage('展示草稿已调整。')}
  function restore(){const raw=localStorage.getItem(draftKey);if(!raw){setMessage('当前柜没有保存的草稿。');return}try{const saved=JSON.parse(raw);if(saved.fingerprint!==fingerprint){setMessage('资产版本已变化，旧草稿不再应用。');return}setHistory([{},saved.positions]);setCursor(1);setEdit(true);set({camera:'front'});setMessage('已恢复本地展示草稿。')}catch{setMessage('草稿无法读取，请重新调整。')}}
  function beginPositionRequest(){
    if(!project.filter){setMessage('请先限定项目，再申请实际安装位置变更。');return}
    const changes=snapshot.items.filter(d=>history[cursor][d.id]!==undefined&&history[cursor][d.id]!==geometry(d).u).map(d=>({op:'asset.relocate',asset_id:d.id,expected_revision:d.revision,attributes:{u_start:history[cursor][d.id]}}));
    positionRequest.current={project:project.filter,fingerprint,changes};setMessage('');setRequesting(true);
  }
  async function requestPosition(){
    setBusy(true);
    try{
      const frozen=positionRequest.current;
      if(!frozen||project.filter!==frozen.project||fingerprint!==frozen.fingerprint)throw Error('项目或资产版本已变化，请重新确认变更范围');
      if(!frozen.changes.length)throw Error('没有安装位置差异');
      if(snapshot.layout?.issues.some(i=>['overlap','range','depth'].includes(i.kind)))throw Error('请先修正草稿中的占位或尺寸冲突');
      const schema=await realCmdbApi.schema();
      const result=await realCmdbApi.createChangeSet({project_id:frozen.project,title:'机柜安装位置调整 '+snapshot.rack.name,reason,schema_version:schema.data.version,changes:frozen.changes});
      setRequesting(false);setMessage('已创建 CMDB 变更草稿 '+result.data.id+'；审核发布前安装事实不变。');
    }catch(e){setMessage((e as Error).message)}finally{setBusy(false)}
  }
  function exportSvg(){
    const css=Array.from(document.styleSheets).flatMap(s=>{try{return Array.from(s.cssRules).map(r=>r.cssText)}catch{return[]}}).filter(s=>s.includes('sr-')||s.includes('rv-svg')||s.includes('rv-connection-panel')||s.includes('rv-port-chip')).join('\n');
    const ns='http://www.w3.org/2000/svg';
    let result:SVGSVGElement;
    if(camera==='dual'){
      const svgs=dual.current?.querySelectorAll<SVGSVGElement>('.sr-dual-pair > svg');if(!svgs?.length)return null;
      const h=svgs[0].viewBox.baseVal.height;
      result=document.createElementNS(ns,'svg');result.setAttribute('viewBox',`0 0 640 ${h+45}`);
      const background=document.createElementNS(ns,'rect');background.setAttribute('width','100%');background.setAttribute('height','100%');background.setAttribute('fill','#eef2f6');result.append(background);
      Array.from(svgs).forEach((svg,i)=>{const copy=svg.cloneNode(true) as SVGSVGElement;copy.setAttribute('x',String(i*320));copy.setAttribute('y','35');copy.setAttribute('width','320');copy.setAttribute('height',String(h));result.append(copy)});
    }else{
      if(!scene.current)return null;result=scene.current.cloneNode(true) as SVGSVGElement;
    }
    result.setAttribute('xmlns',ns);result.setAttribute('width',String(result.viewBox.baseVal.width));result.setAttribute('height',String(result.viewBox.baseVal.height));
    const style=document.createElementNS(ns,'style');style.textContent=css;result.prepend(style);
    const title=document.createElementNS(ns,'text');title.setAttribute('x','20');title.setAttribute('y','20');title.setAttribute('font-size','10');title.textContent=`${snapshot.rack.name} · CMDB · ${edit?'展示草稿':'登记占位'} · 非实装测量图`;result.append(title);
    return result;
  }
  async function exportScene(){
    const svg=exportSvg();if(!svg){setMessage('当前没有可导出的机柜图。');return}
    const filename=snapshot.rack.name+'-'+camera;
    try{
      if(exportFormat==='pdf'){
        // A separate print document contains only the exported rack, never the application shell.
        const frame=document.createElement('iframe');frame.title='机柜图打印';frame.style.cssText='position:fixed;width:1px;height:1px;right:0;bottom:0;border:0';document.body.append(frame);
        const doc=frame.contentDocument!,win=frame.contentWindow!;
        doc.title=filename;const style=doc.createElement('style');style.textContent='@page{size:A4 landscape;margin:12mm}html,body{margin:0;background:white}body>svg{display:block;width:100%;height:180mm}';doc.head.append(style);doc.body.append(doc.importNode(svg,true));
        win.addEventListener('afterprint',()=>frame.remove(),{once:true});await doc.fonts.ready;await new Promise<void>(resolve=>win.requestAnimationFrame(()=>resolve()));win.focus();win.print();setMessage('已打开机柜图打印，可在打印目标中选择保存为 PDF。');return;
      }
      const xml=new XMLSerializer().serializeToString(svg);
      if(exportFormat==='svg'){exportFile(filename+'.svg',xml,'image/svg+xml');return}
      const url=URL.createObjectURL(new Blob([xml],{type:'image/svg+xml'}));
      try{
        const image=new Image();image.src=url;await image.decode();
        const canvas=document.createElement('canvas');canvas.width=Math.ceil(svg.viewBox.baseVal.width*2);canvas.height=Math.ceil(svg.viewBox.baseVal.height*2);
        const ctx=canvas.getContext('2d');if(!ctx)throw Error('浏览器无法创建 PNG 画布');ctx.drawImage(image,0,0,canvas.width,canvas.height);
        const blob=await new Promise<Blob|null>(resolve=>canvas.toBlob(resolve,'image/png'));if(!blob)throw Error('PNG 导出失败');
        const downloadUrl=URL.createObjectURL(blob),link=document.createElement('a');link.href=downloadUrl;link.download=filename+'.png';link.click();setTimeout(()=>URL.revokeObjectURL(downloadUrl),1000);
      }finally{URL.revokeObjectURL(url)}
    }catch(e){setMessage('导出失败：'+(e as Error).message)}
  }
  const tabs=[['asset','资产'],['mapping','面板与端接'],['installation','安装'],['capacity','容量'],['observation','观测'],['source','来源']];
  const viewModels={asset:'资产',capacity:'空间',compute:'计算资源'};
  return <div className="sr-workspace">
    <aside className="sr-object-tree"><div className="sr-tree-heading"><Layers3 size={14}/><b>柜内对象</b><span>{snapshot.items.length}</span></div><SearchBox value={q} onChange={setQ} placeholder="搜索设备"/><div className="sr-object-list">{sorted.filter(d=>[d.id,d.name,d.fields.model].join(' ').toLowerCase().includes(q.toLowerCase())).map(d=><button key={d.id} onClick={()=>select(d.id)} className={device?.id===d.id?'active':''}><span className={'sr-tree-icon '+d.type}>{d.type==='server'?<Server size={15}/>:<Network size={15}/>}</span><span><b>{d.name}</b><small>{d.fields.u_height===0?'0U 挂装':`U${d.fields.rack_u??'—'}`}{edit&&history[cursor][d.id]!=null&&history[cursor][d.id]!==d.fields.rack_u&&<em> → {history[cursor][d.id]} 草稿</em>} · {d.fields.u_height}U</small></span></button>)}</div><div className="sr-tree-summary"><div><span>服务器</span><b>{snapshot.servers}</b></div><div><span>交换机</span><b>{snapshot.switches}</b></div><div><span>登记 GPU</span><b>{gpuText(snapshot)}</b></div></div><RackContext current={snapshot.rack.id} onOpen={onOpen}/><div className="sr-rack-navigation"><small>{shortPod(snapshot.rack.fields.pod_id)}</small><b>{snapshot.rack.name}</b><div><Button aria-label="上一个机柜" disabled={rackIndex===0} onClick={()=>onOpen(rackApi.list()[rackIndex-1].id)}><ChevronLeft size={14}/></Button><Button aria-label="下一个机柜" disabled={rackIndex===rackApi.list().length-1} onClick={()=>onOpen(rackApi.list()[rackIndex+1].id)}><ChevronRight size={14}/></Button></div></div></aside>
    <section className="sr-space"><div className="sr-scene-toolbar"><div className="rv-segment" role="tablist" aria-label="工程视角">{cameras.map(([v,label])=><button key={v} role="tab" aria-selected={camera===v} onClick={()=>{set({camera:v,...(v==='front'?{face:'front',tab:'asset'}:v==='rear'?{face:'rear',tab:'mapping'}:v==='section'?{tab:'installation'}:{})});setPan({x:0,y:0});setZoom(1)}}>{label}</button>)}</div><span className="spacer"/><Button title="隐藏参考柜侧板，查看内部关系" aria-label="透视柜体" aria-pressed={cutaway} disabled={camera!=='spatial'} onClick={()=>set({cutaway:cutaway?'0':'1'})}><Eye size={14}/></Button><Button title="仅突出当前设备及选中连接的两端" aria-label="聚焦设备" aria-pressed={isolate} disabled={!device} onClick={()=>set({isolate:isolate?'0':'1'})}><Focus size={14}/></Button><Button title="分离选中设备的展示平面，不修改安装位置" aria-label="分解设备" aria-pressed={explode} disabled={!device||camera!=='spatial'} onClick={()=>set({explode:explode?'0':'1'})}><Layers3 size={14}/></Button></div>
      <div className="sr-stage">{camera==='dual'?<div className="sr-dual" ref={dual}><div className="sr-dual-labels"><button className={tab!=='mapping'?'active':''} onClick={()=>chooseFace(id,'front')}>FRONT <span>资产与安装</span></button><button className={tab==='mapping'?'active':''} onClick={()=>chooseFace(id,'rear')}>REAR <span>端口与连接</span></button></div><div className="sr-dual-pair" style={{transform:`scale(${zoom})`,transformOrigin:'50% 20%'}}>{(['front','rear'] as const).map(face=><Elevation key={face} snapshot={snapshot} face={face} overlay={overlay} selected={device?.id} onSelect={asset=>chooseFace(asset,face)} onPort={choosePort} selectedPort={params.get('port')||''} onMove={edit?move:undefined} focusIds={isolate?focusIds:undefined} relatedIds={cable?focusIds:[]}/>)}</div><span className="sr-dual-note">同一资产同步选中 · 左侧查资产，右侧查连接 · 端口为索引，非实物坐标</span></div>:<SpatialScene snapshot={snapshot} selected={device?.id||''} camera={camera} overlay={overlay} explode={explode} isolate={isolate} cutaway={cutaway} cable={cable?.id||''} zoom={zoom} pan={pan} onSelect={select} onMove={edit&&camera!=='section'?move:undefined} onPan={setPan} svgRef={scene} onPort={choosePort} selectedPort={params.get('port')||''} onEndpoint={locateEndpoint} draft={edit}/>}<div className="sr-scene-badge"><i/>{camera==='rear'?'端口与连接':camera==='front'?'资产与安装':viewModels[overlay]+'视图'} {edit&&<strong>展示草稿</strong>}</div><div className="sr-zoom"><button aria-label="缩小空间视图" disabled={zoom<=.8} onClick={()=>setZoom(Math.max(.8,zoom-.2))}>−</button><span>{Math.round(zoom*100)}%</span><button aria-label="放大空间视图" disabled={zoom>=1.8} onClick={()=>setZoom(Math.min(1.8,zoom+.2))}>+</button><button aria-label="适配空间视图" onClick={()=>{setPan({x:0,y:0});setZoom(1)}}><Expand size={13}/></button></div></div>
      {snapshot.layout&&[...snapshot.layout.zeroU,...snapshot.layout.unplaced].length>0&&<div className="sr-unplaced"><b>0U / 未定位</b>{[...snapshot.layout.zeroU,...snapshot.layout.unplaced].map(p=><button key={p.id} onClick={()=>select(p.id)}>{name(p.id)} · {p.height===0?'0U，挂装位置待补齐':'缺少位置 / 高度'}</button>)}</div>}<div className="sr-u-navigator"><span>U1</span><div>{Array.from({length:snapshot.height||0},(_,i)=>{const u=i+1,parts=snapshot.layout?.placed.filter(p=>p.u<=u&&p.u+p.height>u)||[],p=parts[0];return <button key={u} disabled={!p} title={`U${u}${p?' · '+name(p.id):' · 未占用'}`} aria-label={`U${u} ${p?name(p.id):'未占用'}`} className={p?.id===device?.id?'selected':p?'occupied':''} onClick={()=>p&&select(p.id)}/>})}</div><span>U{snapshot.height}</span></div>
      <div className="sr-scene-bottom"><span><i className="sr-legend-solid"/>{edit?'草稿位置':'登记位置'}</span><span><i className="sr-legend-dash"/>未定界几何</span><span className="spacer"/><Button onClick={()=>{setEdit(!edit);set({camera:'front',explode:''});setMessage('')}} aria-pressed={edit}><Move size={12}/>{edit?'退出草稿':'安装草稿'}</Button><select className="sr-export-format" aria-label="机柜图导出格式" value={exportFormat} onChange={e=>setExportFormat(e.target.value as typeof exportFormat)}><option value="svg">SVG</option><option value="png">PNG</option><option value="pdf">PDF</option></select><Button onClick={exportScene}><Download size={12}/>导出</Button></div>
      {edit&&<div className="sr-draft-controls"><span>0.5U 吸附 · 仅展示</span><Button disabled={!project.filter||!project.permissions.includes('cmdb:edit')||!cursor} title="需要限定项目、CMDB 编辑权限和位置差异" onClick={beginPositionRequest}>申请位置变更</Button><Button aria-label="撤销草稿" disabled={!cursor} onClick={()=>setCursor(cursor-1)}><Undo2 size={13}/></Button><Button aria-label="重做草稿" disabled={cursor>=history.length-1} onClick={()=>setCursor(cursor+1)}><Redo2 size={13}/></Button><Button onClick={restore}><RotateCcw size={12}/>恢复</Button><Button disabled={!Object.keys(history[cursor]).length||!!snapshot.layout?.issues.length} onClick={()=>setSave(true)}><Save size={12}/>保存草稿</Button></div>}
      {snapshot.layout?.issues.length?<div className="sr-alert" role="alert">{snapshot.layout.issues.map((i,n)=><div key={n}>{i.ids.map(name).join(' / ')}：{i.message}</div>)}</div>:null}
      {message&&<div role="status" className="sr-message">{message}</div>}
    </section>
    <aside className="sr-engineering"><div className="sr-engineering-heading"><span className="sr-kicker">{device?.type==='server'?'COMPUTE / SERVER':device?.type==='switch'?'FABRIC / SWITCH':'RACK / SPACE'}</span><h2>{device?.name||snapshot.rack.name}</h2><span>{device?.fields.model||`${snapshot.height}U · ${snapshot.items.length} 设备`}</span><div className="sr-identity"><code>{device?.id||snapshot.rack.id}</code><span>rev. {device?.revision||snapshot.rack.revision}</span></div></div>
      {id&&!device&&<div className="sr-alert">此设备不属于当前机柜；请按资产所在位置定位。</div>}
      <div className="sr-detail-tabs" role="tablist" aria-label="工程详情">{tabs.map(([v,label])=><button key={v} role="tab" aria-selected={tab===v} onClick={()=>set({tab:v})}>{label}</button>)}</div>
      <div className="sr-details">
        {tab==='mapping'?device?<><div className="sr-section-heading"><Link2 size={14}/><b>端口级关系</b><span>索引，非安装坐标</span></div><Mapping key={id} id={id} params={params} set={set} go={go} onEndpoint={locateEndpoint}/></>:<p>选择柜内设备查看端口和连接。</p>:tab==='source'?<Source selected={device?.id} snapshot={snapshot}/>:tab==='installation'?<Installation snapshot={snapshot} deviceId={device?.id||''} draft={edit}/>:tab==='capacity'?<Capacity snapshot={snapshot} draft={edit}/>:tab==='observation'?<Observation deviceId={device?.id||snapshot.rack.id}/>:device&&g?<>
          <div className="sr-property-strip"><div><b>{g.height===0?'0U 挂装':`U${g.u??'—'}`}</b><span>{positionChanged?'展示草稿':'登记位置'}</span>{positionChanged&&<small className="sr-baseline-u">登记 U{baselineGeometry?.u}</small>}</div><div><b>{g.height??'—'}U</b><span>设备高度</span></div><div><b>{rackApi.ports(device.id).length}</b><span>登记端口</span></div></div>
          <div className="sr-section-heading"><Cpu size={14}/><b>设备与组成</b></div>
          <dl className="sr-property-grid"><dt>资产编号</dt><dd>{device.fields.serial||'未记录'}</dd><dt>项目</dt><dd>{device.project_id}</dd>{device.type==='server'?<><dt>GPU / NIC</dt><dd>{device.fields.gpu_count??'—'} / {device.fields.nic_count??'—'} <small>登记数量</small></dd><dt>CPU / 内存</dt><dd>{device.fields.cpu||'未采集'} / {device.fields.memory_gb?device.fields.memory_gb+' GB':'未采集'}</dd></>:<><dt>网络角色</dt><dd>{device.fields.role||'未记录'}</dd><dt>NOS</dt><dd>{device.fields.nos_version||'未采集'}</dd></>}<dt>运维状态</dt><dd>{device.fields.maintenance_state||'未记录'} <small>登记状态</small></dd></dl>
          <div className="sr-section-heading"><Box size={14}/><b>安装几何</b><button onClick={()=>set({tab:'installation',camera:'section'})}>查看剖面<ArrowUpRight size={12}/></button></div><dl className="sr-property-grid"><dt>登记安装</dt><dd>{snapshot.rack.name} → {g.height===0?'0U，挂装位置待补齐':`U${baselineGeometry?.u??'—'}`}</dd>{positionChanged&&<><dt>草稿预览</dt><dd className="sr-draft-value">U{g.u} · 未写入 CMDB</dd></>}<dt>安装面 / 深度</dt><dd>{faceLabel(g.face)} / {g.depth===null?'未记录':g.depth+' mm'}</dd><dt>深度占用</dt><dd>{g.fullDepth===null?'未记录':g.fullDepth?'全深度 · 对面同 U 保留':'非全深度 · 毫米尺寸独立核查'}</dd><dt>面板</dt><dd>标准资产符号 <small>精确型号素材待绑定</small></dd></dl>

          <div className="sr-inline-actions"><Button onClick={()=>openDevice?.(device)}>设备完整详情<ArrowUpRight size={13}/></Button><Button onClick={()=>set({tab:'mapping',camera:'rear',face:'rear'})}><Link2 size={13}/>关联设备与线缆</Button><Button onClick={()=>go('global-topology',{asset:id,scope:snapshot.rack.fields.pod_id})}><ArrowUpRight size={13}/>定位拓扑</Button></div>
        </>:<Capacity snapshot={snapshot} draft={edit}/>}
      </div>
    </aside>
    <Dialog open={requesting} onOpenChange={setRequesting}><DialogContent><DialogTitle>申请修改实际安装位置</DialogTitle><DialogDescription>创建 CMDB ChangeSet 草稿。需要既有审核发布流程，不直接改变实际位置。确认范围：{positionRequest.current?.project} / {positionRequest.current?.changes.length||0} 台设备，版本已冻结。</DialogDescription><textarea aria-label="位置变更原因" value={reason} onChange={e=>setReason(e.target.value)} placeholder="填写现场依据和变更原因"/><Button disabled={busy||!reason.trim()} onClick={requestPosition}>确认创建变更草稿</Button>{message&&<p role="alert">{message}</p>}</DialogContent></Dialog><Dialog open={save} onOpenChange={setSave}><DialogContent><DialogTitle>保存展示草稿</DialogTitle><DialogDescription>本地记录视图位置，不修改 CMDB 安装事实，不产生实装签署。实际位置变更继续使用既有 ChangeSet。</DialogDescription><table><thead><tr><th>设备</th><th>登记</th><th>草稿</th></tr></thead><tbody>{Object.entries(history[cursor]).map(([asset,u])=><tr key={asset}><td>{name(asset)}</td><td>U{rackApi.asset(asset)?.fields.rack_u}</td><td>U{u}</td></tr>)}</tbody></table><Button variant="default" onClick={()=>{localStorage.setItem(draftKey,JSON.stringify({fingerprint,positions:history[cursor]}));setSave(false);setMessage('展示草稿已保存；CMDB 安装位置未变。')}}>确认保存展示草稿</Button></DialogContent></Dialog>
  </div>
}
function RackContext({current,onOpen}:{current:string;onOpen:(id:string)=>void}){const rackApi=useRackModel();const racks=rackApi.list(),pods=[...new Set(racks.map(r=>r.fields.pod_id))];return <section className="sr-context-map"><div><b>机柜范围</b><button onClick={()=>onOpen(current)}>定位当前</button></div>{pods.map(pod=><div className="sr-context-pod" key={pod}><small>{shortPod(pod)}</small><div>{racks.filter(r=>r.fields.pod_id===pod).map(r=>{const s=rackApi.snapshot(r.id)!;return <button key={r.id} onClick={()=>onOpen(r.id)} aria-label={`范围定位 ${r.name}`} aria-pressed={r.id===current} title={`${r.name} · ${s.layout?.occupied}/${s.height}U`}><i style={{height:`${s.height?(s.layout?.occupied||0)/s.height*100:0}%`}}/><span>{r.name.replace('RACK-','')}</span></button>})}</div></div>)}</section>}
function Installation({snapshot,deviceId,draft}:{snapshot:RackSnapshot;deviceId:string;draft:boolean}){const rackApi=useRackModel();const d=rackApi.asset(deviceId),g=d?geometry(d):null,preview=snapshot.layout?.placed.find(p=>p.id===deviceId);return <><div className="sr-section-heading"><ScanLine size={14}/><b>安装约束与尺寸</b><span>规划登记</span></div><div className="sr-section-diagram"><span>FRONT</span><div><i/>{g?.depth?`${g.depth} mm`:'设备深度未定'}<i/></div><span>REAR</span></div><dl className="sr-property-grid"><dt>机柜 U 高</dt><dd>{snapshot.height}U</dd><dt>机柜宽 × 深</dt><dd>{snapshot.rack.fields.width_mm??'—'} × {snapshot.rack.fields.depth_mm??'—'} mm</dd><dt>设备 U / U 高</dt><dd>{g?(g.height===0?'0U · 不分配竖向 U 位':`U${g.u??'—'} / ${g.height??'—'}U`):'请选择设备'}</dd>{draft&&preview?.u!==g?.u&&<><dt>草稿 U 位</dt><dd className="sr-draft-value">U{preview?.u??'—'} · 未写入 CMDB</dd></>}<dt>设备宽 × 深</dt><dd>{d?.fields.width_mm??'—'} × {g?.depth??'—'} mm</dd><dt>安装面</dt><dd>{g?faceLabel(g.face):'—'}</dd><dt>全深度占用</dt><dd>{g?.fullDepth==null?'未记录':g.fullDepth?'是':'否'}</dd></dl><div className="sr-constraint-list"><div><b>{draft?'草稿 U 区间':'登记 U 区间'}</b><span>{snapshot.layout?.issues.length?'有冲突 / 待确认项':'未发现区间冲突'}</span></div><div><b>前后净空</b><span>尺寸与安装面缺失，无法核算</span></div><div><b>维护抽出空间</b><span>缺机房通道与厂家维护包络</span></div><div><b>实装确认</b><span>未提供，不能由规划自动转正</span></div></div><p className="sr-footnote">剖面中的开放边界保留未知范围。它不是猜测出的机箱深度，也不代表安装可行性已通过。</p></>}
function Capacity({snapshot,draft}:{snapshot:RackSnapshot;draft:boolean}){const layout=snapshot.layout;return <><div className="sr-section-heading"><Layers3 size={14}/><b>空间与资源容量</b></div><div className="sr-capacity-number"><b>{occupiedText(snapshot)}<small> / {snapshot.height}U</small></b><span>{draft?'草稿占位并集':'登记占位并集'}</span></div><div className="sr-capacity-bar"><span style={{width:`${snapshot.height?(layout?.occupied||0)/snapshot.height*100:0}%`}}/></div><div className="sr-capacity-legend"><span>{freeText(snapshot)}U 未占用</span><span>前后不重复计数</span></div><h3>{snapshot.placementComplete?'连续未占用区间':'已加载位置之间的间隔（非空闲结论）'}</h3><div className="sr-free-ranges">{layout?.freeRanges.map(r=><div key={r.u}><span>U{r.u} 起</span><b>{r.height}U</b></div>)}</div><dl className="sr-property-grid"><dt>登记服务器</dt><dd>{snapshot.servers}</dd><dt>登记 GPU</dt><dd>{gpuText(snapshot)} <small>{snapshot.gpuCoverage}/{snapshot.gpuTotal} 台有数量</small></dd><dt>交换机 / 端口</dt><dd>{snapshot.switches} / {snapshot.ports.length}</dd><dt>关联 Cable</dt><dd>{snapshot.connections.length}</dd><dt>可安装容量</dt><dd>未确认 <small>缺深度、承重及供电依据</small></dd><dt>功率 / 冷却余量</dt><dd>未采集</dd></dl></>}
function Observation({deviceId}:{deviceId:string}){const rackApi=useRackModel();const d=rackApi.asset(deviceId);return <><div className="sr-section-heading"><Eye size={14}/><b>运行状态与证据</b><span>当前无有效遥测</span></div><table className="sr-observation-table"><thead><tr><th>指标域</th><th>当前值</th><th>可用性</th></tr></thead><tbody>{['硬件健康','功耗 · W','温度 · °C','风扇 / 电源','GPU 利用率 · %'].map(label=><tr key={label}><td>{label}</td><td>—</td><td>unknown</td></tr>)}</tbody></table><dl className="sr-property-grid"><dt>历史登记值</dt><dd>{d?.fields.health_status||'未记录'}</dd><dt>历史来源</dt><dd>{d?.source||'未提供'}</dd><dt>参考时间</dt><dd>{rackApi.source.referenceAt}</dd><dt>当前采集时间</dt><dd>未提供</dd><dt>freshness</dt><dd>unknown</dd></dl><div className="sr-observation-note"><b>遥测绑定后启用对应图层</b><p>需要具体测点、作用范围、单位和有效时间。登记健康值与 BOM 功率预算均不作为实时监测结果。</p></div></>}
