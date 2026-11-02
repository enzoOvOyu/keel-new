import initial from './fixture.json';
import { tables, tableById } from './catalog';
import { calculateWorkspace, rowsFor } from './calculations';
import type { Capability, Device, KickoffAPI, ModelRow, Publication, PublishPreview, Workspace } from './types';

const storageKey = 'keel-kickoff-workspace-v1';
const now = () => new Date().toISOString();
const id = () => crypto.randomUUID();

export function eligibleDevices(s: Workspace) {
  return s.devices.filter(d => !s.issues.some(i => !i.resolved && i.scope === 'identity' && i.entityId === d.id));
}
export function publishPreview(s: Workspace, ids: string[]): PublishPreview {
  const devices = eligibleDevices(s).filter(d => ids.includes(d.id));
  const keys = new Set(devices.map(d => d.id));
  const ports = s.ports.filter(p => keys.has(p.deviceId));
  const portIds = new Set(ports.map(p => p.id));
  const connections = s.connections.filter(c => portIds.has(c.a) && portIds.has(c.b));
  const components = s.components.filter(c => keys.has(c.deviceId));
  const placements = devices.filter(d => placementReady(s, d));
  return { devices, ports, connections, components, placements, held: s.devices.filter(d => !keys.has(d.id)),
    counts: { devices: devices.length, ports: ports.length, connections: connections.length, components: components.length, placements: placements.length } };
}
export function placementReady(s: Workspace, d: Device) {
  const rack = s.racks.find(r => r.id === d.rackId);
  return !!rack && !!d.startU && !!d.heightU && d.startU + d.heightU - 1 <= rack.capacityU &&
    !s.issues.some(i => !i.resolved && i.scope === 'placement' && i.entityId === d.id);
}
export function capabilities(s: Workspace): Capability[] {
  const publishedIds = new Set(s.publications.flatMap(p => p.deviceIds));
  const deviceMap = new Map<string, Device>();
  for (const publication of s.publications) for (const device of publication.deviceSnapshot) deviceMap.set(device.id, device);
  const current = [...deviceMap.values()];
  const latest = s.publications.at(-1);
  const publishedState = {...s, devices: current, models: latest?.modelSnapshot ?? [], racks: latest?.rackSnapshot ?? []};
  const count = (n: number, total: number) => total > 0 && n === total ? 'ready' : n > 0 ? 'partial' : 'needs_input';
  const located = current.filter(d => placementReady(publishedState,d)).length;
  const hosts = current.filter(d => d.type === 'server');
  const boots = rowsFor(publishedState,'boot_interfaces');
  const installs = rowsFor(publishedState,'install_profiles');
  const installation = hosts.filter(d => boots.some(b => b.device_id === d.id && b.port && b.mac && b.boot_mode && b.network) && installs.some(p => (p.scope === d.id || p.scope === 'servers') && p.image_ref && p.disk_policy && p.architecture)).length;
  const network = current.filter(d => rowsFor(publishedState,'network_roles').some(r => r.device_id === d.id) && rowsFor(publishedState,'network_addresses').some(a => a.device_id === d.id && a.address && a.asn) && rowsFor(publishedState,'network_policies').some(p => (p.scope === d.id || p.scope === d.type+'s') && p.profile)).length;
  const ready = eligibleDevices(s).length;
  return [
    {id:'cmdb',title:'CMDB 资产与关系',detail:current.length ? '统一身份已可供下游读取' : '确认后即可建立统一资产视图',state:count(current.length,s.devices.length),available:current.length,total:s.devices.length,action:current.length?'查看入库版本':`确认 ${ready} 台可用设备`,href:'publish'},
    {id:'rack',title:'机柜图与空间',detail:'立面按已知U位绘制，空间参数可后补',state:count(located,s.devices.length),available:located,total:s.devices.length,action:'补充空间与落位',href:'models/facility'},
    {id:'fabric',title:'Fabric 网络交付',detail:current.length?'物理清单可用，配置生成按目标检查意图':'先接入资产，再逐步形成网络计划',state:count(network,s.devices.length),available:network,total:s.devices.length,action:'完善网络意图',href:'models/network'},
    {id:'install',title:'装机管理',detail:hosts.length?`${hosts.length} 台服务器可进入待装机清单`:'设备入库后即可建立待装机清单',state:count(installation,s.devices.filter(d=>d.type==='server').length),available:installation,total:s.devices.filter(d=>d.type==='server').length,action:'关联启动与安装策略',href:'models/provisioning'},
  ];
}

