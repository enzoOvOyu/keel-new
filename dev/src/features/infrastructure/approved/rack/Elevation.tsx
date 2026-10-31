import {useId,useRef} from 'react';
import type {PointerEvent} from 'react';
import {unitY,faceProjection} from './geometry';
import {type RackSnapshot,type Overlay,typeLabel} from './model';
import {ConnectionPanel} from './ConnectionPanel';
export function Elevation({snapshot,face='front',overlay,selected='',mini=false,onSelect,onMove,focusIds,relatedIds=[],selectedPort,onPort}:{snapshot:RackSnapshot;face?:'front'|'rear';overlay:Overlay;selected?:string;mini?:boolean;onSelect?:(id:string)=>void;onMove?:(id:string,u:number)=>void;focusIds?:string[];relatedIds?:string[];selectedPort?:string;onPort?:(asset:string,port:string)=>void}){
  const uid=useId().replaceAll(':',''),svg=useRef<SVGSVGElement>(null);
  const drag=useRef<{id:string;y:number;u:number}|null>(null);
  const {height,layout,items}=snapshot; if(!height||!layout)return <div className="rv-unavailable">缺少机柜高度</div>;
  const width=300,unit=15,top=40,bottom=top+height*unit,total=bottom+30;
  const point=(e:PointerEvent<SVGGElement>)=>{const rect=svg.current!.getBoundingClientRect();return(e.clientY-rect.top)/rect.height*total};
  function finish(e:PointerEvent<SVGGElement>){const s=drag.current;if(!s)return;drag.current=null;const distance=s.y-point(e);if(Math.abs(distance)>unit/4)onMove?.(s.id,Math.round((s.u+distance/unit)*2)/2);else onSelect?.(s.id)}
  return <svg ref={svg} className={'rv-elevation '+(mini?'mini':'')} viewBox={`0 0 ${width} ${total}`} role={mini?'img':'group'} aria-label={`${snapshot.rack.name} ${face==='front'?'正面':'背面'}立面`} data-face={face} xmlns="http://www.w3.org/2000/svg">
    <defs><pattern id={`empty-${uid}`} width="6" height="6" patternUnits="userSpaceOnUse"><path d="M0 6L6 0" stroke="#dce2eb" strokeWidth=".5"/></pattern><pattern id={`unknown-${uid}`} width="5" height="5" patternUnits="userSpaceOnUse"><circle cx="1" cy="1" r=".5" fill="#98a5b6"/></pattern></defs>
    <rect className="rv-svg-rack-frame" x="26" y="13" width="256" height={total-23} rx="5" fill="#e7ecf2" stroke="#aebacb"/>
    <rect className="rv-svg-interior" x="42" y={top} width="224" height={height*unit} fill="#f8fafc" stroke="#b8c3d2"/>
    {!mini&&<><text x="42" y="29" className="rv-svg-rail-label">{snapshot.rack.name}</text><text x="265" y="29" textAnchor="end" className="rv-svg-rail-label">{height}U</text></>}
    {Array.from({length:height},(_,i)=>{const u=height-i,y=top+i*unit;return <g key={u}><line x1="42" x2="266" y1={y+unit} y2={y+unit} stroke="#e2e7ef" strokeWidth=".5"/>{!mini&&<text x="21" y={y+unit*.73} textAnchor="end" className="rv-svg-u">{u}</text>}<rect x="32" y={y+5} width="3" height="5" rx="1" fill="#bac5d3"/><rect x="273" y={y+5} width="3" height="5" rx="1" fill="#bac5d3"/></g>})}
    {overlay==='capacity'&&layout.freeRanges.map(r=><g key={r.u}><rect x="43" width="222" y={unitY(height,r.u,r.height,unit,top)} height={r.height*unit} fill={`url(#empty-${uid})`}/>{!mini&&r.height>=2&&<text x="154" y={unitY(height,r.u,r.height,unit,top)+r.height*unit/2+3} textAnchor="middle" className="rv-svg-empty">{r.height}U 未占用</text>}</g>)}
    {layout.placed.map(p=>{
      const d=items.find(d=>d.id===p.id)!,y=unitY(height,p.u,p.height,unit,top),h=p.height*unit;
      const projection=faceProjection(p,face);if(!projection.visible)return null;
      const uncertain=projection.kind==='reference',outline=uncertain,active=selected===p.id,conflict=layout.issues.some(i=>i.ids.includes(p.id));
      const cls=`rv-svg-device ${d.type==='server'?'server':'switch'} ${outline?'unresolved':''} ${active?'is-selected':''} ${relatedIds.includes(p.id)?'related':''} overlay-${overlay} ${conflict?'conflict':''}`;
      return <g key={p.id} className={cls} opacity={focusIds&&!focusIds.includes(p.id) ? .22 : 1} style={{touchAction:onMove?'none':undefined}} data-device={p.id} data-u={p.u} data-height={p.height} data-projection={projection.kind} data-panel={projection.panel} role={mini?undefined:'button'} tabIndex={mini?-1:0} aria-label={`${d.name} U${p.u} ${p.height}U${uncertain?' 安装几何未确认':''}`} aria-pressed={active} onClick={onMove?undefined:()=>onSelect?.(p.id)} onKeyDown={e=>{if(e.key==='Enter'||e.key===' '){e.preventDefault();onSelect?.(p.id)}if(onMove&&['ArrowUp','ArrowDown'].includes(e.key)){e.preventDefault();onMove(p.id,p.u+(e.key==='ArrowUp' ? .5 : -.5))}}} onPointerDown={onMove?e=>{e.preventDefault();e.currentTarget.setPointerCapture(e.pointerId);drag.current={id:p.id,y:point(e),u:p.u}}:undefined} onPointerUp={onMove?finish:undefined} onPointerCancel={()=>{drag.current=null}}>
        <title>{d.name} · {typeLabel(d)} · {d.fields.model} · U{p.u} 起 · {p.height}U · {uncertain?'安装几何未确认，仅显示登记占位':projection.kind==='through'?'全深度反面占位':'安装面 '+p.face} · 面板素材未绑定</title>
        <rect x="44" y={y+1} width="220" height={h-2} rx="2" className="rv-svg-body" strokeDasharray={outline?'4 2':undefined}/>
        <rect x="44" y={y+1} width="4" height={h-2} className="rv-svg-type"/>
        {face==='rear'&&!mini?<ConnectionPanel id={d.id} x={44} y={y} width={220} height={h} selectedPort={selectedPort} onPort={onPort}/>:<>
        {p.height>=3&&!mini&&<><path d={`M55 ${y+12}h10v7H55z M55 ${y+22}h10v7H55z`} fill="none" stroke="currentColor" strokeWidth="1"/><text x="74" y={y+18} className="rv-svg-category">{typeLabel(d)}</text>{!mini&&<text x="74" y={y+31} className="rv-svg-model">{d.fields.model||'型号未记录'}</text>}</>}
        {p.height>=1&&!mini&&<text x={p.height>=3?55:54} y={p.height>=3?y+h-24:y+h/2+3} className="rv-svg-name">{d.name.length>24?d.name.slice(0,23)+'…':d.name}</text>}
        {p.height>=3&&!mini&&<text x="55" y={y+h-10} className="rv-svg-meta">{overlay==='compute'?(d.fields.gpu_count===undefined?'GPU 未登记':`${d.fields.gpu_count} GPU · 登记数量`):`${p.height}U · ${d.type==='server'?`${d.fields.gpu_count??'—'} GPU`:'网络设备'}`}</text>}
        {p.height>=3&&!mini&&<text x="253" y={y+18} textAnchor="end" className="rv-svg-position">U{p.u}</text>}

        {!mini&&p.height>=3&&projection.kind==='through'&&<text x="253" y={y+32} textAnchor="end" className="rv-svg-meta">全深度 · 反面</text>}
        </>}
        {active&&<rect x="41" y={y-1} width="226" height={h+2} rx="3" fill="none" stroke="#6473df" strokeWidth="2"/>}
      </g>;
    })}
    {!mini&&<text x="154" y={bottom+19} textAnchor="middle" className="rv-svg-footer">{mini?'登记占位':face==='front'?'FRONT · 设备与安装':'REAR · 端口索引，非面板坐标'}</text>}
  </svg>
}
