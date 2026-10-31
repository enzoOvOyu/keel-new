# Keel Flows 模块：产品架构、信息架构与全栈建设规范

> **唯一执行基准**：本文件 + `keel-flows-ia-and-ui-prototypes.drawio` +
> 项目现有 `design-system.md` / `system-design.md` / `AGENTS.md`。\
> **重构模块名称**：`Flows`。\
> **路由根路径**：`/flows`。\
> **目标**：以 Kestra 为编排引擎，建设 Keel 原生的 Flow 管理、Execution
> 管理、Keel Agent 管理及 SSH 双通道执行能力。

------------------------------------------------------------------------

## 0. 先明确范围：只重构 Flows 模块

本次要重构的菜单模块名称严格为 **Flows**。

-   左侧导航显示名称：`Flows`
-   路由根路径：`/flows`
-   不得将其改名为 `工作流中心`、`Job Platform`、`Workflow Center`
    或其他名称。
-   不得把本次任务扩大为整个 Keel 的 UI 重写。
-   不得删除 Flows 之外的任何业务页面、菜单、路由或后端模块。

### 0.1 最终菜单结构

只保留三个高价值入口：

``` text
Flows                         /flows
├── Executions                /flows/executions
└── Agents                    /flows/agents
```

说明：

-   `Flows` 本身就是默认入口和 Flow 定义管理页，不再额外增加 `Flow 管理`
    子菜单。
-   `Executions` 管理所有执行实例及其生命周期。
-   `Agents` 管理安装在目标 Linux 主机上的 Keel Agent。
-   Flow 详情、Flow
    Builder、执行启动、模板选择、文件资源、触发器和调度均属于上述页面的上下文功能，不单独占用左侧菜单。

### 0.2 旧功能迁移映射

  ------------------------------------------------------------------------------------
  现有功能/菜单           新归属                  处理方式
  ----------------------- ----------------------- ------------------------------------
  Flow 管理               Flows                   重构为新的 Flow 列表与详情

  Execution 执行          Executions              重构为统一执行中心

  蓝图库                  Flows / New Flow /      改为模板选择抽屉，不再独立占菜单
                          Builder                 

  介质库                  Flow Detail / Builder / 改为当前 Namespace 的文件资源面板
                          Namespace Files         

  Cron / 调度频率         Flow Detail / Triggers  合并进 Flow 触发器管理

  DAG / YAML              Flow Builder            同一 Flow 定义的不同编辑视图

  Logs                    Execution Detail        在执行详情中查看，不单独设菜单

  SSH 信任                现有系统管理 / SSH      继续复用现有入口，不新建重复管理页
                          信任管理                

  目标设备清单            CMDB                    CMDB 是设备资产权威来源，不在 Flows
                                                  中复制一套 Inventory

  Agent 导入/注册         Agents                  作为 Agent 管理页中的主操作
  ------------------------------------------------------------------------------------

**不建设无意义的 Dashboard、Schedules、Blueprints、Media Library、SSH
Profiles、Commands 等独立菜单。**\
这些能力有真实使用场景，但必须融入最接近的业务上下文。

------------------------------------------------------------------------

## 1. 参考产品研究与设计结论

本次 IA
不是按现有菜单拼接，而是参考成熟作业编排和基础设施自动化产品后重新归纳。

### 1.1 Kestra：以 Flows 为中心的编排工作台

官方资料：

-   https://kestra.io/docs/ui/flows
-   https://kestra.io/docs/ui
-   https://kestra.io/docs/workflow-components/namespace
-   https://kestra.io/docs/workflow-components/execution

值得借鉴：

1.  Flows 是创建、编辑和运行工作流的主入口。
2.  单个 Flow 详情中组织
    Overview、Executions、Edit、Revisions、Triggers、Logs、Metrics、Dependencies
    等上下文。
3.  编辑器提供 YAML、No-code、Topology 等视图。
4.  No-code 画布中的节点选择后，在侧边属性面板编辑。
5.  Namespace Files 和 Blueprints
    作为编辑上下文能力，而不是必须独立成为一级菜单。
