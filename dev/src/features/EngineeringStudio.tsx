import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {Activity,ArrowDownToLine,ArrowRight,Cable,Check,ChevronRight,FolderOpen,GitBranch,LayoutDashboard,ListChecks,MapPin,Network,Search,Server,ShieldCheck,SlidersHorizontal,Sparkles,X,Zap} from 'lucide-react';
import * as Dialog from '@radix-ui/react-dialog';
import {Button} from '../components/ui';
import {adoptEngineeringModelingInputs,engineeringInputCatalog,engineeringInputTemplateURL,engineeringModelingInputs,engineeringRevisions,type EngineeringWorkspace,type EngineeringPlanView} from '../lib/api/engineering';
import {EngineeringStepInputs} from './EngineeringStepInputs';
import {EngineeringRuleParameters} from './EngineeringRuleParameters';
import {modelCategories,modelForTable,modelInputState,modelStateLabels,studioModels,type StudioModel} from './engineeringStudioCatalog';

export const studioViews=[['studio','启动总览',LayoutDashboard],['inputs','资料与建模',FolderOpen],['model','五步工程',GitBranch],['release','审核与交接',ShieldCheck],['benchmark','基准验收',Activity]] as const;
export const studioSteps=[
 {name:'设备关联',module:'equipment',tab:'model',icon:Server,inputs:'本端 SN / MAC / 端口 · 对端 SN / MAC / 端口 · 位置按 SN 关联第二步落位资料',outputs:'设备身份 · 端口 · 规划连接'},
 {name:'机房与落位',module:'facility_placement',tab:'model',icon:MapPin,inputs:'机房与坐标系 · 机柜尺寸与位置 · 设备高度、U 位和安装面',outputs:'FacilityModel · RackPlacement'},
 {name:'连线与布线',module:'connectivity_cabling',tab:'connectivity',icon:Cable,inputs:'两端端接 · 允许路径 · 线缆与光模块规格 · 已批准计算规则',outputs:'ConnectivityPlan · CablePlan · 材料与工程量'},
 {name:'逻辑网络',module:'network_design',tab:'network',icon:Network,inputs:'已发布物理连接版本 · 客户明确的端口、IP、ASN 与邻居',outputs:'NetworkPlan Revision · Golden 输入'},
 {name:'电力与冷却',module:'power_cooling',tab:'power',icon:Zap,inputs:'设备落位 · 负载场景 · 供电回路 · 容量与冷却依据',outputs:'PowerPlan · RackCapacity'},
];
export const studioStateLabels:Record<string,string>={DETECTED:'待确认',NEEDS_INPUT:'待补充',READY:'可审核',PROCESSING:'处理中',IN_REVIEW:'待审核',APPROVED:'已批准',PUBLISHED:'已发布',BLOCKED:'已阻断',STALE:'待重新核对',NOT_APPLICABLE:'不适用'};
export function StudioStepStrip({plan,selected,onStep}:{plan?:EngineeringPlanView;selected:number;onStep:(step:number)=>void}){
 return <div className="es-steps" aria-label="五步工程状态">{studioSteps.map((step,i)=>{const module=plan?.plan.modules.find(m=>m.definition_id===step.module);return <button className={selected===i+1?'active':''} key={step.module} onClick={()=>onStep(i+1)}><span className="es-step-top">0{i+1}<step.icon size={18}/></span><strong>{step.name}</strong><small data-state={module?.state}>{module?studioStateLabels[module.state]:'待建立工程依据'}</small></button>})}</div>;
}
export function StudioOverview({project,workspace,plan,onOpen,onStep,onModel}:{project:string;workspace?:EngineeringWorkspace;plan?:EngineeringPlanView;onOpen:(tab:string)=>void;onStep:(step:number)=>void;onModel:(step:number,model:string)=>void}){
 const inputs=useQuery({queryKey:['engineering-modeling-inputs',project,workspace?.id,workspace?.revision],queryFn:({signal})=>engineeringModelingInputs(project,workspace!.id,'',signal),enabled:!!workspace});
 const events=useQuery({queryKey:['engineering-revisions',project,workspace?.id,workspace?.revision],queryFn:({signal})=>engineeringRevisions(project,workspace!.id,signal),enabled:!!workspace});
 const items=inputs.data?.items||[],adopted=studioModels.filter(m=>modelInputState(m,items)==='adopted').length;
 const modules=plan?.plan.modules.filter(m=>studioSteps.some(s=>s.module===m.definition_id))||[];
 const ready=modules.filter(m=>['READY','IN_REVIEW','APPROVED','PUBLISHED'].includes(m.state)).length;
 const missing=studioModels.filter(m=>modelInputState(m,items)!=='adopted');
 const identity=items.find(i=>i.table_id==='terminations');
 const blocked=workspace?.issues.filter(i=>i.severity==='BLOCK')||[];
 const next=!workspace?{label:'提供开局资料',hint:'上传开局工作簿，或从标准建模模板开始。',run:()=>onOpen('inputs')}:
 !workspace.head_revision_id||!identity||identity.state!=='adopted'?{label:'确认设备关联',hint:'核对本端与对端信息，确认设备和端口关联。',run:()=>onStep(1)}:
 missing.length?{label:'核对工程建模',hint:`${missing[0].name}尚未核对；相关步骤共同复用这份输入。`,run:()=>onModel(missing[0].step,missing[0].id)}:
 blocked.length?{label:'处理工程校验问题',hint:`还有 ${blocked.length} 项阻断；定位问题后继续对应工程模块。`,run:()=>onOpen('issues')}:
 {label:'进入审核与交接',hint:'核对产物版本与计算依据，按域审核并发布。',run:()=>onOpen('release')};
 const gaps=items.flatMap(i=>i.issues.map(issue=>({table:i.table_id,...issue})));
 return <><div className="es-summary-grid"><section className="es-next"><Sparkles size={25}/><div><small>当前建议</small><h3>{next.label}</h3><p>{next.hint}</p><Button variant="primary" onClick={next.run}>{next.label}<ArrowRight size={14}/></Button></div></section><section className="panel padded es-readiness"><div className="es-section-title"><h3>就绪情况</h3><span>由输入与校验决定</span></div><strong>{ready}<small> / 5</small></strong><p>工程模块可审核</p><div className="es-progress-track"><span style={{width:ready*20+'%'}}/></div><span>{adopted} / 9 建模已核对 · {gaps.length+blocked.length} 项待处理</span></section></div>
 <div className="es-summary-grid"><section className="panel padded"><div className="es-section-title"><h3>集中补充清单</h3><Button onClick={()=>onOpen('inputs')}>查看全部<ChevronRight size={14}/></Button></div><p className="muted">按依赖合并缺口，一处修改，相关步骤共同复用。</p>{!workspace?<p className="empty">先提供资料，再按工程对象汇总缺失与冲突。</p>:<>{gaps.slice(0,5).map((g,i)=><button className="es-gap" key={i} onClick={()=>{const m=modelForTable(g.table);if(g.table==='terminations')onStep(1);else if(m)onModel(m.step,m.id)}}><span><b>{modelForTable(g.table)?.name||'设备关联'}</b><small>第 {g.row} 行 · {g.message}</small></span><ChevronRight size={15}/></button>)}{!gaps.length&&missing.slice(0,4).map(m=><button className="es-gap" key={m.id} onClick={()=>onModel(m.step,m.id)}><span><b>{m.name}</b><small>{m.purpose}</small></span><ChevronRight size={15}/></button>)}{blocked.slice(0,3).map(i=><button className="es-gap" key={i.id} onClick={()=>onOpen('issues')}><span>{i.message}</span><ChevronRight size={15}/></button>)}{workspace.questions.slice(0,3).map((q,i)=><button className="es-gap" key={i} onClick={()=>onOpen('inputs')}><span>{q}</span><ChevronRight size={15}/></button>)}{!gaps.length&&!missing.length&&!blocked.length&&!workspace.questions.length&&<p>当前输入完整，可以进入审核。</p>}</>}</section><section className="panel padded"><h3>一次提供，贯穿五步</h3>{[['01','开局资料','设备、端接、机房资料与网络设计，保留原件和来源。'],['02','工程建模包','房间、机柜、U 位、路径和规则，模板导入或页面补充。'],['03','集中审核与交接','确认差异和计算依据，分域发布，交付主链从待执行开始。']].map(([number,title,text])=><div className="es-input-story" key={number}><span>{number}</span><div><b>{title}</b><p>{text}</p></div></div>)}<Button onClick={()=>onOpen('inputs')}>进入资料与建模<ArrowRight size={14}/></Button></section></div>
 <section className="panel padded es-recent"><div className="es-section-title"><h3>最近变更</h3><Button onClick={()=>onOpen('versions')}>查看版本差异</Button></div>{events.error&&<p role="alert">{events.error.message}</p>}{events.data?.length?events.data.slice(0,3).map(e=><p key={e.id}><span>v{e.sequence}</span>{e.reason}<small>{e.actor}</small></p>):<p className="muted">尚无工程修订记录。</p>}</section></>;
}

