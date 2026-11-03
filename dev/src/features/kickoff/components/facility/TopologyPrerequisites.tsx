import {useEffect,useState} from 'react';
import {useNavigate} from 'react-router-dom';
import {infrastructureApi,type InfraAsset,type InfraQuerySnapshot} from '../../../../lib/api/infrastructure';
import {useWorkspace} from '../../context';
import {Badge,Button,DataTable,Notice} from '../ui';
import {ModelEditor} from '../ModelEditor';
import {modelRows} from './FacilityEntities';

// Reuse the server's missing-endpoint filter; never load all ports or connections.
export function TopologyPrerequisites({assets,snapshot,rackGaps,partial,refresh}:{assets:InfraAsset[];snapshot:string;rackGaps:number;partial:boolean;refresh:()=>void}){
 const {state:s,can,go}=useWorkspace(),navigate=useNavigate();
 const [result,setResult]=useState<{missing:InfraQuerySnapshot;hasConnections:boolean}>();
 const [error,setError]=useState(''),[cursor,setCursor]=useState<string>();
 const [editRoles,setEditRoles]=useState(false);
 useEffect(()=>{const abort=new AbortController();setResult(undefined);setError('');
  Promise.all([infrastructureApi.connections(s.projectId,{status:'incomplete',limit:10,snapshot_id:snapshot,cursor},abort.signal),infrastructureApi.connections(s.projectId,{limit:1,snapshot_id:snapshot},abort.signal)])
   .then(([missing,all])=>{if(!abort.signal.aborted)setResult({missing,hasConnections:all.cables.length>0})}).catch(e=>{if(!abort.signal.aborted)setError(e.message)});
  return()=>abort.abort();
 },[s.projectId,snapshot,cursor]);
 const switches=assets.filter(a=>a.type==='switch'),roles=switches.filter(a=>!String(a.fields.role??'').trim());
 const locations=assets.filter(a=>a.type==='rack'&&!assets.some(r=>r.type==='pod'&&r.id===a.fields.pod_id)).length+assets.filter(a=>['server','switch','device','pdu','patch_panel'].includes(a.type)&&!assets.some(r=>r.type==='rack'&&r.id===a.fields.rack_id)).length;
 const missing=result?.missing;
 const connectionStatus=error?'检查失败':!result?'正在读取':missing?.truncated?'返回不完整':missing?.cables.length||missing?.page.has_more?'待补充双端关联':cursor?'请返回首批检查':result.hasConnections?'双端资料已齐':'尚无连接资料';
 const connectionReady=!!result&&!missing?.truncated&&!missing?.cables.length&&!missing?.page.has_more&&!cursor&&result.hasConnections;
 const baseStatus=(count:number)=>partial?'待完成检查':!assets.length?'尚无入库资料':count?'待补充':'已满足';
 const rows=[
  {name:'全局拓扑',requirement:'机房 → 机柜 → 设备归属',status:baseStatus(locations)},
  {name:'网络架构',requirement:'设备角色与双端连接；Fabric 使用已登记归属',status:partial?'待完成检查':!switches.length?'尚无交换机资料':roles.length?`${roles.length} 台缺少角色`:connectionStatus},
  {name:'物理布线',requirement:'线缆两端端口及所属设备',status:connectionStatus},
  {name:'链路追踪',requirement:'已登记的双端物理连接',status:connectionStatus},
  {name:'机柜与机房',requirement:'机房归属、机柜容量、设备机柜与 U 位 / 高度',status:baseStatus(rackGaps)},
 ];
 const cmdb=(id:string,changeset?:string)=>navigate('/cmdb/history?'+new URLSearchParams({project_id:s.projectId,asset:id,...(changeset?{changeset}:{})}));
 return <div className="facility-topology-check">
  {!partial&&assets.length>0&&!locations&&!rackGaps&&!roles.length&&connectionReady&&<p className="facility-relation-ready">当前已登记资料满足基础拓扑绘制要求，下方为 CMDB 机柜清单。</p>}
  <div className="table-scroll"><table className="facility-requirements-table"><thead><tr><th>消费视图</th><th>使用的已入库信息</th><th>资料状态</th></tr></thead><tbody>{rows.map(row=><tr key={row.name}><td>{row.name}</td><td>{row.requirement}</td><td><Badge state={['已满足','双端资料已齐'].includes(row.status)?'ready':'needs_input'}>{row.status}</Badge></td></tr>)}</tbody></table></div>
  {error&&<Notice tone="error">连接资料检查未完成：{error}<Button onClick={refresh}>重新检查</Button></Notice>}
  {!!roles.length&&<div className="facility-inline-gaps"><b>{roles.length} 台交换机尚未登记角色</b><p className="muted small-copy">角色用于网络分层；复用网络工程输入补录。保存后仍为工程输入，CMDB 读取到发布值前保持待补充。</p><Button kind="primary" disabled={!can('bom:edit')} onClick={()=>setEditRoles(true)}>批量补录交换机角色</Button></div>}
  {!!missing?.cables.length&&<div className="facility-inline-gaps"><b>待补充双端关联</b><DataTable data={missing.cables} pageSize={10} columns={[
   {id:'name',header:'线缆',cell:({row})=>missing.assets.find(a=>a.id===row.original.id)?.name||row.original.id},
   {id:'gap',header:'缺少信息',cell:({row})=>[!row.original.a&&'本端端口',!row.original.b&&'对端端口',!row.original.device_a&&'本端设备归属',!row.original.device_b&&'对端设备归属'].filter(Boolean).join('、')},
   {id:'action',header:'处理',cell:({row})=><Button disabled={!can('cmdb:edit')} onClick={()=>cmdb(row.original.id)}>补充 CMDB 关系</Button>},
  ]}/><div className="actions">{cursor&&<Button onClick={()=>setCursor(undefined)}>返回首批缺项</Button>}{missing.page.has_more&&<Button onClick={()=>setCursor(missing.page.next_cursor??undefined)}>查看下一批缺项</Button>}</div></div>}
  {cursor&&!missing?.cables.length&&<Button onClick={()=>setCursor(undefined)}>返回首批缺项</Button>}
  {result&&!result.hasConnections&&<p className="facility-relation-summary">尚未入库连接资料。<Button onClick={()=>go('review')}>核对连接资料</Button></p>}
  <details className="facility-topology-notes"><summary>显示信息与补录范围</summary><p>线缆介质、额定速率与长度可继续增补，不阻断基础连线。逻辑 Fabric 只使用明确归属；链路实际状态、流量、电力和温度来自采集，不通过手填判定正常。</p><p>机房空间视图目前为示意布局；实际坐标与尺寸不作为当前绘图的必填项。物理追踪只表示登记的连接，不等于实测连通。</p></details>
  {editRoles&&<ModelEditor tableId="network_roles" targetIds={roles.map(a=>a.id)} initialRows={[...modelRows(s,'network_roles'),...roles.filter(a=>!modelRows(s,'network_roles').some(r=>r.device_id===a.id)).map(a=>({device_id:a.id,role:''}))]} entityChoices={{device_id:roles.map(a=>({id:a.id,label:a.name}))}} close={()=>setEditRoles(false)}/>}
 </div>;
}