6.  调度、触发器、错误处理、Finally、重试和并发限制属于 Flow 定义本身。

**Keel 采用：** - Flows 作为唯一主入口。 - Flow Detail
作为工作流的上下文工作台。 - Builder 采用 Visual / YAML / Topology
同源编辑。 - Templates 与 Files 融入 Builder。 - Triggers / Schedules
融入 Flow Detail。 - Executions 既能全局查看，也能从单个 Flow 详情查看。

不要复制 Kestra 的整个 UI，也不要依赖 Kestra Enterprise 专有能力。

### 1.2 Rundeck：基础设施批量操作与节点执行

官方资料：

-   https://docs.rundeck.com/docs/manual/03-getting-started.html
-   https://docs.rundeck.com/docs/manual/07-executions.html
-   https://docs.rundeck.com/docs/manual/jobs/
-   https://docs.rundeck.com/docs/manual/key-storage/

值得借鉴：

1.  Job 定义包含步骤、输入选项和执行目标。
2.  Node 代表可访问的物理或虚拟主机。
3.  支持按节点过滤后执行 Ad-hoc Command。
4.  Execution 统一记录运行状态、节点结果、日志、输出、开始和结束时间。
5.  Key Storage 为执行凭据提供受控引用。

**Keel 采用：** - 从 Flows 页面提供
`Quick Run`，支持一次性批量命令和脚本执行。 - 目标设备通过 CMDB Resolver
选择，不复制 Rundeck Nodes Inventory。 - Quick Run 也创建真实 Kestra
Execution，结果统一进入 Executions。 - 可将 Quick Run 保存为可复用
Flow。 - SSH 信任继续复用 Keel 现有系统管理，不重复开发凭据库。

### 1.3 AWX / Ansible Automation Platform：模板化执行与资源边界

官方资料：

-   https://docs.ansible.com/projects/awx/en/latest/userguide/main_menu.html
-   https://docs.redhat.com/en/documentation/red_hat_ansible_automation_platform/2.6/using_automation_execution/

值得借鉴：

1.  Job Template
    将执行内容、输入参数、凭据和目标清单组合为可重复执行单元。
2.  Job 与 Template 的职责不同：Template 定义如何执行，Job
    是一次真实执行。
3.  Inventory / Hosts 是执行目标，Credentials 是受控凭据，Schedules
    是自动触发机制。
4.  Workflow Template 组织多个执行步骤。

**Keel 采用：** - Flow 定义与 Execution 实例严格分离。 - Flow
的参数、目标解析、凭据引用、执行策略形成可复用定义。 - CMDB 负责
Inventory，Keel 不再复制一套 Hosts 页面。 - 模板是 Flow
创建时的起点，不单独建设 Templates 菜单。 - Schedule 是 Flow
Trigger，不独立建设 Schedules 菜单。

### 1.4 Argo Workflows：DAG 与执行可视化

官方资料：

-   https://argo-workflows.readthedocs.io/
-   https://github.com/argoproj/argo-workflows

值得借鉴：

1.  DAG / Steps 工作流可视化。
2.  可复用 Workflow Template。
3.  Artifact 和参数输入输出。
4.  并行、循环、条件分支、重试、暂停、恢复和取消。
5.  执行状态与拓扑节点关联。

**Keel 采用：** - DAG 作为 Flow Builder 的主要视觉表达。 - Execution
详情展示节点状态和目标设备执行矩阵。 - Outputs / Artifacts 在 Execution
Detail 中统一呈现。 - 仅借鉴编排交互，不复制 Argo 的 Kubernetes
CRD、Pod、Namespace 等 Kubernetes 专属资源模型。

