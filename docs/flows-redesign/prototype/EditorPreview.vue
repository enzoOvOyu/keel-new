<template><div class="jp-editor"><MultiPanelGenericEditorView ref="editorView" :editorElements="elements" :defaultActiveTabs="['visual','topology']" :saveKey="'keel-design-preview-editor-v1:'+scopeKey"><template #actions><KsButton size="small" @click="openFiles('new')"><Plus :size="14"/>新增脚本</KsButton><KsButton size="small" @click="openFiles('import')">导入脚本</KsButton></template></MultiPanelGenericEditorView><KsDialog v-model="adding" title="添加任务" :teleported="false" :appendToBody="false" width="480px"><div class="jp-task-picker"><button v-for="item in catalog" :key="item.type" @click="add(item.type)"><component :is="item.icon" :size="20"/><span><b>{{item.name}}</b><small>{{item.description}}</small></span><Plus :size="16"/></button></div></KsDialog></div></template>
<script setup lang="ts">
import {computed,defineComponent,defineAsyncComponent,h,markRaw,nextTick,ref} from 'vue';
import YAML from 'yaml';
import MultiPanelGenericEditorView from '../../../dev/src/vendor/kestra/editor/components/MultiPanelGenericEditorView.vue';
import BlockSectionCard from '../../../dev/src/vendor/kestra/no-code/BlockSectionCard.vue';
import LeafBlockCard from '../../../dev/src/vendor/kestra/no-code/LeafBlockCard.vue';
import BlockEmptyDrop from '../../../dev/src/vendor/kestra/no-code/BlockEmptyDrop.vue';
import KsDialog from '../../../dev/src/vendor/kestra/design-system/components/Feedback/KsDialog.vue';
import KsButton from '../../../dev/src/vendor/kestra/design-system/components/Basic/KsButton/KsButton.vue';
import CodeTags from 'vue-material-design-icons/CodeTags.vue';
import ViewSequential from 'vue-material-design-icons/ViewSequential.vue';
import VectorPolyline from 'vue-material-design-icons/VectorPolyline.vue';
import FolderOutline from 'vue-material-design-icons/FolderOutline.vue';
import BookOpenOutline from 'vue-material-design-icons/BookOpenOutline.vue';
import Plus from 'vue-material-design-icons/Plus.vue';
import Console from 'vue-material-design-icons/Console.vue';
import CallSplit from 'vue-material-design-icons/CallSplit.vue';
import FileDocumentOutline from 'vue-material-design-icons/FileDocumentOutline.vue';
import ScriptFilesPreview from './ScriptFilesPreview.vue';
import {scriptTask,scriptTemplate,languageFor,type ScriptFile,type ScriptUse} from './script-files';
const props=defineProps<{source:string;theme:'dark'|'light';selected:string;scopeKey:string;files:Record<string,ScriptFile>}>();
const emit=defineEmits<{change:[source:string];select:[id:string];save:[];filesChange:[files:Record<string,ScriptFile>]}>();
const codeComponent=defineAsyncComponent(async()=>{await import('../../../dev/src/features/jobs-native/monaco-workers');return (await import('../../../dev/src/features/jobs-native/NativeCodeEditor.vue')).default});
const adding=ref(false),dragged=ref(''),error=ref(''),filePath=ref('scripts/preflight.sh'),createRequest=ref(0),importRequest=ref(0),editorView=ref<InstanceType<typeof MultiPanelGenericEditorView>|null>(null);
const catalog=[{name:'Shell Script',type:'io.kestra.plugin.scripts.shell.Script',description:'新增可在线编辑的 Shell 脚本任务',icon:Console},{name:'Python Script',type:'io.kestra.plugin.scripts.python.Script',description:'新增可在线编辑的 Python 脚本任务',icon:CodeTags},{name:'Log',type:'io.kestra.plugin.core.log.Log',description:'记录结构化运行日志',icon:Console},{name:'Parallel',type:'io.kestra.plugin.core.flow.Parallel',description:'并行组织原生任务',icon:CallSplit},{name:'Output Values',type:'io.kestra.plugin.core.output.OutputValues',description:'保存任务输出',icon:FileDocumentOutline}];
const doc=computed(()=>{try{return YAML.parse(props.source)||{}}catch{return null}});
const tasks=computed(()=>doc.value?.tasks||[]);
const selected=computed(()=>tasks.value.find((t:any)=>t.id===props.selected)||tasks.value[0]);
function edit(fn:(d:any)=>void){try{const d=YAML.parse(props.source);fn(d);emit('change',YAML.stringify(d));error.value=''}catch(e){error.value=String(e)}}
function add(type:string){let n=tasks.value.length+1;while(tasks.value.some((t:any)=>t.id==='task_'+n))n++;const id='task_'+n;edit(d=>{d.tasks||=[];d.tasks.push({id,type,...(type.endsWith('.Script')?{script:scriptTemplate(type.includes('.python.')?'python':'shell')}:type.endsWith('.Log')?{message:'New task'}:type.endsWith('.Parallel')?{tasks:[]}:{values:{}})})});emit('select',id);adding.value=false}
function field(key:string,value:string){edit(d=>{const t=d.tasks.find((t:any)=>t.id===selected.value.id);t[key]=value})}
function reorder(target:string){if(!dragged.value||target===dragged.value)return;edit(d=>{const from=d.tasks.findIndex((t:any)=>t.id===dragged.value);const to=d.tasks.findIndex((t:any)=>t.id===target);if(from>=0&&to>=0){const [t]=d.tasks.splice(from,1);d.tasks.splice(to,0,t)}});dragged.value=''}
const visual=markRaw(defineComponent({setup:()=>()=>h('div',{class:'jp-visual'},[
 h('div',{class:'jp-blocks'},[h('div',{class:'jp-panel-caption'},[h('span','VISUAL EDITOR'),h('small',`${tasks.value.length} tasks`)]),
 doc.value?h(BlockSectionCard,{name:'tasks',title:'Tasks',icon:ViewSequential,count:tasks.value.length,addLabel:'添加任务',onAdd:()=>adding.value=true},{default:()=>[...tasks.value.map((t:any,i:number)=>h('div',{class:'jp-block-wrap',key:t.id},[h('span',{class:'jp-step-number'},String(i+1).padStart(2,'0')),h(LeafBlockCard,{block:t,path:`tasks[${i}]`,label:t.id,selected:props.selected===t.id,focused:props.selected===t.id,draggable:true,showOpenSplit:false,showDuplicate:false,runnable:false,onSelect:()=>emit('select',t.id),'onDrag-start':()=>dragged.value=t.id,'onDrop':()=>reorder(t.id),'onDrag-end':()=>dragged.value='',onDelete:()=>edit(d=>{d.tasks=d.tasks.filter((n:any)=>n.id!==t.id)})})])),h(BlockEmptyDrop,{label:'任务',hint:'同一份 YAML，双向同步',onAdd:()=>adding.value=true})]}):h('p',{class:'jp-error'},'YAML 有误，请在代码面板修正'),h('div',{class:'jp-editor-note'},'规划、编辑与预览不会触发设备执行。')]),
 h('aside',{class:'jp-inspector'},[h('div',{class:'jp-panel-caption'},'PROPERTIES'),selected.value?h('div',[h('div',{class:'jp-inspector-kind'},[h(Console,{size:22}),h('strong',selected.value.type.split('.').at(-1))]),h('label',[h('span','Task ID'),h('input',{value:selected.value.id,readonly:true})]),h('label',[h('span','Type'),h('code',selected.value.type)]),h('label',[h('span','Description'),h('input',{value:selected.value.description||'',placeholder:'为任务补充说明',onChange:(e:any)=>field('description',e.target.value)})]),selected.value.type.includes('.scripts.')?h('div',[h('label',[h('span','Script source'),h('code',selected.value.namespaceFiles?.include?.[0]||'Inline script')]),h(KsButton,{size:'small',onClick:editSelectedScript},{default:()=> '在脚本编辑器中打开'})]):selected.value.type.endsWith('.Log')?h('label',[h('span','Message'),h('textarea',{'aria-label':'Task message',rows:5,value:selected.value.message||'',onChange:(e:any)=>field('message',e.target.value)})]):h('div',{class:'jp-code-hint'},selected.value.tasks?`包含 ${selected.value.tasks.length} 个子任务，完整定义可在 YAML 查看。`:'结构化属性保留在同一 YAML 定义。'),h('label',[h('span','Timeout'),h('input',{placeholder:'PT10M',value:selected.value.timeout||'',onChange:(e:any)=>field('timeout',e.target.value)})]),h('small',{class:'jp-muted'},'参数修改即时同步 YAML 与拓扑。')]):h('p','选择一个节点以编辑属性'),error.value?h('p',{class:'jp-error'},error.value):null])
 ])}));
