# V3 四页原型验证 · 2026-10-31

范围：全局拓扑、网络架构、物理布线、链路追踪的隔离设计原型。机柜源码与数据、正式前端、数据库和业务 API 未修改。旧 V2 验证记录不作为本轮验证结果。

## 实际运行

- `node docs/topology/prototype/run.mjs check`：通过。
- `node docs/topology/prototype/run.mjs build`：通过，Vite 6.4.5。主 JS 1,005.51 kB / gzip 150.10 kB，有超过 500 kB 的 chunk 提示；本轮未做生产拆包或性能达标承诺。
- `node docs/topology/prototype/verify.cjs`：23 项定向检查通过，Chrome/Playwright，1440×900 和 1280×900。完整结果：[validation-v3.json](validation-v3.json)。浏览器未捕获异常 0；业务 `/api` 请求 0。
- `git diff --check`：通过。
- 服务继续运行于 `http://localhost:5181`，tmux `keel-topology-prototype`；5180 正式开发服务未替换。

## 实际检查内容

1. 四个 Pod 的 Hybrid 容器；展开 Pod 对应 16 台服务器；搜索、稳定资产 ID、刷新恢复与 Escape 关闭面板。
2. 网络 Pod1 的 175 条完整 Cable 聚合为 20 束；选中39条成员束可展开24条，余下仍在分页清单；Bipartite每页12条且可翻页。
3. 物理布线默认矩阵；b300-001 × leaf-01 对应9条完整记录，第10条缺端记录仍在托盘；单元格保存稳定设备 ID。
4. 选中上述矩阵集合后，BioFabric显示9列、18个真实端点标记；点列打开正确NIC1/P1与swp1；刷新保留表达模式与对象。
5. 缺B端线缆跳转Metro Route保留断点；正常记录展示端口/Cable/端口；关联树10条独立端接，选择NIC4切换到缺端记录。Logical模式缺FIB，开始按钮禁用。
6. 1280宽度无整页横向溢出；矩阵和密集索引内部滚动。

检查中发现并修正：Pod标题组没有独立命中区域；水平束线的SVG几何包围盒高度为零。现在标题与束线均有明确可点击区域，鼠标与键盘操作均有入口。未通过强制点击绕过问题。

## 截图检查

| 视图 | 截图 |
|---|---|
| Cluster Map / Hybrid | [global-topology.png](screenshots-v3/global-topology.png) |
| Fabric Flow | [network-architecture.png](screenshots-v3/network-architecture.png) |
| 连接束展开 | [network-selection.png](screenshots-v3/network-selection.png) |
| Bipartite | [bipartite.png](screenshots-v3/bipartite.png) |
| Connection Matrix | [physical-cabling.png](screenshots-v3/physical-cabling.png) |
| BioFabric | [biofabric.png](screenshots-v3/biofabric.png) |
| Metro + Tree/Table | [link-tracing.png](screenshots-v3/link-tracing.png) |
| 物理断点 | [trace-breakpoint.png](screenshots-v3/trace-breakpoint.png) |
| 1280宽度 | [physical-cabling-1280.png](screenshots-v3/physical-cabling-1280.png) |

实际查看截图确认：没有四页复用同一个普通节点图；默认没有空详情栏；图例与数量口径可见；矩阵异常不依赖颜色单独表达；路径与证据在同一工作区。未做自动标签碰撞检测或受试者可用性实验。

## 数据口径与未验证能力

沿用原合成切片：64服务器、512GPU的服务器关联、24交换机、16机柜、1664端口、704 Cable；没有生成新的资产副本。缺端/stale/conflict为既有演示反例，不是真实故障。

不同视图的数量按其范围计算：网络Pod1角色图只含本地Leaf相关骨架；物理Pod范围含任一端设备位于该Pod的记录（可能含跨Pod到本地Spine的连接）。矩阵进一步按轴类型投影，页上分开展示进入矩阵和范围总数，不以差额伪装丢包或漏线。

未验证：生产API与权限、真实设备、千卡/万节点性能、完整多跳/patch-through、逻辑转发路径、冗余故障影响、Graphon算法复现、论文全套量化指标。机柜未重新设计或重测。正式实施等待用户评审。