### 1.5 最终设计判断

  -----------------------------------------------------------------------
  设计问题                采用的产品思路          Keel 的具体决定
  ----------------------- ----------------------- -----------------------
  工作流定义放哪里        Kestra Flows            `/flows`

  运行记录放哪里          Kestra Executions +     `/flows/executions`
                          Rundeck Activity        

  目标设备从哪里来        Rundeck/AWX             CMDB Resolver
                          的目标选择思想          

  Agent 如何管理          Keel 自有目标端代理     `/flows/agents`

  一次性命令如何执行      Rundeck Ad-hoc Commands Flows 内 Quick Run

  模板放哪里              Kestra Blueprints / AWX New Flow / Builder 抽屉
                          Templates               

  调度放哪里              Kestra Triggers / AWX   Flow Detail / Triggers
                          Schedules               

  文件放哪里              Kestra Namespace Files  Flow Builder / Files
                                                  面板

  SSH 凭据放哪里          Rundeck Key Storage     复用 Keel 现有 SSH
                                                  信任管理

  日志放哪里              Kestra Execution Logs   Execution Detail
  -----------------------------------------------------------------------

------------------------------------------------------------------------

## 2. Flows 模块页面信息架构

### 2.1 左侧菜单

唯一导航结构：

``` text
Flows
  ├─ Executions
  └─ Agents
```

`Flows` 是父菜单，也是默认页面。点击父菜单进入 `/flows`。

### 2.2 Flows 页面

职责：管理 Flow 定义及其生命周期。

主页面包含：

-   Flow 列表。
-   搜索与筛选。
-   创建 Flow。
-   Quick Run。
-   Flow 状态。
-   Revision。
-   最近执行。
-   触发器状态。
-   目标类型。
-   操作菜单。

不单独显示蓝图库、调度、介质库菜单。

### 2.3 Flow Detail

单个 Flow 的详情页使用上下文 Tab：

-   Overview：基本信息、状态、最近运行、DAG 摘要。
-   Builder：可视化编辑、YAML、Topology。
-   Executions：当前 Flow 的历史执行。
-   Revisions：Flow 版本历史与差异。
-   Triggers：Schedule、Webhook、事件触发器及启停状态。
-   Dependencies：Subflow 和 Flow 依赖关系。
-   Files：当前 Namespace Files。

如果某个 Tab 没有对应后端能力，必须明确标识未启用，不得伪造数据。

### 2.4 Executions 页面

职责：统一查看所有 Flow、Quick Run 和业务模块触发的执行。

列表筛选：

-   时间范围。
-   Execution 状态。
-   Flow。
-   项目。
-   来源模块。
-   发起人。
-   目标类型。
-   实际执行通道（Agent / SSH）。
-   目标设备。
-   Simulation / Real。

执行详情 Tab：

-   Overview
-   Targets
-   Task Runs / DAG
-   Logs
-   Outputs & Artifacts
-   Audit / Context

不建设独立 Logs 菜单。

### 2.5 Agents 页面

职责：管理 Keel Agent 的注册、在线状态、能力和生命周期。

页面操作：

-   Import / Enroll Agent
-   从 CMDB 选择 Linux 主机。
-   SSH Bootstrap。
-   手动注册指引。
-   批量安装。
-   批量健康检查。
-   Agent 版本查看。
-   证书状态。
-   隔离与撤销。

Agent 详情：

-   Overview
-   Capabilities
-   Tasks
-   Connection
-   Certificate
-   Logs / Audit

Agent 只用于适合安装 Agent 的 Linux 主机。交换机默认使用 SSH
通道，不强制安装 Agent。

------------------------------------------------------------------------

## 3. UI 原型与页面设计要求

必须打开并逐页参照：

`keel-flows-ia-and-ui-prototypes.drawio`

原型页面：

  页码   内容
  ------ -----------------------------
  00     Flows 模块 IA 与菜单边界
  01     Flows 列表
  02     Flow Detail
  03     Flow Builder
  04     Executions 列表
  05     Execution Detail
  06     Agents 管理与导入
  07     Quick Run 与 Agent/SSH 路由

### 3.1 Flows 列表

布局：

-   页面标题 `Flows`。
-   主操作：`Create Flow`。
-   次操作：`Quick Run`。
-   紧凑筛选栏。
-   Data Table。
-   行操作菜单。

表格字段：

-   Name / Flow ID
-   Description
-   Namespace
-   Target Type
-   Trigger
-   Revision
-   Last Execution
-   Last Status
-   Enabled
-   Actions

