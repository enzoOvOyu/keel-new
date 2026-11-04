import { identityGuidance } from '../lib/understanding';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { ArrowRight, Check, CheckCircle2, ChevronRight, Database, Download, History, ShieldCheck, Loader2 } from 'lucide-react';
import { useWorkspace } from '../context';
import { capabilities, eligibleDevices, type Publication, type PublicationOptions } from '../lib/api';
import { Badge, Button, DataTable, Empty, Field, Heading, KV, Modal, Notice, Section, formatDate } from '../components/ui';
import { downloadFile } from '../lib/api/workbook';
import './publication-scope.css';

type ScopeRow = { key:string; id:string; name:string; reference:string; category:string; state:string; reason:string; owners:string[]; root:'device'|'facility'|null; review:string };
const scopeCategories:Record<string,string>={device:'设备',site:'场地',room:'机房',rack:'机柜',port:'端口',component:'组件',connection:'连接',placement:'落位 / 位置关联'};
const scopeStates:Record<string,string>={new:'新增',changed:'变化 / 待增补',unchanged:'无变化',blocked:'阻断',pending:'处理中',dependency:'随设备范围核对',loading:'正在检查'};

export default function Publish(){
  const {api,state:s,go,run,can,inspect}=useWorkspace();
  const [params]=useSearchParams();
  useEffect(()=>{const id=params.get('publication');if(!id)return;void run('版本明细已读取',async()=>{const full=await api.publicationDetail!(id);const previous=[...s.publications].filter(p=>p.revision<full.revision).sort((a,b)=>b.revision-a.revision)[0];setPreviousDetail(previous?await api.publicationDetail!(previous.id):undefined);setDetail(full);});},[params.get('publication')]);
  const history=[...s.publications].sort((a,b)=>b.revision-a.revision);
  const latestByDevice=new Map<string,Publication>(),publishedByDevice=new Map<string,Publication>();
  for(const record of history)for(const id of record.deviceIds){if(!latestByDevice.has(id))latestByDevice.set(id,record);if(record.state==='published'&&!publishedByDevice.has(id))publishedByDevice.set(id,record);}
  const eligible=eligibleDevices(s);
  const publicationKey=s.publications.map(v=>v.id+':'+v.state).join('|');
  const optionsKey=`${s.projectId}:${s.revision}:${publicationKey}`;
  const [optionResult,setOptionResult]=useState<{key:string;value:PublicationOptions}>(),[optionsError,setOptionsError]=useState(''),[optionsRetry,setOptionsRetry]=useState(0);
  const initialized=useRef(false);
  const options=optionResult?.key===optionsKey?optionResult.value:undefined;
  const optionsLoading=!options&&!optionsError;
  const optionByDevice=new Map(options?.devices.map(v=>[v.id,v])??[]);
  const valid=new Set(eligible.filter(d=>['new','changed'].includes(optionByDevice.get(d.id)?.state??'')&&latestByDevice.get(d.id)?.state!=='pending').map(d=>d.id));
  const [selectedFacilities,setSelectedFacilities]=useState<string[]>([]);
  const validFacilities=new Set((options?.facilities??[]).filter(f=>f.state==='new'||f.state==='changed').map(f=>f.id));
  const activeFacilities=selectedFacilities.filter(id=>validFacilities.has(id));
  const [publishing,setPublishing]=useState(false);
  const [selected,setSelected]=useState<string[]>([]);
  const [frozenScope,setFrozenScope]=useState<{devices:string[];facilities:string[];counts:Record<string,number>;key:string}>();
  const [confirm,setConfirm]=useState(false),[expected,setExpected]=useState(s.revision),[reason,setReason]=useState('确认设备身份与已知关系，未完成的工程信息后续增补');
  const [detail,setDetail]=useState<Publication>(),[success,setSuccess]=useState<Publication>();
  const openDetail=async(v:Publication)=>{await run('版本明细已读取',async()=>{const full=await api.publicationDetail!(v.id);const previous=history.find(p=>p.revision<v.revision);if(previous){const value=await api.publicationDetail!(previous.id);setPreviousDetail(value);}else setPreviousDetail(undefined);setDetail(full);});};
  const [capabilityBaseline,setCapabilityBaseline]=useState<Record<string,number>>({});
  const [previousDetail,setPreviousDetail]=useState<Publication>();
  useEffect(()=>{let active=true;setOptionsError('');void api.publicationOptions(s.revision).then(value=>{if(!active)return;setOptionResult({key:optionsKey,value});if(!initialized.current){initialized.current=true;const ready=new Set(value.devices.filter(v=>v.state==='new'||v.state==='changed').map(v=>v.id));setSelectedFacilities((value.facilities??[]).filter(f=>f.state==='new'||f.state==='changed').map(f=>f.id));setSelected((params.get('targets')?.split(',')??[...ready]).filter(id=>ready.has(id)));}}).catch(e=>{if(active)setOptionsError(e.message);});return()=>{active=false;};},[api,optionsKey,optionsRetry]);
  useEffect(()=>{if(success){setSelected([]);setSelectedFacilities([]);}},[success?.id]);
  useEffect(()=>{if(options&&!publishing){setSelected(ids=>ids.filter(id=>valid.has(id)));setSelectedFacilities(ids=>ids.filter(id=>validFacilities.has(id)));}},[options,publishing]);
  const selectionKey=[...selected].sort().join(',')+'|'+[...activeFacilities].sort().join(',');
  const activeSelected=selected.filter(id=>valid.has(id));
  const local=api.previewPublication(activeSelected);
  const [evaluation,setEvaluation]=useState<{revision:number;counts:Record<string,number>;selectionKey:string}>(),[evaluating,setEvaluating]=useState(true),[evaluationError,setEvaluationError]=useState('');
  useEffect(()=>{let active=true;setEvaluating(true);setEvaluationError('');const timer=setTimeout(()=>{if(!options||(!activeSelected.length&&!activeFacilities.length)){if(active){setEvaluation(undefined);setEvaluating(false);}return;}void api.evaluatePublication!(activeSelected,s.revision,activeFacilities).then(v=>{if(active)setEvaluation({...v,selectionKey});}).catch(e=>{if(active)setEvaluationError(e.message);}).finally(()=>{if(active)setEvaluating(false);});},150);return()=>{active=false;clearTimeout(timer);};},[api,selected,selectedFacilities,s.revision,options]);
  const p={...local,counts:evaluation?.revision===s.revision&&evaluation.selectionKey===selectionKey?evaluation.counts:local.counts};
  const labels:Record<string,string>={devices:'设备',ports:'端口',connections:'双端连接',components:'组件',placements:'精确落位',facilities:'机房与机柜',locations:'已知位置关联'};
  const caps=s.capabilities??capabilities(s);
  const latest=history[0];
  const current=history.find(p=>p.state==='published');
  const [dependenciesLoading,setDependenciesLoading]=useState(false);
  const dependencySections=(['ports','connections','components'] as const).filter(key=>s[key].length<(s.totals?.[key]??s[key].length));
  const loadDependencies=async()=>{
    setDependenciesLoading(true);
    await run('依赖对象已读取',async()=>{for(const section of dependencySections)await api.loadSection!(section);});
    setDependenciesLoading(false);
  };
  const [category,setCategory]=useState('all'),[change,setChange]=useState('delta');
  const selectedDevices=new Set(activeSelected),selectedFacilitySet=new Set(activeFacilities);
  const selectedCount=activeSelected.length+activeFacilities.length;
  const allSelected=valid.size+validFacilities.size>0&&activeSelected.length===valid.size&&activeFacilities.length===validFacilities.size;
  const selectAll=()=>{setSelected(allSelected?[]:[...valid]);setSelectedFacilities(allSelected?[]:[...validFacilities]);};
  const scopeRows=useMemo(()=>{
    const rows:ScopeRow[]=[];
    const devices=new Map(s.devices.map(d=>[d.id,d]));
    const ports=new Map(s.ports.map(port=>[port.id,port]));
    const names=(ids:string[])=>ids.map(id=>devices.get(id)?.name||devices.get(id)?.serial||id).join(' ↔ ');
    const deviceOptions=new Map(options?.devices.map(v=>[v.id,v])??[]);
    const dependencyCounts=new Map<string,number>();
    const addDependency=(id:string,name:string,reference:string,category:string,owners:string[])=>{
      for(const owner of owners)dependencyCounts.set(owner,(dependencyCounts.get(owner)??0)+1);
      rows.push({key:category+':'+id,id,name,reference,category,state:'dependency',reason:'随关联设备范围核对，不可独立选择',owners,root:null,review:category==='placement'?'devices':category+'s'});
    };
    for(const port of s.ports)addDependency(port.id,port.name,port.id,'port',[port.deviceId]);
    for(const component of s.components)addDependency(component.id,component.name,component.model||component.id,'component',[component.deviceId]);
    for(const connection of s.connections){
      const owners=[...new Set([ports.get(connection.a)?.deviceId,ports.get(connection.b)?.deviceId].filter((id):id is string=>!!id))];
      addDependency(connection.id,`${ports.get(connection.a)?.name||connection.a} ↔ ${ports.get(connection.b)?.name||connection.b}`,connection.id,'connection',owners);
    }
    const racks=new Map(s.racks.map(rack=>[rack.id,rack.name]));
    for(const device of s.devices)if(device.rackId)addDependency(device.id,names([device.id]),`${racks.get(device.rackId)||device.rackId} · ${device.startU!==null&&device.heightU!==null?`U${device.startU} / ${device.heightU}U（精确落位待核对）`:'已知位置关联，U位待补充'}`,'placement',[device.id]);
    const roots:ScopeRow[]=s.devices.map(device=>{
      const option=deviceOptions.get(device.id);
      return {key:'device:'+device.id,id:device.id,name:device.name||device.serial||'名称未提供',reference:[device.serial&&device.serial!==device.name?'SN：'+device.serial:'',device.model].filter(Boolean).join(' · '),category:'device',state:option?.state??'loading',reason:option?.reason??'等待范围检查',owners:[],root:'device',review:'devices'};
    });
    for(const facility of options?.facilities??[])roots.push({key:'facility:'+facility.id,id:facility.id,name:facility.label,reference:facility.id,category:facility.kind,state:facility.state,reason:facility.reason,owners:[],root:'facility',review:'facility'});
    return {rows:[...roots,...rows],names,dependencyCounts,deviceOptions};
  },[s.devices,s.ports,s.components,s.connections,s.racks,options]);
  const filteredRows=useMemo(()=>scopeRows.rows.filter(row=>{
    if(category==='all'&&!row.root)return false;
    if(category!=='all'&&row.category!==category)return false;
    if(change==='all')return true;
    if(change==='delta')return row.root?['new','changed'].includes(row.state):row.owners.some(id=>['new','changed'].includes(scopeRows.deviceOptions.get(id)?.state??''));
    return row.state===change;
  }),[scopeRows,category,change]);
  const isSelected=(row:ScopeRow)=>row.root==='device'?selectedDevices.has(row.id):row.root==='facility'?selectedFacilitySet.has(row.id):row.owners.some(id=>selectedDevices.has(id));
  return <div className="publication-page">
    <Heading eyebrow="PUBLISH & EVOLVE" title="入库与版本" description="统一选择新增或变化对象，关联依赖随范围核对；历史回执在右侧查看。" action={<Button kind="primary" disabled={optionsLoading||!!optionsError||evaluating||!!evaluationError||evaluation?.selectionKey!==selectionKey||evaluation?.revision!==s.revision||publishing||(!p.devices.length&&!activeFacilities.length)||!can('bom:execute')||!can('bom:review')} onClick={()=>{setExpected(s.revision);setFrozenScope({devices:[...activeSelected],facilities:[...activeFacilities],counts:{...p.counts},key:optionsKey});setCapabilityBaseline(Object.fromEntries(caps.map(c=>[c.id,c.available])));setConfirm(true);}}><Database size={16}/>确认本次入库</Button>}/>
    {optionsError&&<Notice tone="warning">{optionsError}<Button onClick={()=>setOptionsRetry(n=>n+1)}>重新检查入库范围</Button></Notice>}
    {evaluationError&&<Notice tone="warning">{evaluationError}</Notice>}
    {success&&<div className="publish-success"><CheckCircle2 size={23}/><div><b>本次范围已入库 · r{success.revision} · {success.counts.devices} 台设备 · {success.counts.facilities??0} 项设施</b><p>已确认数据进入 CMDB；本次 {success.counts.placements??0} 项精确落位、{success.counts.locations??0} 项已知位置关联。空间布局仍取决于已确认坐标与尺寸。</p></div><Button onClick={()=>go('overview')}>查看启动总览<ArrowRight size={14}/></Button><div className="publication-capability-delta">{caps.map(c=><span key={c.id}><b>{c.title}</b> {c.available}/{c.total} · 较确认前 {c.available-(capabilityBaseline[c.id]??c.available)>=0?'+':''}{c.available-(capabilityBaseline[c.id]??c.available)}</span>)}</div></div>}
    <div className="publish-layout"><div><Section title="本次确认范围" caption="设备、设施与关联依赖统一查看；默认展示新增和变化对象，包含信息可按类别查看。" action={<Button kind="secondary" className="publication-select-all" aria-busy={optionsLoading||evaluating} disabled={!valid.size&&!validFacilities.size||publishing||optionsLoading} onClick={selectAll}>{optionsLoading||evaluating?<Loader2 size={14} className="animate-spin"/>:<Check size={14}/>}<span>{allSelected?'取消全选':'全选可用增量'}</span>{(optionsLoading||evaluating)&&<small>· 核对中</small>}</Button>}>
      <div className="publication-scope-note"><ShieldCheck size={17}/><div><b>设备与设施可选择，关联对象随范围发布。</b><p>所选对象的端口、组件、连接和落位一起核对；无变化数据作为引用保留，不重复选择。缺失的布局、网络和电力信息可以后补。</p><p>{identityGuidance}</p></div></div>

      <div className="publication-selection"><span><b>已选 {activeSelected.length} 台设备 · {activeFacilities.length} 项设施</b></span><span>全选不受筛选限制，仅纳入可用增量</span>{selectedCount>0&&<Button kind="ghost" disabled={publishing} onClick={()=>{setSelected([]);setSelectedFacilities([]);}}>清空选择</Button>}<button className="text-link" onClick={()=>go('review')}>前往数据核对<ArrowRight size={13}/></button></div>
      {!selectedCount&&<div className="publication-empty-selection">仅新增或有变化且可用的设备、设施可选；无变化、阻断和处理中对象不会纳入全选。</div>}
      {dependencySections.length>0&&["port","component","connection"].includes(category)&&<div className="publication-dependency-loading" role="status"><span>依赖明细尚未完整载入：{dependencySections.map(key=>`${labels[key]} ${s[key].length.toLocaleString()} / ${s.totals?.[key]?.toLocaleString()}`).join(' · ')}。下表依赖数仅计已载入对象，不影响服务端范围核对。</span><Button disabled={dependenciesLoading||!api.loadSection} onClick={()=>void loadDependencies()}>{dependenciesLoading?<><Loader2 size={14} className="animate-spin"/>读取依赖中…</>:'读取完整依赖明细'}</Button></div>}
      <DataTable data={filteredRows} pageSize={25} searchPlaceholder="搜索对象名称、标识、型号或归属设备" toolbar={<div className="publication-scope-filters"><label>类别<select value={category} onChange={e=>setCategory(e.target.value)}><option value="all">待确认对象</option>{Object.entries(scopeCategories).map(([id,label])=><option key={id} value={id}>{label}</option>)}</select></label><label>变化<select value={change} onChange={e=>setChange(e.target.value)}><option value="delta">新增 / 变化</option><option value="all">全部状态</option>{Object.entries(scopeStates).filter(([id])=>id!=='loading').map(([id,label])=><option key={id} value={id}>{label}</option>)}</select></label></div>} columns={[
        {id:'select',header:()=> <input aria-label="全选全部可用增量（不受筛选限制）" type="checkbox" checked={allSelected} ref={node=>{if(node)node.indeterminate=!allSelected&&selectedCount>0;}} disabled={!valid.size&&!validFacilities.size||publishing||optionsLoading} onChange={selectAll}/>,size:44,enableSorting:false,cell:({row:{original:row}})=>row.root?<input type="checkbox" aria-label={'入库 '+row.name} disabled={publishing||optionsLoading||!(row.root==='device'?valid:validFacilities).has(row.id)} checked={isSelected(row)} onChange={e=>{const update=row.root==='device'?setSelected:setSelectedFacilities;update(ids=>e.target.checked?[...new Set([...ids,row.id])]:ids.filter(id=>id!==row.id));}}/>:<span className="publication-dependency-marker" title="依赖对象不可独立选择">随范围</span>},
        {id:'name',accessorFn:row=>`${row.name} ${row.reference}`,header:'对象',cell:({row:{original:row}})=><div className="table-name"><b>{row.name}</b><small>{row.reference}</small></div>},
        {id:'category',accessorFn:row=>scopeCategories[row.category]||row.category,header:'类别'},
        {id:'state',accessorFn:row=>scopeStates[row.state],header:'变化 / 状态',cell:({row:{original:row}})=><div className="publication-device-status"><Badge state={row.state==='blocked'?'blocked':row.state==='unchanged'?'published':row.state==='pending'?'pending':'needs_input'}>{optionsError&&row.state==='loading'?'范围检查失败':scopeStates[row.state]}</Badge><small>{row.reason}</small>{row.root==='device'&&latestByDevice.has(row.id)&&<small>最近记录 r{latestByDevice.get(row.id)!.revision}{publishedByDevice.has(row.id)?` · 已有回执 r${publishedByDevice.get(row.id)!.revision}`:''}</small>}</div>},
        {id:'dependency',accessorFn:row=>scopeRows.names(row.owners),header:'依赖 / 纳入方式',enableSorting:false,cell:({row:{original:row}})=>row.root?<div className="publication-scope-dependency">{row.root==='device'?`${scopeRows.dependencyCounts.get(row.id)??0} 项已载入依赖`:'祖先设施自动核对'}<small>{isSelected(row)?'已选入本次范围':'未选择'}</small></div>:<div className="publication-scope-dependency">{scopeRows.names(row.owners)||'端点归属待核对'}<small>{isSelected(row)?'关联设备已选 · 随范围核对':'关联设备未选'}{row.category==='connection'?' · 对端与引用由服务端核对':''}</small></div>},
        {id:'review',header:'核对',enableSorting:false,cell:({row:{original:row}})=><button className="text-link" onClick={()=>row.root==='facility'?go('models/facility'):inspect(row.category==='device'||row.category==='placement'?'device':row.category==='port'?'port':row.category==='component'?'component':'connection',row.id)}>查看<ArrowRight size={12}/></button>},
      ]}/>
    </Section></div>
    <aside><Section title="版本与增补记录" caption="按时间追溯每次确认及入库回执。" action={latest&&<Button kind="ghost" onClick={()=>void openDetail(latest)}><History size={14}/>最新记录</Button>}>{history.length?<div className="publication-history-list">{history.map(record=><button className="publication-history-entry" key={record.id} onClick={()=>void openDetail(record)}><div className="publication-history-heading"><b>r{record.revision}</b><Badge state={record.state}/>{record.id===current?.id&&<small>最近成功</small>}</div><div className="publication-history-meta"><time>{formatDate(record.createdAt)}</time><span title={record.actor}>确认人：{record.actor||'未记录'}</span></div><p className="publication-history-counts">{record.counts.devices} 台设备 · {record.counts.connections} 条连接 · {record.counts.facilities??0} 项设施</p><p className="publication-history-reason" title={record.reason}>{record.reason}</p><span className="publication-history-open">查看记录<ChevronRight size={13}/></span></button>)}</div>:<Empty title="暂无入库记录" description="首次确认后，版本与入库回执显示在这里。"/>}</Section>
    <Section title="下游可以开始什么"><div className="consumer-list">{caps.map(c=><button key={c.id} onClick={()=>go(c.href)}><div><b>{c.title}</b><Badge state={c.state}/></div><p>{c.detail}</p><span>{c.action}<ArrowRight size={13}/></span></button>)}</div></Section>
    <div className="quiet-note"><ShieldCheck size={17}/><p>入库建立规划事实，不代表设备已发现、网络已配置或装机已完成。</p></div></aside></div>
    {confirm&&frozenScope&&<Modal title="确认本次入库" description="冻结当前对象范围与输入版本，未纳入的数据继续保留。" close={()=>{if(!publishing)setConfirm(false);}}><div className="confirm-counts">{Object.entries(frozenScope.counts).map(([key,n])=><span key={key}><b>{n}</b>{labels[key]}</span>)}</div><Notice tone="success">{identityGuidance} 无需等待机房坐标、布线计算、网络参数或电力信息全部齐全。</Notice><Field label="本次说明"><textarea rows={3} value={reason} onChange={e=>setReason(e.target.value)}/></Field><p className="muted small-copy">确认后提交本次范围，并读取 CMDB 入库回执。</p>{frozenScope.key!==optionsKey&&<Notice tone="warning">工程数据或入库回执已变化，请返回核对最新范围。</Notice>}<div className="modal-actions"><Button disabled={publishing} onClick={()=>setConfirm(false)}>返回核对</Button><Button kind="primary" disabled={publishing||frozenScope.key!==optionsKey} onClick={async()=>{setPublishing(true);let result:Publication|undefined;if(await run('本次入库版本已建立',async()=>{result=await api.publish(frozenScope.devices,reason,expected,frozenScope.facilities);})){setSuccess(result);setConfirm(false);}setPublishing(false);}}>{publishing?'正在等待 CMDB 回执…':'确认入库'}<ArrowRight size={15}/></Button></div></Modal>}
    {detail&&<Modal title={`工程输入 r${detail.revision}`} description="同一资产身份上的可追溯增补。" close={()=>setDetail(undefined)} wide><KV values={[["确认人",detail.actor],["时间",formatDate(detail.createdAt)],["说明",detail.reason],["版本引用",detail.id]]}/><h3 className="detail-title">相对上一版本</h3>{(()=>{const prev=previousDetail;const added=detail.deviceIds.filter(id=>!prev?.deviceIds.includes(id));const changed=detail.deviceSnapshot.filter(d=>{const old=prev?.deviceSnapshot.find(x=>x.id===d.id);return old&&JSON.stringify(old)!==JSON.stringify(d);});const removed=prev?.deviceIds.filter(id=>!detail.deviceIds.includes(id))??[];return <div className="diff-summary"><span><b>+{added.length}</b>新增至本次范围</span><span><b>{changed.length}</b>设备字段更新</span><span><b>{removed.length}</b>本次范围外，保留历史</span></div>;})()}<div className="version-fields">{Object.entries(detail.modelVersions).map(([key,version])=><span key={key}><code>{key}</code><b>v{version}</b></span>)}</div><div className="modal-actions"><Button onClick={()=>downloadFile(JSON.stringify(detail,null,2),`工程输入-r${detail.revision}.json`,'application/json')}><Download size={14}/>导出版本记录</Button><Button onClick={()=>setDetail(undefined)}>关闭</Button></div></Modal>}
  </div>;
}
