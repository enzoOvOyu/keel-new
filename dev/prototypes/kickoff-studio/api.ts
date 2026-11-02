import type { StudioSnapshot, StudioModel, StudioAction, StudioModelInput, StudioSourceInput, StudioBenchmarkInput } from './contracts';
export type { StudioSnapshot, StudioModel, StudioAction } from './contracts';
import fixtureUrl from '../../../fixtures/demo-full-64-b300/kickoff-studio.json?url';
import seedUrl from '../../../fixtures/demo-full-64-b300/demo-seed.json?url';

type ModelInput = StudioModelInput;
type Row = Record<string, string>;
type Fixture = Pick<StudioSnapshot, 'project_id' | 'project_name' | 'models' | 'sources' | 'benchmark'> & {
  asset_refs: string[];
  supplemental_b_mac: string;
  steps: Pick<StudioSnapshot['steps'][number], 'id' | 'name' | 'summary' | 'input_labels' | 'outputs'>[];
};
const storageKey = 'keel-kickoff-studio-v1';
let state: StudioSnapshot;
let fixture: Fixture;
let loading: Promise<StudioSnapshot> | undefined;

export function modelRows(model: StudioModel, rows = model.rows): Row[] {
  return rows.map(row => Object.fromEntries(model.columns.map((column, i) => [column.key, row[i]?.trim() || ''])));
}

function modelOf(snapshot: StudioSnapshot, id: string) {
  return snapshot.models.find(model => model.id === id)!;
}