不要继续使用当前的大型纵向卡片列表。

### 3.2 Flow Detail

顶部展示：

-   Flow 名称。
-   Namespace。
-   Revision。
-   Enabled / Disabled。
-   运行按钮。
-   编辑入口。

Tab 按上述 Flow Detail 定义实现。

Overview 使用紧凑摘要，不要用大量 KPI 卡片占据页面。

### 3.3 Flow Builder

采用三栏式工作台：

左栏： - 节点目录。 - Flow 结构大纲。 - 模板入口。 - Namespace Files
入口。

中栏： - DAG 画布。 - 节点连接。 - 分支和循环。 - 缩放与自动布局。

右栏： - 当前节点属性。 - 输入参数。 - 输出字段。 - 执行通道。 -
超时。 - 重试。 - 目标能力要求。

底部可展开： - YAML。 - Script Editor。 - Validation Errors。

提供视图切换：

`Visual | YAML | Topology`

Visual、YAML 和 Topology 必须描述同一个 Kestra
Flow，不允许产生两套独立工作流数据。

模板通过 `Start from Template` 抽屉选择，选择后生成 Flow 草稿。

Namespace Files 作为上下文文件浏览器，不再单独放在左侧菜单。

### 3.4 节点类型

至少支持：

-   CMDB Target Resolver
-   Transport Selector
-   Agent Task
-   SSH Command
-   Bash / Python Script
-   File Transfer
-   Loop
-   Parallel
-   Condition
-   Subflow
-   Output
-   Finally / Error Handling

节点目录要区分：

-   Kestra 原生任务。
-   Keel 自定义任务。
-   通用控制流任务。

不能把并不存在的插件伪装为已经可执行的原生能力。

### 3.5 Quick Run

从 Flows 页面发起一次性批量命令或脚本任务。

步骤：

1.  选择目标类型。
2.  从 CMDB 选择设备。
3.  选择 Command / Bash / Python。
4.  输入命令或引用脚本。
5.  选择 Agent Preferred / Agent Only / SSH Only。
6.  执行前预检。
7.  展示每台设备预计使用的执行通道。
8.  确认并执行。
9.  自动生成真实 Kestra Execution。
10. 在 Executions 中查看结果。

Quick Run 不建立第二套执行引擎。

使用系统维护的受控 Flow 模板创建 Execution。

Quick Run 完成后支持 `Save as Flow`，将可复用操作转化为正式 Flow。

对高风险命令实施权限校验、命令策略或审批，不允许 Quick Run
绕过安全门禁。

### 3.6 Executions 列表

采用 Data Table。

字段：

-   Execution ID
-   Flow
-   Project
-   Source Module
-   Target Count
-   Success / Failed
-   Execution Mode
-   Started By
-   Duration
-   Started At
-   State

执行状态必须来自 Kestra，不允许前端定时器模拟。

### 3.7 Execution Detail

顶部显示执行摘要。

核心内容：

-   DAG / Task Run 状态。
-   目标设备执行矩阵。
-   实际执行通道。
-   设备级耗时。
-   Attempt。
-   Exit Code。
-   stdout / stderr。
-   Outputs。
-   Artifacts。

执行控制：

-   Pause
-   Resume
-   Kill
-   Restart
-   Retry Failed Targets

Retry Failed Targets 必须只对失败设备创建新的
Execution，不能重新执行全部设备。

### 3.8 Agents 列表

表格字段：

-   Agent ID
-   CMDB Asset
-   Hostname
-   OS
-   Version
-   Status
-   Last Heartbeat
-   Capabilities
-   Certificate
-   Active Tasks
-   Last Seen

Agent 导入向导：

1.  从 CMDB 选择 Linux 主机。
2.  检查 SSH Trust。
3.  生成一次性 Enrollment Token。
4.  选择 Agent 版本。
5.  通过 SSH Bootstrap 推送安装包。
6.  安装 systemd 服务。
7.  Agent 主动建立 mTLS 连接。
8.  身份匹配。
9.  注册完成。

身份不匹配时进入 Pending Verification，禁止自动绑定错误资产。

------------------------------------------------------------------------