export function EngineeringModelingCenter({project,sourceID,revision,canEdit,canUpload,canReview=false,onSaved,initialStep=0,initialModel='',compact=false}:{project:string;sourceID?:string;revision?:number;canEdit:boolean;canUpload:boolean;canReview?:boolean;initialStep?:number;initialModel?:string;compact?:boolean;onSaved:()=>Promise<void>}){
 const catalog=useQuery({queryKey:['engineering-input-catalog',project],queryFn:({signal})=>engineeringInputCatalog(project,signal)});
 const inputs=useQuery({queryKey:['engineering-modeling-inputs',project,sourceID,revision],queryFn:({signal})=>engineeringModelingInputs(project,sourceID!,'',signal),enabled:!!sourceID});
 const [filter,setFilter]=useState('all'),[search,setSearch]=useState(''),[editing,setEditing]=useState(initialModel),[tableID,setTableID]=useState('');
 const [batch,setBatch]=useState(false),[selected,setSelected]=useState<string[]>([]),[reason,setReason]=useState(''),[busy,setBusy]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState('');
 const group=studioModels.find(g=>g.id===editing),table=catalog.data?.tables.find(t=>t.id===(tableID||group?.tables[0]));
 const groups=studioModels.filter(g=>(compact?g.steps.includes(initialStep):filter==='all'||g.group===filter)&&(g.name+g.purpose).includes(search));
 const items=inputs.data?.items||[],gaps=items.flatMap(i=>i.issues.map(issue=>({table:i.table_id,...issue})));
 async function refresh(){await inputs.refetch();await onSaved()}
 function open(id:string,table=''){setEditing(id);setTableID(table)}
 function showBatch(){setSelected(studioModels.filter(m=>items.some(i=>m.tables.includes(i.table_id)&&i.rows>0&&i.state!=='adopted')).map(m=>m.id));setBatch(true);setError('')}
 async function adopt(){
  if(!sourceID||!revision||!reason.trim())return;
  setBusy(true);setError('');let currentRevision=revision;const completed:string[]=[],failures:string[]=[];
  try{
   const chosen=studioModels.filter(m=>selected.includes(m.id));
   for(const step of [2,3,4,5]){
    const models=chosen.filter(m=>m.step===step);if(!models.length)continue;
    try{
    const current=await engineeringModelingInputs(project,sourceID);
    const requested=current.items.filter(i=>models.some(m=>m.tables.includes(i.table_id)));
    // Cabling is computed with the related inputs as one existing server operation.
    const values=step===3?current.items.filter(i=>catalog.data?.tables.some(t=>t.id===i.table_id&&t.step===3)):requested;
    if(!values.length)throw Error('请先填写 '+models.map(m=>m.name).join('、'));
    if(values.some(i=>i.issues.length))throw Error('存在未解决的字段问题，请先修正再采用。');
    const updated=await adoptEngineeringModelingInputs(project,sourceID,{expected_revision:currentRevision,step,inputs:values.map(i=>({table_id:i.table_id,version:i.version})),reason});
    currentRevision=updated.revision;completed.push(...models.map(m=>m.name));
    }catch(e){failures.push(models.map(m=>m.name).join('、')+'：'+(e as Error).message)}
   }
   if(completed.length)setNotice('已核对采用：'+completed.join('、'));
   if(failures.length){setError(failures.join('；'));setSelected(v=>v.filter(id=>!completed.includes(studioModels.find(m=>m.id===id)?.name||'')))}else setBatch(false);
  }catch(e){setError((completed.length?'已采用 '+completed.join('、')+'；剩余输入未采用：':'')+(e as Error).message)}finally{await refresh();setBusy(false)}
 }
 const card=(m:StudioModel)=>{const values=items.filter(i=>m.tables.includes(i.table_id)),state=modelInputState(m,items);return <article className="panel es-model-card" key={m.id}><div className="es-section-title"><span className={'es-model-icon '+m.group}>{m.group==='facility'?<MapPin size={19}/>:m.group==='cabling'?<Cable size={19}/>:<SlidersHorizontal size={19}/>}</span><small data-state={state}>{modelStateLabels[state]}</small></div><h4>{m.name}</h4><p>{m.purpose}</p><div className="es-model-meta"><span>{values.reduce((n,v)=>n+v.rows,0)} 行 · v{Math.max(0,...values.map(v=>v.version))}</span><span>步骤 {m.steps.join(' / ')}</span></div><footer><a href={engineeringInputTemplateURL(project,m.tables[0],'xlsx')}><ArrowDownToLine size={14}/>模板</a><Button onClick={()=>open(m.id)}>导入 / 编辑<ChevronRight size={15}/></Button></footer></article>};
 return <section className="es-modeling"><div className="es-section-title"><div><h3>{compact?'本步建模输入':'工程建模输入'}</h3><p>支持分表导入，也可一次核对采用已填好的建模包。</p></div>{!compact&&<Button variant="primary" disabled={!sourceID||!canEdit||busy||!items.some(i=>i.rows>0&&i.state!=='adopted')} onClick={showBatch}><ListChecks size={15}/>批量核对建模</Button>}</div>
 {!compact&&<div className="es-filter"><div className="es-segments" aria-label="模型分类">{modelCategories.map(([id,label])=><button className={filter===id?'active':''} key={id} onClick={()=>setFilter(id)}>{label}</button>)}</div><label className="es-search"><Search size={15}/><input aria-label="搜索建模输入" placeholder="搜索建模输入" value={search} onChange={e=>setSearch(e.target.value)}/></label></div>}
 {catalog.isPending&&<p aria-busy="true">正在读取建模契约…</p>}{[catalog.error?.message,inputs.error?.message].filter(Boolean).map((v,i)=><p className="notice error" role="alert" key={i}>{v}</p>)}{notice&&<p className="notice" role="status">{notice}</p>}
 <div className="es-model-grid">{groups.map(card)}</div>{!groups.length&&<div className="empty">没有匹配的建模输入。<Button onClick={()=>{setFilter('all');setSearch('')}}>清除筛选</Button></div>}
 {!compact&&gaps.length>0&&<section className="panel padded es-supplement"><h3>集中补充清单</h3><div className="table-wrap"><table><thead><tr><th>建模输入</th><th>行 / 字段</th><th>缺口</th><th/></tr></thead><tbody>{gaps.map((v,i)=><tr key={i}><td>{catalog.data?.tables.find(t=>t.id===v.table)?.name||v.table}</td><td>{v.row} / {v.field}</td><td>{v.message}</td><td><Button onClick={()=>open(modelForTable(v.table)?.id||'terminations',v.table)}>补充</Button></td></tr>)}</tbody></table></div></section>}
 {!compact&&<p className="es-editor-note"><GitBranch size={17}/>身份 → 落位；身份 + 端口 → 物理连接；落位 + 路径 + 规则 → 布线；物理连接 → 网络；落位 + 供电 → 电力。网络与电力可并行。</p>}
 {(group&&table||editing==='terminations')&&<Dialog.Root open onOpenChange={value=>{if(!value)setEditing('')}}><Dialog.Portal><Dialog.Overlay className="es-model-dialog-overlay"/><Dialog.Content className="es-model-dialog"><div className="es-model-dialog-header"><div><Dialog.Title>{group?.name||'设备与端口关联'}</Dialog.Title><Dialog.Description>{group?.purpose||'核对本端和对端信息，关联统一设备身份。'} · 模型 v{Math.max(0,...items.filter(i=>group?.tables.includes(i.table_id)||i.table_id==='terminations').map(i=>i.version))}</Dialog.Description></div><Dialog.Close aria-label="关闭" className="es-editor-icon"><X size={19}/></Dialog.Close></div>
 {group&&group.requires.length>0&&<div className="es-model-dependencies"><span>关联依据</span>{group.requires.map(id=>{const dependency=studioModels.find(m=>m.id===id)!;return <button key={id} onClick={()=>open(id)}>{dependency.name}<small>{modelStateLabels[modelInputState(dependency,items)]}</small><ChevronRight size={12}/></button>})}</div>}
 {group&&group.tables.length>1&&<nav className="tabs es-model-subtables" aria-label="模型明细">{group.tables.map(id=><button key={id} className={id===table?.id?'active':''} onClick={()=>setTableID(id)}>{catalog.data?.tables.find(t=>t.id===id)?.name||id}{items.find(i=>i.table_id===id)?.issues.length?<span> · 待补充</span>:null}</button>)}</nav>}
 {group?.id==='cabling_rules'?<div className="es-rule-dialog-body"><EngineeringRuleParameters project={project} canEdit={canEdit} canReview={canReview}/><EngineeringStepInputs key={project+table!.id} project={project} sourceID={sourceID} revision={revision} canEdit={canEdit} canUpload={canUpload} initialStep={3} initialTable={table!.id} onSaved={refresh} onAdopted={()=>setEditing('')}/></div>:<EngineeringStepInputs key={project+(table?.id||'terminations')} project={project} sourceID={sourceID} revision={revision} canEdit={canEdit} canUpload={canUpload} initialStep={table?.step||1} initialTable={table?.id||'terminations'} onSaved={refresh} onAdopted={()=>setEditing('')}/>}
 </Dialog.Content></Dialog.Portal></Dialog.Root>}
 {batch&&<Dialog.Root open onOpenChange={v=>{if(!v&&!busy)setBatch(false)}}><Dialog.Portal><Dialog.Overlay className="es-model-dialog-overlay"/><Dialog.Content className="es-model-dialog es-batch-dialog"><div className="es-model-dialog-header"><div><Dialog.Title>批量核对工程建模</Dialog.Title><Dialog.Description>采用所选输入；每张表保留独立版本，领域校验按依赖顺序执行。</Dialog.Description></div><Dialog.Close disabled={busy} aria-label="关闭" className="es-editor-icon"><X size={18}/></Dialog.Close></div><div className="es-model-editor-body"><div className="es-batch-models">{studioModels.map(m=>{const values=items.filter(i=>m.tables.includes(i.table_id));return <label key={m.id}><input type="checkbox" disabled={busy||!values.some(i=>i.rows>0)||values.some(i=>i.issues.length>0)} checked={selected.includes(m.id)} onChange={e=>setSelected(s=>e.target.checked?[...s,m.id]:s.filter(id=>id!==m.id))}/><span>{m.name}<small>{modelStateLabels[modelInputState(m,items)]} · {values.reduce((n,i)=>n+i.rows,0)} 行</small></span><Button disabled={busy} onClick={()=>{setBatch(false);open(m.id)}}>查看</Button></label>})}</div><label className="es-editor-reason">核对依据<input aria-label="批量核对依据" value={reason} onChange={e=>setReason(e.target.value)}/></label><p className="muted">此操作更新工程输入。正式发布仍需审核；已发布历史版本保留。</p>{error&&<p role="alert" className="notice error">{error}</p>}</div><div className="es-model-editor-footer"><Button disabled={busy} onClick={()=>setBatch(false)}>继续编辑</Button><Button variant="primary" disabled={busy||!selected.length||!reason.trim()} onClick={()=>void adopt()}><Check size={15}/>{busy?'正在采用…':'已核对，采用所选建模'}</Button></div></Dialog.Content></Dialog.Portal></Dialog.Root>}
 </section>;
}
