import type { SourceUnderstanding } from './api';

export const understandingStepIds = ['structure', 'regions', 'semantics', 'relationships', 'fields'] as const;
export const terminationNames = {structure:'端接资料定位',regions:'设备与端口识别',semantics:'关系推理与佐证',relationships:'位置与线缆关联',fields:'候选完整性核对'};
export const understandingStates = {complete:'已完成',partial:'部分完成',pending:'待处理',unsupported:'暂无结果',not_applicable:'不适用'};

// The version is supplied by the backend; never reinterpret historical counts.
export function understandingStepName(_report: SourceUnderstanding | undefined, id: SourceUnderstanding['steps'][number]['id']) {
  return terminationNames[id];
}
export function understandingDescription(report: SourceUnderstanding | undefined) {
  if (!report || (!report.flowVersion && report.origin !== 'history' && report.steps.every(step => step.state === 'pending' || step.state === 'unsupported'))) {
    return '等待分析证据；尚无可确认的阶段结果。';
  }
  return report.flowVersion === 'termination-v1'
    ? '端接资料识别 · 五步按实际执行证据展示；候选需人工采用、核对后发布。'
    : '等待端接分析，生成设备、端口与连接候选。';
}
export const candidateFieldLabels: Record<string, string> = {
  device_ref:'设备名称', a_device_ref:'A端设备名称', b_device_ref:'B端设备名称',
  a_serial:'A端SN（可选）', b_serial:'B端SN（可选）',
  a_model:'A端设备型号', b_model:'B端设备型号', a_role:'A端设备角色', b_role:'B端设备角色',
  a_equipment_type:'A端设备类型', b_equipment_type:'B端设备类型', a_mac:'A端MAC地址', b_mac:'B端MAC地址',
  cable_length_m:'线缆长度（米）',
  sn:'SN（可选）', serial:'SN（可选）', device_sn:'设备SN（可选）', a_device_sn:'A端SN（可选）', b_device_sn:'B端SN（可选）',
};
export const identityGuidance = '无 SN 时以资料中的设备名称识别身份；SN 可选。缺 MAC、线缆长度或位置可后补，身份冲突与端点歧义仍需核对。';
