import { useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { ArrowRight, Check, CheckCircle2, ChevronRight, Database, Download, GitCompareArrows, History, Layers, ShieldCheck } from 'lucide-react';
import { useWorkspace } from '../context';
import { capabilities, eligibleDevices, type Publication } from '../lib/api';
import { Badge, Button, DataTable, Empty, Field, Heading, KV, Modal, Notice, Section, formatDate } from '../components/ui';
import { downloadFile } from '../lib/api/workbook';

export default function Publish(){
  const {api,state:s,go,run}=useWorkspace();
  const [params]=useSearchParams();
  const [selected,setSelected]=useState<string[]>(()=>params.get('targets')?.split(',')??eligibleDevices(s).map(d=>d.id));
  const [confirm,setConfirm]=useState(false),[expected,setExpected]=useState(s.revision),[reason,setReason]=useState('确认设备身份与已知关系，未完成的工程信息后续增补');
  const [detail,setDetail]=useState<Publication>(),[success,setSuccess]=useState<Publication>();
  const p=api.previewPublication(selected);
  const labels:Record<string,string>={devices:'设备',ports:'端口',connections:'双端连接',components:'组件',placements:'精确落位'};
  const caps=capabilities(s);
  const valid=new Set(eligibleDevices(s).map(d=>d.id));
  const latest=s.publications.at(-1);
  return <>
    <Heading eyebrow="PUBLISH & EVOLVE" title="确认可用数据，开始最小开局" description="本次范围独立入库，缺口保留在原处，后续补充沿用同一资产身份。" action={<Button kind="primary" disabled={!p.devices.length} onClick={()=>{setExpected(s.revision);setConfirm(true);}}><Database size={16}/>确认本次入库</Button>}/>
    {success&&<div className="publish-success"><CheckCircle2 size={23}/><div><b>版本 r{success.revision} 已建立 · {success.counts.devices} 台设备</b><p>已确认数据进入当前资产视图，工程补充不会生成另一套身份。</p></div><Button onClick={()=>go('overview')}>查看启动总览<ArrowRight size={14}/></Button></div>}
    <div className="publish-layout"><div><Section title="本次确认范围" caption="选择设备时，自动包含它的组件、端口及双端均在范围内的连接。" action={<Button kind="ghost" onClick={()=>setSelected([...valid])}><Check size={14}/>选择全部可用设备</Button>}><div className="publish-counts">{Object.entries(p.counts).map(([key,n])=><div key={key}><strong>{n.toLocaleString()}</strong><span>{labels[key]}</span></div>)}</div><DataTable data={s.devices} columns={[
      {id:'select',header:'选择',size:45,enableSorting:false,cell:({row})=><input type="checkbox" aria-label={`入库 ${row.original.name}`} disabled={!valid.has(row.original.id)} checked={selected.includes(row.original.id)&&valid.has(row.original.id)} onChange={e=>setSelected(e.target.checked?[...selected,row.original.id]:selected.filter(id=>id!==row.original.id))}/>},
      {accessorKey:'name',header:'设备',cell:({row})=><div className="table-name"><b>{row.original.name}</b><small>{row.original.model}</small></div>},
      {accessorKey:'serial',header:'身份依据',cell:({getValue})=><code>{getValue()}</code>},
      {id:'status',header:'本次状态',cell:({row})=><Badge state={!valid.has(row.original.id)?'blocked':selected.includes(row.original.id)?'ready':'needs_input'}>{!valid.has(row.original.id)?'身份待确认':selected.includes(row.original.id)?'本次入库':'暂不纳入'}</Badge>},
    ]}/></Section><Section title="版本与增补记录" caption="增补创建新版本，保留旧记录和人工确认。" action={latest&&<Button kind="ghost" onClick={()=>setDetail(latest)}><GitCompareArrows size={14}/>查看最新差异</Button>}>{s.publications.length?<div className="version-list">{[...s.publications].reverse().map((v,i)=><button key={v.id} onClick={()=>setDetail(v)}><span className="version-icon"><Layers size={19}/></span><div><b>工程输入 r{v.revision}{i===0&&<span className="latest-tag">当前版本</span>}</b><p>{v.deviceIds.length} 设备 · {v.connectionIds.length} 连接 · {v.reason}</p></div><span>{formatDate(v.createdAt)}</span><ChevronRight size={16}/></button>)}</div>:<Empty title="首次入库后，版本从这里开始" description="后续补充按差异确认，已有设备身份保持稳定。"/>}</Section></div>
    <aside><Section title="入库条件"><div className="check-list">{['对象身份明确','关系端点可解析','本次范围有来源依据','未确认信息继续保留'].map(t=><div key={t}><CheckCircle2 size={16}/><span>{t}</span></div>)}</div><div className="held-note"><b>{p.held.length} 台设备暂未纳入</b><p>不会丢弃来源，不影响已选择对象入库。</p>{p.held.length>0&&<button className="text-link" onClick={()=>go('review?filter=issues')}>查看待确认项<ArrowRight size={13}/></button>}</div></Section>
    <Section title="下游可以开始什么"><div className="consumer-list">{caps.map(c=><button key={c.id} onClick={()=>go(c.href)}><div><b>{c.title}</b><Badge state={c.state}/></div><p>{c.detail}</p><span>{c.action}<ArrowRight size={13}/></span></button>)}</div></Section>
    <div className="quiet-note"><ShieldCheck size={17}/><p>入库建立规划事实，不代表设备已发现、网络已配置或装机已完成。</p></div></aside></div>
    {confirm&&<Modal title="确认本次入库" description="冻结当前对象范围与输入版本，未纳入的数据继续保留。" close={()=>setConfirm(false)}><div className="confirm-counts">{Object.entries(p.counts).map(([key,n])=><span key={key}><b>{n}</b>{labels[key]}</span>)}</div><Notice tone="success">无需等待机房坐标、布线计算、网络参数或电力信息全部齐全。</Notice><Field label="本次说明"><textarea rows={3} value={reason} onChange={e=>setReason(e.target.value)}/></Field><p className="muted small-copy">确认后提交本次范围，并读取 CMDB 入库回执。</p><div className="modal-actions"><Button onClick={()=>setConfirm(false)}>返回核对</Button><Button kind="primary" onClick={async()=>{let result:Publication|undefined;if(await run('本次入库版本已建立',async()=>{result=await api.publish(selected,reason,expected);})){setSuccess(result);setConfirm(false);}}}>确认入库<ArrowRight size={15}/></Button></div></Modal>}
    {detail&&<Modal title={`工程输入 r${detail.revision}`} description="同一资产身份上的可追溯增补。" close={()=>setDetail(undefined)} wide><KV values={[["确认人",detail.actor],["时间",formatDate(detail.createdAt)],["说明",detail.reason],["版本引用",detail.id]]}/><h3 className="detail-title">相对上一版本</h3>{(()=>{const prev=s.publications.find(p=>p.revision===detail.revision-1);const added=detail.deviceIds.filter(id=>!prev?.deviceIds.includes(id));const changed=detail.deviceSnapshot.filter(d=>{const old=prev?.deviceSnapshot.find(x=>x.id===d.id);return old&&JSON.stringify(old)!==JSON.stringify(d);});const removed=prev?.deviceIds.filter(id=>!detail.deviceIds.includes(id))??[];return <div className="diff-summary"><span><b>+{added.length}</b>新增至本次范围</span><span><b>{changed.length}</b>设备字段更新</span><span><b>{removed.length}</b>本次范围外，保留历史</span></div>;})()}<div className="version-fields">{Object.entries(detail.modelVersions).map(([key,version])=><span key={key}><code>{key}</code><b>v{version}</b></span>)}</div><div className="modal-actions"><Button onClick={()=>downloadFile(JSON.stringify(detail,null,2),`工程输入-r${detail.revision}.json`,'application/json')}><Download size={14}/>导出版本记录</Button><Button onClick={()=>setDetail(undefined)}>关闭</Button></div></Modal>}
  </>;
}
