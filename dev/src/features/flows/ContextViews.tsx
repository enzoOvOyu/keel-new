import { ReactFlow, Background, Controls, MarkerType } from '@xyflow/react';
import { useNavigate } from 'react-router-dom';
import { lazy,Suspense } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import YAML from 'yaml';
import { flowsKey, flowsApi, type Engine } from '@/lib/api/flows';
import { Button, Table, Notice, Empty, State } from './ui';
const ContextView=lazy(()=>import('../jobs-native/NativeFlowContextHost').then(m=>({default:m.NativeFlowContextHost})));
export function RevisionView({values,current,onRestore,canEdit}:{values:Engine[];current:string;onRestore:(source:string)=>void;canEdit:boolean}){return <Suspense fallback={<Empty loading/>}><ContextView kind="revisions" source={current} revisions={values} executions={[]} canEdit={canEdit} onRestore={onRestore}/></Suspense>}
export function TriggerView({source,onChange,canEdit}:{source:string;onChange:(source:string)=>void;onBuilder:()=>void;canEdit:boolean}){let doc:any={};try{doc=YAML.parse(source)||{}}catch{}return <Suspense fallback={<Empty loading/>}><ContextView kind="triggers" source={source} revisions={[]} executions={[]} canEdit={canEdit} onToggle={(index:number)=>{doc.triggers=(doc.triggers||[]).map((t:any,i:number)=>i===index?{...t,disabled:!t.disabled}:t);onChange(YAML.stringify(doc))}} onAddTrigger={(t:{id:string;cron:string;timezone:string})=>{if((doc.triggers||[]).some((v:any)=>v.id===t.id))return;doc.triggers=[...(doc.triggers||[]),{...t,type:'io.kestra.plugin.core.trigger.Schedule',disabled:true}];onChange(YAML.stringify(doc))}}/></Suspense>}
export { NamespaceFiles } from './NamespaceFiles';
export function DependencyView({ project, id }: {
    project: string;
    id: string;
}) {
    const nav = useNavigate();
    const query = useQuery({ queryKey: flowsKey('dependencies', project, id), queryFn: () => flowsApi.dependencies(project, id) });
    const graph = query.data;
    return <><Notice error={query.error}/>{graph?.partial && <p className="fl-note">仅展示已授权且已登记的当前项目依赖。</p>}{graph?.nodes.length ? <div className="fl-overview-dag"><ReactFlow nodes={graph.nodes.map((n, i) => ({ id: n.uid, position: { x: (i % 3) * 250, y: Math.floor(i / 3) * 140 }, data: { label: n.flow_id }, style: { width: 200 } }))} edges={graph.edges.map((e, i) => ({ id: String(i), source: e.source, target: e.target, markerEnd: { type: MarkerType.ArrowClosed } }))} fitView fitViewOptions={{ maxZoom: 1 }} onNodeClick={(_, n) => {
                const b = graph.nodes.find(v => v.uid === n.id)?.binding_id;
                if (b)
                    nav('/flows/definitions/' + b);
            }}><Background /><Controls /></ReactFlow></div> : <Empty loading={query.isLoading}>当前没有已登记的 Flow 依赖。</Empty>}</>;
}
