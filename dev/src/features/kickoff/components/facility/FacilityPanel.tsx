import { useEffect, useMemo, useState } from 'react';
import { useWorkspace } from '../../context';
import { placementReady, type PublicationOptions } from '../../lib/api';
import { Badge, Button, DataTable, Notice, Section } from '../ui';
import { entityOptions, filled, modelRows, numeric, positiveU, resolveEntity } from './FacilityEntities';
import { FacilityGapEditor } from './FacilityGapEditor';
import { RackDetail } from './RackDetail';
import { FacilityLayoutEditor } from './FacilityLayoutEditor';
import { FacilityBatchEditor } from './FacilityBatchEditor';
import './facility.css';

type CoverageKey = 'conflicts' | 'room' | 'capacity' | 'height' | 'u' | 'placement' | 'coordinates';
export function FacilityPanel({ activeRack, selectRack, edit }: { activeRack: string | null; selectRack: (id: string | null) => void; edit: (table: string) => void }) {
  const { state: s, api, can } = useWorkspace();
  const [filter, setFilter] = useState<CoverageKey>();
  const [geometryFields,setGeometryFields]=useState<string[]>();
  const [gapEditor,setGapEditor]=useState<'height'|'placement'>();
  const [selected, setSelected] = useState<string[]>([]);
  const [batch, setBatch] = useState<'room' | 'geometry' | 'height' | 'coordinates'>();
  const publicationKey = `${s.projectId}:${s.revision}:` + s.publications.map(p => `${p.id}:${p.state}`).sort().join('|');
  const [publicationResult, setPublicationResult] = useState<{ key: string; value?: PublicationOptions; error?: string }>();
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    let active = true;
    void api.publicationOptions(s.revision).then(value => {
      if (active && value.revision === s.revision) setPublicationResult({ key: publicationKey, value });
    }).catch((e: Error) => { if (active) setPublicationResult({ key: publicationKey, error: e.message }); });
    return () => { active = false; };
  }, [api, publicationKey, retry]);
  const publicationOptions = publicationResult?.key === publicationKey ? publicationResult.value : undefined;
  const publicationError = publicationResult?.key === publicationKey ? publicationResult.error : undefined;
  const facilityOptions = new Map(publicationOptions?.facilities?.map(f => [f.id, f]) ?? []);
  const rooms = entityOptions(s, 'room_id');
  const rackInputs = modelRows(s, 'racks');
  const rackEntities = entityOptions(s, 'rack_id');
  const racks = useMemo(() => s.racks.map(r => {
    const row = rackInputs.find(v => resolveEntity(rackEntities, v.rack_id) === r.id);
    const roomId = resolveEntity(rooms, row?.room_id || r.room);
    const room = rooms.find(v => v.id === roomId);
    const devices = s.devices.filter(d => d.rackId === r.id);
    const conflicts = new Set<string>();
    for (const d of devices) {
      if (positiveU(d.startU) && positiveU(d.heightU) && positiveU(r.capacityU) && d.startU + d.heightU - 1 > r.capacityU) conflicts.add(d.id);
    }
    // Missing fields remain gaps; actual placement conflicts are reported separately.
    const conflictIssues = s.issues.filter(i => !i.resolved && (i.entityId === r.id || i.scope === 'placement' && devices.some(d => d.id === i.entityId)) &&
      !/未知|缺少|缺失|待补充|不足以|无法|不能确认/.test(i.title) && /冲突|重叠|越界|超出|重复|多个规划|无效/.test(i.title));
    for (const issue of conflictIssues) if (devices.some(d => d.id === issue.entityId)) conflicts.add(issue.entityId);
    const complete = devices.filter(d => placementReady(s, d) && !conflicts.has(d.id));
    const published = devices.filter(d => s.publications.some(p => p.state === 'published' && p.deviceIds.includes(d.id))).length;
    const pending = devices.filter(d => s.publications.some(p => p.state === 'pending' && p.deviceIds.includes(d.id))).length;
    const failed = devices.filter(d => s.publications.some(p => p.state === 'failed' && p.deviceIds.includes(d.id)) && !s.publications.some(p => p.state === 'published' && p.deviceIds.includes(d.id))).length;
    const rackReceipt = s.publications.some(p => p.state === 'published' && (p.facilityIds?.includes(r.id) ?? p.rackSnapshot.some(v => v.id === r.id)));
    const coordinates = !!row && numeric(row.x) && numeric(row.y) && filled(row.frame) && ['mm', 'cm', 'm'].includes(row.unit);
    const gaps = [!room && '机房归属', !positiveU(r.capacityU) && '容量', !numeric(row?.width_mm) || Number(row?.width_mm) <= 0 ? '宽度' : '', !numeric(row?.depth_mm) || Number(row?.depth_mm) <= 0 ? '深度' : '',
      devices.some(d => !positiveU(d.heightU)) && '设备高度', devices.some(d => !positiveU(d.startU)) && 'U位', !coordinates && '平面位置'].filter(Boolean) as string[];
    return { ...r, row, room, roomId, devices, conflicts, conflictIssues, complete, published, pending, failed, rackReceipt, coordinates, gaps };
  }), [s, rackInputs]);
  const visibleRacks = filter === 'conflicts' ? racks.filter(r=>r.conflicts.size||r.conflictIssues.length) : racks;
  const selectedIds = selected.filter(id => racks.some(r => r.id === id));
  const publicationLabel = (id: string, receipt: boolean) => {
    const option = facilityOptions.get(id);
    if (option) return { new: '待发布', changed: '待增补', unchanged: '已发布 · 无变化', blocked: '待处理 · 发布阻断', pending: '等待设施回执' }[option.state];
    return receipt ? '有历史设施回执 · 当前输入待核对' : publicationOptions || publicationError ? '设施发布状态待核对' : '正在核对设施发布状态';
  };
  const openBatch=(mode:'room'|'geometry'|'coordinates',ids:string[],fields?:string[])=>{setSelected(ids);setGeometryFields(fields);setBatch(mode)};
  const roomGaps=racks.filter(r=>!r.room),capacityGaps=racks.filter(r=>!positiveU(r.capacityU));
  const heightGaps=s.devices.filter(d=>!positiveU(d.heightU));
  const positionGaps=s.devices.filter(d=>!s.racks.some(r=>r.id===d.rackId)||!positiveU(d.startU)||!d.model&&!positiveU(d.heightU));
  const conflicts=racks.filter(r=>r.conflicts.size||r.conflictIssues.length);
  const layoutGaps=racks.filter(r=>!r.coordinates);
  const gapRows=[
   {title:'机柜尚未关联机房',count:roomGaps.length,unit:'个机柜',note:'指定这些机柜属于哪个机房。',action:'批量关联机房',run:()=>openBatch('room',roomGaps.map(r=>r.id))},
   {title:'机柜容量未明确',count:capacityGaps.length,unit:'个机柜',note:'填写机柜可用的总 U 数，用于检查设备是否装得下。',action:'批量补录容量',run:()=>openBatch('geometry',capacityGaps.map(r=>r.id),['capacity_u'])},
   {title:'设备高度未明确',count:heightGaps.length,unit:'台设备',note:`涉及 ${new Set(heightGaps.map(d=>d.model).filter(Boolean)).size} 种已知型号，同型号填写一次。`,action:'按型号批量补录',run:()=>setGapEditor('height')},
   {title:'设备安装位置待补充',count:positionGaps.length,unit:'台设备',note:'补充所在机柜、起始 U 位；无型号设备可逐台填写高度。',action:'补录安装位置',run:()=>setGapEditor('placement')},
  ];
  const pending=gapRows.filter(g=>g.count>0);
  const sizeGaps=racks.filter(r=>r.gaps.includes('宽度')||r.gaps.includes('深度'));
  return <Section title="机房、机柜与设备关联" caption="展示前置资料已有的归属与落位信息；仅对缺失或冲突的信息提供补录。">
    <div className="facility-relation-summary"><span>{rooms.length} 个机房</span><span>{racks.length} 个机柜 · {racks.length-roomGaps.length} 个已关联机房</span><span>{s.devices.length} 台设备 · {s.devices.filter(d=>s.racks.some(r=>r.id===d.rackId)).length} 台已关联机柜</span></div>
    <DataTable data={visibleRacks} pageSize={5} searchPlaceholder="搜索机柜、机房或缺失项" columns={[
      {accessorKey:'name',header:'机柜',cell:({row})=><span>{row.original.name}</span>},
      {id:'room',accessorFn:r=>r.room?.label||'待关联',header:'所属机房'},
      {accessorKey:'capacityU',header:'容量',cell:({getValue})=>getValue()?`${getValue()} U`:'待补充'},
      {id:'devices',header:'设备 / 完整占位',cell:({row})=>`${row.original.devices.length} / ${row.original.complete.length}`},
      {id:'gaps',accessorFn:r=>r.gaps.join('、'),header:'缺口与冲突',cell:({row})=><div className="facility-table-gaps">{!!row.original.conflicts.size&&<Badge state="blocked">{row.original.conflicts.size} 台占位冲突</Badge>}{row.original.conflictIssues.some(i=>i.entityId===row.original.id)&&<Badge state="blocked">机柜字段冲突</Badge>}<span>{row.original.gaps.join('、')||'当前统计项齐备'}</span></div>},
      {id:'publication',header:'入库状态',cell:({row})=>publicationLabel(row.original.id,row.original.rackReceipt)},
    ]}/>

    {!!pending.length&&<div className="facility-inline-gaps"><b>以下信息需要补充</b>{pending.map(g=><div className="facility-inline-gap" key={g.title}><span>{g.title} · <strong>{g.count} {g.unit}</strong></span><small>{g.note}</small><Button kind="ghost" disabled={!can('bom:edit')} onClick={g.run}>{g.action}</Button></div>)}</div>}
    {!pending.length&&!conflicts.length&&racks.length>0&&<p className="facility-relation-ready">机房归属与机柜立面所需信息已齐备。</p>}
    {!!conflicts.length&&<Notice tone="warning">{conflicts.length} 个机柜存在占位或字段冲突。<Button onClick={()=>setFilter(filter==='conflicts'?undefined:'conflicts')}>{filter==='conflicts'?'查看全部关联':'查看冲突机柜'}</Button><Button disabled={!can('bom:edit')} onClick={()=>edit('placements')}>核对安装位置</Button></Notice>}
    {(layoutGaps.length>0||sizeGaps.length>0)&&<details className="facility-later"><summary>平面拓扑所需信息 · 可后补，不阻断机柜立面</summary><div className="facility-inline-gaps">{!!layoutGaps.length&&<div className="facility-inline-gap"><span>{layoutGaps.length} 个机柜缺少平面位置</span><Button disabled={!can('bom:edit')} kind="ghost" onClick={()=>openBatch('coordinates',layoutGaps.map(r=>r.id))}>补录机房布局</Button></div>}{!!sizeGaps.length&&<div className="facility-inline-gap"><span>{sizeGaps.length} 个机柜缺少宽度或深度</span><Button disabled={!can('bom:edit')} kind="ghost" onClick={()=>openBatch('geometry',sizeGaps.map(r=>r.id),['width_mm','depth_mm'])}>批量补录尺寸</Button></div>}</div></details>}
    {publicationError&&<Notice tone="warning">入库状态暂未读取：{publicationError}<Button onClick={()=>setRetry(v=>v+1)}>重新读取</Button></Notice>}
    {gapEditor&&<FacilityGapEditor mode={gapEditor} close={()=>setGapEditor(undefined)}/>}
    {activeRack&&racks.some(r=>r.id===activeRack)&&<RackDetail key={activeRack} id={activeRack} close={()=>selectRack(null)} edit={edit}/>}
    {batch==='coordinates'&&<FacilityLayoutEditor rackIds={selectedIds} close={()=>setBatch(undefined)}/>}
    {batch && batch!=='coordinates' && <FacilityBatchEditor geometryFields={geometryFields} key={batch} mode={batch} rackIds={selectedIds} close={() => setBatch(undefined)}/>}
  </Section>;
}
