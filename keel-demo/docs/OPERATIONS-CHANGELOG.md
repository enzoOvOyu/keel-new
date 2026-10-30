# OPS-2026-10-29 文档变更与检查记录

本记录仅报告交付后运维设计与Mock契约变更；新增运维页面、Go API、采集器和真实设备能力均未在本轮实现。

## 变更清单

| 文件 | 增补内容 |
|---|---|
| PRD.md | 保留FR-0…17；增加FR-18服务器运维、FR-19交换机运维、FR-20配置/固件、FR-21持续采集、FR-22维修复验/退役；明确FR-9的maintained/IPMI-only |
| ARCHITECTURE.md | 运维/采集/配置/固件模块、CMDB事实与业务所有权、时序/日志适配器、策略执行边界 |
| SYSTEM-DESIGN.md | 基线/意图/实测/diff版本链；生命周期与维护双状态；采集/升级Job与对账；保留语义 |
| DATA-DICTIONARY.md | 实体复用/所有权/字段/关联/版本/保留矩阵，避免Machine/Switch/Component等重复身份 |
| API-CONVENTIONS.md | 运维过滤/分页/历史、异步状态、幂等与版本、权限/通道错误和脱敏 |
| DOMAIN-API-CONTRACTS.md | 页面到设备详情/配置/端口/观测/BMC/固件/采集/维护/退役接口映射与操作规则 |
| openapi.yaml | 新增30路径、32操作、66个专用Ops schema；补齐既有9路径缺失的path参数；原请求/响应schema及操作定义保留 |
| DESIGN-SYSTEM.md | 独立运维导航、服务器/交换机详情Tab、趋势与三方diff、能力/策略区分、危险操作确认 |
| UI-REFERENCE.md / UI-FLOWS.md | 运维列表/详情/端口/差异/确认线框，采集→告警→维修→复验→退役流程 |
| MOCK-DATA-CONTRACT.md | 64服务器+24交换机的运维集合/异常场景/外键与状态检查要求；不宣称已接入新数据 |
| IMPLEMENTATION-PLAN.md | 设计→完整Mock页面/API→逐模块Go；安全约束先于真实适配器 |
| ACCEPTANCE.md / TEST-STRATEGY.md | OPS-001…016设计验收、契约/领域/浏览器/适配器测试层；运行时保持NOT RUN |
| SECURITY.md / DEPLOYMENT.md | maintained/IPMI-only、凭据与高危操作、调度恢复/fallback/egress、保留/备份/监控 |
| AGENTS.md | Codex执行边界、实体复用、前端先行、禁止恢复带内、不得虚报运维已实现 |
| README.md / DECISIONS.md | 入口与已解决冲突；存储/阈值/保留/能力矩阵未决事项及安全默认 |
| OPERATIONS-IMPACT.md / OPERATIONS-DESIGN.md | 全局影响、既有/缺失/冲突，完整运维链路、页面、事实所有权和安全约束 |
| fixtures/operations/contract-examples.json | 7个设计用合成样例与5项拒绝用例；没有装载到运行中的Mock Provider |

## 实际检查

- JSON Schema/跨文档/本地引用/operationId/专用schema/旧操作与schema保留检查：285项通过，详见OPERATIONS-CONSISTENCY.json。
- 7个合成契约样例通过JSON Schema 2020-12校验；5个反例正确拒绝：maintained使用SSH、Redfish；unsupported返回数值0；缺实测配置却PASS；无审批却声明approved baseline。
- `npx --yes @redocly/cli lint docs/openapi.yaml --format json`：0 errors，35 warnings；报告OPERATIONS-OPENAPI-LINT.json。
- 警告为既有tag描述、license、旧操作缺4xx响应、Session未引用和既有rack-diagrams动态路径歧义。本次没有新增Ops schema lint警告；不通过忽略规则或删除旧接口隐藏问题。旧路径歧义保留，未擅自更改已存在的URL。
- 运行时OPS-001…016：NOT RUN。本文档检查不能代替Mock页面/Go/采集器/真实设备验收。

## 待确认且不擅自决定

具体采样周期、型号阈值、保留天数、法律冻结责任、生产MetricStore/LogStore产品、容量、RPO/RTO，见DECISIONS OPS-P01…04。maintained无法通过IPMI或批准独立来源获取的数据保持unsupported/unknown或policy_denied；本次没有创建恢复SSH/Agent/Redfish的例外。

根目录docs与AGENTS为规范源；keel-demo内同步设计文档副本并保留原交付演示检查报告。新增OpenAPI尚未生成/接入前端运维Provider；后续进入运维实现阶段时按新契约生成类型并验证，不把现有交付演示标为新增运维功能已完成。