## 4. 后端架构

``` text
Keel React
    |
    v
Keel Go API / BFF
    |
    +---- Flow Adapter -------- Kestra REST API
    |
    +---- Execution Adapter --- Kestra REST API / SSE
    |
    +---- Job Service
    |       |
    |       +---- CMDB Target Resolver
    |       |
    |       +---- Agent Gateway
    |       |
    |       +---- SSH Trust Resolver
    |
    +---- Agent Registry / Certificate / Audit
            |
            v
        Keel Agent (目标 Linux 主机)

Kestra Worker:
    - 执行 Kestra Tasks
    - 调用 Agent Gateway
    - 或通过 SSH Plugin 访问目标设备
```

### 4.1 职责边界

Kestra： - Flow 定义和 Revision。 - DAG。 - Scheduler / Triggers。 -
Loop / 并发。 - Retry。 - Execution / TaskRun。 - 状态。 - 日志和
Outputs。

Keel Go： - 项目和用户权限。 - CMDB 目标解析。 - Agent 注册。 - Agent
Gateway。 - SSH Trust 解析。 - 双通道路由。 - 业务 Job 映射。 - 审计。

Keel Agent： - 目标主机本地命令。 - Bash / Python。 - 文件操作。 -
主机信息采集。 - 任务取消。 - 日志和结果回传。

SSH： - 无 Agent 设备。 - 网络交换机。 - Agent
不可用但可安全回退的设备。 - Bootstrap 安装 Agent。

------------------------------------------------------------------------

## 5. Kestra 集成

官方资料：

-   https://kestra.io/docs/architecture
-   https://kestra.io/docs/installation/docker-compose
-   https://kestra.io/docs/how-to-guides/api
-   https://kestra.io/docs/workflow-components/execution
-   https://kestra.io/docs/workflow-components/loop

要求：

1.  Kestra 独立部署。
2.  固定稳定版本，不使用 latest。
3.  独立 PostgreSQL。
4.  Go 后端封装 Kestra API。
5.  浏览器不直接访问 Kestra。
6.  Kestra 是 Flow 和 Execution 状态权威来源。
7.  Go 只保存业务映射和必要状态投影。
8.  不直接操作 Kestra 内部数据库。

### 5.1 并发语义

必须区分：

-   Flow 并发：同一 Flow 同时运行的 Execution 数。
-   目标并发：单次 Execution 中同时执行多少目标设备。
-   Worker 资源：Worker 实际可承载的任务能力。

目标并发使用 Kestra Loop `concurrencyLimit`。

Flow 并发使用 Kestra Flow concurrency 配置。

不能混成一个参数。

### 5.2 重试

Kestra `maxAttempts` 包含首次执行。

用户配置最大重试 2 次，应映射为 `maxAttempts = 3`。

对有副作用的命令必须先设计幂等性。

### 5.3 日志

通过 Go 后端代理 Kestra Logs Follow / SSE。

断线后支持重新拉取执行状态和日志位置。

不允许浏览器直接访问 Kestra 内部地址。

------------------------------------------------------------------------

## 6. Keel Agent

### 6.1 定位

Keel Agent 是安装在目标 Linux 主机上的轻量 Go 代理。

它不是 Kestra Worker，也不是 AI Agent。

Kestra 负责调度，Keel Agent 负责目标端执行。

### 6.2 核心能力

第一阶段：

-   exec.command
-   script.bash
-   script.python
-   file.upload
-   file.download
-   inventory.collect
-   process.control
-   service.control
-   task.cancel
-   task.stream

### 6.3 通信

-   gRPC 双向流。
-   mTLS。
-   Agent 主动出站连接。
-   心跳。
-   任务下发。
-   实时日志回传。
-   结果回传。
-   证书轮换。
-   证书撤销。

### 6.4 注册

1.  平台生成一次性 Token。
2.  Token 绑定项目和允许注册的 CMDB 资产。
3.  Token 短有效期，建议 15 分钟。
4.  Token 一次性使用。
5.  Agent 注册并提交主机身份。
6.  平台校验 Machine ID、DMI UUID、Serial、Hostname、MAC 等。
7.  校验通过后签发 Agent 证书。
8.  Agent 建立 mTLS。
9.  注册成功。

