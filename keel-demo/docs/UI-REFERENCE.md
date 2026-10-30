# UI-REFERENCE.md — keel 前端参考与页面线框

## 1. 参考项目与具体借鉴点

| 参考 | 地址 | 借鉴范围 | 不做什么 |
|---|---|---|---|
| Kestra UI | https://kestra.io/docs/ui | Flows/Executions/Logs 导航、执行详情分 Tab、Topology/Gantt、任务日志和产物 | 不复制 Kestra 源码/品牌，不把首期做成复杂通用 DAG 编辑器 |
| Kestra Execution | https://kestra.io/docs/workflow-components/execution | 执行/任务状态、重试 attempt、Outputs、手动运行和调度语义 | 不照搬 Kestra 状态名；以 keel API 状态契约为准 |
| shadcn/ui Blocks | https://ui.shadcn.com/blocks | Sidebar、Dashboard、Data Table、表单、Dialog 组合 | 不直接把 dashboard 模板复制到每个页面 |
| shadcn/ui Data Table | https://ui.shadcn.com/docs/components/data-table | 服务端分页/排序/筛选、列显示、行选择 | 不用无分页的大表加载数万条数据 |
| shadcn/ui Sidebar | https://ui.shadcn.com/docs/components/sidebar | Sidebar provider、折叠、导航分组和响应式行为 | 不自制第二套导航组件 |

## 2. 视觉方向

- 产品类型是高密度工程控制台，不是营销网站。优先清晰层级、数据密度、对齐、可扫读性和可靠操作反馈。
- 以 shadcn/ui 原生 token 为基础，建立全局 spacing/type/color/radius/shadow tokens；色彩只表示状态或主动作。
- 主要内容宽度充分利用桌面视口；避免卡片套卡片、巨大标题、无意义环形图、过度渐变、过宽空白。
- 状态文案使用一致的中文动作/状态词，时间与单位格式统一；设备/端口 ID 使用等宽字体。
- 页面按任务设计：BOM 是解析/差异审核工作台，CMDB 是关系与生命周期视图，Flow 是脚本执行控制台，网络设备详情是工程证据面板。

## 3. 全局壳层线框

```text
┌──────────────────────────────────────────────────────────────────────────┐
│ keel   项目: [B300 项目 ▾]   全局搜索                    通知  用户 ▾     │
├────────────────┬─────────────────────────────────────────────────────────┤
│ 总览            │ 面包屑 / 页面标题                         主操作         │
│ 项目与交付       │ 简短描述 / 状态 / 更新时间                                │
│ BOM 开局导入     │ ┌────────────筛选/搜索/批量操作────────────────────────┐ │
│ CMDB            │ │                                                    │ │
│ 网络底座         │ └────────────────────────────────────────────────────┘ │
│ 服务器交付       │ 主内容：表格 / 关系视图 / 时间线 / 编辑器                 │
│ 工作流           │                                                         │
│ 执行记录         │                                                         │
│ 工单/监控        │                                                         │
│ 型号/模板/介质    │                                                         │
│ 审计/平台设置     │                                                         │
└────────────────┴─────────────────────────────────────────────────────────┘
```

## 4. BOM 智能导入线框

```text
[上传文件] → [工作簿识别] → [字段/关系映射] → [校验问题] → [差异审核] → [提交预览]
┌──────────────────┬──────────────────────────────┬────────────────────────┐
│ Sheet/区域列表    │ 原始表格/布局预览              │ 标准化字段/关系           │
│ - Device Info    │ 单元格坐标、原始值              │ target field / 值        │
│ - Device OOB     │ 空白继承来源标记                │ 映射来源：模板/规则/AI      │
│ - P2P design     │ 高亮当前问题                   │ 规则与置信提示              │
│ - GPU / CPU      │                               │ 修改/确认/忽略（有审计）     │
├──────────────────┴──────────────────────────────┴────────────────────────┤
│ BLOCK 12 | WARN 28 | 未知列 4 | 端点未匹配 7 | 敏感列已遮蔽 1               │
│ 问题表：级别 / Sheet / 单元格 / 原始字段 / 规则 / 处理状态 / 操作           │
└───────────────────────────────────────────────────────────────────────────┘
```

## 5. Flow 与 Execution 线框（Kestra-inspired）

