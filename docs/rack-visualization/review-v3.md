# 机柜原型参考资料自审 · 2026-10-31

本轮修正仅在 `docs/topology/prototype/` 和相关设计文件中完成。正式 CMDB、数据库、业务 API、生产页面与其余四个拓扑页面未修改。运行入口：[机柜双视图](http://localhost:5181/rack-detail?camera=dual&rack=rack-0001)。

## 发现与整改

1. 正背视角只有图形变化、工作内容没有区别：Front 现在打开资产/型号/安装，Rear 打开端口/Cable/对端；后视图具备可点击的登记端口索引。双面共享设备 ID、U 坐标、选择和上下文。
2. 草稿 U 被称为登记位置：已分开草稿与 CMDB 原值，退出草稿回到登记位置，未写入业务数据。
3. 设备选择重置详情 Tab：已保持当前 Tab。明确切换正背面时才按业务目标切换资产/端接。
4. 跨柜 Cable 被画成本柜端点、缺端点仍显示完整连接：已增加柜外端点定位；缺失端点虚线终止，禁止定位不存在的设备。
5. Gallery 缩略文字不可读：改为几何缩略和外置摘要，保留当前柜标识，增强设备类型边界。
6. 安装几何解释不一致：双面和单面共用面投影，全深度不再误当未知；背装剖面从 rear 定位，0U 不占竖向 U。未把这些几何逻辑当成主要用户流程。
7. 规划/实装只剩空说明：按用户最新要求隐藏入口，旧链接返回工作台。完整分级比较和 Dual-State 交互设计在 [主文档 §4.4](../rack-visualization.md#44-dual-state-rack-comparison数据接入后开放)，未声称运行时 Diff 已完成。

## 实际检查

- `node docs/topology/prototype/run.mjs check`：通过。
- `node docs/topology/prototype/run.mjs build`：通过；仍有独立原型主 bundle >500 kB 的提示，没有将构建通过等同于大规模性能验收。
- `node docs/rack-visualization/verify-review.mjs`：8 组定向浏览器/几何检查通过。覆盖双面同身份、前后业务内容、端口→线缆→对端、Tab 保持、跨柜与刷新、缺失端点、草稿事实分离、对比入口隐藏和 Gallery。
- 补充检查：双面 SVG 导出保留端口索引内容/样式/说明，紧凑交换机背视显示端口/端接数量；通过。
- 浏览器检查期间无 pageerror、无 `/api/` 业务请求。几何样本仅在测试运行时使用，没有插入 fixture 或正式 CMDB。
- 更新了旧 V2 检查脚本中受产品变化影响的入口断言，**未重复运行整套 V2 检查**，历史结果仍是历史记录。

机器记录：[review-v3.json](review-v3.json)。实际截图已目视检查：

- [正面资产工作区](screenshots-v3/dual-assets.png)
- [背面端口与双端关系](screenshots-v3/dual-connections.png)
- [跨柜连接定位](screenshots-v3/cross-rack.png)
- [1440px Gallery](screenshots-v3/gallery-1440.png)

## 保留的真实限制

当前原型使用既有隔离合成资产投影。依据已保存的 CMDB 审计，缺少可信的安装面/深度/面板资料、已确认规划与实装快照配对、有效实时遥测。背视端口是**登记信息索引**，不是产品照片或真实端口坐标；Cable 是登记关系，不是验证通过的实际链路。真实面板、完整 Dual-State 对比和生产接入仍未实现。

没有增加图形引擎、独立资产库或新的业务审批模型。本轮设计依据与参考资料对应关系统一维护在 [主文档 §12](../rack-visualization.md#12-参考资料自审与本轮修正2026-10-31)。
