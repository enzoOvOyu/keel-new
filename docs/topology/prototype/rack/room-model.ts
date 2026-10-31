import {rackApi,type RackSnapshot} from './model';

export type RoomRack={snapshot:RackSnapshot;x:number;z:number;height:number;pod:string};
// Presentation coordinates only. No inferred room footprint, aisle width or surveyed XY.
export function roomModel(){
  const snapshots=rackApi.list().map(r=>rackApi.snapshot(r.id)!).sort((a,b)=>a.rack.id.localeCompare(b.rack.id));
  const pods=[...new Set(snapshots.map(s=>s.rack.fields.pod_id as string))];
  const racks:RoomRack[]=pods.flatMap((pod,row)=>{
    const members=snapshots.filter(s=>s.rack.fields.pod_id===pod);
    return members.map((snapshot,column)=>({snapshot,pod,x:(column-(members.length-1)/2)*1.65,z:(row-(pods.length-1)/2)*2.9,height:(snapshot.height||0)/20}));
  });
  return {racks,pods,source:'schematic' as const,coordinateCoverage:0};
}
