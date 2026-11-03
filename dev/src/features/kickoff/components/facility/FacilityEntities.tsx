import { useState } from 'react';
import type { ModelRow, Workspace } from '../../lib/api';

export type EntityOption = { id: string; label: string };
export const modelRows = (state: Workspace, table: string) => state.models.find(m => m.tableId === table)?.rows ?? [];
export const filled = (value: string | undefined) => !!value?.trim();
export const numeric = (value: string | undefined) => filled(value) && Number.isFinite(Number(value));
export const positiveU = (value: number | null | undefined): value is number => value != null && Number.isInteger(value) && value > 0;

export function entityOptions(state: Workspace, key: string, localTable?: string, localRows?: ModelRow[]): EntityOption[] {
  if (key === 'device_id') return state.devices.map(d => ({ id: d.id, label: [d.name, d.serial !== d.name ? d.serial : '', d.model].filter(Boolean).join(' · ') || d.id }));
  if (key === 'rack_id') return state.racks.map((r, index) => ({ id: r.id, label: r.name && r.name !== r.id ? r.name : `未命名机柜 ${index + 1}` }));
  if (key === 'room_id') {
    const rows = localTable === 'rooms' && localRows ? localRows : modelRows(state, 'rooms');
    return [...new Map(rows.filter(r => filled(r.room_id)).map((r, index) => [r.room_id, { id: r.room_id, label: r.name || `未命名机房 ${index + 1}` }])).values()];
  }
  return [];
}

// Only an exact, unique label is an alias. Display labels never replace an ID.
export function resolveEntity(options: EntityOption[], value: string) {
  if (options.some(o => o.id === value)) return value;
  const matches = options.filter(o => o.label === value);
  return matches.length === 1 ? matches[0].id : value;
}

export function EntityField({ value, options, label, onChange, own = false, disabled = false, ownName, onNameChange }: {
  value: string; options: EntityOption[]; label: string; onChange: (value: string) => void; own?: boolean; disabled?: boolean;
  ownName?: string; onNameChange?: (value: string) => void;
}) {
  const canonical = resolveEntity(options, value);
  const selected = options.find(o => o.id === canonical);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState('');
  const [code, setCode] = useState('');
  const visibleName = ownName ?? name;
  const assignNew = (nextName: string, nextCode: string) => {
    setName(nextName); setCode(nextCode);
    onChange(nextCode.trim() || nextName.trim());
    onNameChange?.(nextName);
  };
  return <div className="facility-entity-field">
    <select aria-label={label} value={creating ? '__new_planning__' : canonical} disabled={disabled} onChange={e => {
      if (e.target.value === '__new_planning__') { setCreating(true); assignNew('', ''); return; }
      setCreating(false); onChange(e.target.value);
      if (own) onNameChange?.(options.find(o => o.id === e.target.value)?.label ?? '');
    }}>
      <option value="">{own ? '选择已有实体或新增规划' : '待关联'}</option>
      {value && !selected && !creating && <option value={value}>引用待核对（可重新选择）</option>}
      {options.map(o => <option key={o.id} value={o.id}>{o.label}</option>)}
      {own && <option value="__new_planning__">＋ 新增规划实体</option>}
    </select>
    {own && creating && <>
      <input aria-label={`${label} 规划名称`} value={visibleName} disabled={disabled} placeholder="规划名称" onChange={e => assignNew(e.target.value, code)}/>
      <input aria-label={`${label} 规划编码（可空）`} value={code} disabled={disabled} placeholder="规划编码（可空，按名称生成）" onChange={e => assignNew(visibleName, e.target.value)}/>
      <small>编码留空时以名称建立规划引用，已有实体 ID 在内部保留。</small>
    </>}
  </div>;
}