身份冲突必须进入人工核验。

### 6.5 Agent 安全

-   默认非 root。
-   高权限操作采用最小权限 sudo 白名单。
-   命令和脚本受授权策略约束。
-   脚本 SHA256 校验。
-   输出大小限制。
-   超时和取消。
-   证书撤销。
-   审计。
-   敏感字段脱敏。

Agent 不实现自己的批量调度和重试引擎。

------------------------------------------------------------------------

## 7. SSH 双通道执行

SSH 互信作为 Agent 的兼容执行通道，也用于 Agent Bootstrap。

必须： - 公钥认证。 - 严格 Host Key 校验。 - Known Hosts。 -
独立执行身份。 - 私钥保存在 Secret Store / Vault。 - 复用现有 Keel SSH
信任管理。 - 不在浏览器或 Flow YAML 中保存私钥。

Kestra SSH 插件需要显式启用
`strictHostKeyChecking: yes`，不能采用默认的不安全配置。

交换机默认走 SSH，不要求安装 Agent。

------------------------------------------------------------------------

## 8. Agent / SSH 路由策略

每个目标设备独立选择执行通道。

### Agent Only

Agent 必须在线且能力匹配，否则阻断。

### SSH Only

只使用 SSH，适合交换机和未安装 Agent 的设备。

### Agent Preferred

-   Agent 在线且能力匹配：Agent。
-   Agent 未安装或不可用：检查 SSH Trust。
-   SSH 可用：SSH。
-   两者不可用：阻断。

### 防止重复执行

如果 Agent 已 ACCEPTED / RUNNING，随后连接断开：

1.  查询原 Agent Task。
2.  检查任务状态。
3.  进入 UNKNOWN / RECONCILING。
4.  不允许直接启动 SSH 副本。
5.  确认任务未产生执行副作用后，才允许人工或受控切换。

每台目标记录： - selected_transport - actual_transport - agent_id -
agent_task_id - ssh_profile_id - idempotency_key - attempt - exit_code -
start_time - end_time

------------------------------------------------------------------------

## 9. 数据模型与数据来源

CMDB 是资产身份和设备目标的权威来源。

Kestra 是 Flow 和 Execution 的权威来源。

Keel Agent Registry 是 Agent 身份、能力、证书和在线状态的权威来源。

SSH Trust 使用现有 Keel 系统管理和 Secret Store。

核心业务关联：

-   FlowBinding
-   JobExecutionBinding
-   JobExecutionTarget
-   AgentRegistry
-   AgentEnrollment
-   AgentTask
-   TransportDecision
-   JobAuditEvent

目标设备快照必须保存，历史 Execution 不得因 CMDB
后续变化而改变目标身份。

不要复制 Kestra 内部数据库结构。

不要复制 CMDB 设备清单。

------------------------------------------------------------------------

## 10. API 边界

### Flows

-   GET `/api/v1/flows`
-   POST `/api/v1/flows`
-   GET `/api/v1/flows/{flowId}`
-   PUT `/api/v1/flows/{flowId}`
-   POST `/api/v1/flows/{flowId}/validate`
-   POST `/api/v1/flows/{flowId}/execute`

### Executions

-   GET `/api/v1/flows/executions`
-   GET `/api/v1/flows/executions/{executionId}`
-   GET `/api/v1/flows/executions/{executionId}/targets`
-   GET `/api/v1/flows/executions/{executionId}/logs`
-   GET `/api/v1/flows/executions/{executionId}/outputs`
-   POST `/api/v1/flows/executions/{executionId}/pause`
-   POST `/api/v1/flows/executions/{executionId}/resume`
-   POST `/api/v1/flows/executions/{executionId}/kill`
-   POST `/api/v1/flows/executions/{executionId}/retry-failed-targets`

### Agents

