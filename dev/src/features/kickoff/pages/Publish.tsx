import { useEffect, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { ArrowRight, Check, CheckCircle2, ChevronRight, Database, Download, History, ShieldCheck, Loader2 } from 'lucide-react';
import { useWorkspace } from '../context';
import { capabilities, eligibleDevices, type Publication, type Device, type PublicationOptions } from '../lib/api';
import { Badge, Button, DataTable, Empty, Field, Heading, KV, Modal, Notice, Section, formatDate } from '../components/ui';
import { downloadFile } from '../lib/api/workbook';

export default function Publish(){
  const {api,state:s,go,run,can}=useWorkspace();
  const [params]=useSearchParams();
  useEffect(()=>{const id=params.get('publication');if(!id)return;void run('版本明细已读取',async()=>{const full=await api.publicationDetail!(id);const previous=s.publications.find(p=>p.revision===full.revision-1);setPreviousDetail(previous?await api.publicationDetail!(previous.id):undefined);setDetail(full);});},[params.get('publication')]);
  const history=[...s.publications].sort((a,b)=>b.revision-a.revision);
  const latestByDevice=new Map<string,Publication>(),publishedByDevice=new Map<string,Publication>();
  for(const record of history)for(const id of record.deviceIds){if(!latestByDevice.has(id))latestByDevice.set(id,record);if(record.state==='published'&&!publishedByDevice.has(id))publishedByDevice.set(id,record);}
  const eligible=eligibleDevices(s);
  const publicationKey=s.publications.map(v=>v.id+':'+v.state).join('|');
  const optionsKey=`${s.revision}:${publicationKey}`;
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
  const [confirm,setConfirm]=useState(false),[expected,setExpected]=useState(s.revision),[reason,setReason]=useState('确认设备身份与已知关系，未完成的工程信息后续增补');
  const [detail,setDetail]=useState<Publication>(),[success,setSuccess]=useState<Publication>();
  const openDetail=async(v:Publication)=>{await run('版本明细已读取',async()=>{const full=await api.publicationDetail!(v.id);const previous=s.publications.find(p=>p.revision===v.revision-1);if(previous){const value=await api.publicationDetail!(previous.id);setPreviousDetail(value);}else setPreviousDetail(undefined);setDetail(full);});};
  const [capabilityBaseline,setCapabilityBaseline]=useState<Record<string,number>>({});
  const [previousDetail,setPreviousDetail]=useState<Publication>();
  useEffect(()=>{let active=true;setOptionsError('');void api.publicationOptions(s.revision).then(value=>{if(!active)return;setOptionResult({key:optionsKey,value});if(!initialized.current){initialized.current=true;const ready=new Set(value.devices.filter(v=>v.state==='new'||v.state==='changed').map(v=>v.id));setSelectedFacilities((value.facilities??[]).filter(f=>f.state==='new'||f.state==='changed').map(f=>f.id));setSelected((params.get('targets')?.split(',')??[...ready]).filter(id=>ready.has(id)));}}).catch(e=>{if(active)setOptionsError(e.message);});return()=>{active=false;};},[api,optionsKey,optionsRetry]);
  useEffect(()=>{if(success){setSelected([]);setSelectedFacilities([]);}},[success?.id]);
  useEffect(()=>{if(options&&!publishing)setSelected(ids=>ids.filter(id=>valid.has(id)));},[options,publishing]);
  const selectionKey=[...selected].sort().join(',')+'|'+[...activeFacilities].sort().join(',');
  const activeSelected=selected.filter(id=>valid.has(id));
  const local=api.previewPublication(activeSelected);
  const [evaluation,setEvaluation]=useState<{revision:number;counts:Record<string,number>;selectionKey:string}>(),[evaluating,setEvaluating]=useState(true),[evaluationError,setEvaluationError]=useState('');
  useEffect(()=>{let active=true;setEvaluating(true);setEvaluationError('');const timer=setTimeout(()=>{if(!options||(!activeSelected.length&&!activeFacilities.length)){if(active){setEvaluation(undefined);setEvaluating(false);}return;}void api.evaluatePublication!(activeSelected,s.revision,activeFacilities).then(v=>{if(active)setEvaluation({...v,selectionKey});}).catch(e=>{if(active)setEvaluationError(e.message);}).finally(()=>{if(active)setEvaluating(false);});},150);return()=>{active=false;clearTimeout(timer);};},[api,selected,selectedFacilities,s.revision,options]);
  const p={...local,counts:evaluation?.revision===s.revision&&evaluation.selectionKey===selectionKey?evaluation.counts:local.counts};
  const labels:Record<string,string>={devices:'设备',ports:'端口',connections:'双端连接',components:'组件',placements:'精确落位',facilities:'机房与机柜',locations:'已知位置关联'};
  const caps=s.capabilities??capabilities(s);
  const latest=s.publications.at(-1);
  const current=s.publications.filter(p=>p.state==='published').at(-1);
  const allSelected=valid.size>0&&[...valid].every(id=>selected.includes(id));
  const selectedCount=selected.filter(id=>valid.has(id)).length;
  const deviceName=(d:Device)=>d.name||d.serial||'名称未提供';
  const typeLabel=(type:string)=>({server:'服务器',switch:'交换机'}[type]||'类型待补充');
  const selectAll=()=>setSelected(allSelected?[]:[...valid]);
  return <div className="publication-page">
    <Heading eyebrow="PUBLISH & EVOLVE" title="入库与版本" description="选择本次设备范围，确认入库；历史回执与增补记录在右侧查看。" action={<Button kind="primary" disabled={evaluating||!!evaluationError||publishing||(!p.devices.length&&!activeFacilities.length)||!can('bom:execute')||!can('bom:review')} onClick={()=>{setExpected(s.revision);setCapabilityBaseline(Object.fromEntries(caps.map(c=>[c.id,c.available])));setConfirm(true);}}><Database size={16}/>确认本次入库</Button>}/>
    {optionsError&&<Notice tone="warning">{optionsError}<Button onClick={()=>setOptionsRetry(n=>n+1)}>重新检查可入库设备</Button></Notice>}
    {evaluationError&&<Notice tone="warning">{evaluationError}</Notice>}
    {success&&<div className="publish-success"><CheckCircle2 size={23}/><div><b>本次范围已入库 · r{success.revision} · {success.counts.devices} 台设备 · {success.counts.facilities??0} 项设施</b><p>已确认数据进入 CMDB；本次 {success.counts.placements??0} 项精确落位、{success.counts.locations??0} 项已知位置关联。空间布局仍取决于已确认坐标与尺寸。</p></div><Button onClick={()=>go('overview')}>查看启动总览<ArrowRight size={14}/></Button><div className="publication-capability-delta">{caps.map(c=><span key={c.id}><b>{c.title}</b> {c.available}/{c.total} · 较确认前 {c.available-(capabilityBaseline[c.id]??c.available)>=0?'+':''}{c.available-(capabilityBaseline[c.id]??c.available)}</span>)}</div></div>}
    <div className="publish-layout"><div><Section title="本次确认范围" caption="以设备为单位选择，关联对象自动随本次范围一同确认。" action={<Button kind="secondary" className="publication-select-all" aria-label={allSelected?'取消全选':'选择全部可入库设备'} aria-busy={optionsLoading||evaluating&&selected.length>0} disabled={!valid.size||publishing||optionsLoading} onClick={selectAll}>{optionsLoading||evaluating&&selected.length>0?<Loader2 size={14} className="animate-spin"/>:<Check size={14}/>}<span>{allSelected?'取消全选':'选择全部可入库设备'}</span>{(optionsLoading||evaluating&&selected.length>0)&&<small>· 核对中</small>}</Button>}>
      <div className="publication-scope-note"><ShieldCheck size={17}/><div><b>确认身份、来源与关系端点，已知数据可以先入库。</b><p>自动包含所选设备的组件、端口和可确认连接；对端已在同项目 CMDB 时直接引用，未入库的端点会提示补充范围。机房与机柜可独立选择，坐标、布线、网络和电力信息可后补。</p></div></div>
      <div className="publish-counts">{Object.entries(p.counts).map(([key,n])=><div key={key}><strong>{(selected.length||activeFacilities.length)&&(evaluating||evaluation?.selectionKey!==selectionKey)?'—':n.toLocaleString()}</strong><span>{labels[key]}</span></div>)}</div>
      <div className="publication-selection"><span><b>已选择 {selectedCount} 台</b> / 共 {s.devices.length} 台设备</span><span>{p.held.length} 台暂未纳入</span>{selected.length>0&&<Button kind="ghost" disabled={publishing} onClick={()=>setSelected([])}>清空选择</Button>}<button className="text-link" onClick={()=>go('review')}>前往数据核对<ArrowRight size={13}/></button></div>
      {!selected.length&&s.devices.length>0&&<div className="publication-empty-selection">仅未入库或有实际增补变化的设备可选；已同步且无变化的设备不参与全选。</div>}
      <DataTable data={s.devices} searchPlaceholder="搜索设备名称、SN 或型号" columns={[
        {id:'select',header:()=> <input aria-label="选择全部可入库设备" type="checkbox" checked={allSelected} aria-checked={allSelected?true:selectedCount>0?'mixed':false} ref={node=>{if(node)node.indeterminate=!allSelected&&selectedCount>0;}} disabled={!valid.size||publishing} onChange={selectAll}/>,size:44,enableSorting:false,cell:({row})=><input type="checkbox" aria-label={`入库 ${deviceName(row.original)}`} disabled={!valid.has(row.original.id)||publishing||optionsLoading} checked={selected.includes(row.original.id)&&valid.has(row.original.id)} onChange={e=>setSelected(e.target.checked?[...new Set([...selected,row.original.id])]:selected.filter(id=>id!==row.original.id))}/>},
        {id:'name',accessorFn:(device:Device)=>`${deviceName(device)} ${device.serial}`,header:'设备',cell:({row})=><div className="table-name"><b>{deviceName(row.original)}</b>{row.original.serial&&row.original.serial!==row.original.name&&<small>SN：{row.original.serial}</small>}</div>},
        {accessorKey:'type',header:'类型',cell:({getValue})=>typeLabel(getValue())},
        {accessorKey:'model',header:'型号',cell:({getValue})=>getValue()||'待补充'},
        {id:'publication',header:'入库情况',cell:({row})=>{const record=latestByDevice.get(row.original.id),published=publishedByDevice.get(row.original.id);const option=optionByDevice.get(row.original.id);return <div className="publication-device-status"><Badge state={option?.state==='blocked'?'blocked':option?.state==='unchanged'?'published':option?.state==='pending'?'pending':'needs_input'}>{!option?'正在检查':option.state==='unchanged'?'已同步 · 无变化':option.state==='changed'?'已入库 · 待增补':option.reason}</Badge>{record&&<small>最近记录 r{record.revision}{published&&published.id!==record.id?` · 已有回执 r${published.revision}`:''}</small>}</div>;}},
        {id:'review',header:'核对',enableSorting:false,cell:({row})=><button className="text-link" onClick={()=>go('review?category=devices&q='+encodeURIComponent(row.original.serial||deviceName(row.original)))}>查看<ArrowRight size={12}/></button>},
      ]}/>
    </Section></div>
    <aside><Section title="机房与机柜增量" caption="设施可独立入库，无需重新选择无变化设备。" action={<Button kind="secondary" disabled={optionsLoading||publishing||!validFacilities.size} onClick={()=>setSelectedFacilities(activeFacilities.length===validFacilities.size?[]:[...validFacilities])}>{activeFacilities.length===validFacilities.size&&validFacilities.size?'取消全选':'选择设施增量'}</Button>}>
      <DataTable data={options?.facilities??[]} pageSize={5} searchPlaceholder="搜索机房或机柜" columns={[
       {id:'select',header:'选择',enableSorting:false,cell:({row})=><input type="checkbox" aria-label={'入库设施 '+row.original.label} disabled={!validFacilities.has(row.original.id)||publishing} checked={activeFacilities.includes(row.original.id)} onChange={e=>setSelectedFacilities(ids=>e.target.checked?[...ids,row.original.id]:ids.filter(id=>id!==row.original.id))}/>},
       {accessorKey:'label',header:'设施'},
       {id:'kind',header:'类型',cell:({row})=>({site:'场地',room:'机房',rack:'机柜'}[row.original.kind]||row.original.kind)},
       {accessorKey:'reason',header:'本次变化'},
      ]}/><p className="muted small-copy">已选择 {activeFacilities.length} 项设施；关联祖先由服务端核对纳入。缺少空间参数不阻断已知身份及归属入库。</p>
    </Section><Section title="版本与增补记录" caption="按时间追溯每次确认及入库回执。" action={latest&&<Button kind="ghost" onClick={()=>void openDetail(latest)}><History size={14}/>最新记录</Button>}>{history.length?<div className="publication-history-list">{history.map(record=><button className="publication-history-entry" key={record.id} onClick={()=>void openDetail(record)}><div className="publication-history-heading"><b>r{record.revision}</b><Badge state={record.state}/>{record.id===current?.id&&<small>最近成功</small>}</div><div className="publication-history-meta"><time>{formatDate(record.createdAt)}</time><span title={record.actor}>确认人：{record.actor||'未记录'}</span></div><p className="publication-history-counts">{record.counts.devices} 台设备 · {record.counts.connections} 条连接 · {record.counts.facilities??0} 项设施</p><p className="publication-history-reason" title={record.reason}>{record.reason}</p><span className="publication-history-open">查看记录<ChevronRight size={13}/></span></button>)}</div>:<Empty title="暂无入库记录" description="首次确认后，版本与入库回执显示在这里。"/>}</Section>
    <Section title="下游可以开始什么"><div className="consumer-list">{caps.map(c=><button key={c.id} onClick={()=>go(c.href)}><div><b>{c.title}</b><Badge state={c.state}/></div><p>{c.detail}</p><span>{c.action}<ArrowRight size={13}/></span></button>)}</div></Section>
    <div className="quiet-note"><ShieldCheck size={17}/><p>入库建立规划事实，不代表设备已发现、网络已配置或装机已完成。</p></div></aside></div>
    {confirm&&<Modal title="确认本次入库" description="冻结当前对象范围与输入版本，未纳入的数据继续保留。" close={()=>setConfirm(false)}><div className="confirm-counts">{Object.entries(p.counts).map(([key,n])=><span key={key}><b>{n}</b>{labels[key]}</span>)}</div><Notice tone="success">无需等待机房坐标、布线计算、网络参数或电力信息全部齐全。</Notice><Field label="本次说明"><textarea rows={3} value={reason} onChange={e=>setReason(e.target.value)}/></Field><p className="muted small-copy">确认后提交本次范围，并读取 CMDB 入库回执。</p><div className="modal-actions"><Button onClick={()=>setConfirm(false)}>返回核对</Button><Button kind="primary" disabled={publishing} onClick={async()=>{setPublishing(true);let result:Publication|undefined;if(await run('本次入库版本已建立',async()=>{result=await api.publish(activeSelected,reason,expected,activeFacilities);})){setSuccess(result);setConfirm(false);}setPublishing(false);}}>{publishing?'正在等待 CMDB 回执…':'确认入库'}<ArrowRight size={15}/></Button></div></Modal>}
    {detail&&<Modal title={`工程输入 r${detail.revision}`} description="同一资产身份上的可追溯增补。" close={()=>setDetail(undefined)} wide><KV values={[["确认人",detail.actor],["时间",formatDate(detail.createdAt)],["说明",detail.reason],["版本引用",detail.id]]}/><h3 className="detail-title">相对上一版本</h3>{(()=>{const prev=previousDetail;const added=detail.deviceIds.filter(id=>!prev?.deviceIds.includes(id));const changed=detail.deviceSnapshot.filter(d=>{const old=prev?.deviceSnapshot.find(x=>x.id===d.id);return old&&JSON.stringify(old)!==JSON.stringify(d);});const removed=prev?.deviceIds.filter(id=>!detail.deviceIds.includes(id))??[];return <div className="diff-summary"><span><b>+{added.length}</b>新增至本次范围</span><span><b>{changed.length}</b>设备字段更新</span><span><b>{removed.length}</b>本次范围外，保留历史</span></div>;})()}<div className="version-fields">{Object.entries(detail.modelVersions).map(([key,version])=><span key={key}><code>{key}</code><b>v{version}</b></span>)}</div><div className="modal-actions"><Button onClick={()=>downloadFile(JSON.stringify(detail,null,2),`工程输入-r${detail.revision}.json`,'application/json')}><Download size={14}/>导出版本记录</Button><Button onClick={()=>setDetail(undefined)}>关闭</Button></div></Modal>}
  </div>;
}
