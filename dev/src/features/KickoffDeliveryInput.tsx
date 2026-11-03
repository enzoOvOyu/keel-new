import {useQuery} from '@tanstack/react-query';
import {Link} from 'react-router-dom';
import {Button} from '../components/ui';
import {useProject} from '../app/ProjectContext';
import {apiRequest} from '../lib/api/request';
import type {Workspace} from '../lib/api/kickoff/types';

export function KickoffDeliveryInput(){
 const {projectId}=useProject();
 const query=useQuery({queryKey:['kickoff-delivery-input',projectId],queryFn:({signal})=>apiRequest<Workspace>(`projects/${encodeURIComponent(projectId)}/kickoff/workspace`,'GET',undefined,undefined,signal)});
 const href=`/p/${encodeURIComponent(projectId)}/kickoff/publish`;
 if(query.isPending)return <p aria-busy="true">读取开局输入…</p>;
 if(query.error)return <p role="alert">{query.error.message}<Button onClick={()=>void query.refetch()}>重试</Button></p>;
 const value=query.data;if(!value)return null;
 const published=value.publications.filter(p=>p.state==='published');
 const devices=new Set(published.flatMap(p=>p.deviceIds)).size;
 return <section className="panel padded"><h2>交付主链 · 开局配置</h2><p>已识别设备 {value.totals?.devices??value.devices.length} · 端口 {value.totals?.ports??value.ports.length} · 连接 {value.totals?.connections??value.connections.length}</p><p>{devices} 台设备已入库 · {published.length} 个已发布版本</p><Link className="btn" to={href}>打开智能开局审核与入库</Link>{value.publications.length>0&&<div className="table-wrap"><table><thead><tr><th>版本</th><th>入库状态</th><th>设备 / 连接</th><th>发布凭据</th></tr></thead><tbody>{[...value.publications].reverse().map(p=><tr key={p.id}><td>r{p.revision}</td><td>{p.state==='published'?'已入库':p.state==='pending'?'投影中':'入库失败'}</td><td>{p.counts.devices??0} / {p.counts.connections??0}</td><td><Link to={`${href}?publication=${encodeURIComponent(p.id)}`}>查看版本</Link></td></tr>)}</tbody></table></div>}<p className="muted">开局输入按可用范围入库，工程信息可以后续补充。后续阶段由实际执行证据决定状态。</p></section>;
}
