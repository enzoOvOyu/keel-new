<template><div class="jp-script-code"><KsEditor :modelValue="source" :path="path" :lang="language" :navbar="false" :readOnly="false" :theme="theme" :options="{fullHeight:true,lineNumbers:true,editor:{automaticLayout:true,fontSize:12,minimap:{enabled:false}}}" @update:modelValue="v=>emit('change',v)" @save="emit('save')"/></div></template>
<script setup lang="ts">
import KsEditor from '../../../dev/src/vendor/kestra/design-system/components/Form/KsEditor.vue';
import '../../../dev/src/features/jobs-native/monaco-workers';
defineProps<{source:string;path:string;language:'shell'|'python'|'plaintext';theme:'dark'|'light'}>();
const emit=defineEmits<{change:[source:string];save:[]}>();
</script>
<style scoped>.jp-script-code{height:100%;min-height:240px;position:relative}.jp-script-code :deep(.ks-editor){height:100%}</style>
