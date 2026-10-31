# 设计原型验证记录

2026-10-31，仅本目录原型。主设计唯一入口：[../../topology.md](../../topology.md)。

## 已执行

- 完整读取原 topology.md；定向审计现有 Sidebar、InfrastructureWorkspace、RackElevation、TopologyCanvas/Worker、CMDB Service、迁移、Schema、OpenAPI。
- PostgreSQL 只读聚合确认实体/关系数量；没有迁移、写入或完整生产数据质量扫描。
- `node run.mjs check`：TypeScript 检查。
- `node run.mjs build`：独立 Vite 构建。G6/ELK bundle 较大，保留构建警告；没有声称满足生产首屏或千卡性能预算。
- `node verify.cjs`：23 项针对性浏览器检查通过，原始结果 [validation.json](validation.json)。主要包括搜索定位、矩阵下钻、交换机详情、端口选择、缺端筛选、线缆→追踪、断点、逻辑证据不足、前后同身份、U重叠阻止保存、草稿本地保存/撤销、Mapping、刷新恢复、PDU缺测、保存视图、1280布局。
- 1440×900 逐页截图并人工查看布局；1280×900 检查整体没有横向溢出。宽矩阵/表格在自身容器滚动。
- 浏览器没有未捕获异常，**业务 `/api` 请求为0**。原型没有登录、AI调用、CMDB写入或正式接口代理。

交互检查过程中修正：Vite将同名顶层tsx误当页面返回（实现文件迁到pages/）；快速连续筛选覆盖新URL（按最新URL合并）；网络全量Leaf导致文字过小（Pod默认聚焦）；G6自动放大少量节点过大（缩放上限与留边）。U33→U32实际与相邻设备重叠，检查确认阻断；U34可保存草稿，未改原始位置。

## 五个页面截图

- [全局拓扑](screenshots/global-topology.png)
- [网络架构](screenshots/network-architecture.png)
- [物理布线 / Port Panel](screenshots/physical-cabling.png)
- [链路追踪](screenshots/link-tracing.png)
- [机柜详情](screenshots/rack-detail.png)
- [1280 布线目录](screenshots/physical-cabling-1280.png)

## 范围与未完成能力

- 原型表现的是明确标记的合成BOM/CMDB投影，不是当前正式数据库实时内容。演示健康不会被标为当前通过。
- 四种布线视图已制作；自动检查重点为端口面板/Inventory和网络矩阵，没有穷举每个图节点、每种排序组合及所有端口。
- 没有配线架贯通、PDU插口/供电、FIB/VRF转发证据；对应路径、逻辑追踪和冗余结论不可用/未知，没有伪造。
- 原型导出SVG/CSV与本地展示草稿；正式ChangeSet提交、PNG/PDF完整交付、服务器五Tab/交换机八Tab完整抽屉仍使用既有业务实现，本阶段原型只示意专属只读对象面板，不重建这些后端功能。
- 没有生产数据写入、数据库迁移、真实设备验证、权限绕过测试或10000节点压力测试。
- 主文档中的专用子图/矩阵/trace契约、快照游标、端口约束ELK、增量布局是后续设计，不表示已实装。
- 类型检查和构建不代表上述生产能力完成。第一阶段结束，等待用户设计确认。
