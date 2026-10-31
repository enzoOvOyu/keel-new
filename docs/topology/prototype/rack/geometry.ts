// One interval model for Gallery and Dual Elevation. U positions denote the lowest U.
export type Face = 'front' | 'rear' | 'both' | 'unknown';
export type GeometryInput = {id:string; u:number|null; height:number|null; face:Face; fullDepth:boolean|null; depth:number|null};
export type Placement = GeometryInput & {u:number; height:number; start:number; end:number};
export type LayoutIssue = {ids:string[]; kind:'missing'|'range'|'overlap'|'uncertain'|'depth'; message:string};
export function layoutRack(height:number, items:GeometryInput[], depth:number|null=null) {
  const placed:Placement[]=[], unplaced:GeometryInput[]=[], zeroU:GeometryInput[]=[], issues:LayoutIssue[]=[];
  for(const item of items){
    if(item.height===0){zeroU.push(item);continue}
    if(item.u===null||item.height===null||item.height<=0||!Number.isInteger(item.u*2)||!Number.isInteger(item.height*2)){
      unplaced.push(item);issues.push({ids:[item.id],kind:'missing',message:'缺少有效 U 位 / U 高（支持 0.5U 精度）'});continue;
    }
    const start=(item.u-1)*2,end=start+item.height*2;
    if(start<0||end>height*2){unplaced.push(item);issues.push({ids:[item.id],kind:'range',message:'位置超出机柜范围'});continue}
    if(depth!==null&&item.depth!==null&&item.depth>depth)issues.push({ids:[item.id],kind:'depth',message:'设备深度超出已知机柜深度'});
    placed.push({...item,u:item.u,height:item.height,start,end});
  }
  for(let i=0;i<placed.length;i++)for(let j=i+1;j<placed.length;j++){
    const a=placed[i],b=placed[j];if(a.start>=b.end||b.start>=a.end)continue;
    const same=a.face!=='unknown'&&b.face!=='unknown'&&(a.face===b.face||a.face==='both'||b.face==='both');
    const opposite=a.face!=='unknown'&&b.face!=='unknown'&&a.face!==b.face;
    const separate=opposite&&a.fullDepth===false&&b.fullDepth===false&&depth!==null&&a.depth!==null&&b.depth!==null&&a.depth+b.depth<=depth;
    if(separate)continue;
    const definite=same||a.fullDepth===true||b.fullDepth===true;
    issues.push({ids:[a.id,b.id],kind:definite?'overlap':'uncertain',message:definite?'安装占位重叠':'同 U 区间，安装面 / 深度不足以确认兼容'});
  }
  const occupiedSlots=new Set<number>();placed.forEach(p=>{for(let s=p.start;s<p.end;s++)occupiedSlots.add(s)});
  const freeRanges:{u:number;height:number}[]=[];let start=-1;
  for(let s=0;s<=height*2;s++){
    if(s<height*2&&!occupiedSlots.has(s)){if(start<0)start=s}
    else if(start>=0){freeRanges.push({u:1+start/2,height:(s-start)/2});start=-1}
  }
  return {placed,unplaced,zeroU,issues,occupied:occupiedSlots.size/2,free:height-occupiedSlots.size/2,freeRanges};
}
export const unitY=(rackHeight:number,u:number,height:number,unit:number,top:number)=>top+(rackHeight-(u-1)-height)*unit;
