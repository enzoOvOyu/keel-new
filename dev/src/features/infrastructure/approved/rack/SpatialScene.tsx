import {useId,useRef} from 'react';
import type {RefObject} from 'react';
import {type RackSnapshot,type Overlay,useRackModel,geometry} from './model';
import type {InfraCable} from '@/lib/api/infrastructure';
import {ConnectionPanel} from './ConnectionPanel';
import {unitY,faceProjection,sectionProjection} from './geometry';

export type Camera='spatial'|'front'|'rear'|'section';
export type SceneProps={snapshot:RackSnapshot;selected:string;camera:Camera;overlay:Overlay;explode:boolean;isolate:boolean;cutaway:boolean;cable:string;zoom:number;pan:{x:number;y:number};onSelect:(id:string)=>void;onMove?:(id:string,u:number)=>void;onPan:(p:{x:number;y:number})=>void;svgRef:RefObject<SVGSVGElement|null>;onEndpoint:(asset:string,port:string)=>void;draft:boolean;selectedPort:string;onPort:(asset:string,port:string)=>void};
// Projection coordinates are view-space. Unmeasured depth NEVER becomes a device dimension.
export function SpatialScene({snapshot,selected,camera,overlay,explode,isolate,cutaway,cable,zoom,pan,onSelect,onMove,onPan,svgRef,onEndpoint,draft,selectedPort,onPort}:SceneProps){
  const rackApi=useRackModel();
  const uid=useId().replaceAll(':',''),drag=useRef<{id?:string;x:number;y:number;u:number}|null>(null);
  const height=snapshot.height,layout=snapshot.layout;
  if(!height||!layout)return <div className="sr-no-data">缺少机柜高度，无法建立空间坐标。</div>;
  const top=106,unit=12.8,bottom=top+height*unit,W=238,X=camera==='section'?232:160;
  const spatial=camera==='spatial',section=camera==='section',dx=spatial?130:0,dy=spatial?-61:0;
  const selectedP=layout.placed.find(p=>p.id===selected),selectedAsset=snapshot.items.find(d=>d.id===selected);
  const connection=rackApi.cable(cable),highlight=new Set(connection?[connection.device_a,connection.device_b]:[selected]);
    const toPoint=(e:{clientX:number;clientY:number})=>{const b=svgRef.current!.getBoundingClientRect(),scale=Math.min(b.width/680,b.height/770);return{x:e.clientX/scale,y:e.clientY/scale}};
  const axisLabel=snapshot.rack.fields.depth_mm?`${snapshot.rack.fields.depth_mm} mm`:'深度未标定';
  const selectedY=selectedP?unitY(height,selectedP.u,selectedP.height,unit,top):0;
  const sectionX=138,sectionWidth=390;
  const dimension=(x:number,y1:number,y2:number,label:string)=><g className="sr-dimension"><path d={`M${x} ${y1}V${y2}M${x-4} ${y1}h8M${x-4} ${y2}h8`}/><text x={x-8} y={(y1+y2)/2} textAnchor="end">{label}</text></g>;
  return <svg ref={svgRef} className="sr-scene-svg" viewBox="0 0 680 770" role="group" aria-label={`${snapshot.rack.name} 工程空间视图`} data-camera={camera} xmlns="http://www.w3.org/2000/svg"
    onPointerMove={e=>{const d=drag.current;if(!d||d.id)return;const pt=toPoint(e);onPan({x:d.u+(pt.x-d.x),y:Number(e.currentTarget.dataset.panY)+(pt.y-d.y)})}}
    onPointerUp={e=>{const d=drag.current;if(!d)return;const pt=toPoint(e);if(d.id&&onMove){const delta=(d.y-pt.y)/zoom;if(Math.abs(delta)>unit/4)onMove(d.id,Math.round((d.u+delta/unit)*2)/2);else onSelect(d.id)}drag.current=null}}
    onPointerCancel={()=>{drag.current=null}}>
    <defs><pattern id={`sr-grid-${uid}`} width="24" height="24" patternUnits="userSpaceOnUse"><path d="M24 0H0V24" fill="none" stroke="#d8e0ea" strokeWidth=".5"/></pattern><pattern id={`sr-free-${uid}`} width="7" height="7" patternUnits="userSpaceOnUse"><path d="M0 7L7 0" stroke="#7890aa" strokeOpacity=".32" strokeWidth="1"/></pattern><pattern id={`sr-unknown-${uid}`} width="5" height="5" patternUnits="userSpaceOnUse"><circle cx="1" cy="1" r=".65" fill="#a9b9cd"/></pattern></defs>
    <rect width="680" height="770" fill="#eef2f6"/>
    <rect width="680" height="770" fill={`url(#sr-grid-${uid})`} onPointerDown={e=>{const pt=toPoint(e);e.currentTarget.setPointerCapture(e.pointerId);svgRef.current!.dataset.panY=String(pan.y);drag.current={x:pt.x,y:pt.y,u:pan.x}}} className="sr-pan-background"/>
    <g transform={`translate(${pan.x} ${pan.y}) translate(340 385) scale(${zoom}) translate(-340 -385)`}>
      {section?<>
        <text x="138" y="75" className="sr-scene-heading">安装剖面 / {selectedAsset?.name||'选取设备'}</text>
        <path d={`M${sectionX} ${top}V${bottom}H${sectionX+sectionWidth}V${top}`} className="sr-section-outline"/>
        <path d={`M${sectionX} ${top}H${sectionX+sectionWidth}`} className="sr-guide"/>
        <text x={sectionX} y={top-12} className="sr-axis-note">FRONT 基准</text><text x={sectionX+sectionWidth} y={top-12} textAnchor="end" className="sr-axis-note">REAR 基准</text>
        {layout.placed.map(p=>{const y=unitY(height,p.u,p.height,unit,top),projection=sectionProjection(p,Number(snapshot.rack.fields.depth_mm)||null),known=projection.anchored,r=projection.ratio;return <g key={p.id} role="button" tabIndex={0} aria-label={`剖面 ${rackApi.asset(p.id)?.name}`} onClick={()=>onSelect(p.id)} onKeyDown={e=>{if(e.key==='Enter')onSelect(p.id)}} opacity={isolate&&selected&&p.id!==selected?0.35:1}>
          <rect x={sectionX+1+(known?sectionWidth*projection.offset:0)} y={y+1} width={known?sectionWidth*r!:sectionWidth-2} height={p.height*unit-2} fill={known?'#627c9c':`url(#sr-unknown-${uid})`} stroke={p.id===selected?'#6473df':'#aab8c9'} strokeDasharray={known?undefined:'5 4'} strokeWidth={p.id===selected?2:1}/>
          {p.height>=3&&<><text x={sectionX+14} y={y+22} className="sr-section-name">{rackApi.asset(p.id)?.name}</text><text x={sectionX+14} y={y+40} className="sr-axis-note">{known?`${p.depth} mm · ${p.face==='rear'?'背面安装':'正面安装'}`:p.depth!==null?`${p.depth} mm · 安装基准未定`:'深度范围未定 · 不计算净空'}</text></>}
        </g>})}
        {dimension(105,top,bottom,`${height}U`)}
        <text x="333" y={bottom+30} className="sr-axis-note" textAnchor="middle">{axisLabel} · 参照区间不是设备体积</text>
      </>:<>
        {/* Rear reference plane: shown as an open boundary, never an invented filled chassis. */}
        {spatial&&<g opacity={cutaway ? .48 : .88}>
          <path d={`M${X+dx} ${top+dy}H${X+W+dx}V${bottom+dy}H${X+dx}Z`} fill="none" stroke="#8e9fb4" strokeWidth="5"/>
          <path d={`M${X} ${top}L${X+dx} ${top+dy}M${X+W} ${top}L${X+W+dx} ${top+dy}M${X} ${bottom}L${X+dx} ${bottom+dy}M${X+W} ${bottom}L${X+W+dx} ${bottom+dy}`} stroke="#8b9db3" fill="none" strokeWidth="3"/>
          {!cutaway&&<path d={`M${X+W} ${top}L${X+W+dx} ${top+dy}V${bottom+dy}L${X+W} ${bottom}Z`} fill="#c1ccd9" fillOpacity=".28" stroke="#8c9fb7" strokeDasharray="5 4"/>}
          <text x={X+dx+W/2} y={top+dy-16} className="sr-axis-note" textAnchor="middle">REAR / 参考平面</text>
          <path d={`M${X+W+14} ${bottom+20}l${dx} ${dy}`} className="sr-guide"/><text x={X+W+dx/2+18} y={bottom+dy/2+40} className="sr-axis-note">{axisLabel}</text>
        </g>}
        <rect x={X-12} y={top-18} width={W+24} height={height*unit+38} rx="4" className="sr-cabinet-frame"/>
        <rect x={X} y={top} width={W} height={height*unit} fill="#172437"/>
        <text x={X+10} y={top-5} className="sr-frame-id">{snapshot.rack.name}</text><text x={X+W-8} y={top-5} textAnchor="end" className="sr-frame-id">{camera==='rear'?'REAR':'FRONT'}</text>
        {Array.from({length:height},(_,i)=>{const u=height-i,y=top+i*unit;return <g key={u}><line x1={X} x2={X+W} y1={y+unit} y2={y+unit} stroke="#2b3c52" strokeWidth=".6"/><text x={X-22} y={y+9} className="sr-u-label" textAnchor="end">{u}</text><rect x={X-8} y={y+4} width="3" height="4" fill="#7d8da4"/><rect x={X+W+5} y={y+4} width="3" height="4" fill="#7d8da4"/></g>})}
        {overlay==='capacity'&&layout.freeRanges.map(r=><g key={r.u}><rect x={X+1} y={unitY(height,r.u,r.height,unit,top)} width={W-2} height={r.height*unit} fill={`url(#sr-free-${uid})`}/>{r.height>=2&&<text x={X+W/2} y={unitY(height,r.u,r.height,unit,top)+r.height*unit/2+3} textAnchor="middle" className="sr-free-label">{r.height}U 未占用</text>}</g>)}
        {[...layout.placed].sort((a,b)=>Number(a.id===selected)-Number(b.id===selected)).map(p=>{
          const d=snapshot.items.find(d=>d.id===p.id)!,y=unitY(height,p.u,p.height,unit,top),h=p.height*unit,active=selected===p.id;
          const projection=faceProjection(p,camera==='rear'?'rear':'front');
          if(!spatial&&!projection.visible)return null;
          const rearMounted=spatial&&p.face==='rear';
          const offset=(rearMounted?dx:0)+(active&&explode?-70:0),oy=(rearMounted?dy:0)+(active&&explode?32:0);
          const dim=isolate&&selected&&!highlight.has(p.id),bad=layout.issues.some(i=>i.ids.includes(p.id));
          const depthKnown=sectionProjection(p,Number(snapshot.rack.fields.depth_mm)||null).anchored;
          const ratio=depthKnown?p.depth!/Number(snapshot.rack.fields.depth_mm):0;
          const faceMismatch=projection.kind==='reference';
          const vx=(rearMounted?-dx:dx)*(depthKnown?ratio:1),vy=(rearMounted?-dy:dy)*(depthKnown?ratio:1);
          return <g key={p.id} data-device={p.id} data-u={p.u} data-depth-known={depthKnown} data-projection={spatial&&rearMounted?'mounted':projection.kind} aria-pressed={active} role="button" tabIndex={0} aria-label={`${d.name} U${p.u} ${p.height}U`} className={`sr-device ${d.type} ${active?'selected':''} ${connection&&highlight.has(p.id)?'connected':''} ${bad?'conflict':''} ${overlay==='compute'&&d.type==='server'?'compute':''}`} opacity={dim ? .22 : 1}
            onClick={onMove?undefined:()=>onSelect(p.id)} onKeyDown={e=>{if(e.key==='Enter'||e.key===' '){e.preventDefault();onSelect(p.id)}if(onMove&&['ArrowUp','ArrowDown'].includes(e.key)){e.preventDefault();onMove(p.id,p.u+(e.key==='ArrowUp' ? .5 : -.5))}}}
            onPointerDown={onMove?e=>{e.stopPropagation();e.preventDefault();e.currentTarget.setPointerCapture(e.pointerId);const pt=toPoint(e);drag.current={id:p.id,...pt,u:p.u}}:undefined}>
            <title>{d.name} · {d.fields.model} · U{p.u}起 / {p.height}U · {p.depth===null?'深度未记录':p.depth+' mm'} · {p.face==='unknown'?'安装面未记录':p.face}</title>
            {active&&explode&&<><rect x={X+1} y={y+1} width={W-2} height={h-2} fill="none" stroke="#a6b5cb" strokeDasharray="5 4"/><path d={`M${X} ${y}l${offset} ${oy}M${X+W} ${y+h}l${offset} ${oy}`} className="sr-guide"/></>}
            {spatial&&active&&<><path d={`M${X+offset} ${y+oy}l${vx} ${vy}h${W}l${-vx} ${-vy}M${X+W+offset} ${y+h+oy}l${vx} ${vy}`} fill={depthKnown?'#6a7b96':'none'} stroke="#a6b4d0" strokeDasharray={depthKnown?undefined:'3 4'} strokeWidth="1.5"/><text x={X+W+dx+12} y={y+dy+12} className="sr-axis-note">{depthKnown?`${p.depth} mm`:'未定界'}</text></>}
            <g transform={`translate(${offset} ${oy})`}>
              <rect x={X+2} y={y+1.5} width={W-4} height={h-3} rx="2" className="sr-device-body" strokeDasharray={faceMismatch?'4 3':undefined}/>
              <rect x={X+4} y={y+2} width="3" height={h-4} className="sr-device-accent"/>
              {camera==='rear'?<ConnectionPanel id={d.id} x={X+2} y={y} width={W-4} height={h} selectedPort={selectedPort} onPort={onPort}/>:<>
              {p.height>=3?<>
                <path d={`M${X+14} ${y+12}h16v12h-16z M${X+18} ${y+15}h8v6h-8z`} className="sr-category-glyph"/>
                <text x={X+39} y={y+22} className="sr-device-name">{d.name}</text><text x={X+W-13} y={y+21} textAnchor="end" className="sr-device-u">{p.height}U</text>
                <text x={X+14} y={y+41} className="sr-device-model">{d.fields.model||'型号待补齐'}</text>
                <line x1={X+14} x2={X+W-14} y1={y+h-25} y2={y+h-25} stroke="#ffffff16"/>
                <text x={X+14} y={y+h-11} className="sr-device-resources">{d.type==='server'?`${d.fields.gpu_count??'—'} GPU  /  ${d.fields.nic_count??'—'} NIC`:'网络设备'}</text>
              </>:p.height>=1?<><text x={X+14} y={y+h/2+3.4} className="sr-device-name compact">{d.name}</text><text x={X+W-12} y={y+h/2+3.4} textAnchor="end" className="sr-device-u">{d.fields.role||d.type}</text></>:null}
              </>}
              {active&&<rect x={X-1} y={y-1} width={W+2} height={h+2} rx="3" fill="none" stroke="#8a8fff" strokeWidth="2"/>}
            </g>
          </g>
        })}
        {dimension(X-48,top,bottom,`${height}U`)}
        <text x={X+W/2} y={bottom+48} textAnchor="middle" className="sr-axis-note">{snapshot.rack.fields.width_mm?`${snapshot.rack.fields.width_mm} mm`:'横向未标定'} · U 向按登记比例</text>
        {connection&&<ConnectionGuide snapshot={snapshot} connection={connection} x={X} width={W} top={top} unit={unit} onEndpoint={onEndpoint}/>}

      </>}
      {selectedP&&!section&&!connection&&<g className="sr-selection-callout"><path d={`M${X+W+9} ${selectedY+selectedP.height*unit/2}h32`} stroke="#8997bb"/><text x={X+W+47} y={selectedY+selectedP.height*unit/2-4} className="sr-callout-name">{selectedAsset?.name}</text><text x={X+W+47} y={selectedY+selectedP.height*unit/2+12} className="sr-axis-note">U{selectedP.u} · {selectedP.height}U · {selectedP.face==='unknown'?'面待确认':selectedP.face}</text></g>}
      <g transform="translate(565 690)" className="sr-axis-triad"><path d="M0 0h38M0 0v-38M0 0l24-17"/><text x="42" y="4">W</text><text x="-4" y="-44">U</text><text x="28" y="-20">D</text><circle r="2"/></g>
    </g>
    <text x="22" y="744" className="sr-axis-note">{section?'INSTALLATION SECTION':camera==='spatial'?'PARALLEL PROJECTION':camera==='rear'?'REAR ORTHOGRAPHIC':'FRONT ORTHOGRAPHIC'}</text>
    <text x="658" y="744" textAnchor="end" className="sr-axis-note">{connection?'端接关联 · 非实体走线':draft?'展示草稿 · 未写入 CMDB':'规划登记 · 几何部分已知'}</text>
  </svg>
}