export function validateStudioModel(model: StudioModel, snapshot: StudioSnapshot): string[] {
  const issues: string[] = [];
  const rows = modelRows(model);
  if (!rows.length) return ['至少需要一行建模输入。'];
  const refs = (id: string, key: string) => new Set(modelRows(modelOf(snapshot, id)).map(row => row[key]));
  const knownDevices = new Set(snapshot.assets.map(asset => asset.id));
  const unique = new Set<string>();
  for (const [i, row] of rows.entries()) {
    const fail = (message: string) => issues.push(`第 ${i + 1} 行：${message}`);
    if (model.rows[i].length !== model.columns.length) fail('列数与模板不一致');
    for (const column of model.columns) {
      const value = row[column.key];
      if (column.required && !value) fail(`${column.label}缺失`);
      if (column.kind !== 'text' && value) {
        const n = Number(value);
        if (!Number.isFinite(n) || (column.kind === 'positive' && n <= 0) || (column.kind === 'integer' && (!Number.isInteger(n) || n < 1))) fail(`${column.label}数值不合法`);
      }
    }
    const key = row[model.columns[0].key];
    if (unique.has(key)) fail('标识重复');
    unique.add(key);
    if (row.device_id && !knownDevices.has(row.device_id)) fail('设备 ID 未匹配当前项目身份');
    if (row.room_id && model.id !== 'rooms' && !refs('rooms', 'room_id').has(row.room_id)) fail('引用的机房不存在');
    if (row.rack_id && model.id !== 'racks' && !refs('racks', 'rack_id').has(row.rack_id)) fail('引用的机柜不存在');
    if (model.id === 'rooms' && row.unit !== 'm') fail('原型坐标统一使用 m；其他单位需先明确转换');
    if (model.id === 'racks' && (Number(row.orientation_deg) < 0 || Number(row.orientation_deg) >= 360)) fail('朝向需在 0–359°');
    if (model.id === 'placements') {
      const rack = modelRows(modelOf(snapshot, 'racks')).find(r => r.rack_id === row.rack_id);
      if (rack && Number(row.u_start) + Number(row.height_u) - 1 > Number(rack.height_u)) fail('设备超出机柜 U 位');
      if (!['front', 'rear'].includes(row.face)) fail('安装面只能为 front / rear');
      if (rows.slice(0, i).some(other => other.rack_id === row.rack_id && Number(row.u_start) < Number(other.u_start) + Number(other.height_u) && Number(other.u_start) < Number(row.u_start) + Number(row.height_u))) fail('U 位与其他设备重叠');
    }
    if (model.id === 'route_edges') {
      const nodes = refs('route_nodes', 'node_id');
      if (!nodes.has(row.from_node) || !nodes.has(row.to_node) || row.from_node === row.to_node) fail('路径端点无效');
      if (row.medium !== 'fiber') fail('当前参考算例的介质为 fiber');
    }
    if (model.id === 'cable_specs') {
      const lengths = row.standard_lengths_m.split(';').map(Number);
      if (lengths.some(n => !Number.isFinite(n) || n <= 0)) fail('标准线长用分号分隔，且必须为正数');
    }
    if (model.id === 'cabling_rules') {
      if (row.algorithm !== 'shortest_path') fail('当前已实现参考算法为 shortest_path');
      if (['lead_a_m', 'lead_b_m', 'slack_percent'].some(k => Number(row[k]) < 0)) fail('引入长度与余量不能为负');
      const calculated = (Number(row.reference_route_m) + Number(row.lead_a_m) + Number(row.lead_b_m)) * (1 + Number(row.slack_percent) / 100);
      const lengths = modelRows(modelOf(snapshot, 'cable_specs'))[0]?.standard_lengths_m.split(';').map(Number).sort((a, b) => a - b) || [];
      const stock = lengths.find(length => length >= calculated);
      if (Math.abs(calculated - Number(row.expected_required_m)) > 0.00001 || stock !== Number(row.expected_stock_m)) fail('独立参考算例未通过，请核对公式、参数、标准线长和预期结果');
    }
    if (model.id === 'network') {
      const ip = /^(?:\d{1,3}\.){3}\d{1,3}$/;
      const [address, prefix] = row.address.split('/');
      if (!ip.test(address) || address.split('.').some(n => Number(n) > 255) || !/^\d+$/.test(prefix || '') || Number(prefix) > 32) fail('IPv4/CIDR 格式错误（原型演示范围）');
      if (!ip.test(row.peer_address) || row.peer_address.split('.').some(n => Number(n) > 255)) fail('对端 IPv4 格式错误');
      if (Number(row.asn) > 4294967295 || Number(row.peer_asn) > 4294967295) fail('ASN 超出范围');
    }
    if (model.id === 'power' && (Number(row.planned_load_kw) < 0 || Number(row.planned_load_kw) > Number(row.capacity_kw))) fail('规划负载不合法或超出容量');
  }
  return issues;
}

function snapshot(): StudioSnapshot {
  const issues: StudioSnapshot['issues'] = [];
  if (!state.analyzed) issues.push({ id: 'analysis', step_id: 1, title: '资料尚未分析', detail: '先用明确标记的合成资料体验分流与提取流程。', action_label: '演示资料分析', model_id: '', severity: 'BLOCK' });
  else if (!state.identity_confirmed) issues.push({ id: 'identity', step_id: 1, title: '确认设备身份关联', detail: '十个合成设备候选引用现有 fixture 身份；必须由用户确认后采用。', action_label: '审核身份关联', model_id: '', severity: 'BLOCK' });
  for (const model of state.models) {
    const errors = validateStudioModel(model, state);
    if (model.status !== 'adopted' || errors.length) issues.push({ id: model.id, step_id: model.step_ids[0], title: model.name + (errors.length ? '存在错误' : '待核对采用'), detail: errors[0] || model.purpose, action_label: '补充与审核', model_id: model.id, severity: 'BLOCK' });
  }
  if (!state.calculation) issues.push({ id: 'calculation', step_id: 3, title: '布线计算待执行', detail: '已核对的路径、规格和规则才能生成计算线长；实装长度保持空值。', action_label: '查看布线计算', model_id: '', severity: 'BLOCK' });
  state.issues = issues;
  state.steps = fixture.steps.map(step => {
    const count = issues.filter(issue => issue.step_id === step.id || state.models.find(m => m.id === issue.model_id)?.step_ids.includes(step.id)).length;
    const identityNeeded = !state.analyzed || !state.identity_confirmed;
    const ready = !count && !identityNeeded;
    return { ...step, asset_count: state.assets.length, issue_count: count, state: state.published_revision !== null ? 'PUBLISHED' : ready ? 'READY' : state.last_published_revision !== null ? 'STALE' : state.analyzed ? 'NEEDS_INPUT' : 'DETECTED' };
  });
  return structuredClone(state);
}

