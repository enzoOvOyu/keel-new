# 作业平台 · 当前实现定版

界面名称为「作业平台」，技术路由/权限/API 保留 flows。原生 Kestra UI 及受控执行已整合，`9441a89` 发布后由当前主环境 `686e9ea` 保留；不再在旧 detached 工作区开发。

## 产品与页面

唯一作业定义/执行平台。已有设备按授权目标组批量执行脚本/命令，业务模块提交受控任务并引用执行结果，不各建调度器。Kestra 管定义/调度/执行，Keel 管项目、身份、审批、CMDB目标、设备通道和领域规则。

| 页面 | 已实现交互 |
|---|---|
| `/flows` | 任务目录、筛选/分页/保存视图、近期执行和来源；新建/Quick Run |
| Flow详情/Builder | Overview、Visual/YAML/Topology/Files/Docs同源编辑、多面板、任务属性/依赖/排序、校验/保存、Revision、Triggers、Dependencies；dirty禁止混用旧版直接运行 |
| Executions | 记录/运行中/关注、状态/来源/通道/时间过滤、真实聚合、批量暂停/继续/终止及逐项结果 |
| Execution详情 | 冻结定义、状态历史、TaskRun/Attempt、Gantt/拓扑、设备目标、日志、输出/文件、指标/依赖；URL选中状态和返回上下文一致 |
| Quick Run | 目标→脚本/通道→预检确认，修改输入后预检失效，冻结版本/审批后执行 |
| Agents | 身份/心跳/能力/任务/证书/审计，注册、批准/隔离/撤销，安装进度、证书续期；Worker与设备Agent身份分开 |

Files 新建/导入为内存草稿，保存有 expected_revision；409保留草稿。Namespace 固定版本引用与 inline 快照分别表示，已保存脚本不会随文件更新自动变化。步骤描述/时限和脚本SHA来源在同一 Flow 定义中；不是另一套文件执行模型。

## 执行闭环和安全

Flow Revision + 输入 + 项目/设备版本 + 通道 → 预检 → 需要时独立审批 → 幂等创建 Execution → 原生TaskRun与逐目标结果 → 证据/审计。任务成功不等于业务验收成功。

- 复用统一 CMDB stable ID、Session/RBAC、Secret Reference。申请人不得自批；批准摘要/目标/版本/有效期再次校验，一个批准绑定一次新执行。
- Agent mTLS 与 SSH 为已有两通道。fallback 只能在任务未接受前按授权/能力决定；接受后结果未知保持 RECONCILING，不能重发。
- maintained 严格 IPMI-only。交换机写配置需批准的 Fabric Adapter；普通脚本执行权限不能绕过。
- 暂停/继续/终止需实际引擎确认。失败目标重试/重跑建立新执行、谱系、预检和审批；不直接将原审批复制到新执行。
- Cron、历史补跑及持久操作批次复用 Kestra/Job Service；后续每次派发重查权限/维护态，不制造永久授权。
- SSE/持久事件用于日志续传；设备Task、TaskRun、Attempt只按明确ID关联，不按Hostname猜。Metrics只显示真实样本，当前页时间筛选不冒充全量时序查询。

## 接口和代码

[真实 API](../api/flows.md)、[注册表](../api/real-api-registry.json)；70个已登记操作不等于70项逐端点运行验收。

[React容器](../../dev/src/features/flows)、[原生界面](../../dev/src/features/jobs-native)、[Adapter](../../dev/src/lib/api/flows.ts)、[Go路由](../../dev/backend/cmd/api/flows.go)、[领域服务](../../dev/backend/internal/flows)、[原生许可](../../dev/src/vendor/kestra/LICENSE)。保留原生组件和许可，不以React近似页面替换；Keel全局壳层仍为React。

## 验证与后续

[发布记录](../flows-redesign/deployment-20261102.json)记录原生界面上线及只读启动/页面检查；[交互整改](../flows-redesign/parity-correction/result.json)、[增强](../flows-redesign/parity-correction/enhancements.json)使用synthetic拦截；[运行专项](../flows-redesign/completion-runtime-validation.json)区分本地Agent/SSH协议仿真。均不能代替生产8设备混合批次、NOS兼容或规模验收。

ZTP事件消费者/匹配待Fabric联调，不因 `source_module=ztp` 筛选存在就声称接通。Namespace动态跟随、任意Worker脚本执行、无明确幂等契约的节点跳过/重放仍不支持。缺能力/超时/断联必须按真实结果展示，不恢复全局模拟开关。
