import { useEffect, useRef, useState, type ReactNode, type ComponentProps } from 'react';
import { useSearchParams } from 'react-router-dom';
import * as Dialog from '@radix-ui/react-dialog';
import { Activity, ArrowDownToLine, ArrowRight, Box, Cable, Check, CheckCircle2, ChevronRight, CircleHelp, Database, FileSpreadsheet, FolderOpen, GitBranch, Layers3, LayoutDashboard, ListChecks, Loader2, MapPin, Network, Pencil, Plus, RefreshCw, Search, Server, ShieldCheck, SlidersHorizontal, Sparkles, Trash2, Upload, X, Zap } from 'lucide-react';
import { Button as SharedButton } from '../../src/components/shadcn/button';
import { downloadStudioFile, kickoffStudioApi, modelCSV, modelRows, parseModelCSV, validateStudioModel, type StudioAction, type StudioModel, type StudioSnapshot } from './api';
import implementationPlan from '../../../docs/intelligent-deployment/KICKOFF-STUDIO-IMPLEMENTATION.txt?raw';
import './kickoff-studio.css';

function Button({ className = '', variant = 'default', ...props }: ComponentProps<typeof SharedButton>) {
  return <SharedButton {...props} variant={variant} className={`ks-button ks-button-${variant} ${className}`} />;
}

type View = 'overview' | 'models' | 'steps' | 'review' | 'benchmark';
const nav = [
  { id: 'overview', label: '启动总览', icon: LayoutDashboard },
  { id: 'models', label: '资料与建模', icon: FolderOpen },
  { id: 'steps', label: '五步工程', icon: GitBranch },
  { id: 'review', label: '审核与交接', icon: ShieldCheck },
  { id: 'benchmark', label: '基准验收', icon: Activity },
] as const;
const stepIcons = [Server, MapPin, Cable, Network, Zap];
const stateLabels: Record<string, string> = { DETECTED: '待分析', NEEDS_INPUT: '待补充', READY: '可审核', PUBLISHED: '已模拟发布', STALE: '待重新核对', adopted: '已核对', draft: '待核对' };
const fieldLabels = ['本端 SN', '本端 MAC', '本端端口', '对端 SN', '对端机架位', '对端 MAC', '对端端口'];

function Status({ value }: { value: string }) {
  return <span className={`ks-status ks-status-${value.toLowerCase()}`}><i />{stateLabels[value] || value}</span>;
}

function StudioDialog({ title, description, children, close, wide = false }: { title: string; description: string; children: ReactNode; close: () => void; wide?: boolean }) {
  return <Dialog.Root open onOpenChange={open => { if (!open) close(); }}><Dialog.Portal>
    <Dialog.Overlay className="ks-dialog-overlay" />
    <Dialog.Content className={`ks-dialog ${wide ? 'ks-dialog-wide' : ''}`}>
      <div className="ks-dialog-heading"><div><Dialog.Title>{title}</Dialog.Title><Dialog.Description>{description}</Dialog.Description></div><Dialog.Close className="ks-icon-button" aria-label="关闭"><X size={19} /></Dialog.Close></div>
      {children}
    </Dialog.Content>
  </Dialog.Portal></Dialog.Root>;
}

