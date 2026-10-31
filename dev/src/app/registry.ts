import {LayoutDashboard,FolderKanban,HardDrive,ShieldCheck,Network,Settings,Workflow,Server,Box,Ticket,Activity,Database,Brain,Users,FileSpreadsheet} from 'lucide-react';
export type LoadMode='SSE'|'poll-5s'|'poll-60s'|'on-demand';
export type Entry={id:string;label:string;icon:typeof Network;scope:'project'|'cross'|'admin';permission:string;load:LoadMode;parent?:string;views?:{id:string;label:string;load?:LoadMode}[]};
export const sections:{label:string;entries:Entry[]}[]=[{label:'部署交付',entries:[
{id:'projects',label:'部署项目',icon:FolderKanban,scope:'project',permission:'project:read',load:'on-demand'},
{id:'overview',label:'项目总览',icon:LayoutDashboard,scope:'project',permission:'delivery:read',load:'poll-60s'},
{id:'bom',label:'BOM 开局配置',icon:FileSpreadsheet,scope:'project',permission:'provision:read',load:'on-demand'},
{id:'provision',label:'装机管理',icon:HardDrive,scope:'project',permission:'provision:read',load:'on-demand',views:[{id:'pxe',label:'PXE 批次',load:'poll-5s'},{id:'media',label:'OS 镜像'},{id:'bios',label:'BIOS 批量置位'},{id:'firmware',label:'固件升级'}]},
{id:'validation',label:'验机管理',icon:ShieldCheck,scope:'project',permission:'validation:read',load:'on-demand',views:[{id:'checks',label:'验机检查与结果'},{id:'collection',label:'预收集记录'},{id:'pcie',label:'PCIe 拓扑'},{id:'lldp',label:'LLDP 四步核查'},{id:'mapping',label:'GPU-NIC Mapping'}]},
{id:'nettest',label:'网络测试',icon:Activity,scope:'project',permission:'test:read',load:'poll-5s',views:[{id:'roce',label:'RoCE 连通性'},{id:'batches',label:'测试批次'},{id:'reports',label:'报告与路径矩阵',load:'on-demand'}]},
{id:'config',label:'配置管理',icon:Settings,scope:'project',permission:'config:read',load:'on-demand',views:[{id:'configset',label:'ConfigSet'},{id:'roles',label:'网络角色方案'},{id:'history',label:'应用记录'},{id:'agent',label:'Agent 分发'}]},
{id:'fabric',label:'网络底座',icon:Network,scope:'project',permission:'fabric:read',load:'poll-60s',views:[{id:'readiness',label:'网络就绪验收',load:'on-demand'},{id:'devices',label:'网络设备',load:'SSE'},{id:'topology',label:'Topology 拓扑与布线',load:'on-demand'},{id:'ztp',label:'ZTP 开通',load:'SSE'},{id:'golden',label:'Golden 配置'},{id:'doublecheck',label:'Doublecheck'},{id:'pushjobs',label:'PushJob',load:'SSE'},{id:'lossless',label:'无损健康'},{id:'ddm',label:'光模块 DDM'},{id:'logs',label:'网络日志'},{id:'rules',label:'规则治理'}]},
{id:'nccl',label:'NCCL 测试',icon:Activity,scope:'project',permission:'test:read',load:'poll-5s',views:[{id:'batches',label:'测试批次'},{id:'reports',label:'测试报告',load:'on-demand'},{id:'baseline',label:'基线对比'},{id:'thresholds',label:'阈值场景'},{id:'trace',label:'网络溯源'}]},
{id:'cleanup',label:'优雅退场',icon:ShieldCheck,scope:'project',permission:'cleanup:read',load:'poll-5s',views:[{id:'plan',label:'清理计划'},{id:'executions',label:'清理执行'},{id:'archive',label:'归档与签署'},{id:'maintained',label:'维护态转换'}]}]},
{label:'运维中心',entries:[
{id:'topology',label:'数据中心拓扑',icon:Network,scope:'cross',permission:'cmdb:read',load:'on-demand'},
{id:'dashboard',label:'运维总览',icon:LayoutDashboard,scope:'cross',permission:'dashboard:read',load:'poll-60s'},
{id:'fleet',label:'服务器舰队',icon:Server,scope:'cross',permission:'fleet:read',load:'poll-5s',views:[{id:'servers',label:'服务器列表'},{id:'ports',label:'网口健康',load:'SSE'},{id:'assets',label:'资产台账'},{id:'collections',label:'采集任务'}]},
{id:'racks',parent:'topology',label:'机柜管理',icon:Box,scope:'cross',permission:'cmdb:read',load:'on-demand'},
{id:'issues',label:'工单中心',icon:Ticket,scope:'cross',permission:'issues:read',load:'poll-5s',views:[{id:'pool',label:'全局工单池'},{id:'board',label:'工单看板'},{id:'statistics',label:'处置统计'},{id:'retest',label:'故障复测'}]},
{id:'monitor',label:'监控中心',icon:Activity,scope:'cross',permission:'monitor:read',load:'SSE',views:[{id:'alerts',label:'告警中心'},{id:'policies',label:'采集策略'},{id:'observations',label:'上报观测'},{id:'maintained',label:'维护期 BMC 监控'}]},
{id:'flows',label:'作业平台',icon:Workflow,scope:'cross',permission:'flows:read',load:'SSE',views:[{id:'executions',label:'Executions'},{id:'agents',label:'Agents'}]},
{id:'cmdb',label:'CMDB',icon:Database,scope:'cross',permission:'cmdb:read',load:'on-demand',views:[{id:'assets',label:'资产目录'},{id:'relations',label:'资产关系'},{id:'configuration',label:'配置与来源'},{id:'quality',label:'数据质量'},{id:'projects',label:'项目资料'},{id:'history',label:'生命周期'}]},
{id:'ai',label:'AI 助手',icon:Brain,scope:'cross',permission:'ai:read',load:'on-demand',views:[{id:'chat',label:'AI 对话'},{id:'providers',label:'Provider 管理'},{id:'knowledge',label:'知识库'},{id:'tasks',label:'识别任务'},{id:'feedback',label:'识别与反馈'},{id:'evaluation',label:'评测报告'}]}]},
{label:'系统管理',entries:[{id:'admin',label:'平台设置',icon:Users,scope:'admin',permission:'admin:read',load:'on-demand',views:[{id:'users',label:'用户与权限'},{id:'members',label:'项目成员'},{id:'roles',label:'角色与权限'},{id:'sessions',label:'Session 管理'},{id:'logins',label:'登录审计'},{id:'approvals',label:'审批记录'},{id:'notifications',label:'通知设置'},{id:'tokens',label:'API Token'},{id:'audit',label:'审计日志'},{id:'templates',label:'项目模板'},{id:'models',label:'型号库'},{id:'parameters',label:'参数目录'},{id:'ssh',label:'SSH 信任管理'},{id:'ai-provider',label:'AI Provider 配置'}]}]}];
const deployOrder=['projects','overview','bom','fabric','config','provision','validation','nettest','nccl','cleanup'];sections[0].entries.sort((a,b)=>deployOrder.indexOf(a.id)-deployOrder.indexOf(b.id));
const operationsOrder=['dashboard','fleet','topology','racks','cmdb','monitor','issues','flows','ai'];sections[1].entries.sort((a,b)=>operationsOrder.indexOf(a.id)-operationsOrder.indexOf(b.id));
export const entries=sections.flatMap(s=>s.entries);
export function entryUrl(entry:Entry,projectId:string){if(entry.id==='projects')return '/projects';return entry.scope==='project'?`/p/${encodeURIComponent(projectId)}/deploy/${entry.id}`:`/${entry.id}`}
