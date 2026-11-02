import { useState } from 'react';
import { ArrowRight, Calculator, Download, FileCode2, Play, Route, Settings2 } from 'lucide-react';
import { useWorkspace } from '../context';
import { calculationDigest, rowsFor, type CalculationRow } from '../lib/api';
import { downloadFile } from '../lib/api/workbook';
import { Badge, Button, DataTable, Empty, Field, KV, Modal, Notice, Section, formatDate } from './ui';

export function CalculationPanel({domain,edit}:{domain:'cabling'|'power';edit:(id:string)=>void}){
  const {api,state:s,run}=useWorkspace();
  const result=s.calculations.filter(c=>c.domain===domain).at(-1);
  const stale=result&&result.inputDigest!==calculationDigest(s,domain);
  const [detail,setDetail]=useState<CalculationRow>(),[quick,setQuick]=useState(false);
  const distribution=new Map<number,number>();result?.rows.forEach(r=>{if(r.stockM!==null)distribution.set(r.stockM,(distribution.get(r.stockM)??0)+1);});
  return <Section className="calculation-section" title={domain==='cabling'?'确定性布线计算':'机柜负荷与容量计算'} caption={domain==='cabling'?'使用明确通道与参数，结果保留完整计算过程。':'只汇总明确的规划功率，缺失不按零计算。'} action={<><Button onClick={()=>domain==='cabling'?setQuick(true):edit('power_profiles')}><Settings2 size={14}/>{domain==='cabling'?'快速配置路径':'补充功率依据'}</Button><Button kind="primary" onClick={()=>run('计算结果已生成',()=>api.calculate(domain))}><Play size={14}/>{result?'重新计算':'运行计算'}</Button></>}>
    {domain==='cabling'?<div className="formula-strip"><span><small>路径算法</small><b>允许通道最短路径</b></span><i>+</i><span><small>双端参数</small><b>机柜引线</b></span><i>+</i><span><small>项目规则</small><b>固定 / 比例余量</b></span><i>=</i><span className="formula-result"><small>计算产物</small><b>需求长度 → 标准规格</b></span></div>:<div className="formula-strip"><span><small>明确规划依据</small><b>设备功率逐柜求和</b></span><i>→</i><span><small>明确容量依据</small><b>机柜可用容量 − 负荷</b></span><i>=</i><span className="formula-result"><small>计算产物</small><b>负荷与剩余容量</b></span></div>}
    {!result?<Empty title={domain==='cabling'?'连接已识别，可以按范围补充计算条件':'已有功率依据可以先计算'} description={domain==='cabling'?'路径、接入点、余量或规格缺失只影响对应连接。':'资料不完整的机柜会单独列出，不阻断其他机柜。'} action={<Button onClick={()=>domain==='cabling'?setQuick(true):edit('power_profiles')}>{domain==='cabling'?'配置第一组布线路径':'查看设备功率'}<ArrowRight size={14}/></Button>}/>:<>
      <div className="calculation-summary"><Badge state={stale?'stale':result.status}/><b>{domain==='cabling'?`${result.rows.length} 条线长结果`:`${result.power.length} 个机柜结果`}</b><span>{formatDate(result.createdAt)}</span><Button kind="ghost" onClick={()=>downloadFile(JSON.stringify(result,null,2),`${domain}-计算结果.json`,'application/json')}><Download size={14}/>导出结果</Button></div>
      {stale&&<Notice tone="warning">输入已改变。以下保留上次结果，重新计算后才能采用新结果。</Notice>}
      {result.issues.length>0&&<details className="calculation-issues" open={!result.rows.length&&domain==='cabling'}><summary>{result.issues.length} 项计算条件或结果需处理</summary>{result.issues.slice(0,25).map((issue,i)=><p key={i}>{issue}</p>)}{result.issues.length>25&&<p>其余 {result.issues.length-25} 项包含在导出结果中。</p>}</details>}
      {domain==='cabling'&&result.rows.length>0&&<><DataTable data={result.rows} columns={[{accessorKey:'from',header:'本端设备'},{accessorKey:'to',header:'对端设备'},{accessorKey:'pathM',header:'路径 m'},{accessorKey:'calculatedM',header:'计算需求 m',cell:({getValue})=><b>{getValue()}</b>},{accessorKey:'stockM',header:'标准长度 m',cell:({getValue})=>getValue()??'待匹配'},{id:'state',header:'数据状态',cell:()=><Badge state="calculation">Calculated</Badge>}]} onRow={setDetail} pageSize={6}/><div className="length-distribution"><span>标准线长分布</span>{[...distribution].sort((a,b)=>a[0]-b[0]).map(([length,count])=><div key={length}><b>{length}m</b><span>{count} 根</span></div>)}</div></>}
      {domain==='power'&&<DataTable data={result.power} columns={[{accessorKey:'rackId',header:'机柜',cell:({getValue})=>s.racks.find(r=>r.id===getValue())?.name??getValue()},{accessorKey:'loadKW',header:'规划负荷 kW',cell:({getValue})=>getValue()??'功率资料不全'},{accessorKey:'capacityKW',header:'可用容量 kW',cell:({getValue})=>getValue()??'待补充'},{accessorKey:'remainingKW',header:'剩余容量 kW',cell:({getValue})=>getValue()===null?'待计算':<b className={getValue()<0?'danger-text':''}>{getValue()}</b>}]} pageSize={6}/>}
    </>}
    {detail&&result&&<Modal title="线长计算过程" description={`${detail.from} → ${detail.to}`} close={()=>setDetail(undefined)}><div className="path-sequence">{detail.route.map((node,i)=><span key={i}>{i>0&&<ArrowRight size={13}/>}<code>{node}</code></span>)}</div><KV values={[["明确通道距离",`${detail.pathM} m`],["双端引线",`${detail.leadM} m`],["规则余量",`${detail.slackM} m`],["计算需求",`${detail.calculatedM} m`],["匹配标准规格",detail.stockM===null?'待匹配':`${detail.stockM} m`],["计算记录",result.id]]}/><Notice>该值为计算需求，不写入实际安装长度。</Notice><div className="modal-actions"><Button onClick={()=>setDetail(undefined)}>关闭</Button></div></Modal>}
    {quick&&<QuickRoute close={()=>setQuick(false)}/>}
  </Section>;
}

