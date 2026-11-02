import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {Link} from 'react-router-dom';
import {Button} from '../components/ui';
import {useProject} from '../app/ProjectContext';
import {createEngineeringKickoffHandoff,engineeringKickoffHandoff,projectEngineeringKickoff} from '../lib/api/engineering';
import {stageIds,stageLabels} from '../lib/schemas/delivery';

const states:Record<string,string>={NEEDS_INPUT:'待完成审核发布',READY:'可以交接',HANDED_OFF:'已交接',STALE:'产物已变化，需重新交接'};
export function EngineeringHandoff({project,id,revision,canExecute}:{project:string;id:string;revision:number;canExecute:boolean}){
 const query=useQuery({queryKey:['engineering-handoff',project,id,revision],queryFn:({signal})=>engineeringKickoffHandoff(project,id,signal),refetchInterval:5000});
 const [reason,setReason]=useState(''),[busy,setBusy]=useState(false),[error,setError]=useState('');
 async function handoff(){setBusy(true);setError('');try{await createEngineeringKickoffHandoff(project,id,{expected_revision:revision,reason});await query.refetch()}catch(e){setError((e as Error).message)}finally{setBusy(false)}}
 return <section className="panel padded es-handoff"><div className="es-section-title"><h3>交付主链交接</h3><span>发布规划不会把执行阶段标记为通过</span></div><div className="es-delivery-stages">{stageIds.map((s,i)=><div key={s}><span>0{i+1}</span><strong>{stageLabels[s]}</strong><small>{s==='KICKOFF'&&query.data?states[query.data.state]:'待执行'}</small></div>)}</div>{query.isPending?<p>正在核对发布产物…</p>:query.data&&<><strong>{states[query.data.state]}</strong><p>{query.data.equipment_ids.length} 台设备 · {query.data.port_ids.length} 个端口 · {query.data.network_plan_ids.length} 份网络规划</p>{query.data.blockers.length>0&&<details><summary>{query.data.blockers.length} 项待完成</summary>{query.data.blockers.map((v,i)=><p key={i}>{v}</p>)}</details>}<p>交接引用已审核发布的版本。后续阶段保持“未开始”，由实际执行证据决定结果。</p>{query.data.state!=='HANDED_OFF'&&<><label>交接依据<input value={reason} disabled={busy} onChange={e=>setReason(e.target.value)}/></label><Button variant="primary" disabled={!canExecute||busy||!reason.trim()||query.data.blockers.length>0} onClick={()=>void handoff()}>确认交接</Button></>}<Link className="btn" to={'/p/'+encodeURIComponent(project)+'/deploy/overview'}>查看交付输入</Link></>}{[error,query.error?.message].filter(Boolean).map((v,i)=><p role="alert" key={i}>{v}</p>)}</section>
}

// Real kickoff consumes publication references; no mock stage is marked passed.
export function KickoffDeliveryInput(){
 const {projectId}=useProject();
 const query=useQuery({queryKey:['project-engineering-kickoff',projectId],queryFn:({signal})=>projectEngineeringKickoff(projectId,signal)});
 if(query.isPending)return <p aria-busy="true">读取开局交接…</p>;
 if(query.error)return <p role="alert">{query.error.message}<Button onClick={()=>void query.refetch()}>重试</Button></p>;
 const value=query.data;if(!value)return null;
 return <section className="panel padded"><h2>交付主链 · 开局配置</h2><div className="es-delivery-stages">{stageIds.map(s=><div key={s}><strong>{stageLabels[s]}</strong><small>{s==='KICKOFF'?states[value.state]:'未开始'}</small></div>)}</div><p>设备 {value.equipment_ids.length} · 端口 {value.port_ids.length} · 网络规划 {value.network_plan_ids.length}</p>{value.blockers.map((b,i)=><p key={i} className="notice">{b}</p>)}<Link className="btn" to={'/p/'+encodeURIComponent(projectId)+'/deploy/bom'+(value.source_id?'/'+value.source_id:'')+'?tab=release'}>打开智能开局审核与交接</Link>{value.publications.length>0&&<div className="table-wrap"><table><thead><tr><th>产物域</th><th>已发布版本</th><th>发布凭据</th></tr></thead><tbody>{value.publications.map(v=><tr key={v.target}><td>{v.target}</td><td><code>{v.revision_id}</code></td><td><code>{v.publication_id}</code></td></tr>)}</tbody></table></div>}<p className="muted">这里只接收开局输入。网络下发、装机、验机和 NCCL 尚未执行。</p></section>
}