function checkRevision(revision: number) {
  if (revision !== state.revision) throw Error('草稿已更新，请重新加载后审核当前版本。');
}

function commit(event: string) {
  state.revision++;
  state.events = [event, ...state.events].slice(0, 16);
  const result = snapshot();
  localStorage.setItem(storageKey, JSON.stringify(state));
  return result;
}

function invalidateModels(id: string) {
  const dependents: Record<string, string[]> = { rooms: ['racks', 'placements', 'route_nodes', 'route_edges', 'power'], racks: ['placements', 'route_nodes', 'route_edges', 'power'], route_nodes: ['route_edges'], cable_specs: ['cabling_rules'] };
  state.models.forEach(model => { if (dependents[id]?.includes(model.id)) model.status = 'draft'; });
  if (modelOf(state, id).step_ids.includes(3)) state.calculation = null;
  state.review_revision = null;
  state.published_revision = null;
}

function calculate(): StudioSnapshot['calculation'] {
  const required = state.models.filter(model => model.step_ids.includes(3));
  if (!state.identity_confirmed || required.some(model => model.status !== 'adopted' || validateStudioModel(model, state).length)) throw Error('请先确认身份并采用机房、落位、路径、规格及规则建模。');
  const placements = modelRows(modelOf(state, 'placements'));
  const nodes = modelRows(modelOf(state, 'route_nodes'));
  const edges = modelRows(modelOf(state, 'route_edges'));
  const rackFor = (device: string) => placements.find(row => row.device_id === device)?.rack_id;
  const start = nodes.find(row => row.rack_id === rackFor('srv-0001'))?.node_id;
  const end = nodes.find(row => row.rack_id === rackFor('sw-leaf-0001'))?.node_id;
  if (!start || !end) throw Error('参考连接两端缺少明确机柜接入节点。');
  const distances = new Map(nodes.map(row => [row.node_id, row.node_id === start ? 0 : Infinity]));
  const pending = new Set(distances.keys());
  while (pending.size) {
    const current = [...pending].sort((a, b) => distances.get(a)! - distances.get(b)!)[0];
    pending.delete(current);
    for (const edge of edges) {
      const next = edge.from_node === current ? edge.to_node : edge.to_node === current ? edge.from_node : null;
      if (next && pending.has(next)) distances.set(next, Math.min(distances.get(next)!, distances.get(current)! + Number(edge.length_m)));
    }
  }
  const route = distances.get(end)!;
  if (!Number.isFinite(route)) throw Error('两端没有可达的允许布线路径。');
  const ruleModel = modelOf(state, 'cabling_rules');
  const rule = modelRows(ruleModel)[0];
  const leads = Number(rule.lead_a_m) + Number(rule.lead_b_m);
  const requiredLength = Number(((route + leads) * (1 + Number(rule.slack_percent) / 100)).toFixed(4));
  const stock = modelRows(modelOf(state, 'cable_specs'))[0].standard_lengths_m.split(';').map(Number).sort((a, b) => a - b).find(length => length >= requiredLength);
  if (stock === undefined) throw Error('可采购规格中没有覆盖计算长度的线缆，请补充规格或调整规划。');
  return { route_m: route, leads_m: leads, slack_percent: Number(rule.slack_percent), required_m: requiredLength, stock_m: stock, cable_count: 1, optic_count: 2, rule_version: ruleModel.version };
}

