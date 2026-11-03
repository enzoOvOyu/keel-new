import { useRef, useState } from 'react';
import { Download, Plus, Save, Trash2, Upload } from 'lucide-react';
import { useWorkspace } from '../context';
import { tableById, type ModelRow, type ImportPreview } from '../lib/api';
import { exportWorkbook, readModelFile } from '../lib/api/workbook';
import { Button, Modal, Notice } from './ui';
import { EntityField, entityOptions, resolveEntity } from './facility/FacilityEntities';
import './facility/facility.css';

export function ModelEditor({ tableId, close }: { tableId: string; close: () => void }) {
  const { api, state, run, can } = useWorkspace();
  const def = tableById(tableId);
  const existing = state.models.find(m => m.tableId === tableId);
  const [rows, setRows] = useState<ModelRow[]>(() => structuredClone(existing?.rows ?? []));
  const [preview, setPreview] = useState<ImportPreview>(), [error, setError] = useState(''), [source, setSource] = useState('页面编辑');
  const [page, setPage] = useState(0), [dirty, setDirty] = useState(false), [saving, setSaving] = useState(false);
  const [baseRevision] = useState(state.revision);
  const picker = useRef<HTMLInputElement>(null);
  const pageSize = 12, pageRows = rows.slice(page * pageSize, (page + 1) * pageSize);
  const editable = can('bom:edit') && !saving;
  const stale = baseRevision !== state.revision;
  const isReference = (key: string) => ['device_id', 'rack_id', 'room_id'].includes(key);
  const ownReference = (key: string) => tableId === 'rooms' && key === 'room_id' || tableId === 'racks' && key === 'rack_id';
  const options = (key: string) => entityOptions(state, key, tableId, rows);
  const labelFor = (key: string, value: string) => options(key).find(o => o.id === resolveEntity(options(key), value))?.label || value;
  async function importFile(file: File) {
    try { setError(''); setPreview(await readModelFile(file, def)); setSource(file.name); }
    catch (e) { setError((e as Error).message); }
  }
  const finish = () => { if (saving || dirty && !window.confirm('有尚未应用的编辑。放弃这些编辑并关闭？')) return; close(); };
  const update = (absolute: number, key: string, value: string) => { setRows(v => v.map((r, i) => i === absolute ? { ...r, [key]: value } : r)); setDirty(true); };
  return <Modal title={def.name + ' · 编辑输入'} description={def.description} close={finish} wide>
    <div className="editor-tools"><div><b>{rows.length} 行</b><span>空配置可以保留，使用时再检查对应条件。</span></div><div className="actions">
      <Button onClick={() => exportWorkbook(def, [], true)}><Download size={14}/>下载XLSX模板</Button>
      <Button onClick={() => exportWorkbook(def, rows, false)}><Download size={14}/>导出当前输入</Button>
      <Button disabled={!editable} onClick={() => picker.current?.click()}><Upload size={14}/>导入XLSX / CSV</Button>
      <Button disabled={!editable} onClick={() => {
        const row = Object.fromEntries(def.fields.map(f => [f.key, '']));
        setRows([...rows, row]); setPage(Math.floor(rows.length / pageSize)); setDirty(true);
      }}><Plus size={14}/>新增行</Button>
    </div></div>
    <input ref={picker} type="file" hidden accept=".xlsx,.csv" disabled={!editable} onChange={e => { const file = e.target.files?.[0]; if (file) void importFile(file); e.target.value = ''; }}/>
    {def.fields.some(f => isReference(f.key)) && <Notice>关联字段按名称选择已有实体，内部保存稳定 ID。新增规划可填写名称及可选编码，编码留空按名称生成；导入引用保留并交由后端校验。</Notice>}
    {error && <Notice tone="error">{error}</Notice>}
    {stale && <Notice tone="warning">工程版本已变化，请保留导出后关闭并重新打开，避免覆盖新输入。</Notice>}
    {preview && <div className="import-preview"><div><b>导入预览 · {preview.sheet}</b><span>{preview.rows.length} 行 · {preview.errors.length ? '需要处理表头' : '字段已匹配'}</span></div>
      {preview.errors.map(e => <Notice key={e} tone="error">{e}</Notice>)}
      <div className="table-scroll"><table><thead><tr>{def.fields.map(f => <th key={f.key}>{f.label}</th>)}</tr></thead><tbody>{preview.rows.slice(0, 3).map((r, i) => <tr key={i}>{def.fields.map(f => <td key={f.key}>{labelFor(f.key, r[f.key]) || '—'}</td>)}</tr>)}</tbody></table></div>
      <div className="actions"><Button disabled={!editable} onClick={() => setPreview(undefined)}>取消导入</Button><Button kind="primary" disabled={!editable || !!preview.errors.length} onClick={() => {
        setRows(preview.rows.map(row => Object.fromEntries(Object.entries(row).map(([key, value]) => [key, isReference(key) ? resolveEntity(options(key), value) : value]))));
        setPreview(undefined); setPage(0); setDirty(true);
      }}>采用 {preview.rows.length} 行到编辑区</Button></div>
    </div>}
    <div className="editor-grid table-scroll"><table><thead><tr><th>#</th>{def.fields.map(f => <th key={f.key}><b>{f.label}</b><code>{f.key}</code></th>)}<th>操作</th></tr></thead><tbody>{pageRows.map((row, index) => {
      const absolute = page * pageSize + index;
      return <tr key={absolute}><td>{absolute + 1}</td>{def.fields.map(f => <td key={f.key}>
        {isReference(f.key) ? <EntityField label={`${f.label} 第${absolute + 1}行`} value={row[f.key] ?? ''} options={options(f.key)} own={ownReference(f.key)} ownName={tableId === 'rooms' && f.key === 'room_id' ? row.name : undefined} onNameChange={tableId === 'rooms' && f.key === 'room_id' ? value => update(absolute, 'name', value) : undefined} disabled={!editable} onChange={value => update(absolute, f.key, value)}/> :
          <input aria-label={`${f.label} 第${absolute + 1}行`} inputMode={f.kind === 'positive' || f.kind === 'number' ? 'decimal' : undefined} disabled={!editable} value={row[f.key] ?? ''} placeholder="待补充" onChange={e => update(absolute, f.key, e.target.value)}/>}
      </td>)}<td><button className="icon-button" disabled={!editable} aria-label={`删除第${absolute + 1}行`} onClick={() => { setRows(rows.filter((_, i) => i !== absolute)); setPage(Math.min(page, Math.max(0, Math.ceil((rows.length - 1) / pageSize) - 1))); setDirty(true); }}><Trash2 size={14}/></button></td></tr>;
    })}</tbody></table>{!rows.length && <div className="editor-empty">从 XLSX 导入，或新增一行开始填写。暂时没有依据时可以留空。</div>}</div>
    <div className="editor-pagination"><span>{rows.length > pageSize ? '分段编辑，所有已填内容会一起保存' : '每个字段只保存明确提供的值'}</span><div className="actions"><Button disabled={page === 0} onClick={() => setPage(page - 1)}>上一页</Button><span>{page + 1} / {Math.max(1, Math.ceil(rows.length / pageSize))}</span><Button disabled={(page + 1) * pageSize >= rows.length} onClick={() => setPage(page + 1)}>下一页</Button></div></div>
    <div className="editor-impact"><b>本次输入用于</b><span>{def.consumer}</span><small>其他工程域可以继续工作；计算结果按依赖标记过期。</small></div>
    <div className="modal-actions"><Button disabled={saving} onClick={finish}>取消</Button><Button kind="primary" disabled={!editable || !!preview || stale} onClick={async () => {
      setSaving(true);
      const normalized = rows.map(row => Object.fromEntries(Object.entries(row).map(([key, value]) => [key, isReference(key) ? resolveEntity(options(key), value) : value])));
      const saved = await run('工程输入已更新', () => api.saveModel(tableId, normalized, source));
      setSaving(false);
      if (saved) { setDirty(false); close(); }
    }}><Save size={14}/>{saving ? '正在保存' : '应用输入'}</Button></div>
  </Modal>;
}
