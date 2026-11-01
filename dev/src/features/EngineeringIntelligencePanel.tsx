import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {Button} from '../components/ui';
import {
  engineeringIntelligence, engineeringSource,
  type EngineeringPlanView, type WorkbookRegion,
} from '../lib/api/engineering';

const stateLabels: Record<string, string> = {
  NOT_APPLICABLE: '不适用', DETECTED: '待确认', NEEDS_INPUT: '待补充', READY: '就绪',
  PROCESSING: '处理中', IN_REVIEW: '待审核', APPROVED: '已批准', PUBLISHED: '已发布',
  BLOCKED: '已阻断', STALE: '待重新审核',
};
const domainTabs: Record<string, string> = {
  equipment: 'model', facility_placement: 'model', connectivity_cabling: 'connectivity',
  power_cooling: 'power', material_workload: 'estimates', network_design: 'network',
  workbook_intelligence: 'understanding', validation_release: 'release',
};

export function EngineeringModuleOverview({view, onOpen}: {
  view: EngineeringPlanView; onOpen: (tab: string) => void;
}) {
  return <section className="engineering-module-overview">
    <div className="work-heading"><div><h3>工程模块</h3><p>资料不完整的模块单独保留问题，其余模块可以继续审核与发布。</p></div></div>
    <div className="engineering-module-grid">
      {view.plan.modules.filter(m => !['workbook_intelligence', 'validation_release'].includes(m.definition_id)).map(m => {
        const definition = view.definitions.find(d => d.id === m.definition_id);
        const unavailable = m.inputs.filter(input => input.required && !input.available);
        return <article className="panel padded" key={m.id}>
          <div className="engineering-module-heading"><h4>{definition?.title || m.definition_id}</h4><span data-state={m.state}>{stateLabels[m.state]}</span></div>
          <p>{m.applicability_reason}</p>
          {unavailable.length > 0 && <p className="muted">待补充：{unavailable.map(input => ({facts: '来源依据', equipment: '设备信息', connectivity: '物理连接'}[input.name] || input.name)).join('、')}</p>}
          <details><summary>输入依赖与审核要求</summary>
            <ul>{m.inputs.map(input => <li key={input.name}>{definition?.inputs.find(v => v.name === input.name)?.entity_type || input.name} · {input.available ? '已有依据' : input.required ? '需要补充' : '按需提供'}</li>)}</ul>
            <p>{definition?.review_gates.length ? '每个发布目标独立人工审核，修改后重新核对受影响的目标。' : '查看分析结果与来源。'}</p>
          </details>
          <Button onClick={() => onOpen(domainTabs[m.definition_id] || 'model')}>查看与处理</Button>
        </article>;
      })}
    </div>
  </section>;
}

export function EngineeringEvidenceExplorer({project, id, revision}: {project: string; id: string; revision: number}) {
  const [offset, setOffset] = useState(0);
  const [selected, setSelected] = useState<WorkbookRegion | null>(null);
  const [rowOffset, setRowOffset] = useState(0);
  const [columnOffset, setColumnOffset] = useState(0);
  const intelligence = useQuery({
    queryKey: ['engineering-intelligence', project, id, revision, offset],
    queryFn: ({signal}) => engineeringIntelligence(project, id, offset, signal),
  });
  const bounds = selected && {
    start_row: selected.bounds.start_row + rowOffset,
    end_row: Math.min(selected.bounds.start_row + rowOffset + 31, selected.bounds.end_row),
    start_column: selected.bounds.start_column + columnOffset,
    end_column: Math.min(selected.bounds.start_column + columnOffset + 15, selected.bounds.end_column),
  };
  const cells = useQuery({
    queryKey: ['engineering-region-source', project, id, selected?.id, rowOffset, columnOffset],
    queryFn: ({signal}) => engineeringSource(project, id, selected!.sheet_id, bounds!, signal),
    enabled: !!selected,
  });
  const data = intelligence.data;
  return <div className="engineering-overview">
    <section>
      <h3>来源区域</h3>
      <p className="muted">原始输入、公式和语义建议分别保留；未映射资料仍保存在原件中。</p>
      {intelligence.isPending && <p>正在读取来源…</p>}
      {intelligence.error && <p role="alert">{intelligence.error.message}</p>}
      {data?.regions.map(region => <button className="engineering-evidence" key={region.id} onClick={() => {setSelected(region);setRowOffset(0);setColumnOffset(0)}}>
        <strong>{region.semantic_label || region.kind}</strong> · {region.sheet_id} · 第 {region.bounds.start_row}–{region.bounds.end_row} 行
        <small> {region.headers.length ? '含表头' : '无指定表头'} · {region.merges.length} 个合并区域</small>
      </button>)}
      {data && !data.regions.length && <p className="empty">分析后展示识别到的数据区域。</p>}
      <h3>语义事实建议</h3>
      <div className="table-wrap"><table><thead><tr><th>来源</th><th>字段</th><th>提取值</th></tr></thead><tbody>{data?.facts.map(f => {
        const region = data.regions.find(r => r.id === f.region_id);
        return <tr key={f.id}><td>{region?.sheet_id || '来源记录'}</td><td>{f.predicate}</td><td>{String(f.value.text ?? f.value.decimal ?? f.value.boolean ?? f.value.reference ?? '未知')} {f.value.unit || ''}</td></tr>;
      })}</tbody></table></div>
      <div className="pagination"><span>共 {data?.total_facts || 0} 项来源事实</span><Button disabled={offset === 0} onClick={() => setOffset(v => Math.max(0, v - 100))}>上一页</Button><Button disabled={!data || offset + 100 >= data.total_facts} onClick={() => setOffset(v => v + 100)}>下一页</Button></div>
    </section>
    <aside className="panel padded">
      <h3>{selected ? selected.sheet_id + ' · 来源单元格' : '查看原始依据'}</h3>
      {!selected && <p className="muted">选择左侧区域查看原始内容。公式缓存与独立重算结果在“计算与规则”中对照。</p>}
      {selected && bounds && <><p>第 {bounds.start_row}–{bounds.end_row} 行，第 {bounds.start_column}–{bounds.end_column} 列</p><div className="bom-review-actions">
        <Button disabled={rowOffset === 0} onClick={() => setRowOffset(v => Math.max(0, v - 32))}>前一段</Button><Button disabled={bounds.end_row >= selected.bounds.end_row} onClick={() => setRowOffset(v => v + 32)}>后一段</Button>
        <Button disabled={columnOffset === 0} onClick={() => setColumnOffset(v => Math.max(0, v - 16))}>左移</Button><Button disabled={bounds.end_column >= selected.bounds.end_column} onClick={() => setColumnOffset(v => v + 16)}>右移</Button>
      </div></>}
      {cells.isFetching && <p>正在读取单元格…</p>}
      {cells.error && <p role="alert">{cells.error.message}</p>}
      {cells.data?.map(cell => <div className="engineering-source-cell" key={cell.source_cell}><strong>{cell.source_cell}</strong><p>{cell.original_value || '空值'}</p>{cell.formula && <code>{cell.formula}</code>}</div>)}
    </aside>
  </div>;
}