function ModelEditor({ model, snapshot, close, save }: { model: StudioModel; snapshot: StudioSnapshot; close: () => void; save: (rows: string[][], adopt: boolean, reason: string) => void }) {
  const [rows, setRows] = useState(() => structuredClone(model.rows));
  const [reason, setReason] = useState(model.source);
  const [error, setError] = useState('');
  const [pasted, setPasted] = useState('');
  const [showPaste, setShowPaste] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);
  const candidate = { ...model, rows };
  const trial = { ...snapshot, models: snapshot.models.map(m => m.id === model.id ? candidate : m) };
  const issues = validateStudioModel(candidate, trial);
  const applyCSV = (text: string, source: string) => {
    try { setRows(parseModelCSV(text, model)); setReason(source); setError(''); setShowPaste(false); }
    catch (e) { setError((e as Error).message); }
  };
  return <StudioDialog title={model.name} description={`${model.purpose} · 模型 v${model.version}`} close={close} wide>
    <div className="ks-dialog-body">
      <div className="ks-inline-note"><FileSpreadsheet size={17} /><span>可实际导入 CSV、粘贴或逐格填写。本原型表用于交互评审；正式 XLSX 契约与完整领域字段见实施方案。</span></div>
      <div className="ks-toolbar">
        <Button variant="outline" onClick={() => downloadStudioFile(`${model.id}.csv`, modelCSV(candidate), 'text/csv;charset=utf-8')}><ArrowDownToLine />下载含示例模板</Button>
        <Button variant="outline" onClick={() => fileInput.current?.click()}><Upload />导入 CSV</Button>
        <Button variant="ghost" onClick={() => setShowPaste(!showPaste)}>粘贴表格</Button>
        <input ref={fileInput} type="file" accept=".csv" hidden aria-label={`导入${model.name} CSV`} onChange={async event => { const file = event.target.files?.[0]; if (file) applyCSV(await file.text(), `用户导入 · ${file.name}`); event.target.value = ''; }} />
        <span className="ks-muted">{rows.length} 行 · {model.columns.length} 列</span>
      </div>
      {showPaste && <div className="ks-paste"><textarea aria-label="CSV 内容" placeholder="粘贴包含英文表头的 CSV 内容" value={pasted} onChange={e => setPasted(e.target.value)} /><Button onClick={() => applyCSV(pasted, '用户粘贴建模表')}>解析到表格</Button></div>}
      {error && <p className="ks-error" role="alert">{error}</p>}
      <div className="ks-table-scroll"><table className="ks-table ks-edit-table"><thead><tr><th>#</th>{model.columns.map(column => <th key={column.key}>{column.label}<small>{column.key}</small></th>)}<th /></tr></thead><tbody>
        {rows.map((row, rowIndex) => <tr key={rowIndex}><td>{rowIndex + 1}</td>{model.columns.map((column, columnIndex) => <td key={column.key}><input aria-label={`${column.label} 第${rowIndex + 1}行`} value={row[columnIndex] || ''} onChange={e => setRows(all => all.map((r, i) => i === rowIndex ? model.columns.map((_, j) => j === columnIndex ? e.target.value : r[j] || '') : r))} /></td>)}<td><button className="ks-icon-button" aria-label={`删除第${rowIndex + 1}行`} onClick={() => setRows(all => all.filter((_, i) => i !== rowIndex))}><Trash2 size={15} /></button></td></tr>)}
      </tbody></table></div>
      <Button variant="ghost" className="ks-add-row" onClick={() => setRows([...rows, model.columns.map(() => '')])}><Plus />添加一行</Button>
      <label className="ks-field">输入依据 / 版本说明<input value={reason} onChange={e => setReason(e.target.value)} /></label>
      {issues.length > 0 ? <div className="ks-validation" role="alert"><b>{issues.length} 项需要处理</b>{issues.slice(0, 5).map((issue, i) => <p key={i}>{issue}</p>)}{issues.length > 5 && <p>以及其他 {issues.length - 5} 项。</p>}</div> : <div className="ks-inline-note ks-success"><CheckCircle2 size={17} />原型范围的字段、引用与约束校验通过。采用前请确认数据依据。</div>}
      <p className="ks-muted">更改将使相关计算及审核失效。既有发布记录保留，重新审核后才能模拟发布新版。</p>
    </div>
    <div className="ks-dialog-footer"><Button variant="outline" onClick={() => save(rows, false, reason)}>保存草稿</Button><Button disabled={issues.length > 0 || !reason.trim()} onClick={() => save(rows, true, reason)}><Check />校验并采用</Button></div>
  </StudioDialog>;
}

