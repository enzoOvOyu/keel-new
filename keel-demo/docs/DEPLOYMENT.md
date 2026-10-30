# DEPLOYMENT.md — 开发、部署与运行约定

> 本文件定义最低运行约定，不替代目标环境的正式运维变更单。未确认的生产拓扑、域名、证书、资源规格和保留期限必须由部署负责人提供，不在代码中臆造。

## 1. 运行组件

开发环境按需运行：
- `web`：React + TypeScript 前端。
- `api`：Go HTTP API。
- `worker`：执行 Excel 解析、AI 映射、校验、导出和异步作业。
- PostgreSQL：业务状态和关系数据权威源。
- S3/MinIO：原始上传、解析产物、日志和报告。
- Redis/队列：仅在所选实现需要时用于缓存/协调/任务传递，不是业务状态权威源。

不要为每个业务域独立部署服务；API/worker 可从同一 Go 代码构建不同进程入口。

## 2. 配置管理
- 提供 `.env.example`，只包含变量名、说明和非秘密示例；禁止提交真实 `.env`。
- 配置至少分组：`APP_ENV`、HTTP bind、数据库 DSN、对象存储 endpoint/bucket、队列配置、AI provider/endpoint/model/timeout、认证/OIDC、日志级别和上传限制。
- 生产秘密通过 Secret Manager/Vault 注入；本地开发可用未提交的 `.env`。
- 启动时校验必需配置；错误只指出缺少的配置键，不打印 DSN 密码、API key 或完整秘密。
- AI provider 未配置或不可用时，已批准规则仍可执行；未知映射进入审核，不绕过校验。

## 3. 数据库迁移
- 所有 schema 变更使用有序、可审查的 SQL migration。
- 部署先备份并执行迁移，再启动兼容版本的应用；破坏性变更采用 expand/migrate/contract。
- 应用启动不得自动 drop/recreate 表；生产迁移失败必须停止发布并保留诊断信息。
- 迁移应支持空库初始化和升级路径；升级前后执行轻量一致性检查。

## 4. 文件与对象存储
- 原始导入文件不可变保存，按权限下载；派生预览、标准包、报告独立存储并记录来源引用。
- 上传文件设置可配置大小/解压/行列限制；拒绝路径穿越、符号链接、宏执行和外部链接访问。
- 对象存储 key 由服务端生成；不得把用户文件名直接拼接成路径。
- 备份数据库与对象存储时保留关联索引/版本；恢复演练需验证文件 hash 和引用可用性。

## 5. 启动与健康检查
- API 提供 `/health/live` 与 `/health/ready`。live 仅检查进程；ready 检查必要依赖连通性。
- Worker 提供可观测的启动状态、队列积压、活跃任务、失败任务和最后心跳。
- 日志使用结构化格式并包含 `request_id`、`trace_id`（若已接入）、project/resource refs；秘密必须脱敏。
- 关闭服务时停止接收新任务，允许安全任务完成/释放租约；运行中的副作用任务需要持久化状态并在恢复后对账。

## 6. 备份与恢复
- 数据库备份、对象存储版本/副本策略、保留周期和 RPO/RTO 由实际部署负责人确认并记录，不使用臆造默认值。
- 恢复演练验证：用户/项目、BOM 版本链、原始文件引用、审批/审计、Flow/Execution、设备证据和对象存储摘要均可追溯。
- 不能只证明数据库可启动；必须验证跨表与对象存储引用完整。

## 7. 发布顺序
1. 备份并确认目标环境/版本。
2. 应用数据库迁移。
3. 部署 API 和 worker 兼容版本。
4. 检查 readiness、队列、对象存储、数据库和日志。
5. 执行 smoke tests：登录/授权、创建导入任务、读取任务状态、预览、校验、拒绝不合法提交。
6. 只有批准后才启用真实设备写操作；开发/演示环境默认使用 simulator。
7. 记录版本、迁移号、smoke test 结果、回滚方案和操作者。

## 8. 回滚
- 应用版本回滚不得假设数据库 schema 可以自动逆向。按 migration 兼容策略回滚应用；数据修复需独立审批和备份。
- 生产网络配置回滚必须使用 PRD PushJob 的快照、审批和验证链路；不能将应用部署回滚等同于交换机配置回滚。
- 不确定设备副作用是否发生时，先对账，禁止盲目重放。

## 6. 前端先行开发环境

- `VITE_API_MODE=mock|http`，开发默认 `mock`；生产环境禁止 mock mode。
- `VITE_MOCK_SCENARIO=happy-path|bom-blocked|execution-running|...` 仅在开发环境可设置。
- mock 场景切换器在非开发构建中必须剔除；mock provider 不得访问真实设备。
- Go API 尚未接入时，前端可以独立启动并完成全主菜单交互；切换到 `http` 后若 API 不可达，应显示错误状态，不自动回退到 mock，避免把真实故障伪装为成功。

## OPS-2026-10-29 · 交付后运维设计增补

运维接入继续采用现有API/worker/PG/对象存储；采集/固件/配置协调按域模块化，不预先拆微服务或更换数据库。MetricStore/LogStore是适配器边界，专用产品与生产容量需评审。

配置项至少含collection schedule/timezone/timeout/retry/concurrency、freshness_policy_ref、retention_policy_ref、model_threshold_revision、metric/log store endpoints与受控凭据绑定。具体采集间隔/保留天数/RPO/RTO不在此臆造；生产启用前由负责人确认，缺失关键策略fail-closed而非默认删数据/放行。

上线前核对maintained IPMI-only的网络ACL、worker egress、适配器allowlist和任务恢复逻辑；禁止保留交付期SSH/Agent/Redfish凭据或重连通道。外部观测接入独立鉴权/来源白名单，不能兼作设备控制代理。

可观测性增加采集队列/超时/最后成功时间、stale/unsupported覆盖率、差异/固件Job回执、凭据/策略吊销、投影积压、归档/清理与legal_hold计数。备份恢复验证身份→配置revision→快照/日志/指标索引→工单/审批链，不只验证数据库启动。过期payload保留安全元数据和hash。

本次未部署任何新增采集器、时序/日志产品或真实设备通道。

## IA v2.4 实施增补

当前仅前端Mock，不接真实设备。环境标识与项目上下文不代表真实部署；采样、阈值、保留及生产设备接入待批准。


## IA v2.4 Mock 前端实施记录

导航树、完整页面清单、路由/权限/作用域/加载策略、五/八Tab抽屉与API对照统一见 [IA24-IMPLEMENTATION-MATRIX.md](IA24-IMPLEMENTATION-MATRIX.md)。原FR-0…24、门禁、BOM与维护态规则保留。当前实现为前端Mock，不表示已实现Go/真实设备/真实AI。权威种子为 `fixtures/demo-full-64-b300/demo-seed.json`；原包83集合，补齐与规范化后集合数量以当前manifest为准，主项目硬件数量不变。结构检查、数据修复、契约与交互结果以IA24-*报告为准，无法采集/缺少证据保持unknown或阻断。


## 当前六阶段主链路修正

本轮用户明确废弃数字门禁作为主链。当前唯一业务顺序、状态机、逐目标放行、安全交接、独立BOM菜单与API约束以[DELIVERY-LIFECYCLE.md](DELIVERY-LIFECYCLE.md)为准。历史编号只按[DELIVERY-MIGRATION.md](DELIVERY-MIGRATION.md)追溯，不得用于解锁。验机失败关联统一Issues并定向复验；NCCL不反写验机；清理/归档/客户签署齐全后才进入维护态。NP仍是网络内部断言。当前仅前端Mock与契约实现，没有真实设备后端。
