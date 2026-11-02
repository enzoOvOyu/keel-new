/** Editing operates on the Kestra document, never on a second graph definition. */
export type Task = { id: string; type: string; [key: string]: any };
export type Definition = { id?: string; namespace?: string; tasks?: Task[]; inputs?: any[]; [key: string]: any };
export const branches = ['tasks', 'then', 'else', 'finally', 'errors'] as const;
export function checkStructure(doc: Definition, root = true) {
  if (!doc || typeof doc !== 'object' || Array.isArray(doc)) throw Error('Flow 文档必须是 YAML 对象');
  for (const key of branches) {
    if (doc[key] === undefined) continue;
    if (!Array.isArray(doc[key])) throw Error(`${key} 必须是任务列表`);
    for (const task of doc[key]) {
      if (!task || typeof task.id !== 'string' || typeof task.type !== 'string') throw Error('每个任务必须有 id 和 type');
      checkStructure(task, false);
    }
  }
  if (!root) return;
  if (doc.inputs !== undefined && !Array.isArray(doc.inputs)) throw Error('inputs 必须是列表');
  const steps = inputDefault(doc, 'steps', []);
  if (!Array.isArray(steps) || steps.some(s => !s || typeof s.id !== 'string' || !Array.isArray(s.depends_on))) throw Error('steps 必须包含任务 ID 与依赖列表');
}
export function taskList(doc: Definition): Task[] {
  return branches.flatMap(key => (Array.isArray(doc[key]) ? doc[key] : []).flatMap((t: Task) => [t, ...taskList(t)]));
}
export function locate(doc: Definition, id: string): { list: Task[]; index: number; parent: Definition; branch: string } | undefined {
  for (const branch of branches) {
    const list = doc[branch] as Task[] | undefined;
    if (!Array.isArray(list)) continue;
    const index = list.findIndex(t => t.id === id);
    if (index >= 0) return { list, index, parent: doc, branch };
    for (const t of list) { const found = locate(t, id); if (found) return found; }
  }
}
export function removeTask(doc: Definition, id: string) {
  const found = locate(doc, id); if (found) found.list.splice(found.index, 1);
}
export function moveTask(doc: Definition, id: string, delta: number) {
  const at = locate(doc, id); if (!at) return;
  const next = at.index + delta;
  if (next < 0 || next >= at.list.length) return;
  const [item] = at.list.splice(at.index, 1); at.list.splice(next, 0, item);
}
export function sequenceAfter(doc: Definition, from: string, to: string) {
  const a = locate(doc, from), b = locate(doc, to);
  if (!a || !b || a.list !== b.list || a.parent.type?.endsWith('.Parallel') || from === to)
    throw Error('连接只能调整同一顺序分支；并行与条件关系请在节点的分支中编辑。');
  const [item] = b.list.splice(b.index, 1);
  a.list.splice(a.list.findIndex(t => t.id === from) + 1, 0, item);
}
export function connectSteps(steps: any[], from: string, to: string) {
  const source = from.replace(/^step:/, ''), target = to.replace(/^step:/, '');
  const node = steps.find(s => s.id === target);
  if (!node || source === target || !steps.some(s => s.id === source)) throw Error('请选择两个不同的设备任务');
  const dependencies = new Map(steps.map(s => [s.id, [...s.depends_on]]));
  dependencies.set(target, [...new Set([...dependencies.get(target)!, source])]);
  const resolved = new Set<string>();
  while (resolved.size < steps.length) {
    const next = steps.find(s => !resolved.has(s.id) && dependencies.get(s.id)!.every(id => resolved.has(id)));
    if (!next) throw Error('该连接会形成循环依赖，未保存');
    resolved.add(next.id);
  }
  node.depends_on = dependencies.get(target);
}
export function isControlled(doc: Definition) { return !!doc.inputs?.some(i => i.id === 'ticket'); }
export function inputDefault(doc: Definition, key: string, fallback: any = '') { return doc.inputs?.find(i => i.id === key)?.defaults ?? fallback; }
export function setDefault(doc: Definition, key: string, value: any, type = 'STRING') {
  doc.inputs ||= []; const input = doc.inputs.find(i => i.id === key);
  if (input) input.defaults = value; else doc.inputs.push({ id: key, type, defaults: value });
}
export function durationSeconds(value: unknown, fallback = 600) {
  const m = /^PT(?:(\d+(?:\.\d+)?)H)?(?:(\d+(?:\.\d+)?)M)?(?:(\d+(?:\.\d+)?)S)?$/.exec(String(value));
  return m ? Number(m[1] || 0) * 3600 + Number(m[2] || 0) * 60 + Number(m[3] || 0) : fallback;
}
export function runDefaults(doc: Definition) {
  const tasks = taskList(doc), loop = tasks.find(t => t.id === 'targets');
  const dispatch = tasks.find(t => ['agent_task', 'ssh_task', 'dispatch'].includes(t.id));
  return { kind: inputDefault(doc, 'kind', 'bash'), command: inputDefault(doc, 'command'), policy: inputDefault(doc, 'transport_policy', 'agent_preferred'), steps: inputDefault(doc, 'steps', []), timeout: durationSeconds(dispatch?.timeout, 610) - 10, concurrency: loop?.concurrencyLimit || 8 };
}
export type GraphItem = { id: string; task: Task; x: number; y: number; lane?: string };
export type GraphLine = { source: string; target: string; label?: string };
export function definitionGraph(doc: Definition) {
  const nodes: GraphItem[] = [], edges: GraphLine[] = []; let lane = 0;
  if(doc.tasks?.length && doc.tasks.every(t=>t.type.startsWith('keel.') && Array.isArray(t.depends_on))){
    const depth:Record<string,number>={}, pending=[...doc.tasks];
    while(pending.length){const i=pending.findIndex(t=>t.depends_on.every((id:string)=>depth[id]!==undefined));if(i<0)break;const [t]=pending.splice(i,1);depth[t.id]=Math.max(-1,...t.depends_on.map((id:string)=>depth[id]))+1;}
    const columns:Record<number,number>={};
    for(const t of doc.tasks){const row=depth[t.id]??0,col=columns[row]||0;columns[row]=col+1;nodes.push({id:t.id,task:t,x:col*272,y:row*118});for(const source of t.depends_on)if(doc.tasks.some(n=>n.id===source))edges.push({source,target:t.id});}
    return {nodes,edges};
  }
  function width(tasks: Task[]): number {
    return Math.max(1,...(tasks||[]).map(t => t.type.endsWith('.Parallel')
      ? Math.max(1,(t.tasks||[]).reduce((n:number,child:Task)=>n+width([child]),0))
      : t.type.endsWith('.If') ? width(t.then||[])+width(t.else||[])
      : width(t.tasks||[])));
  }
  function sequence(tasks: Task[], incoming: string[], col: number, row: number, label?: string): { ends: string[]; row: number } {
    let ends = incoming;
    for (const task of tasks || []) {
      nodes.push({ id: task.id, task, x: col * 272, y: row * 118, lane: label });
      for (const source of ends) edges.push({ source, target: task.id, label });
      row++; ends = [task.id];
      if (task.type?.endsWith('.Parallel')) {
        let bottom = row, column = col;
        ends = (task.tasks || []).flatMap((child: Task) => {
          const branch = sequence([child], [task.id], column, row, 'Parallel'); column += width([child]); bottom = Math.max(bottom, branch.row); return branch.ends;
        }); if(!ends.length)ends=[task.id]; row = bottom;
      } else if (task.type?.endsWith('.If')) {
        const yes = sequence(task.then || [], [task.id], col, row, 'True');
        const no = sequence(task.else || [], [task.id], col + width(task.then || []), row, 'False');
        ends = [...new Set([...yes.ends, ...no.ends])]; row = Math.max(yes.row, no.row);
      } else if (task.tasks?.length) {
        const inner = sequence(task.tasks, [task.id], col, row, task.type?.endsWith('.Loop') ? 'Iteration' : undefined);
        ends = inner.ends; row = inner.row;
      }
    }
    return { ends, row };
  }
  for (const branch of ['tasks', 'errors', 'finally']) {
    if (!doc[branch]?.length) continue;
    sequence(doc[branch], [], lane, 0, branch === 'tasks' ? undefined : branch);
    lane = Math.max(lane + 2, ...nodes.map(n => n.x / 272 + 2));
  }
  return { nodes, edges };
}

