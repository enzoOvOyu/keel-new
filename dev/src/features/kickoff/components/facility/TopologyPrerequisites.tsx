import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {identitySnapshot} from '../../../../lib/api/identityBridge';
import {useNavigate} from 'react-router-dom';
import {infrastructureApi,type InfraAsset,type InfraQuerySnapshot} from '../../../../lib/api/infrastructure';
import {useWorkspace} from '../../context';
import {Badge,Button,DataTable,Notice,Modal,Tabs} from '../ui';
import {ModelEditor} from '../ModelEditor';
import {modelRows} from './FacilityEntities';

// Reuse the server's missing-endpoint filter; never load all ports or connections.
export function TopologyPrerequisites({assets,snapshot,rackGaps,partial,refresh,cacheRevision,supplement}:{supplement:(view:'global'|'rack')=>void;assets:InfraAsset[];snapshot:string;rackGaps:number;partial:boolean;refresh:()=>void;cacheRevision:number}){
 const {state:s,can,go}=useWorkspace(),navigate=useNavigate();
 const [cursor,setCursor]=useState<string>();
 const [dialog,setDialog]=useState<{title:string;tab:string;network:boolean}>();
 const identity=identitySnapshot();
 const connectionKey=['kickoff-facility-connections',identity?.user.id,identity?.permission_revision,s.projectId,snapshot,cacheRevision,cursor];
 const query=useQuery({queryKey:connectionKey,staleTime:Infinity,gcTime:30*60*1000,refetchOnMount:false,refetchOnWindowFocus:false,
  queryFn:async({signal})=>{const [missing,all]=await Promise.all([infrastructureApi.connections(s.projectId,{status:'incomplete',limit:10,snapshot_id:snapshot,cursor},signal),infrastructureApi.connections(s.projectId,{limit:1,snapshot_id:snapshot},signal)]);return {missing,hasConnections:all.cables.length>0}}
 });
 const result=query.data,error=query.error?.message;
 const switches=assets.filter(a=>a.type==='switch'),roles=switches.filter(a=>!String(a.fields.role??'').trim());
 const locations=assets.filter(a=>a.type==='rack'&&!assets.some(r=>r.type==='pod'&&r.id===a.fields.pod_id)).length+assets.filter(a=>['server','switch','device','pdu','patch_panel'].includes(a.type)&&!assets.some(r=>r.type==='rack'&&r.id===a.fields.rack_id)).length;
 const missing=result?.missing;
 const connectionStatus=error?'检查失败':!result?'正在读取':missing?.truncated?'返回不完整':missing?.cables.length||missing?.page.has_more?'待补充双端关联':cursor?'请返回首批检查':result.hasConnections?'双端资料已齐':'尚无连接资料';
 const connectionReady=!!result&&!missing?.truncated&&!missing?.cables.length&&!missing?.page.has_more&&!cursor&&result.hasConnections;
 const baseStatus=(count:number)=>partial?'待完成检查':!assets.length?'尚无入库资料':count?'待补充':'已满足';
 const rows=[
  {id:'global',name:'全局拓扑',requirement:'机房 → 机柜 → 设备归属',status:baseStatus(locations)},
  {id:'network',name:'网络架构',requirement:'设备角色与双端连接；Fabric 使用已登记归属',status:partial?'待完成检查':!switches.length?'尚无交换机资料':roles.length?`${roles.length} 台缺少角色`:connectionStatus},
  {id:'physical',name:'物理布线',requirement:'线缆两端端口及所属设备',status:connectionStatus},
  {id:'trace',name:'链路追踪',requirement:'已登记的双端物理连接',status:connectionStatus},
  {id:'rack',name:'机柜与机房',requirement:'机房归属、U位 / 高度、机柜供电容量与设备规划功率',status:baseStatus(rackGaps)},
 ];
 const cmdb=(id:string,changeset?:string)=>navigate('/cmdb/history?'+new URLSearchParams({project_id:s.projectId,asset:id,...(changeset?{changeset}:{})}));
 return <div className="facility-topology-check">
  {!partial&&assets.length>0&&!locations&&!rackGaps&&!roles.length&&connectionReady&&<p className="facility-relation-ready">当前已登记资料满足基础拓扑绘制要求，下方为 CMDB 机柜清单。</p>}
  <div className="table-scroll"><table className="facility-requirements-table"><thead><tr><th>消费视图</th><th>使用的已入库信息</th><th>资料状态</th><th>补充资料</th></tr></thead><tbody>{rows.map(row=><tr key={row.name}><td>{row.name}</td><td>{row.requirement}</td><td><Badge state={['已满足','双端资料已齐'].includes(row.status)?'ready':'needs_input'}>{row.status}</Badge></td><td>{!['已满足','双端资料已齐'].includes(row.status)&&<Button kind="primary" disabled={row.status==='正在读取'} onClick={()=>{if(row.status==='待完成检查'||row.status==='检查失败'||row.status==='返回不完整'){refresh();return;}if(row.id==='global'||row.id==='rack'){if(!assets.length){setDialog({title:row.name,tab:'connections',network:false});return;}supplement(row.id);return;}setDialog({title:row.name,tab:row.id==='network'&&roles.length?'network_roles':'connections',network:row.id==='network'});}}>{['待完成检查','检查失败','返回不完整'].includes(row.status)?'重新检查':'补充信息'}</Button>}</td></tr>)}</tbody></table></div>
  {error&&<Notice tone="error">连接资料检查未完成：{error}<Button onClick={refresh}>重新检查</Button></Notice>}
  <details className="facility-topology-notes"><summary>显示信息与补录范围</summary><p>线缆介质、额定速率与长度可继续增补，不阻断基础连线。逻辑 Fabric 只使用明确归属；链路实际状态、流量、电力和温度来自采集，不通过手填判定正常。</p><p>机房空间视图目前为示意布局；实际坐标与尺寸不作为当前绘图的必填项。物理追踪只表示登记的连接，不等于实测连通。</p></details>
  {dialog?.tab==='network_roles'&&<ModelEditor keepOpenOnSave fieldKeys={['device_id','role']} title={dialog.title+' · 补充信息'} tableId="network_roles" targetIds={roles.map(a=>a.id)} initialRows={[...modelRows(s,'network_roles'),...roles.filter(a=>!modelRows(s,'network_roles').some(r=>r.device_id===a.id)).map(a=>({device_id:a.id,role:''}))]} entityChoices={{device_id:roles.map(a=>({id:a.id,label:a.name}))}} relatedTables={[{id:'network_roles',label:'设备角色'},...(!connectionReady?[{id:'connections',label:'双端关联'}]:[])]} onTableChange={tab=>setDialog({...dialog,tab})} close={()=>setDialog(undefined)}/>}
  {dialog?.tab==='connections'&&<Modal title={dialog.title+' · 补充信息'} description="仅处理当前视图缺失的资料；已登记关系无需重填。" wide close={()=>setDialog(undefined)}>
   <Tabs value="connections" items={[...(dialog.network&&roles.length?[{id:'network_roles',label:'设备角色'}]:[]),{id:'connections',label:'双端关联'}]} onChange={tab=>setDialog({...dialog,tab})}/>
   {dialog.network&&!switches.length?<Notice>尚无已入库交换机资料。<Button onClick={()=>go('review')}>核对设备资料</Button></Notice>:!assets.length?<Notice>尚无入库资料，请先核对并确认入库。<Button onClick={()=>go('publish')}>查看入库范围</Button></Notice>:<>
  {!!missing?.cables.length&&<div className="facility-inline-gaps"><b>待补充双端关联</b><DataTable data={missing.cables} pageSize={10} columns={[
   {id:'name',header:'线缆',cell:({row})=>missing.assets.find(a=>a.id===row.original.id)?.name||row.original.id},
   {id:'gap',header:'缺少信息',cell:({row})=>[!row.original.a&&'本端端口',!row.original.b&&'对端端口',!row.original.device_a&&'本端设备归属',!row.original.device_b&&'对端设备归属'].filter(Boolean).join('、')},
   {id:'action',header:'处理',cell:({row})=><Button disabled={!can('cmdb:edit')} onClick={()=>cmdb(row.original.id)}>补充 CMDB 关系</Button>},
  ]}/><div className="actions">{cursor&&<Button onClick={()=>setCursor(undefined)}>返回首批缺项</Button>}{missing.page.has_more&&<Button onClick={()=>setCursor(missing.page.next_cursor??undefined)}>查看下一批缺项</Button>}</div></div>}
  {cursor&&!missing?.cables.length&&<Button onClick={()=>setCursor(undefined)}>返回首批缺项</Button>}
  {result&&!result.hasConnections&&<p className="facility-relation-summary">尚未入库连接资料。<Button onClick={()=>go('review')}>核对连接资料</Button></p>}

   {error&&<Notice tone="error">{error}<Button onClick={refresh}>重新检查</Button></Notice>}
   {connectionReady&&<Notice tone="success">双端资料已齐，无需补充。</Notice>}
   <p className="muted small-copy">端口与设备归属通过现有 CMDB 关系变更处理，保留审核与发布。完成后点击刷新检查更新资料状态。</p>
   <Button onClick={refresh}>刷新检查</Button></>}
  </Modal>}

 </div>;
}
