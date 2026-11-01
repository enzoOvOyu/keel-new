# 作业平台交互设计预览

预览入口：<http://localhost:5181/jobs-preview/index.html#flows>。

此目录是独立设计原型，不修改正式`/flows`、不访问业务API、不写数据库或设备。所有项目、设备、执行、日志与产物均来自`data.ts`，明确标记synthetic；运行/暂停/批量动作只改变原型内存。保存视图仅使用独立的`keel-jobs-preview-view`会话偏好，不保存凭据。

## 页面与操作

- `#flows`：Flow目录、搜索、Namespace/类型/状态筛选、原生表格选择、批量启停、列显示、分页及最近执行入口。
- `#editor/gpu-preflight`：原生多面板Visual/YAML/Topology/Files/Docs；节点选择、属性编辑、拖动排序、添加/删除、代码同步和原型版本保存。Overview/Revisions/Triggers/Executions是同一Flow上下文中的设计预览。
- `#execution`：Gantt/任务详情、Attempt选择、目标矩阵/列表、日志筛选、变量与输出、上下文和操作历史；失败目标重试与全部重跑分别确认。
- `#executions`：执行目录与运行/关注筛选。
- `#agents`：Agent列表、设备详情抽屉、演示纳管与批准。
- Quick Run：目标→任务→预检确认→原型执行，支持暂停/继续/终止，均不调用真实执行服务。

Keel外壳仅保留一套导航、项目切换与主题；其他业务菜单在此独立原型中禁用，不制造假业务入口。项目切换用于目录筛选，本原型不提供真实授权。

## 原生来源与边界

复用固定Kestra `49b62a2c820d37e98a1c7b63aa35fd83f960d4ec` 的KsDataTable、KsButton、KsInput、KsExecutionStatus、KsDialog、MultiPanelGenericEditorView/MultiPanelTabs、KsEditor/Monaco、SidebarList及No-code卡片。许可证与适配说明见`dev/src/vendor/kestra/PROVENANCE.md`。

拓扑与Gantt在本原型中是用于审核布局和联动的专属表达；不宣称已完成上游全部Topology/Gantt组件的生产接入。No-code原型使用上游卡片，演示有限核心任务的属性修改，不是完整动态插件Schema编辑器。历史版本差异为固定演示样例，不是正式版本服务。

## 构建与现有5181发布

在当前功能分支工作区运行：

```bash
NODE_OPTIONS=--max-old-space-size=2048 node dev/node_modules/vite/bin/vite.js build --config docs/flows-redesign/prototype/vite.config.ts
```

该目录依赖仓库`dev/node_modules`（可在本目录建立`node_modules -> ../../../dev/node_modules`链接），不单独引入依赖体系。产物输出到忽略的`dist/`，再复制到现有5181原型的`docs/topology/prototype/public/jobs-preview/`。后者只存生成文件，本地Git忽略；不会覆盖已有全局拓扑、机柜等原型，不需要新服务或端口。

浏览器定向检查：`node docs/flows-redesign/prototype/verify.mjs`。结果写入`validation.json`，实际截图在`screenshots/`；只验证本原型，不能作为正式API、设备、完整编辑器或生产验收证据。

本原型仅供用户先确认UI/UX。它不增加正式作业平台完成率，也不授权部署5180或合并dev/main。
