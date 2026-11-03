import { useEffect, useMemo, useState } from 'react';
import { ArrowUpRight, ChevronRight, Server } from 'lucide-react';
import { useWorkspace } from '../../context';
import { placementReady, type PublicationOptions } from '../../lib/api';
import { Badge, Button, DataTable, Empty, Notice, Section } from '../ui';
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
  const completeIds = new Set(racks.flatMap(r => r.complete.map(d => d.id)));
  const metrics: { id: CoverageKey; label: string; done: number; total: number; unit: string; table: string; detail: string }[] = [
    { id: 'room', label: '机房归属', done: racks.filter(r => r.room).length, total: racks.length, unit: '柜', table: 'racks', detail: '机柜引用可解析到已有机房实体' },
    { id: 'capacity', label: '机柜容量', done: racks.filter(r => positiveU(r.capacityU)).length, total: racks.length, unit: '柜', table: 'racks', detail: '明确的正整数容量 U' },
    { id: 'height', label: '设备高度', done: s.devices.filter(d => positiveU(d.heightU)).length, total: s.devices.length, unit: '台', table: 'equipment_specs', detail: '设备实际有效高度，正整数 U' },
    { id: 'u', label: '起始 U 位', done: s.devices.filter(d => positiveU(d.startU)).length, total: s.devices.length, unit: '台', table: 'placements', detail: '明确的正整数起始 U，不代表完整占位' },
    { id: 'placement', label: '完整落位', done: completeIds.size, total: s.devices.length, unit: '台', table: 'placements', detail: '机柜、容量、U 位、高度齐备，无越界、重叠及后端落位问题' },
    { id: 'coordinates', label: '平面位置已明确', done: racks.filter(r => r.coordinates).length, total: racks.length, unit: '柜', table: 'racks', detail: '已确认机柜在机房内的平面位置；不影响机柜立面入库' },
  ];
  const metric = metrics.find(v => v.id === filter);
  const rackMissing = (r: typeof racks[number]) => filter === 'room' ? !r.room : filter === 'capacity' ? !positiveU(r.capacityU) : filter === 'coordinates' ? !r.coordinates : filter==='conflicts'?!!(r.conflicts.size||r.conflictIssues.length):false;
  const missingDevices = s.devices.filter(d => filter === 'height' ? !positiveU(d.heightU) : filter === 'u' ? !positiveU(d.startU) : filter === 'placement' ? !completeIds.has(d.id) : false);
  const deviceFilter = filter === 'height' || filter === 'u' || filter === 'placement';
  const visibleRacks = !filter ? racks : deviceFilter ? racks.filter(r => r.devices.some(d => missingDevices.some(v => v.id === d.id))) : racks.filter(rackMissing);
  const selectedIds = selected.filter(id => racks.some(r => r.id === id));
  const toggle = (id: string) => setSelected(v => v.includes(id) ? v.filter(x => x !== id) : [...v, id]);
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
  return <Section title="待补充的信息" caption="已识别的信息自动保留。选择下方事项，一次补录或关联受影响对象；无需先勾选机柜。" action={<a className="text-link" href={'/racks?project_id=' + encodeURIComponent(s.projectId)} target="_blank" rel="noreferrer">查看已入库机柜图<ArrowUpRight size={14}/></a>}>
    <div className="facility-gap-tasks">{gapRows.map(g=><div className="facility-gap-task" key={g.title}><div><b>{g.title}</b><p>{g.note}</p></div><span className={g.count?'gap-count':'gap-complete'}>{g.count?`${g.count} ${g.unit}待补充`:'此项已补齐'}</span><Button kind={g.count?'primary':'ghost'} disabled={!g.count||!can('bom:edit')} onClick={g.run}>{g.action}</Button></div>)}</div>
    {!!conflicts.length&&<Notice tone="warning">{conflicts.length} 个机柜存在占位或字段冲突，不能用批量补空解决。<Button onClick={()=>{setFilter('conflicts');document.getElementById('facility-full-list')?.setAttribute('open','')}}>查看冲突与受影响设备</Button></Notice>}
    <details className="facility-later"><summary>可稍后补充 · 机房平面布局与机柜尺寸</summary><div className="facility-gap-task"><div><b>{layoutGaps.length} 个机柜尚未设置平面位置</b><p>用于机房平面图和布线路径计算，不影响已有数据入库和机柜立面。</p></div><Button disabled={!layoutGaps.length||!can('bom:edit')} onClick={()=>openBatch('coordinates',layoutGaps.map(r=>r.id))}>批量设置布局</Button><Button disabled={!can('bom:edit')} onClick={()=>openBatch('geometry',racks.filter(r=>r.gaps.includes('宽度')||r.gaps.includes('深度')).map(r=>r.id),['width_mm','depth_mm'])}>补录机柜尺寸</Button></div></details>
    <details id="facility-full-list" className="facility-full-list"><summary>查看完整清单与批量导入 · {racks.length} 个机柜</summary>
    <div className="facility-toolbar"><div className="actions"><Button onClick={()=>edit('rooms')}>机房管理</Button><Button onClick={()=>edit('equipment_specs')}>型号规格导入</Button><Button onClick={()=>edit('racks')}>机柜导入 / 编辑</Button><Button onClick={()=>edit('placements')}>落位导入 / 编辑</Button></div></div>
    {filter==='conflicts'&&<div className="facility-filter-heading">正在查看存在冲突的机柜<Button onClick={()=>setFilter(undefined)}>查看全部</Button></div>}
    {publicationError && <Notice tone="warning">设施发布状态读取失败：{publicationError}<Button onClick={() => setRetry(v => v + 1)}>重新核对</Button></Notice>}
    {metric && <div className="facility-gap-list"><div className="facility-filter-heading"><span><b>{metric.label}缺口</b> · {metric.detail}</span><div className="actions"><Button disabled={!can('bom:edit')} onClick={() => edit(metric.table)}>补充输入</Button><Button kind="ghost" onClick={() => setFilter(undefined)}>清除筛选</Button></div></div>
      {deviceFilter && <DataTable key={filter} data={missingDevices} pageSize={8} columns={[
        { accessorKey: 'name', header: '设备' }, { accessorKey: 'model', header: '型号' },
        { id: 'rack', header: '机柜', cell: ({ row }) => s.racks.find(r => r.id === row.original.rackId)?.name || '待关联' },
        { id: 'gap', header: '缺失 / 冲突', cell: ({ row }) => { const d = row.original; return [!s.racks.some(r => r.id === d.rackId) && '机柜归属', !positiveU(d.startU) && 'U位', !positiveU(d.heightU) && '高度', s.racks.some(r => r.id === d.rackId && !positiveU(r.capacityU)) && '容量', ...s.issues.filter(i => !i.resolved && i.scope === 'placement' && i.entityId === d.id).map(i => i.title), racks.some(r => r.conflicts.has(d.id)) && '占位冲突'].filter(Boolean).join(' · ') || '落位待核对'; } },
      ]}/>}
    </div>}
    <div className="facility-toolbar"><label><input type="checkbox" aria-label="选择当前筛选的机柜" checked={visibleRacks.length > 0 && visibleRacks.every(r => selectedIds.includes(r.id))} ref={el => { if (el) el.indeterminate = visibleRacks.some(r => selectedIds.includes(r.id)) && !visibleRacks.every(r => selectedIds.includes(r.id)); }} onChange={e => setSelected(e.target.checked ? [...new Set([...selectedIds, ...visibleRacks.map(r => r.id)])] : selectedIds.filter(id => !visibleRacks.some(r => r.id === id)))}/>已选 {selectedIds.length} 柜</label>
      <div className="actions"><Button kind="ghost" disabled={!selectedIds.length} onClick={() => setSelected([])}>清空选择</Button><Button disabled={!can('bom:edit') || !selectedIds.length} onClick={() => setBatch('room')}>批量关联机房</Button><Button disabled={!can('bom:edit') || !selectedIds.length} onClick={() => {setGeometryFields(undefined);setBatch('geometry')}}>批量容量与尺寸</Button><Button disabled={!can('bom:edit')} onClick={() => setBatch('height')}>按型号补高度</Button><Button disabled={!can('bom:edit') || !selectedIds.length} onClick={() => setBatch('coordinates')}>设置机房布局</Button></div>
    </div>
    <DataTable data={visibleRacks} pageSize={25} searchPlaceholder="搜索机柜、机房或缺失项" columns={[
      {id:'select',header:'选择',enableSorting:false,cell:({row})=><input type="checkbox" aria-label={'选择机柜 '+row.original.name} checked={selectedIds.includes(row.original.id)} onChange={()=>toggle(row.original.id)}/>},
      {accessorKey:'name',header:'机柜',cell:({row})=><button className="text-link" onClick={()=>selectRack(row.original.id)}>{row.original.name}<ChevronRight size={12}/></button>},
      {id:'room',accessorFn:r=>r.room?.label||'待关联',header:'所属机房'},
      {accessorKey:'capacityU',header:'容量',cell:({getValue})=>getValue()?`${getValue()} U`:'待补充'},
      {id:'devices',header:'设备 / 完整占位',cell:({row})=>`${row.original.devices.length} / ${row.original.complete.length}`},
      {id:'gaps',accessorFn:r=>r.gaps.join('、'),header:'缺口与冲突',cell:({row})=><div className="facility-table-gaps">{!!row.original.conflicts.size&&<Badge state="blocked">{row.original.conflicts.size} 台占位冲突</Badge>}{row.original.conflictIssues.some(i=>i.entityId===row.original.id)&&<Badge state="blocked">机柜字段冲突</Badge>}<span>{row.original.gaps.join('、')||'当前统计项齐备'}</span></div>},
      {id:'publication',header:'入库状态',cell:({row})=>publicationLabel(row.original.id,row.original.rackReceipt)},
    ]}/>
    </details>
    {gapEditor&&<FacilityGapEditor mode={gapEditor} close={()=>setGapEditor(undefined)}/>}
    {activeRack&&racks.some(r=>r.id===activeRack)&&<RackDetail key={activeRack} id={activeRack} close={()=>selectRack(null)} edit={edit}/>}
    <div className="facility-candidates">机房候选 {rooms.length} 个 · 未被机柜引用 {rooms.filter(room => !racks.some(r => r.room?.id === room.id)).length} 个 · 未关联机柜设备 {s.devices.filter(d => !s.racks.some(r => r.id === d.rackId)).length} 台</div>
    <Notice>补录后缺口自动更新；确认入库仍在“入库与版本”完成。无依据的信息可稍后补充。</Notice>
    {batch==='coordinates'&&<FacilityLayoutEditor rackIds={selectedIds} close={()=>setBatch(undefined)}/>}
    {batch && batch!=='coordinates' && <FacilityBatchEditor geometryFields={geometryFields} key={batch} mode={batch} rackIds={selectedIds} close={() => setBatch(undefined)}/>}
  </Section>;
}