```text
Flows
┌─────────────────────────────────────────────────────────────────────────┐
│ Flows   [搜索] [目标类型 ▾] [状态 ▾] [新建 Flow]                          │
│ 名称 | 目标类型 | 定时频率/时区 | 最近执行 | 成功率 | 状态 | 操作             │
└─────────────────────────────────────────────────────────────────────────┘

Execution Detail: exec-...
┌─────────────────────────────────────────────────────────────────────────┐
│ RUNNING  256 targets  181 success  4 failed  71 running  elapsed 00:04:32 │
│ [Pause] [Kill] [Retry failed]     Flow rev 7 | Trigger: schedule 02:00 TZ │
├─────────────────────────────────────────────────────────────────────────┤
│ Overview/Gantt | Topology | Targets | Logs | Outputs | CMDB & Issues       │
├─────────────────────────────────────────────────────────────────────────┤
│ Gantt / task lifecycle             │ Logs: time / level / target / attempt│
│ Task A ███████████ success         │ 12:00:01 [INFO] target-001 started   │
│ Task B █████ running               │ 12:00:02 [INFO] command accepted     │
│ Task C ░░░ queued                  │ 12:00:03 [WARN] target-009 timeout   │
└─────────────────────────────────────────────────────────────────────────┘
```

## 6. CMDB 线框

```text
CMDB Entity Detail
[Device identity] [Current status] [Source: BOM batch #] [Projection status]
Tabs: Overview | Relationships | Lifecycle Timeline | Evidence | Executions | Audit
Relationship graph: Project -> BOM Batch -> BOM Item -> Device -> Port -> Cable
Timeline: imported -> validated -> collected -> configured -> tested -> delivered -> maintained
```

## 7. Codex UI 实施要求

1. 在写页面前先画上述结构或更新线框；先确定页面信息架构和交互，再写组件。
2. 用 shadcn/ui Sidebar、Table、Tabs、Sheet、Dialog、Command、Form、Badge 等已有组件组合；只有有明确复用价值才新增业务组件。
3. 先做真实可点选的 mock 流程；按钮需改变数据状态、打开正确表单或发起 API，不允许只弹“成功”。
4. 每个页面都需实现 loading/empty/error/permission/stale/partial 状态。
5. 完成后用浏览器检查 1440×900、1280px；截图检查密度、对齐、字体、表格溢出、焦点状态和按钮位置。

## 8. Rack Diagram 参考项目

- Rackula：<https://github.com/RackulaLives/Rackula> / <https://docs.racku.la/>。参考交互：拖放、画布缩放、撤销/重做、前后视图、设备图像、导出和布局版本体验。其当前实现使用 Svelte，Keel 必须用 React 重实现，不把其框架带入项目。
- RackView（GitHub）：<https://github.com/kursatbal/rackview>。参考交付逻辑：真实设备/端口、正反视图、智能线缆、端口跳转、LLDP 对比、活动日志、搜索和多机柜视图。只借鉴产品行为，不依赖其 Flask/vanilla JS 实现。
- Keel 自动生成：以 BOM/CMDB + DeviceModelRevision + rack/U 位 + cable endpoints 作为事实输入，自动生成待审核布局、冲突清单和版本 diff。不能把手工拖放画布当作唯一数据源。

不得直接复制参考项目源码或素材。设备图片优先来自已授权的型号库资产；缺少图片时使用规范化 SVG 设备外观，确保可读性与版权合规。

## OPS-2026-10-29 · 交付后运维设计增补

### 运维服务器与交换机工作区线框
```text
运维管理 / 服务器（或交换机）                项目/数据时间/采集策略
[项目][机柜][型号][交付/维护][健康/告警]   搜索  刷新（先预检）
紧凑目录：身份/位置/配置摘要/告警/维护/最近采集 → 设备详情
┌ 详情：稳定ID / 来源BOM / 项目 / 生命周期 / 维护 / IPMI-only ┐
│ 概览 | 配置 | 硬件或端口 | BMC/LLDP | 固件/FDB/ARP/DDM | 事件 │
│ 来源 / collected_at / 保鲜 / 能力 / 质量 / 策略允许性          │
│ 专属面板：趋势图、配置三方diff、邻居链路、端口/光模块抽屉      │
└ 采集Job/审批/变更/Issue/复验/审计与历史快照可追溯           ┘
```

```text
配置管理：approved baseline rev | intended rev | observed snapshot / 时间
结构化diff：路径 | 期望 | 实测 | 漂移等级 | 规则/阈值/来源
动作：创建变更计划 → 精确目标/版本预检 → 既有审批 → PushJob/配置Job → 复采
```

```text
端口详情：物理UID / breakout父子 / OS接口 / 永久MAC / 端口角色
期望邻居 ── LLDP实测邻居（Chassis/Port ID/Sysname/Sysdesc/采集时间）
端口配置/链路/错误 | FDB/ARP历史 | DDM Rx/Tx/温度/电压/偏置趋势
阈值版本 / 数据源 / 能力 / stale缺口；点击链接回到线缆与CMDB关系
```

```text
高危操作确认：项目 + 稳定资产/SN + 动作 + 配置/固件/介质revision
维护状态/允许通道 + 独占租约 + 审批 + 回滚/验证 + 理由
[取消] [校验并创建异步任务]；accepted ≠ 完成；未知结果显示“先对账”
```

视觉验收需分别覆盖服务器硬件趋势、交换机端口/DDM、三方配置diff和策略拒绝，不用一张运维Dashboard替代。
