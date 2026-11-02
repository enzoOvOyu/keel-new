<template><div ref="root" class="native-diff-editor" aria-label="Flow 修订差异"/></template>
<script setup lang="ts">
import {onMounted,onBeforeUnmount,ref,watch} from 'vue';import * as monaco from 'monaco-editor';
const props=defineProps<{original:string;modified:string}>(),root=ref<HTMLElement|null>(null);let editor:monaco.editor.IStandaloneDiffEditor|undefined,original:monaco.editor.ITextModel|undefined,modified:monaco.editor.ITextModel|undefined,observer:MutationObserver|undefined;
function theme(){monaco.editor.setTheme(document.documentElement.classList.contains('ia-dark')||document.documentElement.classList.contains('dark')?'vs-dark':'vs')}
onMounted(()=>{original=monaco.editor.createModel(props.original,'yaml');modified=monaco.editor.createModel(props.modified,'yaml');editor=monaco.editor.createDiffEditor(root.value!,{readOnly:true,automaticLayout:true,renderSideBySide:true,minimap:{enabled:false},fontSize:12,lineNumbers:'on',scrollBeyondLastLine:false,originalAriaLabel:'历史修订',modifiedAriaLabel:'当前草稿',renderOverviewRuler:false});editor.setModel({original,modified});theme();observer=new MutationObserver(theme);observer.observe(document.documentElement,{attributes:true,attributeFilter:['class']})});watch(()=>props.original,v=>original?.setValue(v));watch(()=>props.modified,v=>modified?.setValue(v));onBeforeUnmount(()=>{observer?.disconnect();editor?.dispose();original?.dispose();modified?.dispose()});
</script>
<style scoped>.native-diff-editor{height:calc(100vh - 355px);min-height:420px;position:relative;overflow:hidden}</style>
