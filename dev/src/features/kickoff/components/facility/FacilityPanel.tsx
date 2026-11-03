import { useEffect, useMemo, useState } from 'react';
import { ArrowUpRight, ChevronRight, Server } from 'lucide-react';
import { useWorkspace } from '../../context';
import { placementReady, type PublicationOptions } from '../../lib/api';
import { Badge, Button, DataTable, Empty, Notice, Section } from '../ui';
import { entityOptions, filled, modelRows, numeric, positiveU, resolveEntity } from './FacilityEntities';
import { FacilityBatchEditor } from './FacilityBatchEditor';
import './facility.css';

type CoverageKey = 'room' | 'capacity' | 'height' | 'u' | 'placement' | 'coordinates';
export function FacilityPanel({ activeRack, selectRack, edit }: { activeRack: string | null; selectRack: (id: string) => void; edit: (table: string) => void }) {
  const { state: s, api, can } = useWorkspace();
  const [filter, setFilter] = useState<CoverageKey>();
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
  const racks = useMemo(() => s.racks.map(r => {
    const row = rackInputs.find(v => resolveEntity(entityOptions(s, 'rack_id'), v.rack_id) === r.id);
    const roomId = resolveEntity(rooms, row?.room_id || r.room);
    const room = rooms.find(v => v.id === roomId);
    const devices = s.devices.filter(d => d.rackId === r.id);
    const conflicts = new Set<string>();
    for (const d of devices) {
      if (positiveU(d.startU) && positiveU(d.heightU) && positiveU(r.capacityU) && d.startU + d.heightU - 1 > r.capacityU) conflicts.add(d.id);
    }
    // Missing fields remain gaps; actual placement conflicts are reported separately.
    const conflictIssues = s.issues.filter(i => !i.resolved && (i.entityId === r.id || i.scope === 'placement' && devices.some(d => d.id === i.entityId)) &&
      /冲突|重叠|越界|重复|多个规划|无效/.test(i.title));
    for (const issue of conflictIssues) if (devices.some(d => d.id === issue.entityId)) conflicts.add(issue.entityId);
    const complete = devices.filter(d => placementReady(s, d) && !conflicts.has(d.id));
    const published = devices.filter(d => s.publications.some(p => p.state === 'published' && p.deviceIds.includes(d.id))).length;
    const pending = devices.filter(d => s.publications.some(p => p.state === 'pending' && p.deviceIds.includes(d.id))).length;
    const failed = devices.filter(d => s.publications.some(p => p.state === 'failed' && p.deviceIds.includes(d.id)) && !s.publications.some(p => p.state === 'published' && p.deviceIds.includes(d.id))).length;
    const rackReceipt = s.publications.some(p => p.state === 'published' && (p.facilityIds?.includes(r.id) ?? p.rackSnapshot.some(v => v.id === r.id)));
    const coordinates = !!row && numeric(row.x) && numeric(row.y) && filled(row.frame) && ['mm', 'cm', 'm'].includes(row.unit);
    const gaps = [!room && '机房归属', !positiveU(r.capacityU) && '容量', !numeric(row?.width_mm) || Number(row?.width_mm) <= 0 ? '宽度' : '', !numeric(row?.depth_mm) || Number(row?.depth_mm) <= 0 ? '深度' : '',
      devices.some(d => !positiveU(d.heightU)) && '设备高度', devices.some(d => !positiveU(d.startU)) && 'U位', !coordinates && '空间坐标'].filter(Boolean) as string[];
    return { ...r, row, room, roomId, devices, conflicts, conflictIssues, complete, published, pending, failed, rackReceipt, coordinates, gaps };
  }), [s, rackInputs]);
  const completeIds = new Set(racks.flatMap(r => r.complete.map(d => d.id)));
  const metrics: { id: CoverageKey; label: string; done: number; total: number; unit: string; table: string; detail: string }[] = [
    { id: 'room', label: '机房归属', done: racks.filter(r => r.room).length, total: racks.length, unit: '柜', table: 'racks', detail: '机柜引用可解析到已有机房实体' },
    { id: 'capacity', label: '机柜容量', done: racks.filter(r => positiveU(r.capacityU)).length, total: racks.length, unit: '柜', table: 'racks', detail: '明确的正整数容量 U' },
    { id: 'height', label: '设备高度', done: s.devices.filter(d => positiveU(d.heightU)).length, total: s.devices.length, unit: '台', table: 'equipment_specs', detail: '设备实际有效高度，正整数 U' },
    { id: 'u', label: '起始 U 位', done: s.devices.filter(d => positiveU(d.startU)).length, total: s.devices.length, unit: '台', table: 'placements', detail: '明确的正整数起始 U，不代表完整占位' },
    { id: 'placement', label: '完整落位', done: completeIds.size, total: s.devices.length, unit: '台', table: 'placements', detail: '机柜、容量、U 位、高度齐备，无越界、重叠及后端落位问题' },
    { id: 'coordinates', label: '空间坐标', done: racks.filter(r => r.coordinates).length, total: racks.length, unit: '柜', table: 'racks', detail: 'X / Y、坐标系和受支持单位均明确；0 是有效坐标' },
  ];
  const metric = metrics.find(v => v.id === filter);
  const rackMissing = (r: typeof racks[number]) => filter === 'room' ? !r.room : filter === 'capacity' ? !positiveU(r.capacityU) : filter === 'coordinates' ? !r.coordinates : false;
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
  return <Section title="机柜与设备归属" caption="按真实字段与实体关系计算覆盖率，点击任一项查看缺口。未引用的机房、机柜保留为候选。" action={<a className="text-link" href={'/racks?project_id=' + encodeURIComponent(s.projectId)} target="_blank" rel="noreferrer">打开机柜工作台（CMDB 已入库）<ArrowUpRight size={14}/></a>}>
    <div className="facility-coverage">{metrics.map(m => <button key={m.id} aria-pressed={filter === m.id} className={filter === m.id ? 'active' : ''} title={m.detail} onClick={() => setFilter(v => v === m.id ? undefined : m.id)}>
      <span>{m.label}</span><b>{m.done}<em> / {m.total} {m.unit}</em></b><small>{m.total ? `${Math.round(m.done / m.total * 100)}% · ${m.total - m.done} ${m.unit}缺口` : '暂无适用对象'}<ChevronRight size={12}/></small>
    </button>)}</div>
    {publicationError && <Notice tone="warning">设施发布状态读取失败：{publicationError}<Button onClick={() => setRetry(v => v + 1)}>重新核对</Button></Notice>}
    {metric && <div className="facility-gap-list"><div className="facility-filter-heading"><span><b>{metric.label}缺口</b> · {metric.detail}</span><div className="actions"><Button disabled={!can('bom:edit')} onClick={() => edit(metric.table)}>补充输入</Button><Button kind="ghost" onClick={() => setFilter(undefined)}>清除筛选</Button></div></div>
      {deviceFilter && <DataTable key={filter} data={missingDevices} pageSize={8} columns={[
        { accessorKey: 'name', header: '设备' }, { accessorKey: 'model', header: '型号' },
        { id: 'rack', header: '机柜', cell: ({ row }) => s.racks.find(r => r.id === row.original.rackId)?.name || '待关联' },
        { id: 'gap', header: '缺失 / 冲突', cell: ({ row }) => { const d = row.original; return [!s.racks.some(r => r.id === d.rackId) && '机柜归属', !positiveU(d.startU) && 'U位', !positiveU(d.heightU) && '高度', s.racks.some(r => r.id === d.rackId && !positiveU(r.capacityU)) && '容量', ...s.issues.filter(i => !i.resolved && i.scope === 'placement' && i.entityId === d.id).map(i => i.title), racks.some(r => r.conflicts.has(d.id)) && '占位冲突'].filter(Boolean).join(' · ') || '落位待核对'; } },
      ]}/>}
    </div>}
    <div className="facility-toolbar"><label><input type="checkbox" aria-label="选择当前筛选的机柜" checked={visibleRacks.length > 0 && visibleRacks.every(r => selectedIds.includes(r.id))} ref={el => { if (el) el.indeterminate = visibleRacks.some(r => selectedIds.includes(r.id)) && !visibleRacks.every(r => selectedIds.includes(r.id)); }} onChange={e => setSelected(e.target.checked ? [...new Set([...selectedIds, ...visibleRacks.map(r => r.id)])] : selectedIds.filter(id => !visibleRacks.some(r => r.id === id)))}/>已选 {selectedIds.length} 柜</label>
      <div className="actions"><Button kind="ghost" disabled={!selectedIds.length} onClick={() => setSelected([])}>清空选择</Button><Button disabled={!can('bom:edit') || !selectedIds.length} onClick={() => setBatch('room')}>批量关联机房</Button><Button disabled={!can('bom:edit') || !selectedIds.length} onClick={() => setBatch('geometry')}>批量容量与尺寸</Button><Button disabled={!can('bom:edit')} onClick={() => setBatch('height')}>按型号补高度</Button><Button disabled={!can('bom:edit') || !selectedIds.length} onClick={() => setBatch('coordinates')}>生成坐标</Button></div>
    </div>
    {visibleRacks.length ? <div className="facility-racks">{visibleRacks.map(r => <article key={r.id} className={'facility-rack ' + (activeRack === r.id ? 'active' : '')}>
      <div className="facility-rack-heading"><label><input type="checkbox" aria-label={`选择机柜 ${r.name}`} checked={selectedIds.includes(r.id)} onChange={() => toggle(r.id)}/><Server size={16}/><b>{r.name}</b></label><Badge state={r.conflicts.size || r.conflictIssues.length ? 'blocked' : r.gaps.length ? 'partial' : 'ready'}>{r.devices.length ? '工程候选' : '未引用候选'}</Badge></div>
      <dl><div><dt>机房</dt><dd>{r.room?.label || (r.roomId ? '引用待核对' : '待关联')}</dd></div><div><dt>容量</dt><dd>{positiveU(r.capacityU) ? `${r.capacityU} U` : '待补充'}</dd></div><div><dt>设备 / 完整占位</dt><dd>{r.devices.length} / {r.complete.length} 台</dd></div><div><dt>冲突</dt><dd className={r.conflicts.size || r.conflictIssues.length ? 'danger-text' : ''}>{r.conflicts.size} 台{r.conflictIssues.some(i => i.entityId === r.id) ? ' · 机柜字段冲突' : ''}</dd></div></dl>
      <p className="facility-rack-gaps">缺失 {r.gaps.length} 类：{r.gaps.join('、') || '当前统计项无缺口'}</p>
      <p className="facility-rack-publication" title={facilityOptions.get(r.id)?.reason}>发布：{publicationLabel(r.id, r.rackReceipt)}<br/>设备已入库 {r.published}/{r.devices.length}{r.pending ? ` · 等待回执 ${r.pending}` : ''}{r.failed ? ` · 入库失败 ${r.failed}` : ''}</p>
      <button className="text-link" onClick={() => selectRack(r.id)}>查看落位数据<ChevronRight size={12}/></button>
    </article>)}</div> : <Empty title={filter ? '当前筛选无机柜缺口' : '暂无机柜'} description={deviceFilter ? '设备缺口见上方列表，未关联机柜的设备仍保留。' : '可在下方四类工程输入中补充；候选不会因未引用而被判废弃。'}/>}
    <div className="facility-candidates">机房候选 {rooms.length} 个 · 未被机柜引用 {rooms.filter(room => !racks.some(r => r.room?.id === room.id)).length} 个 · 未关联机柜设备 {s.devices.filter(d => !s.racks.some(r => r.id === d.rackId)).length} 台</div>
    <Notice>覆盖率反映当前工程输入。机柜工作台使用 CMDB 已入库数据，不能作为未发布草稿预览；保存工程输入后仍需按原流程确认入库。</Notice>
    {batch && <FacilityBatchEditor key={batch} mode={batch} rackIds={selectedIds} close={() => setBatch(undefined)}/>}
  </Section>;
}
