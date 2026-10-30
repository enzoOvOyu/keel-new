# Keel GPU 交付 Mock 平台

启动：`npm ci && npm run dev`，访问 http://localhost:5180 。服务绑定 0.0.0.0，远程使用时通过当前工作区端口转发访问。可选 `docker compose up -d`（先停止同端口的宿主 Vite）。

## 页面与操作

- 交付总览：阶段轨道、关键阻断、活动执行、16柜健康矩阵；项目交付与退场有独立页面。
- BOM：独立导入任务路由，文件摘要、固定模拟解析、Sheet来源导航、字段/端点修正、警告确认、重新校验、预览、提交与幂等冲突。
- CMDB与设备：分类目录、稳定身份、硬件/端口、连接、生命周期、作业与证据；PXE/带外/固件为带权限和确认的模拟作业。
- 机柜图：多柜 SVG、正反面、缩放/平移、拖放/键盘、冲突拒绝、撤销/重做、草稿/不可变发布版本、差异、SVG/PNG/打印PDF。
- 网络：设备/ZTP、批准基线、13态PushJob操作、独立NE/DE/NH签署、执行后复采与回滚、线缆/端口/能力状态。
- 验机与Mapping：对象级G1–G6、NP前置、三方线序、逐GPU-NIC候选、人工确认和限定范围发布、ConfigSet、RoCE/NCCL证据。
- 工作流：目录、独立编辑器、脚本、策略、调度、目标勾选与冻结、独立执行详情、Gantt/Topology/Targets/Logs/Outputs、失败目标重试、未知结果核对。
- 工单：调查队列、证据、责任人、离线来源引用建议、关闭校验、定向重验。
- 监控、模板/型号、介质、参数、权限、审计、设置均有对应的工作区与操作。

## 数据

`fixtures/demo-64-b300/demo-seed.json` 保留原始种子，`expanded-seed.json` 是确定性孵化的覆盖层；由统一 Mock Provider 合并装载。硬件范围保持 64 B300 / 512 GPU / 640 NIC / 16机柜 / 24交换机 / 704线缆。

扩充后：48执行、1824目标任务、3648日志、40产物、512映射候选、384步骤报告、384对象级门禁、35工单、18告警、7952 CMDB实体、9168关系。候选映射明确为 needs_review，不按序号自动当成正式Mapping。所有观测和结论为合成场景。

`npm run incubate:data` 确定性再生成扩充JSON；`npm run check:expanded` 检查范围、ID、映射端点、目标快照、计数与CMDB引用。原始 SQL 仍只装载原始种子，本演示不运行 SQL 或连接真实数据库。

## 验证

- `npm run build`：TypeScript与Vite。
- `npm run check:data`：原始数据检查。
- `npm run check:expanded`：扩充数据检查。
- `npm run check:browser`：浏览器路径、业务闭环、机柜导出和布局检查。首次 `npx playwright install chromium`。
- 实际报告：`docs/P0-CHECK.json`、`docs/PRODUCT-CHECK.json`、`docs/EXPANSION-CHECK.json`、`docs/FINAL-UI-CHECK.json`；截图 `docs/final-*.png`。

## 边界

这是前端 Mock 演示，不是生产后端能力。上传仅记录摘要并用固定合成数据模拟解析；不执行真实Excel解析、设备命令、AI或固件刷写。刷新恢复固定数据，顶部可切换身份、场景和重置。真实HTTP契约的完整运行时验证、生产状态持久化、真实设备与后端接入均不在本演示实现范围；不能据此声称真机或生产规则已验收。后续运维域设计更新也不代表新增运维功能已经实现。
