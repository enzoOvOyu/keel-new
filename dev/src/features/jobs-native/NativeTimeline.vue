<template><ElConfigProvider namespace="kel"><RunTimeline v-if="histories.length" :histories="histories"/><span v-else class="timeline-empty">引擎尚未提供状态历史</span></ElConfigProvider></template>
<script setup lang="ts">
import {ElConfigProvider} from 'element-plus';
import RunTimeline from '../../vendor/kestra/RunTimeline.vue';
defineProps<{histories:{date:string;state:string}[]}>();
</script>
