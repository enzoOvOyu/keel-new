<template>
 <ElConfigProvider namespace="kel">
  <section class="jobs-native">
   <header><div><h1>{{ executionMode ? 'Executions' : '作业平台' }}</h1><p>{{ executionMode ? '执行记录与设备结果' : 'Flows' }}</p></div><div class="actions"><KsButton :loading="loading" @click="emit('refresh')">刷新</KsButton><KsButton v-if="!executionMode" :disabled="!canExecute" @click="emit('quickRun')">Quick Run</KsButton><KsButton v-if="!executionMode" type="primary" :disabled="!canCreate" @click="emit('create')">Create Flow</KsButton></div></header>
   <div v-if="error" class="error" role="alert">{{ error }}</div>
   <KsDataTable :data="visible" :total="filtered.length" :loading="loading" :currentPage="page" :pageSize="size" :rowKey="'id'" :fitHeight="true" :loadData="loadPage" :selectable="false" noDataText="暂无符合条件的记录" @row-click="row=>emit('open',row)">
    <template #navbar><div class="filters"><KsInput v-model="search" placeholder="搜索名称、ID、Namespace或来源" clearable @update:modelValue="page=1"/><select v-model="status" aria-label="状态筛选" @change="page=1"><option value="">全部状态</option><option v-for="s in statuses" :key="s">{{s}}</option></select><KsButton v-if="search||status" @click="search='';status='';page=1">清除筛选</KsButton><span>{{ filtered.length }} 条{{executionMode?'已加载执行':''}}</span></div></template>
    <KsTableColumn prop="name" :label="executionMode?'Execution / Flow':'Name / Flow ID'" minWidth="250"><template #default="{row}"><button class="row-link" @click.stop="emit('open',row)">{{ executionMode?row.id.slice(0,12):row.name }}</button><small>{{executionMode?row.engine_flow_id:row.flow_id}}</small></template></KsTableColumn>
    <KsTableColumn v-if="!executionMode" prop="description" label="Description" minWidth="180"/>
    <KsTableColumn :prop="executionMode?'project_id':'namespace'" :label="executionMode?'Project':'Namespace'" minWidth="160"/>
    <KsTableColumn v-if="!executionMode" prop="target_type" label="Target" width="100"/>
    <KsTableColumn v-if="!executionMode" prop="revision" label="Revision" width="85"/>
    <KsTableColumn v-if="executionMode" prop="source_module" label="Source" width="110"/>
    <KsTableColumn v-if="executionMode" label="Targets / Result" width="160"><template #default="{row}">{{row.targets.length}} targets<small>{{row.targets.filter(t=>t.state==='SUCCESS').length}} success / {{row.targets.filter(t=>t.state==='FAILED').length}} failed</small></template></KsTableColumn>
    <KsTableColumn v-if="executionMode" label="Mode / Transport" width="160"><template #default="{row}">{{row.mode}}<small>Agent {{row.targets.filter(t=>t.actual_transport==='AGENT').length}} / SSH {{row.targets.filter(t=>t.actual_transport==='SSH').length}}</small></template></KsTableColumn>
    <KsTableColumn v-if="executionMode" prop="created_at" label="Started" minWidth="160"><template #default="{row}">{{new Date(row.created_at).toLocaleString()}}</template></KsTableColumn>
    <KsTableColumn label="State" width="145"><template #default="{row}"><KsExecutionStatus v-if="executionMode&&row.engine?.state?.current" :status="row.engine.state.current" size="small"/><span v-else>{{rowState(row)}}</span></template></KsTableColumn>
    <KsTableColumn v-if="!executionMode" label="Actions" width="150"><template #default="{row}"><KsButton size="small" :disabled="!canExecute||row.disabled||row.draft" @click.stop="emit('run',row)">Run</KsButton><KsButton size="small" @click.stop="emit('edit',row)">Edit</KsButton></template></KsTableColumn>
   </KsDataTable>
   <footer v-if="executionMode&&hasMore"><span>当前仅包含已加载记录</span><KsButton :loading="loading" @click="emit('loadMore')">加载更早执行</KsButton></footer>
  </section>
 </ElConfigProvider>
</template>
<script setup lang="ts">
import {computed,ref,watch} from 'vue';
import {ElConfigProvider} from 'element-plus';
import KsDataTable from '../../vendor/kestra/design-system/components/Data/KsDataTable/KsDataTable.vue';
import KsTableColumn from '../../vendor/kestra/design-system/components/Data/KsTable/KsTableColumn.vue';
import KsButton from '../../vendor/kestra/design-system/components/Basic/KsButton/KsButton.vue';
import KsInput from '../../vendor/kestra/design-system/components/Form/KsInput.vue';
import KsExecutionStatus from '../../vendor/kestra/design-system/components/Data/KsExecutionStatus/KsExecutionStatus.vue';
const props=defineProps<{rows:any[];executionMode:boolean;loading:boolean;error:string;canCreate:boolean;canExecute:boolean;hasMore:boolean;query:string;stateFilter:string}>();
const emit=defineEmits(['refresh','quickRun','create','open','run','edit','loadMore','filter']);
const page=ref(1),size=ref(25);
const search=computed({get:()=>props.query,set:value=>emit('filter','q',value)});
const status=computed({get:()=>props.stateFilter,set:value=>emit('filter','state',value)});
const rowState=(row:any)=>props.executionMode?row.engine?.state?.current||row.error_code||'unknown':!row.engine||!Object.keys(row.engine).length?'unknown':row.draft?'DRAFT':row.disabled?'DISABLED':'ENABLED';
const statuses=computed(()=>[...new Set(props.rows.map(rowState))]);
const filtered=computed(()=>props.rows.filter(r=>(!status.value||rowState(r)===status.value)&&JSON.stringify([r.id,r.name,r.namespace,r.flow_id,r.engine_flow_id,r.source_module]).toLowerCase().includes(search.value.toLowerCase())));
const visible=computed(()=>filtered.value.slice((page.value-1)*size.value,page.value*size.value));
function loadPage(p:{page:number;size:number}){page.value=p.page;size.value=p.size}
watch(()=>[props.executionMode,props.query,props.stateFilter],()=>{page.value=1});
</script>
<style scoped>
.jobs-native{display:flex;flex-direction:column;min-height:650px;height:calc(100vh - 120px);color:var(--ks-text-primary);font-family:var(--ks-font-family-sans);font-size:var(--ks-font-size-base);background:var(--ks-bg-surface)}
header{display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--ks-border-default)}h1{font-size:24px;line-height:1.4;margin:0;font-weight:600}p{margin:3px 0 0;color:var(--ks-text-secondary);font-size:12px}.actions,.filters{display:flex;align-items:center;gap:8px}.filters{padding:12px 16px}.filters>.kel-input{max-width:360px}.filters>span{margin-left:auto;font-size:12px;color:var(--ks-text-secondary)}select{background:var(--ks-bg-input);border:1px solid var(--ks-border-default);border-radius:4px;padding:7px;color:inherit}small{display:block;color:var(--ks-text-secondary);font-size:11px;margin-top:4px}.row-link{font:inherit;font-weight:600;border:0;background:none;cursor:pointer;color:var(--ks-text-link)}.ks-data-table-wrapper{flex:1;min-height:0}.error{padding:12px;color:#d34359}footer{display:flex;align-items:center;justify-content:flex-end;gap:12px;padding:12px;font-size:12px}
</style>
