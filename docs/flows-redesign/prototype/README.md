# 作业平台交互设计预览

预览入口：<http://localhost:5181/jobs-preview/index.html#flows>。`/jobs-preview`和`/jobs-preview/`也会跳转到独立HTML，不再落入拓扑SPA。原有拓扑左侧“作业平台样式”按钮可直接进入。

此目录是独立设计原型，不修改正式`/flows`、不访问业务API、不写数据库或设备。所有项目、设备、执行、日志与产物均来自`data.ts`，明确标记synthetic；运行/暂停/批量动作只改变原型内存。保存视图仅使用独立的`keel-jobs-preview-view`会话偏好，不保存凭据。

## 页面与操作

- `#flows`：Flow目录、搜索、Namespace/类型/状态筛选、原生表格选择、批量启停、列显示、分页及最近执行入口。
- `#editor/gpu-preflight`：原生多面板Visual/YAML/Topology/Files/Docs；节点选择、属性编辑、拖动排序、添加/删除、代码同步和原型版本保存。Overview/Revisions/Triggers/Executions是同一Flow上下文中的设计预览。
- `#execution`：Gantt/任务详情、Attempt选择、目标矩阵/列表、日志筛选、变量与输出、上下文和操作历史；失败目标重试与全部重跑分别确认。
- `#executions`：执行目录与运行/关注筛选。
- `#agents`：Agent列表、设备详情抽屉、演示纳管与批准。
- Quick Run：目标→任务→预检确认→原型执行，支持暂停/继续/终止，均不调用真实执行服务。

原型侧栏仅保留Flows、Executions、Agents三个作业平台入口，删除部署、拓扑、CMDB、系统管理等无关菜单。顶栏继续保留项目切换与主题；项目筛选不代表真实授权。此限制只针对独立演示，不修改正式应用的全局IA。

## 原生来源与边界

复用固定Kestra `49b62a2c820d37e98a1c7b63aa35fd83f960d4ec` 的KsDataTable、KsButton、KsInput、KsExecutionStatus、KsDialog、MultiPanelGenericEditorView/MultiPanelTabs、KsEditor/Monaco、SidebarList及No-code卡片。许可证与适配说明见`dev/src/vendor/kestra/PROVENANCE.md`。

拓扑与Gantt在本原型中是用于审核布局和联动的专属表达；不宣称已完成上游全部Topology/Gantt组件的生产接入。No-code原型使用上游卡片，演示有限核心任务的属性修改，不是完整动态插件Schema编辑器。历史版本差异为固定演示样例，不是正式版本服务。

## 构建与现有5181发布

在当前功能分支工作区运行：

```bash
NODE_OPTIONS=--max-old-space-size=2048 node dev/node_modules/vite/bin/vite.js build --config docs/flows-redesign/prototype/vite.config.ts
```

该目录依赖仓库`dev/node_modules`（可在本目录建立`node_modules -> ../../../dev/node_modules`链接），不单独引入依赖体系。产物输出到忽略的`dist/`，再复制到本工作区`docs/topology/prototype/public/jobs-preview/`。后者只存生成文件，本地Git忽略；不会覆盖已有全局拓扑、机柜等原型。原型资源使用相对路径，兼容带前缀的端口转发。

当前5181由独立作业平台工作区 `/home/ubuntu/keel-kestra-ui-dev/docs/topology/prototype` 提供，仍为原tmux `keel-topology-prototype`。本次只为预览路由重启了5181，未触碰5180/8081。拓扑与机柜原型源码保持原内容，仅增加作业平台入口及目录重定向；保留原工作区的fs兼容访问以支持旧页面刷新。此前主工作区的静态副本不是当前运行入口。

浏览器定向检查：`node docs/flows-redesign/prototype/verify.mjs`。结果写入`validation.json`，实际截图在`screenshots/`；只验证本原型，不能作为正式API、设备、完整编辑器或生产验收证据。

本原型仅供用户先确认UI/UX。它不增加正式作业平台完成率，也不授权部署5180或合并dev/main。

## 2026-11-02 UI尺寸与分页整改

独立原型补齐Element Plus基础样式：icon、checkbox、select/option、scrollbar、popper、loading及overlay。组件命名空间保持kel；基础样式和主题变量限定在jp-preview-document，兼容挂载到body的下拉菜单，不影响拓扑原型或正式页面。下拉箭头原先150×150，加载圆环还会扩大到容器宽度；采用官方尺寸规则，加载圆环在紧凑原型中为24px。

Flow、目标列表和Agent列表的页码/每页数量均与切片数据绑定；过滤后重置页码。目标列表只滚动数据行，分页栏保持可见。定向遍历记录见 [ui-audit.json](ui-audit.json)，复现脚本ui-audit.mjs；截图以screenshots/ui-fix-开头。未运行业务、后端或设备测试。

## 脚本导入、新增与在线编辑补齐

编辑器顶部“新增脚本”“导入脚本”，或“Files / 脚本”面板进入。支持Shell/Python新增、多文本文件导入、Monaco编辑、未保存状态、Ctrl/⌘+S保存、下载、搜索及原型内修订编号。文件状态由外层原型按项目/Namespace保存，切换上下文Tab不丢失，不写浏览器持久存储；刷新页面仍重置。

保存后“在Flow中使用”可选择Namespace文件引用或内联脚本快照；可新增任务或替换明确选中的兼容脚本任务，不覆盖Log等节点。生成的脚本任务为Kestra语义示例，不能当作Keel真实服务器/交换机任务已接通。当前不实现真实文件上传、Namespace持久化、实际执行或完整版本历史。

官方功能依据：[Namespace Files](https://kestra.io/docs/concepts/namespace-files)、[Shell Script/Commands](https://kestra.io/docs/how-to-guides/shell)。固定上游已核对`FileExplorer.vue`和`FlowFileEditorTab.vue`；原型复用KsEditor及原生组件，通过本地数据适配，不声称已移植全部上游文件管理。定向检查入口`verify-scripts.mjs`，结果`scripts-validation.json`，截图`screenshots/scripts-files.png`。
