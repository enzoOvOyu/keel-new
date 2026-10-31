# 机房 3D 空间模式 · 2026-10-31

入口：[机柜阵列 → 空间](http://localhost:5181/rack-detail?view=gallery&overlay=capacity)。本次只替换独立原型阵列中的空间展示，保留资产/计算资源阵列、单柜双面、Mapping、正式 CMDB 与业务 API。

## 实际实现

- Three.js 0.187.0 / WebGL2，机房斜俯视、垂直俯视、OrbitControls 旋转/平移/缩放、聚焦与复位；鼠标射线拾取机柜，键盘可操作机柜标签。
- 所有资产来自原 `rackApi`；16 柜、64 服务器、512 GPU、24 交换机，430 / 672 U 登记占位，242 U 未占用。U 占用复用原区间并集计算，不重复累加正背面。
- Pod/设备/机柜搜索筛选；点击机柜联动容量、立面和设备清单；双击或详情入口下钻既有双面工作台，同一资产 ID；返回保留范围，URL 可恢复。
- Three.js 只安装于 `docs/topology/prototype/rack/room/`，采用 MIT 许可的 npm 包与其 OrbitControls；未复制 Sunbird 素材。3D 模块按需加载，离开时释放 GPU/事件资源，未引入生产应用依赖。

## 数据边界

没有测绘机房 XY、建筑轮廓、机柜宽深或朝向证据。按原 Pod/机柜 ID 生成的视图坐标和代理外形**只供示意**，不写入 CMDB，不是施工依据或确认规划快照。过滤时不重新排布剩余机柜。

颜色仅表达 U 占用率。没有伪造温度、功耗、健康、PDU、空调或建筑设施。设备条带按已登记 U/高度生成，实际安装面与精确面板仍待资料补齐。

## 实际验证

- TypeScript 检查与 Vite 构建通过。3D 为独立按需 chunk；仍有 >500 kB 体积提示，未通过修改警告阈值隐藏。
- `node docs/rack-visualization/verify-room.mjs`：5 组定向浏览器检查通过，包含实际点击 WebGL 机柜实体、标签选择、右侧联动、相机模式、聚焦/缩放、URL 恢复、Pod/搜索筛选、设备下钻、返回、空结果、1440px 适配和保留资产阵列。
- 浏览器无 pageerror，无 `/api/` 业务请求。未运行全平台回归、真实 CMDB 写入或大规模压力测试。
- 只证明当前 16 柜隔离原型，不宣称千卡级生产性能或完整机房数字孪生。

记录：[room-validation.json](room-validation.json)。已目视检查截图：

- [1600px 3D 俯视](screenshots-room/oblique-1600.png)
- [垂直俯视](screenshots-room/top-1600.png)
- [1440px 适配](screenshots-room/oblique-1440.png)

设计与参考出处统一见 [rack-visualization.md §4.1a](../rack-visualization.md#41a-机柜阵列--空间机房-3d-总览)。
