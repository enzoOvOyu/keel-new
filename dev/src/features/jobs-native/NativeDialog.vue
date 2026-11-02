<template><ElConfigProvider namespace="kel"><KsDialog :modelValue="open" :teleported="false" :appendToBody="false" :title="title" width="1000px" @update:modelValue="v=>{if(!v)emit('close')}"><p v-if="description" class="native-dialog-description">{{description}}</p><NativePanelSlot name="content" :attach="attach"/></KsDialog></ElConfigProvider></template>
<script setup lang="ts">
import {ElConfigProvider} from 'element-plus';import KsDialog from '../../vendor/kestra/design-system/components/Feedback/KsDialog.vue';import NativePanelSlot from './NativePanelSlot.vue';defineProps<{open:boolean;title:string;description:string;attach:(name:string,element:HTMLElement|null)=>void}>();const emit=defineEmits(['close']);
</script>
<style scoped>.native-dialog-description{font-size:11px;line-height:1.6;color:var(--ks-text-secondary);margin:0 0 16px}.native-react-panel{max-height:70vh;font-size:12px;color:var(--ks-text-primary)}</style>