export const kickoffStudioApi = {
  async get(): Promise<StudioSnapshot> {
    if (state) return snapshot();
    loading ||= (async () => {
      const responses = await Promise.all([fetch(fixtureUrl), fetch(seedUrl)]);
      if (responses.some(response => !response.ok)) throw Error('合成资料加载失败，请重试。');
      const [metadata, seed] = await Promise.all(responses.map(response => response.json()));
      fixture = metadata;
      const inventory = [...seed.collections.servers, ...seed.collections.switches] as Record<string, unknown>[];
      state = {
        project_id: fixture.project_id, project_name: fixture.project_name, simulation_only: true, revision: 1,
        analyzed: false, identity_confirmed: false, review_revision: null, published_revision: null, last_published_revision: null,
        models: structuredClone(fixture.models), sources: structuredClone(fixture.sources), benchmark: structuredClone(fixture.benchmark),
        assets: inventory.filter(asset => fixture.asset_refs.includes(String(asset.id))).map(asset => ({ id: String(asset.id), label: String(asset.hostname), serial: String(asset.serial || asset.sn || ''), model: String(asset.model), rack: String(asset.rack_id), position: `U${asset.rack_u} · ${asset.u_height}U`, source: `demo-seed.json / ${String(asset.id).startsWith('srv') ? 'servers' : 'switches'} / ${asset.id}` })),
        termination: { a_sn: String(inventory.find(a => a.id === 'srv-0001')?.serial || ''), a_mac: String(seed.collections.ports.find((p: Record<string, string>) => p.id === 'srv-0001-nic-01-p1').mac), a_port: 'NIC1/P1', b_sn: String(inventory.find(a => a.id === 'sw-leaf-0001')?.serial || ''), b_rack_position: 'rack-0002 / U2', b_mac: fixture.supplemental_b_mac, b_port: 'swp1', termination_type: 'fiber' },
        steps: [], issues: [], calculation: null, events: ['建立独立交互草稿；所有资产引用统一合成 fixture，未写入 CMDB。'],
      };
      const saved = localStorage.getItem(storageKey);
      if (saved) state = JSON.parse(saved) as StudioSnapshot;
      return snapshot();
    })();
    try { return await loading; } catch (error) { loading = undefined; throw error; }
  },
  act(input: StudioAction) {
    checkRevision(input.expected_revision);
    switch (input.action) {
      case 'analyze_example':
        state.analyzed = true;
        state.sources = state.sources.map(source => source.kind === 'example' ? { ...source, status: '合成候选已展示' } : source);
        return commit('展示合成分流与提取候选；未调用 gptsol6.1，用户上传文件未解析。');
      case 'confirm_identity':
        if (!state.analyzed) throw Error('请先展示资料候选。');
        if (!/^([0-9a-f]{2}:){5}[0-9a-f]{2}$/i.test(input.b_mac || '')) throw Error('请补充有效的对端 MAC，再确认关联。');
        state.termination.b_mac = input.b_mac!.toLowerCase();
        state.identity_confirmed = true;
        state.review_revision = null;
        state.published_revision = null;
        return commit('用户确认十个 fixture 身份引用；缺失的真实 SN / MAC 不在原型中补造。');
      case 'adopt_models': {
        const errors = state.models.flatMap(model => validateStudioModel(model, state).map(error => model.name + '：' + error));
        if (errors.length) throw Error(errors[0]);
        state.models.forEach(model => { model.status = 'adopted'; });
        state.review_revision = null;
        state.published_revision = null;
        return commit('用户批量核对采用九份合成建模表；规则参考算例通过。');
      }
      case 'calculate':
        state.calculation = calculate();
        state.review_revision = null;
        state.published_revision = null;
        return commit('完成一条明确参考连接的确定性布线计算，结果为 Calculated。');
      case 'review':
        if (snapshot().issues.length) throw Error('请先完成补充清单并计算。');
        if (!input.reason.trim()) throw Error('请记录审核依据。');
        state.review_revision = state.revision + 1;
        return commit('用户审核当前模型与计算版本：' + input.reason);
      case 'publish':
        if (snapshot().issues.length || state.review_revision !== state.revision) throw Error('请先审核当前修订；变更后旧审核不可复用。');
        state.published_revision = state.revision + 1;
        state.last_published_revision = state.published_revision;
        return commit('模拟发布完成：展示 CMDB / 工程计划 / 网络计划收据；交付主链待执行。未写入任何真实服务。');
    }
  },
  saveModel(id: string, input: ModelInput) {
    checkRevision(input.expected_revision);
    const current = modelOf(state, id);
    const candidate = { ...current, rows: input.rows, status: input.decision === 'adopt' ? 'adopted' as const : 'draft' as const };
    const trial = { ...state, models: state.models.map(model => model.id === id ? candidate : model) };
    const errors = validateStudioModel(candidate, trial);
    if (input.decision === 'adopt' && errors.length) throw Error(errors[0]);
    const changed = JSON.stringify(current.rows) !== JSON.stringify(input.rows);
    state.models = state.models.map(model => model.id === id ? { ...candidate, version: current.version + (changed ? 1 : 0), source: input.reason || current.source } : model);
    invalidateModels(id);
    return commit(`${current.name}：${input.decision === 'adopt' ? '核对并采用' : '保存草稿'}；关联产物需按新输入复核。`);
  },
  registerSources(input: StudioSourceInput) {
    checkRevision(input.expected_revision);
    state.sources.push(...input.filenames.map((name, index) => ({ id: `upload-${state.revision}-${index}`, name, kind: 'user_metadata' as const, step_ids: [], status: '仅登记文件名 · 原型未读取内容' })));
    return commit('登记用户文件名；真实上传、脱敏和 Agent 分析留待后端实施。');
  },
  configureBenchmark(input: StudioBenchmarkInput) {
    checkRevision(input.expected_revision);
    if (!input.provider_ref.trim()) throw Error('请填写 Provider 引用；不要填写 API Key。');
    state.benchmark.provider_ref = input.provider_ref.trim();
    // Benchmark metadata does not change the engineering revision or its review.
    localStorage.setItem(storageKey, JSON.stringify(state));
    return snapshot();
  },
};

