import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { ArrowRight, Check, CheckCircle2, ChevronRight, Database, Download, History, ShieldCheck, Loader2 } from 'lucide-react';
import { useWorkspace } from '../context';
import { capabilities, eligibleDevices, type Publication, type Device } from '../lib/api';
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
  const valid=new Set(eligible.filter(d=>latestByDevice.get(d.id)?.state!=='pending').map(d=>d.id));
  const [publishing,setPublishing]=useState(false);
  const [selected,setSelected]=useState<string[]>(()=>(params.get('targets')?.split(',')??eligible.filter(d=>!publishedByDevice.has(d.id)).map(d=>d.id)).filter(id=>valid.has(id)));
  const [confirm,setConfirm]=useState(false),[expected,setExpected]=useState(s.revision),[reason,setReason]=useState('确认设备身份与已知关系，未完成的工程信息后续增补');
  const [detail,setDetail]=useState<Publication>(),[success,setSuccess]=useState<Publication>();
  const openDetail=async(v:Publication)=>{await run('版本明细已读取',async()=>{const full=await api.publicationDetail!(v.id);const previous=s.publications.find(p=>p.revision===v.revision-1);if(previous){const value=await api.publicationDetail!(previous.id);setPreviousDetail(value);}else setPreviousDetail(undefined);setDetail(full);});};
  const [previousDetail,setPreviousDetail]=useState<Publication>();
  useEffect(()=>{if(success)setSelected([]);},[success?.id]);
  useEffect(()=>{if(!publishing)setSelected(ids=>ids.filter(id=>valid.has(id)));},[s.revision,publishing]);
  const selectionKey=[...selected].sort().join(',');
  const local=api.previewPublication(selected);
  const [evaluation,setEvaluation]=useState<{revision:number;counts:Record<string,number>;selectionKey:string}>(),[evaluating,setEvaluating]=useState(true),[evaluationError,setEvaluationError]=useState('');
  useEffect(()=>{let active=true;setEvaluating(true);setEvaluationError('');const timer=setTimeout(()=>{if(!selected.length){if(active){setEvaluation(undefined);setEvaluating(false);}return;}void api.evaluatePublication!(selected,s.revision).then(v=>{if(active)setEvaluation({...v,selectionKey});}).catch(e=>{if(active)setEvaluationError(e.message);}).finally(()=>{if(active)setEvaluating(false);});},150);return()=>{active=false;clearTimeout(timer);};},[api,selected,s.revision]);
  const p={...local,counts:evaluation?.revision===s.revision&&evaluation.selectionKey===selectionKey?evaluation.counts:local.counts};
  const labels:Record<string,string>={devices:'设备',ports:'端口',connections:'双端连接',components:'组件',placements:'精确落位'};
  const caps=s.capabilities??capabilities(s);
  const latest=s.publications.at(-1);
  const current=s.publications.filter(p=>p.state==='published').at(-1);
  const allSelected=valid.size>0&&[...valid].every(id=>selected.includes(id));
  const selectedCount=selected.filter(id=>valid.has(id)).length;
  const deviceName=(d:Device)=>d.name||d.serial||'名称未提供';
  const typeLabel=(type:string)=>({server:'服务器',switch:'交换机'}[type]||'类型待补充');
  const selectAll=()=>setSelected(allSelected?[]:[...valid]);
  return <div className="publication-page">
    <Heading eyebrow="PUBLISH & EVOLVE" title="入库与版本" description="选择本次设备范围，确认入库；历史回执与增补记录在右侧查看。" action={<Button kind="primary" disabled={evaluating||!!evaluationError||publishing||!p.devices.length||!can('bom:execute')||!can('bom:review')} onClick={()=>{setExpected(s.revision);setConfirm(true);}}><Database size={16}/>确认本次入库</Button>}/>
    {evaluationError&&<Notice tone="warning">{evaluationError}</Notice>}
    {success&&<div className="publish-success"><CheckCircle2 size={23}/><div><b>本次范围已入库 · r{success.revision} · {success.counts.devices} 台设备</b><p>已确认数据进入当前资产视图，工程补充不会生成另一套身份。</p></div><Button onClick={()=>go('overview')}>查看启动总览<ArrowRight size={14}/></Button></div>}
    <div className="publish-layout"><div><Section title="本次确认范围" caption="以设备为单位选择，关联对象自动随本次范围一同确认。" action={<Button kind="secondary" className="publication-select-all" aria-label={allSelected?'取消全选':'选择全部可入库设备'} aria-busy={evaluating&&selected.length>0} disabled={!valid.size||publishing} onClick={selectAll}>{evaluating&&selected.length>0?<Loader2 size={14} className="animate-spin"/>:<Check size={14}/>}<span>{allSelected?'取消全选':'选择全部可入库设备'}</span>{evaluating&&selected.length>0&&<small>· 核对中</small>}</Button>}>
      <div className="publication-scope-note"><ShieldCheck size={17}/><div><b>确认身份、来源与关系端点，已知数据可以先入库。</b><p>自动包含所选设备的组件、端口，以及双端均在范围内的连接。机房坐标、布线、网络和电力信息可后补；未选设备及原始资料继续保留。</p></div></div>
      <div className="publish-counts">{Object.entries(p.counts).map(([key,n])=><div key={key}><strong>{selected.length&&(evaluating||evaluation?.selectionKey!==selectionKey)?'—':n.toLocaleString()}</strong><span>{labels[key]}</span></div>)}</div>
      <div className="publication-selection"><span><b>已选择 {selectedCount} 台</b> / 共 {s.devices.length} 台设备</span><span>{p.held.length} 台暂未纳入</span>{selected.length>0&&<Button kind="ghost" disabled={publishing} onClick={()=>setSelected([])}>清空选择</Button>}<button className="text-link" onClick={()=>go('review')}>前往数据核对<ArrowRight size={13}/></button></div>
      {!selected.length&&s.devices.length>0&&<div className="publication-empty-selection">请选择本次需要入库的设备。已有回执的设备不默认勾选，增补时可按需选择。</div>}
      <DataTable data={s.devices} searchPlaceholder="搜索设备名称、SN 或型号" columns={[
        {id:'select',header:()=> <input aria-label="选择全部可入库设备" type="checkbox" checked={allSelected} aria-checked={allSelected?true:selectedCount>0?'mixed':false} ref={node=>{if(node)node.indeterminate=!allSelected&&selectedCount>0;}} disabled={!valid.size||publishing} onChange={selectAll}/>,size:44,enableSorting:false,cell:({row})=><input type="checkbox" aria-label={`入库 ${deviceName(row.original)}`} disabled={!valid.has(row.original.id)||publishing} checked={selected.includes(row.original.id)&&valid.has(row.original.id)} onChange={e=>setSelected(e.target.checked?[...new Set([...selected,row.original.id])]:selected.filter(id=>id!==row.original.id))}/>},
        {id:'name',accessorFn:(device:Device)=>`${deviceName(device)} ${device.serial}`,header:'设备',cell:({row})=><div className="table-name"><b>{deviceName(row.original)}</b>{row.original.serial&&row.original.serial!==row.original.name&&<small>SN：{row.original.serial}</small>}</div>},
        {accessorKey:'type',header:'类型',cell:({getValue})=>typeLabel(getValue())},
        {accessorKey:'model',header:'型号',cell:({getValue})=>getValue()||'待补充'},
        {id:'publication',header:'入库情况',cell:({row})=>{const record=latestByDevice.get(row.original.id),published=publishedByDevice.get(row.original.id);const blocked=!eligible.some(d=>d.id===row.original.id);return <div className="publication-device-status"><Badge state={blocked?'blocked':record?.state??'needs_input'}>{blocked?'身份待确认':record?.state==='published'?'已入库':record?.state==='pending'?'等待入库回执':record?.state==='failed'?'入库失败':'尚未入库'}</Badge>{record&&<small>最近记录 r{record.revision}{published&&published.id!==record.id?` · 已有回执 r${published.revision}`:''}</small>}</div>;}},
        {id:'review',header:'核对',enableSorting:false,cell:({row})=><button className="text-link" onClick={()=>go('review?category=devices&q='+encodeURIComponent(deviceName(row.original)))}>查看<ArrowRight size={12}/></button>},
      ]}/>
    </Section></div>
    <aside><Section title="版本与增补记录" caption="按时间追溯每次确认及入库回执。" action={latest&&<Button kind="ghost" onClick={()=>void openDetail(latest)}><History size={14}/>最新记录</Button>}>{history.length?<div className="publication-history-list">{history.map(record=><button className="publication-history-entry" key={record.id} onClick={()=>void openDetail(record)}><div className="publication-history-heading"><b>r{record.revision}</b><Badge state={record.state}/>{record.id===current?.id&&<small>最近成功</small>}</div><div className="publication-history-meta"><time>{formatDate(record.createdAt)}</time><span title={record.actor}>确认人：{record.actor||'未记录'}</span></div><p className="publication-history-counts">{record.counts.devices} 台设备 · {record.counts.connections} 条连接</p><p className="publication-history-reason" title={record.reason}>{record.reason}</p><span className="publication-history-open">查看记录<ChevronRight size={13}/></span></button>)}</div>:<Empty title="暂无入库记录" description="首次确认后，版本与入库回执显示在这里。"/>}</Section>
    <Section title="下游可以开始什么"><div className="consumer-list">{caps.map(c=><button key={c.id} onClick={()=>go(c.href)}><div><b>{c.title}</b><Badge state={c.state}/></div><p>{c.detail}</p><span>{c.action}<ArrowRight size={13}/></span></button>)}</div></Section>
    <div className="quiet-note"><ShieldCheck size={17}/><p>入库建立规划事实，不代表设备已发现、网络已配置或装机已完成。</p></div></aside></div>
    {confirm&&<Modal title="确认本次入库" description="冻结当前对象范围与输入版本，未纳入的数据继续保留。" close={()=>setConfirm(false)}><div className="confirm-counts">{Object.entries(p.counts).map(([key,n])=><span key={key}><b>{n}</b>{labels[key]}</span>)}</div><Notice tone="success">无需等待机房坐标、布线计算、网络参数或电力信息全部齐全。</Notice><Field label="本次说明"><textarea rows={3} value={reason} onChange={e=>setReason(e.target.value)}/></Field><p className="muted small-copy">确认后提交本次范围，并读取 CMDB 入库回执。</p><div className="modal-actions"><Button onClick={()=>setConfirm(false)}>返回核对</Button><Button kind="primary" disabled={publishing} onClick={async()=>{setPublishing(true);let result:Publication|undefined;if(await run('本次入库版本已建立',async()=>{result=await api.publish(selected,reason,expected);})){setSuccess(result);setConfirm(false);}setPublishing(false);}}>{publishing?'正在等待 CMDB 回执…':'确认入库'}<ArrowRight size={15}/></Button></div></Modal>}
    {detail&&<Modal title={`工程输入 r${detail.revision}`} description="同一资产身份上的可追溯增补。" close={()=>setDetail(undefined)} wide><KV values={[["确认人",detail.actor],["时间",formatDate(detail.createdAt)],["说明",detail.reason],["版本引用",detail.id]]}/><h3 className="detail-title">相对上一版本</h3>{(()=>{const prev=previousDetail;const added=detail.deviceIds.filter(id=>!prev?.deviceIds.includes(id));const changed=detail.deviceSnapshot.filter(d=>{const old=prev?.deviceSnapshot.find(x=>x.id===d.id);return old&&JSON.stringify(old)!==JSON.stringify(d);});const removed=prev?.deviceIds.filter(id=>!detail.deviceIds.includes(id))??[];return <div className="diff-summary"><span><b>+{added.length}</b>新增至本次范围</span><span><b>{changed.length}</b>设备字段更新</span><span><b>{removed.length}</b>本次范围外，保留历史</span></div>;})()}<div className="version-fields">{Object.entries(detail.modelVersions).map(([key,version])=><span key={key}><code>{key}</code><b>v{version}</b></span>)}</div><div className="modal-actions"><Button onClick={()=>downloadFile(JSON.stringify(detail,null,2),`工程输入-r${detail.revision}.json`,'application/json')}><Download size={14}/>导出版本记录</Button><Button onClick={()=>setDetail(undefined)}>关闭</Button></div></Modal>}
  </div>;
}