const topology=markRaw(defineComponent({setup:()=>()=>h('div',{class:'jp-editor-graph'},[h('div',{class:'jp-panel-caption'},[h('span','TOPOLOGY'),h('span',{class:'jp-dot-grid-label'},'自动布局')]),h('div',{class:'jp-topology-flow'},[h('div',{class:'jp-flow-terminal'},'START'),...tasks.value.map((t:any,i:number)=>h('div',{class:'jp-topology-step',key:t.id},[h('div',{class:'jp-connector'}),h('button',{class:['jp-topology-node',props.selected===t.id?'selected':''],onClick:()=>emit('select',t.id)},[h('span',{class:'jp-node-icon'},[h(t.type.endsWith('.Parallel')?CallSplit:Console,{size:20})]),h('span',[h('b',t.id),h('small',t.type.split('.').at(-1))]),h('span',{class:'jp-node-index'},i+1)]),...(t.tasks?.length?[h('div',{class:'jp-child-nodes'},t.tasks.map((c:any)=>h('span',{key:c.id},[h(Console,{size:14}),c.id])))]:[])])),h('div',{class:'jp-connector'}),h('div',{class:'jp-flow-terminal'},'END')]),h('div',{class:'jp-canvas-legend'},[h('span',{class:'jp-small-dot'}),'定义预览 · 点击节点编辑'])])}));
const code=markRaw(defineComponent({setup:()=>()=>h(codeComponent,{source:props.source,path:'flow.yaml',readOnly:false,theme:props.theme,onChange:(s:string)=>emit('change',s),onSave:()=>emit('save')})}));
const canReplace=computed(()=>/^io\.kestra\.plugin\.scripts\.(shell|python)\.(Script|Commands)$/.test(selected.value?.type||''));
async function openFiles(action?:'new'|'import'){
 const view=editorView.value;if(!view)return;
 if(view.openTabs.includes('files'))view.focusTab('files');
 else{const tab=elements.find(e=>e.uid==='files')!;const panel=view.panels.at(-1);if(panel){panel.tabs.push(tab);panel.activeTab=tab}else view.setTabValue('files')}
 await nextTick();if(action==='new')createRequest.value++;if(action==='import')importRequest.value++;await nextTick();createRequest.value=0;importRequest.value=0;
}
function createFiles(entries:{path:string;content:string}[]){const files={...props.files};for(const e of entries)files[e.path]={content:e.content,saved:null,revision:0};emit('filesChange',files);if(entries.length)filePath.value=entries[0].path}
function editFile(path:string,content:string){emit('filesChange',{...props.files,[path]:{...props.files[path],content}})}
function saveFile(path:string){const current=props.files[path];if(current)emit('filesChange',{...props.files,[path]:{...current,saved:current.content,revision:current.revision+1}})}
function useFile(value:ScriptUse){const file=props.files[value.path];if(!file||file.saved===null||file.content!==file.saved)return;
 let id=value.target==='selected'?selected.value?.id:'run_'+value.path.split('/').at(-1)!.replace(/\.[^.]+$/,'').replace(/[^A-Za-z0-9_]/g,'_');
 if(value.target==='new'){const base=id;let n=2;while(tasks.value.some((t:any)=>t.id===id))id=base+'_'+n++}
 const task=scriptTask(value.path,value.mode,file.saved,id);edit(d=>{if(value.target==='selected'&&canReplace.value){const i=d.tasks.findIndex((t:any)=>t.id===id);const {script,commands,namespaceFiles,type,...preserved}=d.tasks[i];d.tasks[i]={...preserved,...task}}else d.tasks.push(task)});emit('select',id);
}
function editSelectedScript(){if(!canReplace.value)return;const t=selected.value;const path=t.namespaceFiles?.include?.[0]||'scripts/'+t.id+(t.type.includes('.python.')?'.py':'.sh');
 if(!props.files[path])createFiles([{path,content:t.script||scriptTemplate(t.type.includes('.python.')?'python':'shell')}]);filePath.value=path;void openFiles();
}
const file=markRaw(defineComponent({setup:()=>()=>h(ScriptFilesPreview,{files:props.files,active:filePath.value,theme:props.theme,createRequest:createRequest.value,importRequest:importRequest.value,canReplace:canReplace.value,canUse:!!doc.value&&Array.isArray(doc.value.tasks),onSelect:(path:string)=>filePath.value=path,onCreate:createFiles,onEdit:editFile,onSave:saveFile,onUse:useFile})}));
const docs=markRaw(defineComponent({setup:()=>()=>h('article',{class:'jp-docs'},[h('div',{class:'jp-eyebrow'},'TASK DOCUMENTATION'),h('h2',selected.value?.type?.split('.').at(-1)||'Flow'),h('code',selected.value?.type||'请选择任务'),h('p','选择节点后在这里查看对应任务的说明与配置示例。此原型仅展示已列出的核心任务，不代表完整插件目录。'),h('h3','属性'),h('dl',[h('dt','id'),h('dd','任务的稳定标识，在 Flow 内唯一。'),h('dt','type'),h('dd','实际任务类型，不随显示名称变化。'),h('dt','timeout'),h('dd','可选的 ISO 8601 持续时间，例如 PT10M。')]),h('h3','示例'),h('pre',YAML.stringify(selected.value||{}))])}));
const elements=[{uid:'visual',button:{label:'Visual',icon:ViewSequential},component:visual},{uid:'code',button:{label:'YAML',icon:CodeTags},component:code},{uid:'topology',button:{label:'Topology',icon:VectorPolyline},component:topology},{uid:'files',button:{label:'Files / 脚本',icon:FolderOutline},component:file},{uid:'docs',button:{label:'Docs',icon:BookOpenOutline},component:docs}].map(e=>({...e,button:{...e.button,icon:markRaw(e.button.icon)},deserialize:(id:string)=>id===e.uid?e:undefined}));
</script>
