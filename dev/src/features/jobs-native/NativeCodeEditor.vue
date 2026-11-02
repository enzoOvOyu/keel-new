<template><ElConfigProvider namespace="kel"><div class="native-code-editor"><KsEditor :modelValue="source" :readOnly="readOnly" :path="path" :lang="language || 'yaml'" :navbar="true" :theme="theme" :options="{fullHeight:true,lineNumbers:true,wordWrap:false,editor:{automaticLayout:true}}" @update:modelValue="value=>emit('change',value)" @save="emit('save')"/></div></ElConfigProvider></template>
<script setup lang="ts">
import {ElConfigProvider} from 'element-plus';
import KsEditor from '../../vendor/kestra/design-system/components/Form/KsEditor.vue';
defineProps<{source:string;readOnly:boolean;path:string;theme:'dark'|'light';language?:'shell'|'python'|'plaintext'}>();
const emit=defineEmits(['change','save']);
</script>
<style scoped>.native-code-editor{height:540px;min-height:360px;background:var(--ks-bg-surface)}.native-code-editor :deep(.ks-editor){height:100%}</style>
