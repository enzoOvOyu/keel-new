import {z} from 'zod';
export const stageIds=['KICKOFF','NETWORK_READINESS','PROVISIONING','VALIDATION','NCCL_VALIDATION','CLEANUP_HANDOVER'] as const;
export type StageId=typeof stageIds[number];
export const stageLabels:Record<StageId,string>={KICKOFF:'开局配置',NETWORK_READINESS:'网络就绪',PROVISIONING:'装机',VALIDATION:'验机',NCCL_VALIDATION:'NCCL 测试',CLEANUP_HANDOVER:'优雅退场'};
export const stageStateSchema=z.enum(['NOT_STARTED','READY','RUNNING','BLOCKED','FAILED','PASSED','WAIVED']);
export const stageCommandSchema=z.object({project_id:z.string().min(1),stage_id:z.enum(stageIds),target_ids:z.array(z.string()).min(1),expected_revisions:z.record(z.string(),z.number().int()).optional(),idempotency_key:z.string().min(1).optional()});
export const validationCheckLabels={hardware_inventory:'硬件在位与健康',gpu_nic_inventory:'GPU/NIC及配对',pcie_topology:'PCIe拓扑',nic_mac:'网卡与MAC',lldp_line_order:'LLDP线序',optics:'光模块与DDM',config_consistency:'配置一致性',roce_connectivity:'RoCE连通性'};
