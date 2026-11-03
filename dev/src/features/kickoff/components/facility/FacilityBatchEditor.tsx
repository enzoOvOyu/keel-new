import { useState } from 'react';
import { Save } from 'lucide-react';
import { useWorkspace } from '../../context';
import { tableById, type ModelRow } from '../../lib/api';
import { Button, DataTable, Field, Modal, Notice } from '../ui';
import { EntityField, entityOptions, filled, modelRows, numeric, positiveU, resolveEntity } from './FacilityEntities';

type Mode = 'room' | 'geometry' | 'height' | 'coordinates';
type Change = { target: string; field: string; before: string; after: string; grid: string };
type Plan = { rows: ModelRow[]; changes: Change[]; notes: string[]; revision: number; projectId: string; affected: number; heightConflict: boolean };
const titles: Record<Mode, string> = { room: '批量关联机房', geometry: '批量容量与尺寸', height: '按型号补高度', coordinates: '按明确参数生成坐标' };

export function FacilityBatchEditor({ mode, rackIds, close }: { mode: Mode; rackIds: string[]; close: () => void }) {
  const { state: s, api, run, can } = useWorkspace();
  const [values, setValues] = useState<Record<string, string>>({});
  const [overwrite, setOverwrite] = useState(false);
  const [plan, setPlan] = useState<Plan>();
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);
  const table = mode === 'height' ? 'equipment_specs' : 'racks';
  const def = tableById(table);
  const racks = rackIds.map(id => s.racks.find(r => r.id === id)).filter((r): r is NonNullable<typeof r> => !!r);
  const models = [...new Set(s.devices.map(d => d.model).filter(Boolean))].sort();
  const matching = s.devices.filter(d => d.model === values.model);
  const missingHeight = matching.filter(d => !positiveU(d.heightU));
  const existingHeight = matching.filter(d => positiveU(d.heightU));
  const heightConflicts = existingHeight.filter(d => numeric(values.height_u) && d.heightU !== Number(values.height_u));
  const change = (key: string, value: string) => { setValues(v => ({ ...v, [key]: value })); setPlan(undefined); setError(''); };
  const input = (key: string, label: string, hint?: string) => <Field label={label} hint={hint}><input value={values[key] ?? ''} inputMode={['frame', 'model'].includes(key) ? undefined : 'decimal'} onChange={e => change(key, e.target.value)}/></Field>;
  function preview() {
    setError('');
    try {
      const rows = structuredClone(modelRows(s, table));
      const changes: Change[] = [], notes: string[] = [];
      let affected = 0;
      const set = (row: ModelRow, key: string, value: string, target: string, grid = '', existing?: string) => {
        const before = filled(row[key]) ? row[key] : existing ?? '';
        if (!filled(value) || (!overwrite && filled(before)) || before === value) return;
        row[key] = value;
        changes.push({ target, field: def.fields.find(f => f.key === key)?.label ?? key, before, after: value, grid });
      };
      const positive = (key: string, integer = false, required = false) => {
        if (!filled(values[key]) && !required) return;
        const n = Number(values[key]);
        if (!numeric(values[key]) || n <= 0 || (integer && !Number.isInteger(n))) throw Error(`${key === 'height_u' ? '设备高度' : def.fields.find(f => f.key === key)?.label ?? key}必须为正${integer ? '整数' : '数'}。`);
      };
      if (mode === 'height') {
        if (!models.includes(values.model)) throw Error('请选择已有设备的明确型号。');
        positive('height_u', true, true);
        const matches = rows.filter(row => row.model === values.model);
        if (matches.length > 1) throw Error('该型号有多条规格，请先在设备型号输入中核对，再批量补充。');
        let row = matches[0];
        if (!row) { row = { model: values.model }; rows.push(row); }
        set(row, 'height_u', String(Number(values.height_u)), values.model);
        affected = changes.length ? missingHeight.length : 0;
        notes.push(`型号匹配 ${matching.length} 台；缺高度 ${missingHeight.length} 台；本次预计补充 ${affected} 台。已有实例或落位高度保持原值，保存后以后端重新校验为准。`);
        if (filled(row.height_u) && !changes.length) notes.push('该型号已有高度，本次只填空值，不覆盖既有规格。');
      } else {
        if (!racks.length) throw Error('请先选择机柜。');
        if (mode === 'room' && !entityOptions(s, 'room_id').some(r => r.id === values.room_id)) throw Error('请选择已有机房实体；可先在机房输入中创建。');
        if (mode === 'geometry') {
          if (!['capacity_u', 'width_mm', 'depth_mm'].some(k => filled(values[k]))) throw Error('至少填写一项容量或尺寸。');
          positive('capacity_u', true); positive('width_mm'); positive('depth_mm');
        }
        if (mode === 'coordinates') {
          if (!filled(values.frame) || !['mm', 'cm', 'm'].includes(values.unit)) throw Error('请明确填写坐标系并选择单位。');
          for (const key of ['rows', 'columns']) if (!numeric(values[key]) || Number(values[key]) < 1 || !Number.isInteger(Number(values[key]))) throw Error('行数和列数必须为正整数。');
          if (Number(values.rows) * Number(values.columns) < racks.length) throw Error('行列容量不足，无法容纳所选机柜。');
          if (!['row', 'column'].includes(values.order)) throw Error('请选择先行后列或先列后行。');
          for (const key of ['origin_x', 'origin_y', 'spacing_x', 'spacing_y']) if (!numeric(values[key])) throw Error('起点 X / Y 和两轴间距必须显式填写，无默认距离。');
          if ((Number(values.columns) > 1 && Number(values.spacing_x) === 0) || (Number(values.rows) > 1 && Number(values.spacing_y) === 0)) throw Error('存在多行或多列时，对应轴间距不能为 0；可填写负数表示反向。');
        }
        racks.forEach((rack, index) => {
          const candidates = rows.filter(r => resolveEntity(entityOptions(s, 'rack_id'), r.rack_id) === rack.id);
          if (candidates.length > 1) throw Error(`机柜 ${rack.name} 存在多条输入，请先合并核对。`);
          let row = candidates[0];
          if (!row) { row = { rack_id: rack.id }; rows.push(row); }
          const start = changes.length;
          if (mode === 'room') set(row, 'room_id', values.room_id, rack.name, '', rack.room);
          if (mode === 'geometry') for (const key of ['capacity_u', 'width_mm', 'depth_mm']) set(row, key, filled(values[key]) ? String(Number(values[key])) : '', rack.name, '', key === 'capacity_u' && rack.capacityU != null ? String(rack.capacityU) : '');
          if (mode === 'coordinates') {
            // Never splice a newly generated point into an existing coordinate pair.
            if (!overwrite && (filled(row.x) || filled(row.y) || (filled(row.frame) && row.frame !== values.frame.trim()) || (filled(row.unit) && row.unit !== values.unit))) {
              notes.push(`${rack.name}：已有坐标、不同坐标系或单位，整组保留；如需重新布置，请明确启用覆盖。`);
              return;
            }
            const column = values.order === 'row' ? index % Number(values.columns) : Math.floor(index / Number(values.rows));
            const line = values.order === 'row' ? Math.floor(index / Number(values.columns)) : index % Number(values.rows);
            const coordinates = { x: String(Number(values.origin_x) + column * Number(values.spacing_x)), y: String(Number(values.origin_y) + line * Number(values.spacing_y)), frame: values.frame.trim(), unit: values.unit };
            // A point is one value group, including its frame and unit.
            for (const [key, value] of Object.entries(coordinates)) {
              const before = row[key] ?? '';
              if (before !== value) { row[key] = value; changes.push({ target: rack.name, field: def.fields.find(f => f.key === key)?.label ?? key, before, after: value, grid: `${line + 1} 行 / ${column + 1} 列` }); }
            }
          }
          if (changes.length > start) affected++;
        });
      }
      const heightConflict = mode === 'height' && heightConflicts.length > 0;
      if (heightConflict) {
        notes.push(`${heightConflicts.length} 台实例已有不同高度，已保护；本次型号批量保存被阻止。请逐设备核对落位或拆分真实型号，不能用全型号默认值覆盖实例。`);
        affected = 0;
      }
      setPlan({ rows, changes, notes, revision: s.revision, projectId: s.projectId, affected, heightConflict });
    } catch (e) { setPlan(undefined); setError((e as Error).message); }
  }
  const stale = !!plan && (plan.revision !== s.revision || plan.projectId !== s.projectId);
  return <Modal title={titles[mode]} description={mode === 'height' ? '只补充明确匹配型号的高度；已有设备和落位高度不覆盖。' : `对所选 ${racks.length} 个机柜赋值，先预览字段差异，再保存工程输入。`} close={() => { if (!saving) close(); }} wide>
    <fieldset disabled={saving} className="facility-batch-fields">
      {mode === 'room' && <Field label="关联机房"><EntityField label="批量关联机房" value={values.room_id ?? ''} options={entityOptions(s, 'room_id')} onChange={v => change('room_id', v)}/></Field>}
      {mode === 'geometry' && <>{input('capacity_u', '机柜容量 U', '正整数，留空表示本次不赋值')}{input('width_mm', '机柜宽度 mm')}{input('depth_mm', '机柜深度 mm')}</>}
      {mode === 'height' && <><Field label="设备型号"><select value={values.model ?? ''} onChange={e => change('model', e.target.value)}><option value="">选择明确型号</option>{models.map(model => <option key={model} value={model}>{model}</option>)}</select></Field>{input('height_u', '设备高度 U', '正整数；型号按原值精确匹配')}<p className="facility-batch-impact">匹配 {matching.length} 台 · 缺高度 {missingHeight.length} 台 · 已有高度 {matching.length - missingHeight.length} 台</p></>}
      {mode === 'coordinates' && <>{input('frame', '坐标系', '由用户明确提供参考系标识')}<Field label="坐标与间距单位"><select value={values.unit ?? ''} onChange={e => change('unit', e.target.value)}><option value="">请选择单位</option><option value="mm">毫米 mm</option><option value="cm">厘米 cm</option><option value="m">米 m</option></select></Field>{input('rows', '行数')}{input('columns', '列数')}<Field label="机柜排列方向"><select value={values.order ?? ''} onChange={e => change('order', e.target.value)}><option value="">请选择排列方式</option><option value="row">先填满一行，再向下一行</option><option value="column">先填满一列，再向下一列</option></select></Field>{input('origin_x', '第 1 行第 1 列起点 X')}{input('origin_y', '第 1 行第 1 列起点 Y')}{input('spacing_x', '列坐标步距 ΔX', '相邻机柜同一参考点的坐标差，非净距；负数表示反向')}{input('spacing_y', '行坐标步距 ΔY', '相邻行同一参考点的坐标差，非净距；负数表示反向')}</>}
      {mode !== 'height' && <label className="facility-overwrite"><input type="checkbox" checked={overwrite} onChange={e => { setOverwrite(e.target.checked); setPlan(undefined); }}/>明确覆盖已有值（默认关闭）</label>}
    </fieldset>
    {mode !== 'height' && <details className="facility-selection"><summary>赋值顺序 · {racks.length} 个机柜（按选择顺序）</summary><ol>{racks.map(r => <li key={r.id}>{r.name}<small>{r.id}</small></li>)}</ol></details>}
    {mode === 'height' && existingHeight.length > 0 && <div className="facility-assignment-preview"><div className="facility-filter-heading"><b>已有实例高度 · {existingHeight.length} 台保护</b><span>{heightConflicts.length} 台与拟填高度不同</span></div>
      <DataTable data={existingHeight} pageSize={8} columns={[{ accessorKey: 'name', header: '设备' }, { accessorKey: 'heightU', header: '已有高度 U' }, { id: 'protection', header: '保护 / 冲突', cell: ({ row }) => numeric(values.height_u) && row.original.heightU !== Number(values.height_u) ? '高度不同 · 阻止全型号保存' : '已有值保留' }]}/>
    </div>}
    {mode === 'coordinates' && <Notice>坐标为规划输入，无默认距离。X = 起点 X + 列偏移 × ΔX；Y = 起点 Y + 行偏移 × ΔY。已有完整或部分坐标默认整组保留；尺寸不用于猜测间距。</Notice>}
    {error && <Notice tone="error">{error}</Notice>}
    {plan && <div className="facility-assignment-preview"><div className="facility-filter-heading"><b>工程草稿 · 赋值预览</b><span>{plan.changes.length} 项字段变更 · 预计影响 {plan.affected} {mode === 'height' ? '台设备' : '个机柜'}</span></div>
      {overwrite && <Notice tone="warning">本次已明确启用覆盖。请逐项核对已有值的替换。</Notice>}
      {plan.notes.map((note, i) => <Notice key={i}>{note}</Notice>)}
      <DataTable data={plan.changes} pageSize={10} columns={[{ accessorKey: 'target', header: mode === 'height' ? '型号' : '机柜' }, ...(mode === 'coordinates' ? [{ accessorKey: 'grid', header: '排列位置' }] : []), { accessorKey: 'field', header: '字段' }, { accessorKey: 'before', header: '原值', cell: ({ getValue }) => getValue() || '空值' }, { accessorKey: 'after', header: '赋值后' }]}/>
      {!plan.changes.length && <Notice>没有可赋值的空缺，已有值保持不变。</Notice>}
    </div>}
    {stale && <Notice tone="warning">工程版本已变化，请重新生成预览后保存。</Notice>}
    <div className="modal-actions"><Button disabled={saving} onClick={close}>取消</Button><Button disabled={saving || !can('bom:edit')} onClick={preview}>生成赋值预览</Button><Button kind="primary" disabled={saving || !can('bom:edit') || !plan?.changes.length || plan.heightConflict || stale} onClick={async () => {
      if (!plan || stale || saving || plan.heightConflict) return;
      setSaving(true);
      const saved = await run('批量工程输入已保存', () => api.saveModel(table, plan.rows, titles[mode]));
      setSaving(false);
      if (saved) close(); else setPlan(undefined);
    }}><Save size={14}/>{saving ? '正在保存' : '确认保存工程输入'}</Button></div>
  </Modal>;
}