function QuickRoute({close}:{close:()=>void}) {
  const {api,state:s,run}=useWorkspace();
  const [a,setA]=useState(s.racks[0]?.id??''),[b,setB]=useState(s.racks[1]?.id??'');
  const [distance,setDistance]=useState(''),[lead,setLead]=useState(''),[slack,setSlack]=useState(''),[stock,setStock]=useState(''),[basis,setBasis]=useState(''),[medium,setMedium]=useState('');
  const apply=async()=>{
    if(!a||!b||a===b||!distance||Number(distance)<=0||lead===''||slack===''||Number(lead)<0||Number(slack)<0||!basis.trim()||!medium)throw Error('请填写不同的双端机柜、正路径距离、引线、余量、线缆规格与依据。');
    const lengths=stock.split(/[,，\s]+/).filter(Boolean).map(Number);
    if(!lengths.length||lengths.some(n=>!Number.isFinite(n)||n<=0))throw Error('请填写有效的标准长度，例如以逗号分隔的正数。');
    const merge=async(id:string,add:Record<string,string>[],key:string)=>{const old=rowsFor(s,id);await api.saveModel(id,[...old.filter(r=>!add.some(a=>a[key]===r[key])),...add],'页面路径建模');};
    await merge('route_nodes',[{code:'entry-'+a,room_id:s.racks.find(r=>r.id===a)?.room??'',x:'',y:'',z:'',unit:''},{code:'entry-'+b,room_id:s.racks.find(r=>r.id===b)?.room??'',x:'',y:'',z:'',unit:''}],'code');
    await merge('route_edges',[{code:`route-${a}-${b}`,from:'entry-'+a,to:'entry-'+b,distance_m:distance,category:'network',source:basis}],'code');
    await merge('rack_access',[{rack_id:a,node:'entry-'+a,lead_m:lead},{rack_id:b,node:'entry-'+b,lead_m:lead}],'rack_id');
    const oldSpec=rowsFor(s,'cable_specs').filter(r=>r.code!=='project-stock');
    await api.saveModel('cable_specs',[...oldSpec,...lengths.map(n=>({code:'project-stock',medium,connector_a:'',connector_b:'',rate:'',length_m:String(n),optic_mode:''}))],'人工提供标准长度');
    await api.saveModel('cabling_rules',[{code:'project-route',version:String((s.models.find(m=>m.tableId==='cabling_rules')?.version??0)+1),strategy:'shortest_path',slack_m:slack,slack_percent:'0',cable_spec:'project-stock',source:basis}],'人工确认路径参数');
    await api.calculate('cabling');
  };
  return <Modal title="配置一组布线路径" description="输入明确的路径长度，先计算这两个机柜范围内的连接。" close={close}><div className="form-grid"><Field label="本端机柜"><select value={a} onChange={e=>setA(e.target.value)}>{s.racks.map(r=><option key={r.id} value={r.id}>{r.name}</option>)}</select></Field><Field label="对端机柜"><select value={b} onChange={e=>setB(e.target.value)}>{s.racks.map(r=><option key={r.id} value={r.id}>{r.name}</option>)}</select></Field><Field label="允许路径长度 m"><input type="number" min="0" value={distance} onChange={e=>setDistance(e.target.value)} placeholder="按测量或规划填写"/></Field><Field label="每端柜内引线 m"><input type="number" min="0" value={lead} onChange={e=>setLead(e.target.value)} placeholder="允许明确填0"/></Field><Field label="每条线路固定余量 m"><input type="number" min="0" value={slack} onChange={e=>setSlack(e.target.value)}/></Field><Field label="标准长度 m" hint="逗号分隔，匹配所选规格的标准长度。"><input value={stock} onChange={e=>setStock(e.target.value)} placeholder="填写可用长度列表"/></Field><Field label="适用线缆规格"><select value={medium} onChange={e=>setMedium(e.target.value)}><option value="">选择来源中明确的规格</option>{[...new Set(s.connections.map(c=>c.medium))].map(m=><option key={m} value={m}>{m.replace(' demo','')}</option>)}</select></Field></div><Field label="参数依据"><input value={basis} onChange={e=>setBasis(e.target.value)} placeholder="测量记录、批准规则或规划来源"/></Field><Notice>沿用已识别的设备和连接，不生成实际施工长度。复杂路径可继续在下方表格编辑。</Notice><div className="modal-actions"><Button onClick={close}>取消</Button><Button kind="primary" onClick={async()=>{if(await run('路径参数已应用，相关连接计算完成',apply))close();}}>应用参数并计算<ArrowRight size={14}/></Button></div></Modal>;
}
