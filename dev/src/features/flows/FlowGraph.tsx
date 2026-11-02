import { lazy, Suspense, useMemo, useState } from 'react';
import { ReactFlow, Background, Controls, MarkerType, Handle, Position, type Node, type Edge } from '@xyflow/react';
import { Box, GitBranch, Terminal, Repeat2, Layers, ArrowDown, ArrowUp, Trash2, Plus, ListTree, FileCode2, FolderOpen, Check, Save, Upload } from 'lucide-react';
import '@xyflow/react/dist/style.css';
import YAML from 'yaml';
import { Button, Notice, Tabs, State } from './ui';
import { branches, checkStructure, taskList, locate, removeTask, moveTask, sequenceAfter, connectSteps, definitionGraph, isControlled, inputDefault, setDefault, type Definition, type Task } from './definition';
const NativeCodeEditorHost=lazy(()=>import('../jobs-native/NativeCodeEditorHost').then(m=>({default:m.NativeCodeEditorHost})));
const core = 'io.kestra.plugin.core.';
const library = [
  ['CONTROL FLOW', 'Loop', core+'flow.Loop', Repeat2], ['CONTROL FLOW', 'Parallel', core+'flow.Parallel', Layers],
  ['CONTROL FLOW', 'Condition', core+'flow.If', GitBranch], ['CONTROL FLOW', 'Subflow', core+'flow.Subflow', Box],
  ['KESTRA TASKS', 'Log', core+'log.Log', Terminal], ['KESTRA TASKS', 'Output', core+'output.OutputValues', Box],
] as const;
function TaskNode({data}: any) {
  const Icon = data.type?.endsWith('.If') ? GitBranch : data.type?.endsWith('.Loop') ? Repeat2 : data.type?.endsWith('.Parallel') ? Layers : Terminal;
  return <><Handle type="target" position={Position.Top}/><div className="native-flow-node-title"><Icon size={16}/><strong>{data.label}</strong>{data.state&&<State value={data.state}/>}</div><small>{data.subtitle}</small>{data.lane&&<span className="native-flow-node-lane">{data.lane}</span>}<Handle type="source" position={Position.Bottom}/></>;
}
function TerminalNode({data}:any){return <div className="native-flow-terminal">{data.kind==='end'&&<Handle type="target" position={Position.Top}/>}<span>{data.label}</span>{data.kind==='start'&&<Handle type="source" position={Position.Bottom}/>}</div>}
const nodeTypes = { task: TaskNode,terminal:TerminalNode };
export function FlowGraph({ document, onSelect, readonly = true, onConnect, selected, states = {} }: {
  document: Definition; onSelect?: (id: string) => void; readonly?: boolean;
  onConnect?: (from:string,to:string)=>void; selected?:string; states?:Record<string,string>;
}) {
  const [offsets,setOffsets]=useState<Record<string,{x:number;y:number}>>({});
  const graph=useMemo(()=>definitionGraph(document || {}),[document]);
  const nodes:Node[]=graph.nodes.map(n=>({id:n.id,type:'task',position:offsets[n.id]||{x:n.x,y:n.y},selected:selected===n.id,
    data:{label:n.id.startsWith('step:')?n.id.slice(5):n.id,type:n.task.type,subtitle:n.task.type?.split('.').at(-1),lane:n.lane,state:states[n.id]},className:n.lane==='Parallel'||n.lane==='Iteration'?'native-flow-compact':undefined,draggable:!readonly}));
  const edges:Edge[]=graph.edges.map((e,i)=>({id:`${e.source}-${e.target}-${i}`,source:e.source,target:e.target,label:e.label,type:'smoothstep',markerEnd:{type:MarkerType.ArrowClosed}}));
  if(graph.nodes.length){
    const roots=graph.nodes.filter(n=>!graph.edges.some(e=>e.target===n.id)),leaves=graph.nodes.filter(n=>!graph.edges.some(e=>e.source===n.id));
    const center=(Math.min(...graph.nodes.map(n=>n.x))+Math.max(...graph.nodes.map(n=>n.x)))/2+70;
    nodes.push({id:'terminal:start',type:'terminal',position:{x:center,y:Math.min(...graph.nodes.map(n=>n.y))-80},data:{label:'START',kind:'start'},draggable:false},{id:'terminal:end',type:'terminal',position:{x:center,y:Math.max(...graph.nodes.map(n=>n.y))+120},data:{label:'END',kind:'end'},draggable:false});
    for(const n of roots)edges.push({id:'terminal:start:'+n.id,source:'terminal:start',target:n.id,type:'smoothstep',markerEnd:{type:MarkerType.ArrowClosed}});
    for(const n of leaves)edges.push({id:'terminal:end:'+n.id,source:n.id,target:'terminal:end',type:'smoothstep',markerEnd:{type:MarkerType.ArrowClosed}});
  }
  return <div className="native-flow-graph"><ReactFlow nodes={nodes} edges={edges} nodeTypes={nodeTypes} fitView fitViewOptions={{maxZoom:1,padding:.25}}
    onNodeClick={(_,n)=>{if(n.type!=='terminal')onSelect?.(n.id)}} onPaneClick={()=>onSelect?.('')} onNodeDragStop={(_,n)=>setOffsets(v=>({...v,[n.id]:n.position}))}
    onConnect={e=>e.source&&e.target&&onConnect?.(e.source,e.target)} nodesConnectable={!readonly&&!!onConnect} nodesDraggable={!readonly} minZoom={.2} maxZoom={1.5}>
    <Controls showInteractive={false}/>
  </ReactFlow></div>;
}
