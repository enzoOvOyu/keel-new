<template><ElConfigProvider namespace="kel"><div class="native-workbench"><MultiPanelGenericEditorView :editorElements="elements" :defaultActiveTabs="['code','outline']" :saveKey="scopeKey?`keel-native-editor:${scopeKey}`:undefined"/><KsDialog v-model="isKeyShortcutsDialogShown" :teleported="false" :appendToBody="false" title="编辑器快捷键"><dl><dt>Ctrl / ⌘ + S</dt><dd>保存草稿</dd><dt>Ctrl / ⌘ + F</dt><dd>查找内容</dd><dt>Ctrl / ⌘ + H</dt><dd>查找替换</dd></dl></KsDialog></div></ElConfigProvider></template>
<script setup lang="ts">
import {computed,defineComponent,h,markRaw} from 'vue';
import YAML from 'yaml';
import {ElConfigProvider} from 'element-plus';
import MultiPanelGenericEditorView from '../../vendor/kestra/editor/components/MultiPanelGenericEditorView.vue';
import {useKeyShortcuts} from '../../vendor/kestra/editor/utils/useKeyShortcuts';
import NativeCodeEditor from './NativeCodeEditor.vue';
import KsDialog from '../../vendor/kestra/design-system/components/Feedback/KsDialog.vue';
import CodeTags from 'vue-material-design-icons/CodeTags.vue';
import FileTree from 'vue-material-design-icons/FileTree.vue';
import {taskList} from '../flows/definition';
const props=defineProps<{source:string;readOnly:boolean;path:string;theme:'dark'|'light';scopeKey?:string}>();
const emit=defineEmits(['change','save']);
const {isKeyShortcutsDialogShown}=useKeyShortcuts();
const structure=computed(()=>{try{const d=YAML.parse(props.source);return {id:d?.id,namespace:d?.namespace,tasks:taskList(d||{})}}catch{return null}});
const code=markRaw(defineComponent({setup:()=>()=>h(NativeCodeEditor,{...props,onChange:(s:string)=>emit('change',s),onSave:()=>emit('save')})}));
const outline=markRaw(defineComponent({setup:()=>()=>h('section',{class:'native-outline'},[h('h3','Flow 结构'),structure.value?h('div',[h('p',String(structure.value.id||'未命名')),h('small',String(structure.value.namespace||'')),...structure.value.tasks.map((t:any)=>h('div',{class:'outline-task',key:t.id},[h('strong',t.id),h('small',t.type)]))]):h('p','YAML 语法无效，修正后显示结构')])}));
const elements=[{uid:'code',button:{label:'YAML',icon:markRaw(CodeTags)},component:code},{uid:'outline',button:{label:'Flow 结构',icon:markRaw(FileTree)},component:outline}].map(e=>({...e,deserialize:(id:string)=>id===e.uid?e:undefined}));
</script>
<style>.native-workbench{height:600px;min-height:400px;background:var(--ks-bg-surface);color:var(--ks-text-primary)}.native-workbench .native-code-editor{height:100%}.native-outline{padding:16px;overflow:auto;height:100%;font-size:12px}.native-outline h3{font-size:14px;margin:0 0 12px}.native-outline small{display:block;color:var(--ks-text-secondary);overflow-wrap:anywhere}.outline-task{padding:12px 0;border-bottom:1px solid var(--ks-border-default)}</style>
