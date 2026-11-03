import {LayoutDashboard,FolderKanban,HardDrive,ShieldCheck,Network,Settings,Workflow,Server,Box,Ticket,Activity,Database,Brain,Users,FileSpreadsheet} from 'lucide-react';
export type LoadMode='SSE'|'poll-5s'|'poll-60s'|'on-demand';
export type Entry={id:string;label:string;icon:typeof Network;scope:'project'|'cross'|'admin';permission:string;load:LoadMode;parent?:string;views?:{id:string;label:string;load?:LoadMode}[]};
export const preservedModules = new Set(['projects','kickoff','topology','racks','cmdb','flows','admin']);
export function isConstructionPath(pathname:string){const parts=pathname.split('/').filter(Boolean);const id=parts[0]==='p'&&parts[2]==='deploy'?parts[3]:parts[0];return entries.some(e=>e.id===id)&&!preservedModules.has(id)&&pathname!=='/ai/providers';}
export const sections:{label:string;entries:Entry[]}[]=[{label:'部署交付',entries:[
{id:'projects',label:'部署项目',icon:FolderKanban,scope:'project',permission:'project:read',load:'on-demand'},
{id:'overview',label:'项目总览',icon:LayoutDashboard,scope:'project',permission:'delivery:read',load:'on-demand'},
{id:'kickoff',label:'智能开局工程',icon:FileSpreadsheet,scope:'project',permission:'provision:read',load:'on-demand',views:[{id:'overview',label:'启动总览'},{id:'sources',label:'资料与识别'},{id:'review',label:'数据核对'},{id:'publish',label:'入库与版本'},{id:'models/facility',label:'机房与机柜'},{id:'models/network',label:'网络与接入'},{id:'models/provisioning',label:'装机准备'},{id:'models/cabling',label:'布线与计算'},{id:'models/power',label:'电力与冷却'}]},
{id:'provision',label:'装机管理',icon:HardDrive,scope:'project',permission:'provision:read',load:'on-demand'},
{id:'validation',label:'验机管理',icon:ShieldCheck,scope:'project',permission:'validation:read',load:'on-demand'},
{id:'nettest',label:'网络测试',icon:Activity,scope:'project',permission:'test:read',load:'on-demand'},
{id:'config',label:'配置管理',icon:Settings,scope:'project',permission:'config:read',load:'on-demand'},
{id:'fabric',label:'网络底座',icon:Network,scope:'project',permission:'fabric:read',load:'on-demand'},
{id:'nccl',label:'NCCL 测试',icon:Activity,scope:'project',permission:'test:read',load:'on-demand'},
{id:'cleanup',label:'优雅退场',icon:ShieldCheck,scope:'project',permission:'cleanup:read',load:'on-demand'}]},
{label:'运维中心',entries:[
{id:'topology',label:'数据中心拓扑',icon:Network,scope:'cross',permission:'cmdb:read',load:'on-demand'},
{id:'dashboard',label:'运维总览',icon:LayoutDashboard,scope:'cross',permission:'dashboard:read',load:'on-demand'},
{id:'fleet',label:'服务器舰队',icon:Server,scope:'cross',permission:'fleet:read',load:'on-demand'},
{id:'racks',parent:'topology',label:'机柜管理',icon:Box,scope:'cross',permission:'cmdb:read',load:'on-demand'},
{id:'issues',label:'工单中心',icon:Ticket,scope:'cross',permission:'issues:read',load:'on-demand'},
{id:'monitor',label:'监控中心',icon:Activity,scope:'cross',permission:'monitor:read',load:'on-demand'},
{id:'flows',label:'作业平台',icon:Workflow,scope:'cross',permission:'flows:read',load:'SSE',views:[{id:'executions',label:'Executions'},{id:'agents',label:'Agents'}]},
{id:'cmdb',label:'CMDB',icon:Database,scope:'cross',permission:'cmdb:read',load:'on-demand',views:[{id:'assets',label:'资产目录'},{id:'relations',label:'资产关系'},{id:'configuration',label:'配置与来源'},{id:'quality',label:'数据质量'},{id:'projects',label:'项目资料'},{id:'history',label:'生命周期'}]},
{id:'ai',label:'AI 助手',icon:Brain,scope:'cross',permission:'ai:read',load:'on-demand'}]},
{label:'系统管理',entries:[{id:'admin',label:'平台设置',icon:Users,scope:'admin',permission:'admin:read',load:'on-demand',views:[{id:'users',label:'用户与权限'},{id:'members',label:'项目成员'},{id:'roles',label:'角色与权限'},{id:'sessions',label:'Session 管理'},{id:'logins',label:'登录审计'},{id:'approvals',label:'审批记录'},{id:'notifications',label:'通知设置'},{id:'tokens',label:'API Token'},{id:'audit',label:'审计日志'},{id:'templates',label:'项目模板'},{id:'models',label:'型号库'},{id:'parameters',label:'参数目录'},{id:'ssh',label:'SSH 信任管理'},{id:'ai-provider',label:'AI Provider 配置'}]}]}];
const deployOrder=['projects','overview','kickoff','fabric','config','provision','validation','nettest','nccl','cleanup'];sections[0].entries.sort((a,b)=>deployOrder.indexOf(a.id)-deployOrder.indexOf(b.id));
const operationsOrder=['dashboard','fleet','topology','racks','cmdb','monitor','issues','flows','ai'];sections[1].entries.sort((a,b)=>operationsOrder.indexOf(a.id)-operationsOrder.indexOf(b.id));
export const entries=sections.flatMap(s=>s.entries);
export function entryUrl(entry:Entry,projectId:string){if(entry.id==='projects')return '/projects';if(entry.id==='kickoff')return `/p/${encodeURIComponent(projectId)}/kickoff`; return entry.scope==='project'?`/p/${encodeURIComponent(projectId)}/deploy/${entry.id}`:`/${entry.id}`}
