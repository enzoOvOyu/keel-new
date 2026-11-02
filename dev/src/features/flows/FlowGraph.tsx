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
  return <><Handle type="target" position={Position.Top}/><div className="fl-node-title"><Icon size={16}/><strong>{data.label}</strong>{data.state&&<State value={data.state}/>}</div><small>{data.subtitle}</small>{data.lane&&<span className="fl-node-lane">{data.lane}</span>}<Handle type="source" position={Position.Bottom}/></>;
}
const nodeTypes = { task: TaskNode };
export function FlowGraph({ document, onSelect, readonly = true, onConnect, selected, states = {} }: {
  document: Definition; onSelect?: (id: string) => void; readonly?: boolean;
  onConnect?: (from:string,to:string)=>void; selected?:string; states?:Record<string,string>;
}) {
  const [offsets,setOffsets]=useState<Record<string,{x:number;y:number}>>({});
  const graph=useMemo(()=>definitionGraph(document || {}),[document]);
  const nodes:Node[]=graph.nodes.map(n=>({id:n.id,type:'task',position:offsets[n.id]||{x:n.x,y:n.y},selected:selected===n.id,
    data:{label:n.id.startsWith('step:')?n.id.slice(5):n.id,type:n.task.type,subtitle:n.task.type?.split('.').at(-1),lane:n.lane,state:states[n.id]},draggable:!readonly}));
  const edges:Edge[]=graph.edges.map((e,i)=>({id:`${e.source}-${e.target}-${i}`,source:e.source,target:e.target,label:e.label,type:'smoothstep',markerEnd:{type:MarkerType.ArrowClosed}}));
  return <ReactFlow nodes={nodes} edges={edges} nodeTypes={nodeTypes} fitView fitViewOptions={{maxZoom:1,padding:.25}}
    onNodeClick={(_,n)=>onSelect?.(n.id)} onPaneClick={()=>onSelect?.('')} onNodeDragStop={(_,n)=>setOffsets(v=>({...v,[n.id]:n.position}))}
    onConnect={e=>e.source&&e.target&&onConnect?.(e.source,e.target)} nodesConnectable={!readonly&&!!onConnect} nodesDraggable={!readonly} minZoom={.2} maxZoom={1.5}>
    <Background gap={20} size={1}/><Controls showInteractive={false}/>
  </ReactFlow>;
}