/** Native starts are complete documents, never mutations of a controlled device template. */
export function nativeTemplate(current: Definition, kind: string): Definition {
 const core='io.kestra.plugin.core.';
 const child={id:'log_result',type:core+'log.Log',message:'Flow started'};
 const task:Task=kind==='Log'?child:kind==='Loop'
  ?{id:'loop',type:core+'flow.Loop',values:'{{ inputs.items }}',concurrencyLimit:1,tasks:[child]}
  :{id:'parallel',type:core+'flow.Parallel',tasks:[child]};
 return {id:current.id,namespace:current.namespace,tasks:[task],...(kind==='Loop'?{inputs:[{id:'items',type:'JSON',defaults:[]}]}:{})};
}

// One canonical device execution, projected for editing without exposing the transport wrapper.
export function deliverySteps(doc:Definition):any[]{const steps=inputDefault(doc,'steps',[]);return steps.length?steps:[{id:'command',kind:inputDefault(doc,'kind','bash'),command:inputDefault(doc,'command',''),depends_on:[],inputs:{},implicit:true}]}

/** Client feedback mirrors JobStep structural limits; the server still authorizes every run. */
export function runDraftError(request: {command:string;timeout_seconds:number;target_concurrency:number;steps?:{description?:string;timeout_seconds?:number;id:string;kind:string;command:string;depends_on:string[];inputs?:Record<string,string>}[]}):string {
  if(!Number.isInteger(request.timeout_seconds)||request.timeout_seconds<1||request.timeout_seconds>3600)return '超时应为 1–3600 秒的整数';
  if(!Number.isInteger(request.target_concurrency)||request.target_concurrency<1||request.target_concurrency>64)return '目标并发应为 1–64 的整数';
  const steps=request.steps||[];
  if(!steps.length)return request.command.trim()?'':'请输入脚本或命令';
  if(steps.length>32)return '设备步骤最多 32 个';
  const ids=new Set<string>();
  for(const step of steps){if(step.timeout_seconds!==undefined&&(!Number.isInteger(step.timeout_seconds)||step.timeout_seconds<0||step.timeout_seconds>request.timeout_seconds))return step.id+'：步骤超时必须在作业时限内，0 表示继承';if(step.description&&[...step.description].length>2000)return step.id+'：说明不能超过2000字符';if(!/^[A-Za-z0-9_-]{1,100}$/.test(step.id))return '步骤 ID 仅支持字母、数字、下划线和连字符，最长 100 字符';if(ids.has(step.id))return '步骤 ID 重复：'+step.id;ids.add(step.id);if(!step.command.trim())return step.id+'：请输入脚本';if(!['bash','python','command'].includes(step.kind))return step.id+'：请选择支持的脚本类型';if(new TextEncoder().encode(step.command).length>131072)return step.id+'：脚本超过 128 KiB';}
  for(const step of steps){if(step.depends_on.some(id=>!ids.has(id)))return step.id+'：前置步骤不存在';for(const [name,id] of Object.entries(step.inputs||{})){if(!/^[A-Z][A-Z0-9_]{0,63}$/.test(name)||!step.depends_on.includes(id))return step.id+'：输出绑定必须使用合法 ENV 名称并引用直接前置步骤';}}
  const done=new Set<string>();while(done.size<steps.length){const next=steps.find(s=>!done.has(s.id)&&s.depends_on.every(id=>done.has(id)));if(!next)return '步骤依赖形成循环';done.add(next.id)}return '';
}