export default function KickoffStudio() {
  const [params, setParams] = useSearchParams();
  const view = (nav.some(item => item.id === params.get('view')) ? params.get('view') : 'overview') as View;
  const selectedStep = Math.max(1, Math.min(5, Number(params.get('step')) || 1));
  const [data, setData] = useState<StudioSnapshot>();
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState(false);
  const [editing, setEditing] = useState<string>();
  const [evidence, setEvidence] = useState<StudioSnapshot['assets'][number]>();
  const [modelFilter, setModelFilter] = useState('all');
  const [query, setQuery] = useState('');
  const [confirmModels, setConfirmModels] = useState(false);
  const [confirmIdentity, setConfirmIdentity] = useState(false);
  const [bMac, setBMac] = useState('');
  const [checked, setChecked] = useState(false);
  const [reviewReason, setReviewReason] = useState('已核对合成身份、建模输入、独立算例及规划状态');
  const [providerRef, setProviderRef] = useState('');
  const upload = useRef<HTMLInputElement>(null);
  const load = () => kickoffStudioApi.get().then(value => { setData(value); setProviderRef(value.benchmark.provider_ref); setError(''); }).catch(e => setError(e.message));
  useEffect(() => { void load(); }, []);
  const navigate = (next: View, step = selectedStep) => { setParams({ view: next, ...(next === 'steps' ? { step: String(step) } : {}) }); setError(''); };
  const update = (operation: () => StudioSnapshot, message: string) => {
    setBusy(true); setError('');
    try { setData(operation()); setNotice(message); setChecked(false); return true; }
    catch (e) { setError((e as Error).message); return false; }
    finally { setBusy(false); }
  };
  const act = (action: StudioAction['action'], message: string, extra: Partial<StudioAction> = {}) => update(() => kickoffStudioApi.act({ expected_revision: data!.revision, action, reason: reviewReason, ...extra }), message);
  const downloadPlan = () => downloadStudioFile('Keel-开局工程实施方案.txt', implementationPlan);
  if (!data) return <div className="ks-loading">{error ? <><p role="alert">{error}</p><Button onClick={() => void load()}>重试</Button></> : <><Loader2 className="ks-spin" /><p>正在读取工程原型与合成资料…</p></>}</div>;
  const adopted = data.models.filter(model => model.status === 'adopted').length;
  const ready = data.steps.filter(step => ['READY', 'PUBLISHED'].includes(step.state)).length;
  const canCalculate = data.identity_confirmed && data.models.filter(model => model.step_ids.includes(3)).every(model => model.status === 'adopted');
  const next = !data.analyzed ? { label: '演示资料分析', hint: '从合成资料展示分流和候选结果', run: () => act('analyze_example', '已展示合成候选。没有调用真实模型。') } : !data.identity_confirmed ? { label: '确认设备关联', hint: '核对本端与对端信息和已有资产身份', run: () => { setBMac(data.termination.b_mac); setConfirmIdentity(true); } } : adopted < data.models.length ? { label: '核对工程建模', hint: '一次补充机房、机柜、路径与规则', run: () => navigate('models') } : !data.calculation ? { label: '计算布线参考链路', hint: '用已核对规则生成计算长度和工程量', run: () => { navigate('steps', 3); act('calculate', '确定性参考计算完成。'); } } : { label: '进入审核与交接', hint: '核对产物版本并模拟分域发布', run: () => navigate('review') };
  const step = data.steps[selectedStep - 1];
  const issueAction = (issue: StudioSnapshot['issues'][number]) => {
    if (issue.model_id) { setEditing(issue.model_id); return; }
    if (issue.id === 'analysis') act('analyze_example', '已展示合成候选。没有调用真实模型。');
    if (issue.id === 'identity') { setBMac(data.termination.b_mac); setConfirmIdentity(true); }
    if (issue.id === 'calculation') navigate('steps', 3);
  };
  const modelCard = (model: StudioModel) => <article className="ks-model-card" key={model.id}>
    <div className="ks-card-top"><span className={`ks-model-icon ${model.group}`} >{model.group === 'facility' ? <MapPin size={19} /> : model.group === 'cabling' ? <Cable size={19} /> : <SlidersHorizontal size={19} />}</span><Status value={model.status} /></div>
    <h3>{model.name}</h3><p>{model.purpose}</p>
    <div className="ks-model-meta"><span>{model.rows.length} 行 · v{model.version}</span><span>步骤 {model.step_ids.join(' / ')}</span></div>
    <div className="ks-model-actions"><button onClick={() => downloadStudioFile(`${model.id}.csv`, modelCSV(model), 'text/csv;charset=utf-8')}><ArrowDownToLine size={14} />模板</button><button onClick={() => setEditing(model.id)}>导入 / 编辑<ChevronRight size={15} /></button></div>
  </article>;

  return <div className="ks-app">
    <aside className="ks-sidebar">
      <a className="ks-brand" href="/kickoff-studio"><span>k</span>keel<small>ENGINEERING</small></a>
      <div className="ks-workspace-label">项目工作台</div>
      <div className="ks-project"><div className="ks-project-symbol"><Box size={20} /></div><div><strong>智能开局工程</strong><small>B300 · 合成规划沙箱</small></div></div>
      <nav aria-label="开局工程主导航"><button onClick={() => window.location.assign("/global-topology")}><Network size={18} />数据中心拓扑</button><button onClick={() => window.location.assign("/jobs-preview/index.html#flows")}><Activity size={18} />作业平台样式</button>{nav.map(item => <button key={item.id} className={view === item.id ? 'active' : ''} onClick={() => navigate(item.id)}><item.icon size={18} />{item.label}{item.id === 'models' && <span>{data.models.length - adopted}</span>}</button>)}</nav>
      <div className="ks-sidebar-guide"><Layers3 size={20} /><b>资料转为工程依据</b><p>AI 提取候选，规则计算结果，用户确认后发布。</p><button onClick={downloadPlan}>查看实施方案<ArrowDownToLine size={14} /></button></div>
      <div className="ks-sidebar-bottom"><span className="ks-avatar">K</span><div>交互评审环境<small>独立原型 · 5081</small></div></div>
    </aside>
    <div className="ks-main">
      <header className="ks-topbar"><div>工程交付 <ChevronRight size={14} /><b>智能开局</b><span className="ks-preview">交互原型</span></div><button onClick={() => navigate('benchmark')} className="ks-model-badge"><Sparkles size={15} />gptsol6.1<span>基准未运行</span></button></header>
      <main className="ks-content">
        <div className="ks-page-heading"><div><div className="ks-eyebrow">KEEL INTELLIGENT DEPLOYMENT</div><h1>{nav.find(item => item.id === view)?.label}<span>工作区 v{data.revision}</span></h1><p>{view === 'models' ? '把空间、规则与工程依据一次补齐，让每一步有明确输入。' : view === 'benchmark' ? '冻结模型与数据，用独立结果判断是否真正完成开局。' : '从资料理解到工程交接，让每一份输入都有去向。'}</p></div><Button variant="outline" onClick={downloadPlan}><ArrowDownToLine />实施方案</Button></div>
        <div className="ks-simulation"><CircleHelp size={16} /><span>当前使用合成资料演示交互，未调用 AI 或写入真实 CMDB。建模编辑与参考计算可实际操作。</span><span className="ks-sim-label">SIMULATION ONLY</span></div>
        {error && <div className="ks-error ks-banner" role="alert">{error}<button aria-label="关闭错误" onClick={() => setError('')}><X size={16} /></button></div>}
        {notice && <div className="ks-notice" role="status"><CheckCircle2 size={16} />{notice}<button aria-label="关闭提示" onClick={() => setNotice('')}><X size={15} /></button></div>}

        <div className="ks-steps" aria-label="五步工程状态">{data.steps.map((item, i) => { const Icon = stepIcons[i]; return <button key={item.id} className={`ks-step ${view === 'steps' && selectedStep === item.id ? 'selected' : ''}`} onClick={() => navigate('steps', item.id)}><div className="ks-step-top"><span className="ks-step-number">0{item.id}</span><Icon size={18} /></div><strong>{item.name}</strong><Status value={item.state} /></button>; })}</div>

        {view === 'overview' && <>
          <div className="ks-overview-grid">
            <section className="ks-next"><div className="ks-next-mark"><Sparkles size={22} /></div><div><span className="ks-eyebrow">当前建议</span><h2>{data.published_revision ? '工程产物已模拟发布' : next.label}</h2><p>{data.published_revision ? '可以查看发布收据，或修改建模输入体验版本失效与重新审核。' : next.hint}</p><Button onClick={next.run} disabled={busy}>{next.label}<ArrowRight /></Button></div><div className="ks-next-orbit"><GitBranch size={70} /></div></section>
            <section className="ks-panel ks-progress-panel"><div className="ks-section-title"><h2>就绪情况</h2><span>由输入与校验决定</span></div><div className="ks-progress-number"><b>{ready}<small> / 5</small></b><span>工程模块可审核</span></div><div className="ks-progress-track"><span style={{ width: `${ready * 20}%` }} /></div><div className="ks-progress-footer"><span>{adopted} / {data.models.length} 建模已核对</span><span>{data.issues.length} 项待处理</span></div></section>
          </div>
          <div className="ks-two-columns">
            <section className="ks-panel"><div className="ks-section-title"><h2>集中补充清单 <span className="ks-count">{data.issues.length}</span></h2><button onClick={() => navigate('models')}>查看全部<ChevronRight size={14} /></button></div><p className="ks-section-description">按依赖合并缺口，一处修改，相关步骤共同复用。</p>{data.issues.length ? data.issues.slice(0, 5).map(issue => <button className="ks-issue" key={issue.id} onClick={() => issueAction(issue)}><span className="ks-issue-dot" /><div><b>{issue.title}</b><p>{issue.detail}</p></div><ChevronRight size={16} /></button>) : <div className="ks-empty"><CheckCircle2 /><b>当前原型输入完整</b><p>进入审核，确认规划状态和版本后模拟发布。</p></div>}</section>
            <section className="ks-panel"><div className="ks-section-title"><h2>一次提供，贯穿五步</h2><Layers3 size={18} /></div><div className="ks-input-story"><div><span>01</span><article><b>开局资料</b><p>设备、端接、机房图与网络规划，批量登记到同一任务。</p></article></div><div><span>02</span><article><b>工程建模包</b><p>房间 / 机柜 / U 位 / 路径 / 规则，模板导入或页面补充。</p></article></div><div><span>03</span><article><b>集中审核与交接</b><p>确认差异和计算依据，分域发布，交付主链从待执行开始。</p></article></div></div><button className="ks-text-link" onClick={() => navigate('models')}>进入资料与建模<ArrowRight size={15} /></button></section>
          </div>
          <section className="ks-panel ks-log"><div className="ks-section-title"><h2>最近变更</h2><span>保存在当前浏览器的原型草稿</span></div>{data.events.slice(0, 3).map((event, i) => <p key={i}><i />{event}</p>)}</section>
        </>}

        {view === 'models' && <>
          <section className="ks-panel ks-source-panel"><div className="ks-section-title"><h2>资料入口</h2><Button variant="outline" onClick={() => upload.current?.click()}><Upload />选择资料</Button></div><div className="ks-source-list">{data.sources.map(source => <div key={source.id}><FileSpreadsheet size={21} /><article><b>{source.name}</b><small>{source.status}</small></article><span>{source.step_ids.length ? `用于步骤 ${source.step_ids.join(' / ')}` : '待真实 Agent 分流'}</span></div>)}</div><input ref={upload} type="file" multiple hidden aria-label="选择开局资料" accept=".xlsx,.xls,.csv,.pdf,.png,.jpg,.jpeg,.docx,.txt" onChange={e => { const filenames = [...(e.target.files || [])].map(file => file.name); if (filenames.length) update(() => kickoffStudioApi.registerSources({ expected_revision: data.revision, filenames }), '已登记文件名。原型不读取这些文件的内容。'); e.target.value = ''; }} /><div className="ks-source-footer"><span>此处仅登记文件名；下方模型编辑器支持实际 CSV 导入。</span><Button variant="ghost" onClick={() => act('analyze_example', '已展示合成候选。真实文件尚未分析。')}><Sparkles />演示资料分析</Button></div></section>
          <div className="ks-model-heading"><div><h2>工程建模输入</h2><p>支持分表导入，也可一次核对采用已填好的建模包。</p></div><Button onClick={() => setConfirmModels(true)} disabled={adopted === data.models.length}><ListChecks />批量核对建模</Button></div>
          <div className="ks-filter-bar"><div className="ks-segmented" aria-label="模型分类">{[['all', '全部'], ['facility', '机房与机柜'], ['cabling', '路径与布线规则'], ['network_power', '网络与电力']].map(([id, label]) => <button className={modelFilter === id ? 'active' : ''} key={id} onClick={() => setModelFilter(id)}>{label}</button>)}</div><label className="ks-search"><Search size={15} /><input aria-label="搜索建模" value={query} onChange={e => setQuery(e.target.value)} placeholder="搜索建模输入" /></label></div>
          <div className="ks-model-grid">{data.models.filter(model => (modelFilter === 'all' || model.group === modelFilter) && (model.name + model.purpose).includes(query)).map(modelCard)}</div>
          {!data.models.some(model => (modelFilter === 'all' || model.group === modelFilter) && (model.name + model.purpose).includes(query)) && <div className="ks-empty">没有匹配的建模输入。<Button variant="ghost" onClick={() => { setQuery(''); setModelFilter('all'); }}>清除筛选</Button></div>}
          <div className="ks-inline-note"><GitBranch size={17} />依赖关系：身份 → 落位；身份 + 端口 → 物理连接；落位 + 路径 + 规则 → 布线；物理连接 → 网络；落位 + 供电 → 电力。网络与电力可并行。</div>
        </>}

        {view === 'steps' && <>
          <section className="ks-panel"><div className="ks-section-title"><h2>0{step.id} / {step.name}</h2><Status value={step.state} /></div><p className="ks-section-description">{step.summary}</p><div className="ks-io"><div><span>固定输入要求</span>{step.input_labels.map(label => <p key={label}><Check size={14} />{label}</p>)}</div><ArrowRight className="ks-io-arrow" /><div><span>本步工程产物</span>{step.outputs.map(label => <p key={label}><Database size={14} />{label}</p>)}</div><div className="ks-io-note"><b>独立满足依赖</b><p>资料不完整时保留草稿，只阻断受影响的产物。</p></div></div></section>
          {selectedStep === 1 && <section className="ks-panel"><div className="ks-section-title"><h2>设备与端口关联</h2><Button disabled={!data.analyzed || data.identity_confirmed} onClick={() => { setBMac(data.termination.b_mac); setConfirmIdentity(true); }}><Check />{data.identity_confirmed ? '关联已确认' : '确认关联'}</Button></div><p className="ks-section-description">下方一条端接为合成参考。对端 MAC 来自人工补充示例；不是 Agent 推测值。</p><div className="ks-table-scroll"><table className="ks-table"><thead><tr>{fieldLabels.map(label => <th key={label}>{label}</th>)}</tr></thead><tbody><tr>{Object.values(data.termination).map((value, i) => <td key={i}>{value || '待补充'}</td>)}</tr></tbody></table></div><div className="ks-section-title ks-spaced"><h3>复用现有身份 · {data.assets.length} 个合成设备</h3><span>规划身份不会变成已发现实物</span></div><div className="ks-table-scroll"><table className="ks-table"><thead><tr><th>设备</th><th>SN</th><th>型号</th><th>已有演示位置</th><th>来源</th></tr></thead><tbody>{data.assets.map(asset => <tr key={asset.id}><td><b>{asset.label}</b><small>{asset.id}</small></td><td>{asset.serial}</td><td>{asset.model}</td><td>{asset.rack} / {asset.position}</td><td><button className="ks-text-link" onClick={() => setEvidence(asset)}>查看依据<ChevronRight size={13} /></button></td></tr>)}</tbody></table></div></section>}
          {selectedStep !== 1 && <div className="ks-model-grid ks-step-models">{data.models.filter(model => model.step_ids.includes(selectedStep)).map(modelCard)}</div>}
          {selectedStep === 2 && <section className="ks-panel"><div className="ks-section-title"><h2>规划落位</h2><span>实际接入时复用现有 CMDB 机柜图</span></div><div className="ks-table-scroll"><table className="ks-table"><thead><tr><th>设备身份</th><th>目标机柜</th><th>U 位</th><th>安装面</th><th>数据状态</th></tr></thead><tbody>{modelRows(data.models.find(model => model.id === 'placements')!).map(row => <tr key={row.device_id}><td>{row.device_id}</td><td>{row.rack_id}</td><td>U{row.u_start} · {row.height_u}U</td><td>{row.face}</td><td>Planning</td></tr>)}</tbody></table></div></section>}
          {selectedStep === 3 && <section className="ks-panel ks-calculation"><div className="ks-section-title"><h2>确定性布线计算</h2><Button disabled={!canCalculate} onClick={() => act('calculate', '参考链路计算完成；未生成实装长度。')}><RefreshCw />{data.calculation ? '重新计算' : '执行参考计算'}</Button></div><p className="ks-section-description">单条参考链路：srv-0001 / NIC1/P1 → sw-leaf-0001 / swp1。按当前规划位置在允许通道上求最短路径。</p>{data.calculation ? <><div className="ks-formula"><span>路径 <b>{data.calculation.route_m} m</b></span><em>+</em><span>两端引入 <b>{data.calculation.leads_m} m</b></span><em>×</em><span>余量 <b>{data.calculation.slack_percent}%</b></span><em>→</em><span className="ks-formula-result">计算长度 <b>{data.calculation.required_m} m</b></span><em>→</em><span>标准长度 <b>{data.calculation.stock_m} m</b></span></div><p className="ks-muted">计算式：(路径长度 + 两端引入) × (1 + 余量率)。项目规则 v{data.calculation.rule_version}；此算法及参数不作为全平台固定默认值。</p><div className="ks-calc-results"><div><small>参考线缆需求</small><b>{data.calculation.cable_count} <span>条</span></b></div><div><small>物理端口光模块</small><b>{data.calculation.optic_count} <span>只</span></b></div><div><small>结果性质</small><b className="ks-state-text">Calculated</b></div><div><small>实装长度</small><b className="ks-state-text">未提供</b></div></div></> : <div className="ks-empty"><Cable /><b>等待输入核对</b><p>先确认身份，采用机房 / 落位 / 通道 / 规格 / 规则；不依赖逻辑网络或电力完成。</p></div>}</section>}
          {selectedStep === 4 && <div className="ks-inline-note"><Network size={18} />原型展示显式网络输入的导入与审核。正式实现必须验证已发布物理连接版本、接口与邻居关系；不从线缆推断 IP、ASN 或协议配置。</div>}
          {selectedStep === 5 && <section className="ks-panel"><div className="ks-section-title"><h2>机柜规划容量预览</h2><span>简化参考，不代表真实回路 / PSU 验收</span></div><div className="ks-table-scroll"><table className="ks-table"><thead><tr><th>机柜</th><th>供电容量</th><th>规划负载</th><th>规划余量</th><th>冷却方式</th></tr></thead><tbody>{modelRows(data.models.find(model => model.id === 'power')!).map(row => <tr key={row.rack_id}><td>{row.rack_id}</td><td>{row.capacity_kw} kW</td><td>{row.planned_load_kw} kW</td><td>{Number(row.capacity_kw) - Number(row.planned_load_kw)} kW</td><td>{row.cooling_type}</td></tr>)}</tbody></table></div></section>}
        </>}

        {view === 'review' && <>
          <section className="ks-panel"><div className="ks-section-title"><h2>分域审核与发布</h2><span>草稿 v{data.revision} · {data.issues.length} 项待处理</span></div><div className="ks-release-grid">{[
            ['CMDB 物理规划', '设备身份 / 规划落位 / 端口 / Planned Connection', '规划数据保持 Planning'],
            ['工程计划', 'CablePlan / PowerPlan / Material / Workload', '估算与计算不等于实际安装或采购'],
            ['网络计划', '已审核的 NetworkPlan Revision', '发布后才具备 Golden 消费资格'],
          ].map(([title, description, foot]) => <div key={title}><Database size={22} /><h3>{title}</h3><p>{description}</p><small>{foot}</small><Status value={data.published_revision ? 'PUBLISHED' : data.issues.length ? 'NEEDS_INPUT' : 'READY'} /></div>)}</div>
          {data.issues.length > 0 ? <div className="ks-review-block"><ListChecks size={20} /><div><b>完成补充清单后才能审核</b><p>当前仍有 {data.issues.length} 项输入或计算待处理。</p></div><Button variant="outline" onClick={() => navigate('overview')}>查看补充清单</Button></div> : data.published_revision ? <div className="ks-published" role="status"><CheckCircle2 /><div><b>模拟发布收据已生成 · v{data.published_revision}</b><p>所有写入仅在本浏览器原型草稿中。真实 CMDB / Outbox / Golden / 主链交接尚未执行。</p></div><Button variant="outline" onClick={() => downloadStudioFile('kickoff-prototype-receipt.json', JSON.stringify({ simulation_only: true, revision: data.published_revision, outputs: data.steps.map(s => ({ step: s.name, outputs: s.outputs })), downstream: 'PENDING_EXECUTION', real_api_called: false }, null, 2), 'application/json')}>导出模拟收据</Button></div> : <div className="ks-review-form"><label className="ks-checkbox"><input type="checkbox" checked={checked} onChange={e => setChecked(e.target.checked)} />我已核对来源、模型版本与计算依据，理解这些是规划数据。</label><label className="ks-field">审核说明<input value={reviewReason} onChange={e => setReviewReason(e.target.value)} /></label><div className="ks-toolbar"><Button variant="outline" disabled={!checked || !reviewReason.trim() || data.review_revision === data.revision} onClick={() => act('review', '当前修订已审核，下一步可模拟发布。')}><ShieldCheck />{data.review_revision === data.revision ? '当前修订已审核' : '审核当前修订'}</Button><Button disabled={data.review_revision !== data.revision} onClick={() => act('publish', '已生成模拟收据；真实服务没有写入。')}><ArrowRight />模拟分域发布</Button></div></div>}
          {data.last_published_revision !== null && !data.published_revision && <div className="ks-validation"><b>上次发布 v{data.last_published_revision} 保留</b><p>输入已变更，相关计算与审核不能沿用。重新核对后生成新版发布记录。</p></div>}</section>
          <section className="ks-panel"><div className="ks-section-title"><h2>交付主链交接</h2><span>发布规划不会把执行阶段标记为通过</span></div><div className="ks-delivery-chain">{['开局配置', '网络就绪', '装机', '验机', 'NCCL 测试', '优雅退场'].map((name, i) => <div key={name}><span>{i + 1}</span><b>{name}</b><small>{i === 0 && data.published_revision ? '模拟规划已交接' : '待执行'}</small></div>)}</div><p className="ks-section-description">正式实施必须补齐真实交付 API 消费发布版本的闭环，不能用 Mock 状态变化作为验收。</p></section>
        </>}

        {view === 'benchmark' && <>
          <div className="ks-two-columns"><section className="ks-panel"><div className="ks-section-title"><h2>固定基准配置</h2><span className="ks-status">NOT RUN</span></div><label className="ks-field">基准模型<input value={data.benchmark.model_id} readOnly /></label><label className="ks-field">Provider 引用<input value={providerRef} onChange={e => setProviderRef(e.target.value)} placeholder="填写配置引用，不填写 API Key" /></label><div className="ks-benchmark-info"><span>Prompt <b>{data.benchmark.prompt_version}</b></span><span>最终验收 <b>{data.benchmark.run_count} 次独立运行</b></span></div><Button variant="outline" onClick={() => update(() => kickoffStudioApi.configureBenchmark({ expected_revision: data.revision, provider_ref: providerRef }), '已保存原型测试配置；未连接 Provider 或运行测试。')}>保存测试配置</Button><p className="ks-muted">模型标识严格使用 gptsol6.1。真实能力探测、凭据配置与测试在正式实现后进行，不自动换模。</p></section><section className="ks-panel"><div className="ks-section-title"><h2>分步 Agent 执行设计</h2><Sparkles size={18} /></div><div className="ks-agent-flow"><span>资料检查 / 脱敏 / 分区</span><ArrowRight size={15} /><span>资料分流 Agent</span><ArrowRight size={15} /><span>五个有界提取任务</span><ArrowRight size={15} /><span>Schema + 确定性关联</span><ArrowRight size={15} /><span>一次汇总补充清单</span></div><p className="ks-section-description">复用现有工具与持久化运行时。中间结果存对象存储，结构化候选和状态入库；每个任务最多一次语义纠正，未解决项交给用户补充。</p></section></div>
          <section className="ks-panel"><div className="ks-section-title"><h2>验收范围</h2><Button variant="outline" onClick={() => downloadStudioFile('gptsol6.1-evaluation-manifest.json', JSON.stringify({ ...data.benchmark, simulation_only: true, dataset: 'synthetic-prototype-references', real_customer_ground_truth: 'REQUIRED', provider_probe: 'NOT_RUN', model_calls: 0, schema_revision: 1, rule_versions: data.models.filter(model => model.id === 'cabling_rules').map(model => model.version) }, null, 2), 'application/json')}><ArrowDownToLine />导出验收配置</Button></div>{data.benchmark.checks.map((check, i) => <div className="ks-benchmark-row" key={check}><span>{String(i + 1).padStart(2, '0')}</span><b>{check}</b><em>未运行</em></div>)}<div className="ks-inline-note"><CircleHelp size={17} />不能承诺任意资料 100% 自动识别。验收承诺应是：信息可追溯、缺口不丢失、必要人工补充后流程能够完成；另需一份结构不同的真实资料验证泛化。</div></section>
        </>}
        <footer className="ks-footer"><span>Keel / Intelligent Deployment Engineering</span><span>工程依据可追溯 · 规划与观测分开</span></footer>
      </main>
    </div>
    {editing && <ModelEditor model={data.models.find(model => model.id === editing)!} snapshot={data} close={() => setEditing(undefined)} save={(rows, adopt, reason) => { if (update(() => kickoffStudioApi.saveModel(editing, { expected_revision: data.revision, rows, decision: adopt ? 'adopt' : 'save', reason }), adopt ? '模型已采用，相关下游结果需要重新核对。' : '草稿已保存；缺失项保留在补充清单。')) setEditing(undefined); }} />}
    {confirmModels && <StudioDialog title="批量核对工程建模" description="一次采用当前建模输入；每张表仍保留独立版本和依赖。" close={() => setConfirmModels(false)}><div className="ks-dialog-body"><p>当前九份表均为合成参考，可以先关闭此窗口逐表修改或导入。请核对机房、机柜、落位、路径、线缆规格、计算规则及网络 / 电力依据。</p><div className="ks-batch-models">{data.models.map(model => <button key={model.id} onClick={() => { setConfirmModels(false); setEditing(model.id); }}><span>{model.name}</span><small>{validateStudioModel(model, data).length ? '存在错误' : `${model.rows.length} 行 · 可采用`}</small><Pencil size={14} /></button>)}</div><p className="ks-muted">正式发布仍需审核；此操作不会写入 CMDB。规则参考结果来自明确输入，不能用 Agent 自评替代。</p></div><div className="ks-dialog-footer"><Button variant="outline" onClick={() => setConfirmModels(false)}>继续编辑</Button><Button onClick={() => { if (act('adopt_models', '九份建模表已核对采用。')) setConfirmModels(false); }}>已核对，采用建模包</Button></div></StudioDialog>}
    {confirmIdentity && <StudioDialog title="确认端接与设备身份" description="复用既有合成设备 ID；对端 MAC 从人工补充依据获得。" close={() => setConfirmIdentity(false)}><div className="ks-dialog-body"><p>本端 {data.termination.a_sn} / {data.termination.a_port}<br />对端 {data.termination.b_sn} / {data.termination.b_port}</p><label className="ks-field">对端 MAC<input aria-label="确认对端 MAC" value={bMac} onChange={e => setBMac(e.target.value)} /></label><div className="ks-inline-note"><CircleHelp size={17} />预填值来自合成补充材料，不是模型猜测。真实缺失值必须导入、手动填写或依据唯一可信关联取得。</div><p>确认后，十个合成候选引用统一资产身份，一条参考端接进入规划。此原型不代表全量端口、关系或真实资料验收。</p>{error && <p className="ks-error" role="alert">{error}</p>}</div><div className="ks-dialog-footer"><Button onClick={() => { if (act('confirm_identity', '端接与合成身份关联已确认。', { b_mac: bMac })) setConfirmIdentity(false); }}>确认并采用关联</Button></div></StudioDialog>}
    {evidence && <StudioDialog title="来源与证据" description="合成 fixture 的明确引用，不冒充真实 Excel 单元格证据。" close={() => setEvidence(undefined)}><div className="ks-dialog-body"><dl className="ks-evidence"><dt>设备</dt><dd>{evidence.label}</dd><dt>统一 ID</dt><dd>{evidence.id}</dd><dt>证据位置</dt><dd>{evidence.source}</dd><dt>数据性质</dt><dd>合成演示引用 / Planning 候选</dd><dt>正式实现要求</dt><dd>原文件 hash、Sheet、区域、单元格、提取任务、变换规则和审核决策可连续追踪。</dd></dl></div></StudioDialog>}
  </div>;
}