export function modelCSV(model: StudioModel): string {
  const quote = (value: string) => /[",\r\n]/.test(value) ? '"' + value.replaceAll('"', '""') + '"' : value;
  return [model.columns.map(column => column.key), ...model.rows].map(row => row.map(quote).join(',')).join('\r\n');
}

export function parseModelCSV(text: string, model: StudioModel): string[][] {
  const rows: string[][] = [];
  let row: string[] = [], value = '', quoted = false;
  const source = text.replace(/^\uFEFF/, '').replace(/\r\n/g, '\n');
  for (let i = 0; i < source.length; i++) {
    const c = source[i];
    if (c === '"') {
      if (quoted && source[i + 1] === '"') { value += '"'; i++; } else quoted = !quoted;
    } else if (!quoted && (c === ',' || c === '\n')) {
      row.push(value); value = '';
      if (c === '\n') { rows.push(row); row = []; }
    } else value += c;
  }
  if (quoted) throw Error('CSV 引号未闭合。');
  if (value || row.length) { row.push(value); rows.push(row); }
  if (JSON.stringify(rows.shift()) !== JSON.stringify(model.columns.map(column => column.key))) throw Error('表头与模板不一致。请下载此模型的 CSV 模板，不丢弃未知列。');
  return rows.filter(r => r.some(cell => cell.trim()));
}

export function downloadStudioFile(name: string, text: string, type = 'text/plain;charset=utf-8') {
  const url = URL.createObjectURL(new Blob([text], { type }));
  const link = document.createElement('a'); link.href = url; link.download = name; link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
