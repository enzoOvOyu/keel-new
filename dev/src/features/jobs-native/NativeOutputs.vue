<template><ElConfigProvider namespace="kel"><section class="native-outputs"><SidebarList :sections="sections" :selectedExpression="selected?.expression" @select="item=>expression=item.expression"/><main><template v-if="selected"><header><div><h3>{{selected.label}}</h3><code>{{selected.expression}}</code></div><KsButton @click="copy">复制值</KsButton><KsButton @click="download">下载 JSON</KsButton></header><p v-if="error" role="alert">{{error}}</p><small>{{selected.type}} · 来自当前 Execution 的实际输出</small><pre>{{serialized.slice(0,65536)}}</pre><p v-if="serialized.length>65536">预览已截断，下载可获取完整返回值。</p></template><KsNoData v-else title="当前执行尚无输出"/></main></section></ElConfigProvider></template>
<script setup lang="ts">
import {computed,ref} from 'vue';
import {ElConfigProvider} from 'element-plus';
import SidebarList from '../../vendor/kestra/outputs/SidebarList.vue';
import KsButton from '../../vendor/kestra/design-system/components/Basic/KsButton/KsButton.vue';
import KsNoData from '../../vendor/kestra/design-system/components/Data/KsNoData.vue';
const props=defineProps<{outputs:Record<string,unknown>}>(),expression=ref(''),error=ref('');
const items=computed(()=>Object.entries(props.outputs||{}).map(([label,value])=>({label,value,expression:'outputs['+JSON.stringify(label)+']',type:value===null?'null':Array.isArray(value)?'array':typeof value,preview:JSON.stringify(value)?.slice(0,100)||String(value)})));
const sections=computed(()=>[{key:'outputs',label:'任务输出',items:items.value}]);
const selected=computed(()=>items.value.find(i=>i.expression===expression.value)||items.value[0]);
const serialized=computed(()=>selected.value?JSON.stringify(selected.value.value,null,2)||String(selected.value.value):'');
async function copy(){try{await navigator.clipboard.writeText(serialized.value);error.value=''}catch{error.value='浏览器未允许剪贴板写入，可下载完整 JSON'}}
function download(){const url=URL.createObjectURL(new Blob([serialized.value],{type:'application/json'}));const a=document.createElement('a');a.href=url;a.download='execution-output.json';a.click();URL.revokeObjectURL(url)}
</script>
<style scoped>.native-outputs{display:grid;grid-template-columns:280px minmax(0,1fr);height:580px;background:var(--ks-bg-surface);color:var(--ks-text-primary);font-size:14px}.native-outputs>main{min-width:0;padding:20px;overflow:auto;border-left:1px solid var(--ks-border-default)}header{display:flex;align-items:center;gap:8px;margin-bottom:20px}header>div{flex:1;min-width:0}h3{margin:0;font-size:16px}code,small{color:var(--ks-text-secondary);overflow-wrap:anywhere;font-size:12px}pre{white-space:pre-wrap;overflow-wrap:anywhere;font:12px/1.7 var(--ks-font-family-mono);padding:16px;background:var(--ks-bg-base);margin-top:12px}@media(max-width:900px){.native-outputs{grid-template-columns:200px 1fr}}</style>