-   GET `/api/v1/flows/agents`
-   GET `/api/v1/flows/agents/{agentId}`
-   POST `/api/v1/flows/agents/enrollment-tokens`
-   POST `/api/v1/flows/agents/bootstrap`
-   POST `/api/v1/flows/agents/{agentId}/approve`
-   POST `/api/v1/flows/agents/{agentId}/quarantine`
-   POST `/api/v1/flows/agents/{agentId}/revoke`
-   GET `/api/v1/flows/agents/{agentId}/tasks`

### Quick Run

-   POST `/api/v1/flows/quick-run`

具体路由按现有 API 规范统一，但不要重复创建功能相同的接口。

ZTP、Provision、Validation、NetTest 等业务模块只能调用统一 Job
Service，不得直接调用 Kestra 或 Agent Gateway。

------------------------------------------------------------------------

## 11. 前端重构范围与代码删除安全规则

这是本次最重要的代码边界约束，必须严格遵守。

### 11.1 允许删除

仅允许删除**完全属于旧 Flows 模块且不再复用的旧页面 UI 实现**：

-   旧 Flow 管理页面组件。
-   旧 Flow 创建页面布局组件。
-   旧 Execution 页面布局组件。
-   旧蓝图库独立页面 UI。
-   旧介质库独立页面 UI。
-   旧工作流菜单的重复导航配置。
-   仅供旧 UI 使用且确认不再需要的 UI Mock 组件。

### 11.2 禁止删除

不得因为页面重构而删除：

-   Keel 全局 Header。
-   左侧全局导航框架。
-   项目上下文和环境选择器。
-   design-system Token。
-   全局 UI 组件。
-   权限和用户系统。
-   CMDB。
-   SSH Trust。
-   Go 后端服务。
-   Kestra Adapter。
-   Agent Gateway。
-   Agent Registry。
-   数据库模型和迁移。
-   其他部署和运维业务模块。
-   仍被其他模块使用的共享组件和 API。

### 11.3 执行删除前的检查

先检查实际代码仓库，生成一份简短的旧页面文件归属表：

  文件类型                    处理
  --------------------------- ------------------------------
  仅旧 Flows 页面使用的组件   删除并由新组件替换
  多模块共享组件              保留或抽取兼容实现
  API Client / Service        保留并改造
  后端接口和数据库            保留并扩展
  旧 Mock 页面数据            仅在确认没有其他消费者后删除
  全局路由和菜单              仅修改 Flows 对应配置

完成归属检查后直接实施，不需要等待用户逐文件确认。

### 11.4 路由和菜单安全

-   左侧菜单显示名称严格为 `Flows`。
-   根路径严格为 `/flows`。
-   子页面只有 `Executions` 和 `Agents`。
-   删除旧蓝图库和介质库的独立菜单入口。
-   模板功能迁移到 Flow 创建和 Builder。
-   文件功能迁移到 Flow Detail / Builder 的 Files 面板。
-   不创建 `/workflow-center` 或 `/job-platform` 新根路由。
-   现有旧路由如存在外部引用，保留兼容 Redirect，但不得继续渲染旧 UI。
-   不修改 Flows 之外的菜单名称和路由。

------------------------------------------------------------------------

## 12. 开发实施顺序

### Phase 0：代码盘点和范围锁定

-   检查现有路由、菜单、页面组件和共享依赖。
-   确认旧 Flows 页面文件范围。
-   确认当前 Kestra、Agent、SSH 代码现状。
-   不进行无关全仓库重构。

### Phase 1：执行底座

-   部署 Kestra。
-   完成 Go Kestra Adapter。
-   完成 Job Service。
-   完成目标快照和执行关联。

### Phase 2：Keel Agent

-   实现 Go Agent。
-   注册和 mTLS。
-   心跳和能力。
-   命令、脚本、文件任务。
-   日志和结果。
-   取消、超时和幂等。

### Phase 3：SSH 与双通道

-   对接 SSH Trust。
-   Known Hosts。
-   SSH Bootstrap。
-   Agent / SSH 路由。
-   状态未知保护。

### Phase 4：Flows UI 全面重构