// A cable terminates at registered ports; an external/missing endpoint is not a local rack coordinate.
function ConnectionGuide({snapshot,connection,x,width,top,unit,onEndpoint}:{snapshot:RackSnapshot;connection:InfraCable;x:number;width:number;top:number;unit:number;onEndpoint:(asset:string,port:string)=>void}){
  const rackApi=useRackModel();
  const height=snapshot.height!,bottom=top+height*unit,rail=x+width+148,start=x+width+12;
  const endpoints=[{side:'A',deviceId:connection.device_a,portId:connection.a},{side:'B',deviceId:connection.device_b,portId:connection.b}].map(e=>{const device=rackApi.asset(e.deviceId),port=rackApi.asset(e.portId),valid=!!device&&!!port&&port.fields.owner_device_id===device.id;const placement=valid?snapshot.layout?.placed.find(p=>p.id===e.deviceId):undefined;return {...e,device,port,valid,placement,y:placement?unitY(height,placement.u,placement.height,unit,top)+placement.height*unit/2:null}});
  const local=endpoints.filter(e=>e.y!==null);if(!local.length)return null;
  const remote=endpoints.find(e=>e.y===null),localY=local[0].y!;
  const remoteY=Math.min(bottom-60,Math.max(top+65,localY<top+height*unit/2?localY+130:localY-130));
  return <g className="sr-connection-guide" data-complete={endpoints.every(e=>e.valid)}>
    {local.map(e=><g key={e.side}><path d={`M${start} ${e.y}H${rail}`} fill="none" stroke="#6e72dc" strokeWidth="2"/><circle cx={start} cy={e.y!} r="4"/><text x={start+8} y={e.y!-8} className="sr-port-caption">{e.side} · {e.port?.name}</text></g>)}
    <path d={`M${rail} ${localY}V${remote?remoteY:local[1].y}`} fill="none" stroke={remote&&!remote.valid?'#b58c52':'#6e72dc'} strokeWidth="2" strokeDasharray={remote&&!remote.valid?'5 5':undefined}/>
    {!remote&&<text x={rail+10} y={(localY+local[1].y!)/2} transform={`rotate(90 ${rail+10} ${(localY+local[1].y!)/2})`}>{connection.fields.cable_type||'Cable'} · {connection.fields.length_m??'—'} m</text>}
    {remote&&<g className={`sr-remote-endpoint ${remote.valid?'known':'missing'}`} transform={`translate(${rail-92} ${remoteY-34})`} role={remote.valid&&remote.device?.fields.rack_id?'button':undefined} tabIndex={remote.valid&&remote.device?.fields.rack_id?0:undefined} aria-label={remote.valid?`定位 ${remote.device?.name}`:'端点缺失'} onClick={()=>{if(remote.valid&&remote.device?.fields.rack_id)onEndpoint(remote.deviceId,remote.portId)}} onKeyDown={e=>{if((e.key==='Enter'||e.key===' ')&&remote.valid&&remote.device?.fields.rack_id){e.preventDefault();onEndpoint(remote.deviceId,remote.portId)}}}>
      <rect width="190" height="79" rx="5"/><text x="10" y="17" className="sr-endpoint-context">{remote.valid?(remote.device?.fields.rack_id?`${rackApi.asset(remote.device.fields.rack_id)?.name||remote.device.fields.rack_id} · 柜外端接`:'位置未登记'):`${remote.side} 端引用不完整`}</text><text x="10" y="35" className="sr-endpoint-name">{remote.device?.name||'设备未登记'}</text><text x="10" y="51" className="sr-endpoint-port">{remote.port?.name||'端口未登记'}</text><text x="10" y="67" className="sr-endpoint-action">{remote.valid&&remote.device?.fields.rack_id?'定位对端 ↗':'待补齐 · 不绘制完整连接'}</text>
    </g>}
  </g>
}
