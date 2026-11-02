<template><div ref="root" class="native-react-panel"/></template>
<script setup lang="ts">
import {ref,onMounted,onBeforeUnmount} from 'vue';
const props=defineProps<{name:string;attach:(name:string,element:HTMLElement|null)=>void}>();
const root=ref<HTMLElement|null>(null);
onMounted(()=>props.attach(props.name,root.value));onBeforeUnmount(()=>props.attach(props.name,null));
</script>
<style scoped>.native-react-panel{height:100%;min-height:0;overflow:auto}.native-react-panel :deep(.fl-files-grid){min-height:440px}</style>
