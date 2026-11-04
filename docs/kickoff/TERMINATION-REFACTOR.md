# 端接驱动的智能开局闭环

## 当前范围

资料与识别 → 人工采用 → 数据核对 → 入库与版本。端接表建立项目第一份规划物理底账；不以SN、MAC、机房、U高、线长或网络配置作为全局开局前置。保留现有后台任务、MinIO、工程版本、CMDB和事务Outbox，不建立平行资产或队列。

## 本轮实现

- AI先定位端接区域，调用inspect_connection_graph读取全量端接的节点、端口、邻接、速率/型号证据及冲突。AI据此判断对象及关系，再用evaluate_relationships验证别名/位置/规格跨表假设。速率和数量不是设备角色硬编码规则。
- 连接的a_device_ref/b_device_ref指向源设备名称；serial是独立可选字段。名称不冒充SN，型号不作为实例唯一键。
- 从有来源的端接投影设备与端口；独立清单只补充端接设备。设备多网络不拆分身份；角色保留到端口，位置与线缆保留其具体关联和来源。
- 程序展开全部行并校验关系。缺一端的有效行仍保留为异常，不因首列为空消失。未知类型不妨碍明确设备/连接采用。
- 线长统一为米；无显式单位按用户确定的米制约定，显式其他单位转换。规划长度不变成实际安装长度。U起点与占用高度分开。
- 源文件不可变、解析及候选按来源版本隔离；新catalog/prompt指纹使旧语义提取计划失效，不从旧候选补回删除字段。
- 新flowVersion=termination-v1五步：端接资料定位、设备与端口识别、关系推理与佐证、位置与线缆关联、候选完整性核对。历史报告保持旧语义；新任务pending依据入队版本，执行日志只报告已发生工作。
- 采用/核对/范围发布使用同一稳定身份与关系；保留并发修订、人工确认与observed保护。可选属性无效可以保留缺口，真实身份和端点冲突仍阻断相关对象。

## 验证边界

使用合成工作簿及模拟Provider检查原生工具调用、无SN全量展开、图关系冲突、不完整端接行保留；定向领域用例覆盖候选投影→采用→核对→冻结发布计划、重复采用及SN增补。模拟结果不代表真实模型识别质量。

真实客户工作簿AI识别、真实数据库采用和CMDB消费验收未运行；不得宣称这些已经通过。部署以DEPLOYMENT.json记录为准。

## 定向验证结果

- `go test ./internal/ai -run TestKickoff -count=1`：通过，含原生工具交互、无SN端接及不完整行保留。
- `go test ./cmd/api ./internal/kickoff -run 'Test(Kickoff|AdoptionEvidence|AdoptionCable|ResolveIdentitySourceBackedName|NameIdentityConflictBlocksPublication|UnassignedRack|KnownRack|PublicationFacts)' -count=1`：通过，含反向连接/证据、名称标准化、冲突SN不可采用、多角色、无SN发布计划。
- `npx tsc --noEmit`、`npx vite build`、`node dev/scripts/check-real-api-docs.mjs`：通过。Vite保留既有大chunk和第三方注释提示。
- 无全仓回归、无真实Provider调用、无客户资产写入。

部署：5180/8081运行`7c26376`。三个真实页面只读加载检查通过，历史结果保持旧语义，无横向溢出、浏览器异常或业务写入；见[页面检查](TERMINATION-UI-CHECK.json)。新版真实Provider识别/客户数据入库验收仍未执行。
