import {useRackModel} from './model';

// A schematic port index over the same CMDB identity, not physical panel coordinates.
export function ConnectionPanel({id,x,y,width,height,selectedPort,onPort}:{id:string;x:number;y:number;width:number;height:number;selectedPort?:string;onPort?:(asset:string,port:string)=>void}){
  const rackApi=useRackModel();
  const device=rackApi.asset(id),ports=rackApi.ports(id),cables=rackApi.connections(id);
  const connected=new Set(cables.flatMap(c=>[c.a,c.b]));
  const indexed=height>=64,shown=ports.slice(0,8),step=(width-20)/4;
  return <g className="rv-connection-panel">
    <text x={x+12} y={y+(height>=28?13:height>=22?10:height/2+3)} className="rv-rear-name">{device?.name}</text>
    {indexed&&shown.map((port,i)=><g key={port.id} transform={`translate(${x+10+(i%4)*step} ${y+24+Math.floor(i/4)*18})`} className={`rv-port-chip ${selectedPort===port.id?'selected':''}`} data-port={port.id} role={onPort?'button':undefined} tabIndex={onPort?0:undefined} aria-label={`查看端口 ${port.name} · ${device?.name}`} onPointerDown={e=>e.stopPropagation()} onClick={e=>{e.stopPropagation();onPort?.(id,port.id)}} onKeyDown={e=>{if(e.key==='Enter'||e.key===' '){e.preventDefault();e.stopPropagation();onPort?.(id,port.id)}}}>
      <title>{port.name} · {port.fields.speed||'速率未知'} · {connected.has(port.id)?'有登记端接':'端接未登记'}</title>
      <rect width={step-4} height="15" rx="3"/><circle cx="6" cy="7.5" r="2" fill={connected.has(port.id)?'#a2c2e9':'none'} stroke="#a2c2e9"/><text x="12" y="10.5">{String(port.fields.physical_name||port.name).length>8?String(port.fields.physical_name||port.name).slice(0,7)+'…':port.fields.physical_name||port.name}</text>
    </g>)}
    {height>=22&&<text x={x+12} y={y+height-5} className="rv-rear-summary">{ports.length} 端口 · {cables.length} 条端接{indexed&&ports.length>8?' · 展开查看全部':''}</text>}
  </g>;
}
