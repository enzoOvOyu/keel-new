import {useState} from 'react';
import {useQuery, useQueryClient} from '@tanstack/react-query';
import {Button, Modal} from '../components/ui';
import {
  engineeringAttributes, engineeringModel, saveEngineeringAttribute, reviewEngineeringAttribute, saveEngineeringModel,
  type AttributeDefinition, type AttributeValue,
} from '../lib/api/engineering';

const groups = [
  ['equipment', 'equipment', '设备'], ['assemblies', 'assembly', '设备组合'],
  ['facilities', 'facility', '场地'], ['placements', 'placement', '落位'],
  ['ports', 'port', '端口'], ['connections', 'connection', '连接'],
  ['cables', 'cable_plan', '线缆'], ['optics', 'optic_demand', '光模块需求'],
  ['power_plans', 'power_plan', '电力规划'], ['materials', 'material_estimate', '物料'],
  ['workloads', 'workload_estimate', '施工量'], ['network_plans', 'network_plan', '网络规划'],
] as const;
const typeLabels = {text: '文本', decimal: '数值与单位', boolean: '是 / 否', reference: '工程对象引用'};

export function EngineeringAttributesPanel({project, id, revision, canEdit, canReview}: {
  project: string; id: string; revision: number; canEdit: boolean; canReview: boolean;
}) {
  const cache = useQueryClient();
  const definitions = useQuery({queryKey: ['engineering-attributes', project], queryFn: ({signal}) => engineeringAttributes(project, signal)});
  const model = useQuery({queryKey: ['engineering-model', project, id, revision], queryFn: ({signal}) => engineeringModel(project, id, signal)});
  const [definition, setDefinition] = useState<AttributeDefinition | null>(null);
  const [definitionVersion, setDefinitionVersion] = useState(0);
  const [selected, setSelected] = useState('');
  const [edit, setEdit] = useState<{id: string; revision: number; values: AttributeValue[]} | null>(null);
  const [reason, setReason] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const objects = groups.flatMap(([group, kind, label]) => (model.data?.model[group] || []).map(entity => {
    const named = entity as {label?: string; name?: string; source_model_label?: string};
    return {group, kind, entity, label: label + ' / ' + (named.label || named.name || named.source_model_label || entity.entity_id.slice(0, 8))};
  }));
  const object = objects.find(v => v.entity.entity_id === (edit?.id || selected));
  const latest = (definitions.data || []).filter(d => !(definitions.data || []).some(other => other.id === d.id && other.version > d.version));
  const applicable = (definitions.data || []).filter(d => d.approval_status === 'approved' && d.entity_kind === object?.kind && !(definitions.data || []).some(other => other.id === d.id && other.approval_status === 'approved' && other.version > d.version));

  async function action(fn: () => Promise<unknown>) {
    setBusy(true); setError('');
    try {
      await fn();
      await Promise.all([
        cache.invalidateQueries({queryKey: ['engineering-attributes', project]}),
        cache.invalidateQueries({queryKey: ['engineering-model', project, id]}),
        cache.invalidateQueries({queryKey: ['engineering-workspace', project, id]}),
      ]);
    } catch (e) {setError((e as Error).message)} finally {setBusy(false)}
  }
  function changeValue(d: AttributeDefinition, input: string) {
    if (!edit) return;
    const values = edit.values.filter(v => v.definition_id !== d.id);
    if (input !== '') {
      const value: AttributeValue['value'] = d.value_type === 'decimal' ? {kind: 'decimal', decimal: input, unit: d.unit} :
        d.value_type === 'boolean' ? {kind: 'boolean', boolean: input === 'true'} :
        d.value_type === 'reference' ? {kind: 'reference', reference: input} : {kind: 'text', text: input};
      values.push({definition_id: d.id, definition_version: d.version, value});
    }
    setEdit({...edit, values});
  }
  async function saveValues() {
    if (!edit || !object || !model.data) return;
    const next = structuredClone(model.data.model);
    const entity = next[object.group].find(v => v.entity_id === edit.id);
    if (!entity) return;
    entity.attributes = edit.values;
    await saveEngineeringModel(project, id, {expected_revision: edit.revision, model: next, reason});
    setEdit(null);
  }
  return <section className="panel padded">
    <div className="work-heading"><div><h3>项目扩展属性</h3><p>为不同型号和项目补充有类型的属性。定义审核后使用，数据随工程模型独立审核发布。</p></div>
      <Button disabled={!canEdit || busy} onClick={() => {setDefinition({id: '', version: 1, entity_kind: 'equipment', key: '', value_type: 'text', enum_values: [], required: false, approval_status: 'draft'});setDefinitionVersion(0);setReason('')}}>定义属性</Button></div>
    {(error || definitions.error || model.error) && <p role="alert" className="notice error">{error || definitions.error?.message || model.error?.message}</p>}
    <div className="table-wrap"><table><thead><tr><th>名称 / 对象</th><th>类型</th><th>版本</th><th>状态</th><th>操作</th></tr></thead><tbody>{latest.map(d => <tr key={d.id}>
      <td>{d.key}<small className="engineering-code">{groups.find(g => g[1] === d.entity_kind)?.[2]}</small></td><td>{typeLabels[d.value_type]} {d.unit}</td><td>{d.version}</td><td>{{draft: '待审核', approved: '已批准', retired: '已驳回'}[d.approval_status]}</td>
      <td><Button disabled={!canEdit || busy} onClick={() => {setDefinition(structuredClone(d));setDefinitionVersion(d.version);setReason('')}}>修订定义</Button>{d.approval_status === 'draft' && <>
        <Button disabled={!canReview || busy || !reason.trim()} onClick={() => void action(() => reviewEngineeringAttribute(project, d.id, {version: d.version, decision: 'approve', reason}))}>批准</Button>
        <Button disabled={!canReview || busy || !reason.trim()} onClick={() => void action(() => reviewEngineeringAttribute(project, d.id, {version: d.version, decision: 'reject', reason}))}>驳回</Button></>}</td>
    </tr>)}</tbody></table></div>
    {latest.some(d => d.approval_status === 'draft') && <label className="bom-review-reason">定义审核依据<textarea value={reason} onChange={e => setReason(e.target.value)} placeholder="填写理由后批准或驳回对应定义"/></label>}
    {!latest.length && <p className="muted">没有项目扩展定义；核心模型字段仍可正常使用。</p>}
    <div className="bom-validation-toolbar"><select aria-label="扩展属性所属对象" value={selected} onChange={e => setSelected(e.target.value)}><option value="">选择工程对象</option>{objects.map(v => <option key={v.entity.entity_id} value={v.entity.entity_id}>{v.label}</option>)}</select>
      <Button disabled={!object || !canEdit || busy} onClick={() => {if (object) {setEdit({id: object.entity.entity_id, revision, values: structuredClone(object.entity.attributes)});setReason('')}}}>查看与修订属性值</Button></div>
    {definition && <Modal sheet title="扩展属性定义" description="每次保存创建独立版本，批准前不能用于正式工程值。" onClose={() => setDefinition(null)}>
      <div className="engineering-editor">
        <label>属性名称<input value={definition.key} onChange={e => setDefinition({...definition, key: e.target.value})}/></label>
        <label>适用对象<select value={definition.entity_kind} onChange={e => setDefinition({...definition, entity_kind: e.target.value})}>{groups.map(([,kind,label]) => <option value={kind} key={kind}>{label}</option>)}</select></label>
        <label>值类型<select value={definition.value_type} onChange={e => setDefinition({...definition, value_type: e.target.value as AttributeDefinition['value_type'], unit: e.target.value === 'decimal' ? '' : undefined, enum_values: []})}>{Object.entries(typeLabels).map(([value,label]) => <option key={value} value={value}>{label}</option>)}</select></label>
        {definition.value_type === 'decimal' && <label>明确单位<input value={definition.unit || ''} onChange={e => setDefinition({...definition, unit: e.target.value})}/></label>}
        {definition.value_type === 'text' && <label>可选值（每行一个，留空为自由文本）<textarea value={definition.enum_values.join('\n')} onChange={e => setDefinition({...definition, enum_values: e.target.value.split('\n').filter(Boolean)})}/></label>}
        <label><input type="checkbox" checked={definition.required} onChange={e => setDefinition({...definition, required: e.target.checked})}/> 此项目该类对象发布前必须填写</label>
      </div>
      <label className="bom-review-reason">定义依据<textarea value={reason} onChange={e => setReason(e.target.value)}/></label>
      <Button variant="primary" disabled={busy || !reason.trim() || !definition.key.trim() || definition.value_type === 'decimal' && !definition.unit?.trim()} onClick={() => void action(async () => {await saveEngineeringAttribute(project, {expected_version: definitionVersion, definition, reason});setDefinition(null)})}>保存待审核定义</Button>
    </Modal>}
    {edit && <Modal sheet title={object?.label || '扩展属性'} description="属性值采用已批准的类型和单位，保存后创建工程新版本。" onClose={() => setEdit(null)}>
      {!applicable.length && <p>此类对象尚无已批准定义。</p>}
      <div className="engineering-editor">{applicable.map(d => {
        const value = edit.values.find(v => v.definition_id === d.id)?.value;
        const text = String(value?.text ?? value?.decimal ?? value?.boolean ?? value?.reference ?? '');
        const options = d.value_type === 'boolean' ? [{id:'true',label:'是'},{id:'false',label:'否'}] : d.value_type === 'reference' ? objects.map(v => ({id:v.entity.entity_id,label:v.label})) : d.enum_values.map(v => ({id:v,label:v}));
        return <label key={d.id}>{d.key} {d.unit || ''}{d.required ? ' · 必填' : ''}{options.length ? <select value={text} onChange={e => changeValue(d,e.target.value)}><option value="">未提供</option>{options.map(v => <option key={v.id} value={v.id}>{v.label}</option>)}</select> : <input value={text} onChange={e => changeValue(d,e.target.value)} />}</label>;
      })}</div>
      <label className="bom-review-reason">修改依据<textarea value={reason} onChange={e => setReason(e.target.value)}/></label>
      <Button variant="primary" disabled={busy || !canEdit || !reason.trim() || !applicable.length} onClick={() => void action(saveValues)}>保存新版本</Button>
    </Modal>}
  </section>;
}