export class LocalKickoffProvider implements KickoffAPI {
  private state: Workspace;
  private listeners = new Set<() => void>();
  constructor() {
    const saved = localStorage.getItem(storageKey);
    if (saved) {
      try { const parsed = JSON.parse(saved); this.state = parsed.schemaVersion === 1 ? parsed : structuredClone(initial) as Workspace; }
      catch { this.state = structuredClone(initial) as Workspace; }
    } else this.state = structuredClone(initial) as Workspace;
  }
  subscribe = (fn: () => void) => { this.listeners.add(fn); return () => { this.listeners.delete(fn); }; };
  snapshot = () => this.state;
  private change(action: string, detail: string, fn: (s: Workspace) => void) {
    const next = structuredClone(this.state);
    fn(next);
    next.revision++;
    next.activity.unshift({ id: id(), action, detail, at: now() });
    localStorage.setItem(storageKey, JSON.stringify(next));
    this.state = next;
    this.listeners.forEach(fn => fn());
  }
  async addSources(files: File[]) {
    if (!files.length) return;
    const sources = await Promise.all(files.map(async file => {
      if (!/\.(xlsx|csv|pdf|txt)$/i.test(file.name)) throw Error('请选择 XLSX、CSV、PDF 或 TXT 资料。');
      const digest = await crypto.subtle.digest('SHA-256', await file.arrayBuffer());
      const hash = [...new Uint8Array(digest)].map(n => n.toString(16).padStart(2,'0')).join('');
      return { id: id(), name: file.name, size: `${Math.ceil(file.size/1024)} KB`, version: 1, state: 'awaiting_analysis' as const, hash, createdAt: now(), regions: [], originalAvailable: false };
    }));
    const fresh = sources.filter(s => !this.state.sources.some(old => old.hash === s.hash));
    if (!fresh.length) throw Error('相同内容的资料已存在，无需重复添加。');
    this.change('添加资料', `${fresh.length} 份资料等待分析`, s => s.sources.push(...fresh));
  }
  removeSource(sourceId: string) {
    const source = this.state.sources.find(s=>s.id===sourceId);
    if (source?.state === 'recognized') throw Error('已有识别结果的来源需要保留，请添加新版本进行增补。');
    this.change('移除待分析资料', source?.name ?? '', s => { s.sources = s.sources.filter(v=>v.id!==sourceId); });
  }
  async analyzeSource(_sourceId: string) { throw Error('分析服务尚未连接。可在工程建模中导入标准 XLSX，或继续核对已有识别结果。'); }
  resolveIssue(issueId: string, value: string) {
    const issue = this.state.issues.find(i=>i.id===issueId);
    if (!issue || issue.resolved) return;
    if (issue.scope==='configuration') throw Error('请通过对应工程建模补充依据。');
    if (issue.scope==='placement' && (!Number.isInteger(Number(value)) || Number(value)<=0)) throw Error('请输入明确的正整数高度U。');
    if (issue.scope==='identity' && value!==issue.entityId) throw Error('请选择明确匹配的设备身份。');
    this.change('确认关联', issue.title, s => {
      s.issues.find(i=>i.id===issueId)!.resolved=true;
      if (issue.scope==='placement') {
        s.devices.find(d=>d.id===issue.entityId)!.heightU=Number(value);
        const model = s.models.find(m=>m.tableId==='placements')!;
        const row = model.rows.find(r=>r.device_id===issue.entityId);
        if (row) row.height_u=value;
        model.version++; model.source='人工确认'; model.updatedAt=now();
      }
    });
  }
  saveModel(tableId: string, rows: ModelRow[], source: string) {
    const def=tableById(tableId);
    if (!def) throw Error('未知建模表');
    const nonempty = rows.filter(r=>Object.values(r).some(v=>v.trim()));
    nonempty.forEach((row,index) => def.fields.forEach(f=> {
      const value=row[f.key]?.trim()??'';
      if (!value) return;
      if (f.kind && f.kind!=='select' && (!Number.isFinite(Number(value)) || (f.kind==='positive' && Number(value)<=0) || (f.kind==='integer' && !Number.isInteger(Number(value))))) throw Error(`第 ${index+1} 行「${f.label}」格式不正确。`);
      if (/credential/.test(f.key) && !/^(vault|secret-ref|credential):\/\//.test(value)) throw Error('凭据仅接受 vault://、secret-ref:// 或 credential:// 引用。');
      if (['slack_m','slack_percent','lead_m'].includes(f.key) && Number(value)<0) throw Error('余量和引线长度不能为负数。');
    }));
    const seen=new Set<string>();
    if (['placements','racks','equipment_specs','rooms','management','boot_interfaces','power_profiles','rack_capacity','rack_access','route_nodes','route_edges'].includes(tableId)) {
      for (const row of nonempty) { const key=row[def.fields[0].key]; if (key && seen.has(key)) throw Error(`引用 ${key} 重复，请合并或核对冲突。`); if(key) seen.add(key); }
    }
    for (const row of nonempty) {
      if (row.device_id && !this.state.devices.some(d=>d.id===row.device_id)) throw Error(`设备引用 ${row.device_id} 尚未匹配，请先核对设备身份。`);
      if (row.rack_id && !this.state.racks.some(r=>r.id===row.rack_id)) throw Error(`机柜引用 ${row.rack_id} 尚未匹配。`);
      if (row.mac && !/^([\da-f]{2}:){5}[\da-f]{2}$/i.test(row.mac)) throw Error('MAC应使用完整的六组十六进制地址。');
    }
    this.change('更新工程输入', `${def.name} · ${nonempty.length} 行`, s=> {
      const old=s.models.find(m=>m.tableId===tableId);
      const next={tableId,rows:nonempty,version:(old?.version??0)+1,confirmed:true,updatedAt:now(),source};
      s.models=s.models.filter(m=>m.tableId!==tableId);s.models.push(next);
      if(tableId==='placements') for(const r of nonempty) {
        const d=s.devices.find(d=>d.id===r.device_id); if(!d) continue;
        if(r.rack_id) d.rackId=r.rack_id;
        if(r.start_u) d.startU=Number(r.start_u);
        if(r.height_u) d.heightU=Number(r.height_u);
        if(d.heightU) s.issues.filter(i=>i.entityId===d.id&&i.scope==='placement').forEach(i=>i.resolved=true);
      }
      if(tableId==='racks') for(const r of nonempty) {
        const rack=s.racks.find(x=>x.id===r.rack_id);if(!rack)continue;
        if(r.capacity_u)rack.capacityU=Number(r.capacity_u);if(r.room_id)rack.room=r.room_id;
      }
      if(tableId==='management') for(const r of nonempty) {
        const d=s.devices.find(d=>d.id===r.device_id);if(d&&r.management_ip)d.managementAddress=r.management_ip;
      }
      if(tableId==='equipment_specs') for(const r of nonempty) {
        if(!r.model||!r.height_u)continue;
        const placement=s.models.find(m=>m.tableId==='placements');
        let changed=false;
        for(const d of s.devices.filter(d=>d.model===r.model&&!d.heightU)) {
          d.heightU=Number(r.height_u);changed=true;
          const row=placement?.rows.find(p=>p.device_id===d.id);if(row&&!row.height_u)row.height_u=r.height_u;
          s.issues.filter(i=>i.entityId===d.id&&i.scope==='placement').forEach(i=>i.resolved=true);
        }
        if(changed&&placement){placement.version++;placement.source='按确认型号补充';placement.updatedAt=now();}
      }
    });
  }
  previewPublication = (ids: string[]) => publishPreview(this.state,ids);
  publish(ids: string[], reason: string, expectedRevision: number): Publication {
    if(expectedRevision!==this.state.revision)throw Error('数据已更新，请重新核对本次提交范围。');
    if(!reason.trim())throw Error('请填写本次入库说明。');
    const p=this.previewPublication(ids);
    if(!p.devices.length)throw Error('请至少选择一个身份明确的设备。');
    const publication: Publication={id:id(),revision:this.state.publications.length+1,createdAt:now(),actor:'陈工',reason,
      deviceIds:p.devices.map(d=>d.id),portIds:p.ports.map(v=>v.id),connectionIds:p.connections.map(v=>v.id),componentIds:p.components.map(v=>v.id),placementIds:p.placements.map(v=>v.id),
      modelVersions:Object.fromEntries(this.state.models.map(m=>[m.tableId,m.version])),modelSnapshot:structuredClone(this.state.models),rackSnapshot:structuredClone(this.state.racks),deviceSnapshot:structuredClone(p.devices),counts:p.counts};
    const last=this.state.publications.at(-1);
    if(last&&JSON.stringify(last.deviceIds)===JSON.stringify(publication.deviceIds)&&JSON.stringify(last.modelVersions)===JSON.stringify(publication.modelVersions)&&JSON.stringify(last.deviceSnapshot)===JSON.stringify(publication.deviceSnapshot)) throw Error('当前范围与最新版本一致，无需重复入库。');
    this.change('确认入库',`版本 r${publication.revision} · ${p.devices.length} 台设备 · ${p.connections.length} 条连接`,s=>s.publications.push(publication));
    return publication;
  }
  calculate(domain: 'cabling' | 'power') {
    const result=calculateWorkspace(this.state,domain);
    this.change('完成工程计算',domain==='cabling'?`线长计算 · ${result.rows.length} 条结果`:`功率计算 · ${result.power.length} 个机柜`,s=>s.calculations.push(result));
    return result;
  }
}

export { tables, tableById, rowsFor };
