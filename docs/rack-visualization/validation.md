# 机柜设计原型 · 必要验证记录

日期：2026-10-31。范围：`docs/topology/prototype` 的独立 rack-detail。不是正式 `/racks` 发布验收，不连接真实设备。

## 实际运行

- `node docs/topology/prototype/run.mjs check`：TypeScript 检查通过。
- `node docs/topology/prototype/run.mjs build`：Vite 构建通过；独立原型 bundle 仍有 >500 kB 提示，包含静态合成数据，未宣称完成生产体积优化。
- `node docs/rack-visualization/verify.mjs`：几何与本页浏览器关键交互通过。早期检查脚本的异步读取/SVG text 访问问题已修正，以下结果取最终成功运行。
- 截图检查发现端口速率已带 G 单位，再拼接会重复；已改为保留原值，并定向确认缩放真实增加画布尺寸。
- Chromium 1600×1040、1440×960 实际打开并检查；页面无未捕获运行异常、**0 条业务 API 请求**。
- 检查原始记录：[validation.json](validation.json)。只读 CMDB 聚合：[cmdb-capabilities.json](cmdb-capabilities.json)。

## 关键结果

| 检查 | 结果 |
|---|---|
| Gallery 16 柜、64 服务器、24 交换机、512 登记 GPU | 通过；来源为独立合成投影 |
| U 占用由区间并集计算 | 430 / 672U；RACK-01 为 29U / 13U 未占用 |
| Pod 筛选、SN/设备 ID 搜索、设备所在柜定位 | 通过；不按名称生成关联 |
| 两面身份/U位相同、选择联动、URL刷新恢复 | 通过 |
| Asset / Capacity / Compute | 通过；无 Power/Health/Thermal 假数据入口 |
| Mapping 设备→端口→线缆→追踪 | 通过；保留完整 ID 与证据异常 |
| 规划/实装前提缺失 | 比较按钮禁用，数据依据可查看 |
| 42/45/48U、半 U、0U、越界、全深度与未知面冲突 | 独立几何用例通过；未将检查样本插入 CMDB |
| 草稿移动、冲突阻断保存、撤销、确认保存、退出恢复 | 通过；正式登记 U 未改变 |
| 双面 SVG 导出 | 包含 FRONT/REAR 和 SYNTHETIC 标识 |
| 1440px 布局、浏览器错误与 API 隔离 | 无页面水平溢出、无运行错误、无业务请求 |

## 截图（已实际查看）

- [原页面基线](screenshots/before.png)
- [Rack Gallery](screenshots/gallery.png)
- [双面立面与资产](screenshots/dual-asset.png)
- [Compute](screenshots/dual-compute.png)
- [Capacity](screenshots/dual-capacity.png)
- [Mapping](screenshots/mapping.png)
- [规划/实装数据前提](screenshots/compare-prerequisites.png)
- [1440px 双面选择状态](screenshots/dual-1440.png)

## 未验证 / 未实施

生产接入、实际位置写入/审批、型号精确面板、PDU/侧挂安装、真实遥测、实际批准规划与实装差异均未实施。半 U/0U 的底层几何已检查，当前 CMDB 没有对应真实场景，不能声称已验证现场设备安装。没有做千卡规模、全站回归或用户完整闭环验收。