-   删除旧 Flows 页面 UI。
-   新建 Flows 列表。
-   新建 Flow Detail。
-   新建 Flow Builder。
-   新建 Quick Run。
-   新建 Executions。
-   新建 Agents。
-   迁移模板和文件能力。
-   完成 SSE。

### Phase 5：端到端验收

-   Flow 创建和 Revision。
-   DAG / YAML 同步。
-   Agent 导入。
-   Agent 批量执行。
-   SSH 批量执行。
-   Agent / SSH 混合路由。
-   部分失败和重试。
-   执行日志。
-   权限和安全。
-   ZTP 调用 Job Service。

------------------------------------------------------------------------

## 13. 必须通过的验收场景

### A. Flow 定义

-   新建 Flow。
-   Visual 编辑。
-   YAML 编辑。
-   两者保持一致。
-   校验和保存 Revision。
-   从模板创建 Flow。
-   查看 Triggers 和依赖。

### B. Quick Run

-   CMDB 选择 8 台设备。
-   输入 Bash 命令或脚本。
-   Agent Preferred。
-   执行前检查 Agent / SSH。
-   触发真实 Kestra Execution。
-   结果出现在 Executions。

### C. Agent

-   从 CMDB 选择 Linux 主机。
-   通过 SSH Bootstrap 安装。
-   注册和证书签发。
-   身份匹配。
-   Agent Online。
-   执行命令并返回真实结果。

### D. SSH

-   选择无 Agent 的交换机。
-   校验 SSH Trust。
-   校验 Known Hosts。
-   执行批量命令和脚本。
-   返回每台设备的真实状态。

### E. 混合执行

-   4 台 Agent 在线服务器。
-   2 台 Agent 离线但 SSH 可用服务器。
-   2 台交换机。
-   Agent Preferred。
-   4 台 Agent 执行、4 台 SSH 执行。
-   同一个 Kestra Execution 汇总。

### F. 失败与重试

-   8 台设备中 2 台失败。
-   6 台成功。
-   只重试失败设备。
-   创建新的 Execution。
-   成功设备不重复执行。

### G. UI

-   左侧只有 Flows、Executions、Agents 三个入口。
-   模板和文件在 Flow 上下文中可访问。
-   Trigger / Schedule 在 Flow Detail 中管理。
-   SSH Trust 复用系统管理。
-   不存在旧的独立蓝图库和介质库菜单。
-   所有页面使用真实 API。
-   所有操作有明确 Loading / Empty / Error / Disabled 状态。

------------------------------------------------------------------------

## 14. 最终交付

-   Kestra 部署文件。
-   Go Job Service。
-   Kestra Adapter。
-   Keel Agent。
-   Agent Gateway。
-   SSH Trust Adapter。
-   Agent / SSH 双通道路由。
-   数据库迁移。
-   Flow YAML 示例。
-   全新 Flows React UI。
-   Flow Builder。
-   Quick Run。
-   Executions。
-   Agents。
-   Namespace Files / Template 上下文集成。
-   API 契约。
-   本地启动文档。
-   端到端验收结果。

不得只输出计划或静态原型，必须实际修改代码并完成联调。

------------------------------------------------------------------------

## 15. 最终原则

1.  **菜单以工作任务组织，不以技术组件组织。**
2.  **Flows 是唯一的模块名称和路由根。**
3.  **左侧只有 Flows、Executions、Agents 三个入口。**
4.  **模板、文件、调度、日志、SSH 信任融入对应上下文。**
5.  **CMDB 是目标设备唯一权威来源。**
6.  **Kestra 是工作流编排和 Execution 生命周期权威来源。**
7.  **Keel Agent 负责目标主机执行，不负责调度。**
8.  **SSH 保证无 Agent 设备仍可批量执行。**
9.  **Agent 与 SSH 统一由 Job Service 路由。**
10. **只删除旧 Flows 页面 UI，不误删其他业务代码。**

最终产品应该是一个简洁、专业、可扩展的基础设施自动化工作台，而不是把
Kestra、Rundeck、AWX 的所有菜单简单叠加。

**优先做好 Flow
定义、批量执行、目标端执行代理和执行结果闭环。任何没有明确用户任务价值的菜单都不要增加。**
