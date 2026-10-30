# Keel IA v2.4 Full Demo Dataset

## Purpose
用于从零构建 Keel 全量可交互前端演示版，并为后续 Go API 接入提供稳定、确定性的演示数据。主项目为 64 台 B300 / 512 GPU，另有两个跨项目运维上下文样本。

## Files
- `demo-seed.json`: 统一数据源，所有 collection 均为 JSON 数组。
- `counts.json`: collection 数量与数据集元信息。
- `seed-demo.sql`: 将所有 collection 写入隔离的 PostgreSQL `keel_demo.demo_records`，便于先起 UI/API Mock。它不是最终生产数据库迁移。
- `manifest.json`: 数据集版本、主项目和校验摘要。

## Load
仅在专用演示数据库运行：
```bash
psql "$KEEL_DEMO_DATABASE_URL" -v ON_ERROR_STOP=1 -f fixtures/demo-full-64-b300/seed-demo.sql
```
SQL 只删除并重建 `keel_demo` schema。不要在生产数据库执行。

## Data rules
- 主项目 `proj-b300-demo` 固定 64 台服务器、512 GPU、24 台交换机、16 个机柜。
- 其他两个项目用于验证 Cross-project 运维视图和 ProjectSwitcher；它们是汇总上下文，不额外伪造真实 B300 服务器。
- 所有 IP 使用文档示例网段或其他保留/演示地址；MAC、SN、UUID、日志、指标均为合成值。
- AI 识别结果、模型准确率和设备观测值均为演示数据，不代表真实模型评测或真实硬件读数。
- 任何设备操作、脚本执行、配置下发、固件升级和 BMC 控制均为 mock。
- 不包含源 Excel 中的真实凭据、SN、MAC、IP 或个人信息。

## UI coverage
部署平面：项目、Provision/BOM/PXE/BIOS/Firmware、Validation/G1-G6/PCIe/LLDP、NetTest、Config、Fabric/ZTP/Golden/PushJob/Lossless/DDM/Logs/Rules、NCCL、Cleanup。
运维平面：Dashboard、Fleet、Rack global + rack elevation、Issues、Monitor、Flows/Executions、CMDB 360、AI。
系统：Users/RBAC、Templates、Device Models、Parameters、Media、Audit、SSH trust。

## API adapter
前端按 OpenAPI 和 DOMAIN-API-CONTRACTS 访问资源。Mock adapter 从 `demo-seed.json` 读取 collection；组件不得直接 import JSON。所有变更走统一 Mock API，更新相关集合并写入 `audit_events` / `lifecycle_events`。

`workbook-profile.json` 提供真实工作簿 33 个 Sheet 的名称、行列规模和候选用途；只保留结构元数据，不包含单元格内容或凭据。候选用途需要人工确认。
