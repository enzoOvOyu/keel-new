# BOM 智能开局配置：真实实现与接入边界

本轮用户已明确授权 BOM 模块真实 Go 开发，取代该模块此前“仅 Mock”的阶段限制。其他模块继续按各自实施阶段推进；不恢复旧数字门禁主链。

## 单一数据链

原始文件以服务端生成对象键进入受控 MinIO；SHA256、导入状态、修订、Sheet Profile 和候选记录在 PostgreSQL。Excelize 只读结构、缓存值和公式文本，不执行公式、宏或外部链接。所有 Sheet、单元格、合并与隐藏状态保留；未知区域须人工映射或明确处置理由，不自动成为资产。

`internal/bom` 按 workbook/profile/mapping/relation/engineering/package 划分解析、映射、关联、规则和标准包职责；`internal/ai` 提供统一候选契约、Rules/Mock/Provider、独立版本 Prompt 与脱敏评测；Go HTTP 层负责 import/review/bom/cmdb 协调与持久化。是一个模块化单体，不是第二套 Python 服务或多个微服务。

候选身份在审核中不变；提交时统一解析父子/端点图并映射为稳定物料身份。相同明确序列号可合并跨 Sheet 一致记录，冲突保持 BLOCKER。名称匹配仅产生候选，不单独批准连接。线缆两端必须明确引用 PhysicalPort；端口由显式设备和端口字段形成候选并人工确认。未知 SN、型号能力、单位、U 高或网络规划不得补造。

## 版本、审批与发布

修正产生新 revision，清除旧审核；历史候选快照不可覆盖。全部 Sheet 处置和字段确认、型号版本、规则校验与最终人工审核绑定当前版本。校验任务持久化，支持超时任务恢复；版本变化使旧任务失效。BLOCKER 阻断提交。

提交同一事务落 bom_revision/bom_item/bom_item_revision/project_asset、审计及 Outbox。物料身份不依赖行号或 Hostname。同批次重复提交返回同版本；幂等键跨导入复用拒绝。相同物料再次导入复用身份，版本保留快照。端口和机柜位置为统一物料投影；不复制资产身份。Outbox 消费者幂等建立 CMDB entity 和组成/线缆关系并记录发布审计，失败保留事件供后续重试。下游只读接口读取正式 BOM 版本，未审核候选不进入已发布目录。

BOM 提交本身不能单独证明 KICKOFF 阶段通过；网络基线与其他开局证据仍需完整。后续五阶段与 maintained/IPMI-only 原规则不变，本模块不连接任何设备。

## AI 边界

默认 Rules；无 Provider 仍可解析、人工映射、校验、审核、提交及发布。Mock 独立标明 SIMULATION_ONLY。Provider 由服务端受控配置调用，只发送批准的结构化表头上下文，不发送原始工作簿、凭据或完整行。JSON 输出校验与体积/超时限制失败时仅生成诊断，不破坏导入。三个 Prompt 独立版本化；候选不自动写正式 BOM。

通用 Provider Adapter 的服务约定为 HTTPS POST，返回 mapping/v1 候选数组或 candidates/usage 信封；具体商业服务可通过同接口适配。没有配置实际供应商和密钥时不宣称真实 AI 验收。token 数据不存在时为 null/unknown，不估造成本。独立脱敏 golden 评测区分 Rules/Mock/Provider，分开记录字段、Sheet 和关联成绩；golden 所需修正比例不等于实际人工工作量。

## API 与前端

HTTP 契约仍为 openapi.yaml。复用原 BOM 导入、Sheet、单字段映射、校验、预览、提交和批次接口；补 profiles/draft/relations/review/publication/history/retry/ai-tasks/evaluate、型号和映射模板专用接口。新增 Schema 由 OpenAPI 生成 TypeScript；没有增加平行 BOM API 数据体系。

前端位于 dev/，保留现有壳层和共享 UI，BOM 使用真实统一 Client、TanStack Query、React Hook Form/Zod。六步工作台呈现解析来源、Sheet/字段配置、关系候选、分类预览、问题修正及审核提交。支持来源定位、筛选、排序和分页；导入 ID 写入路由。其他原 Mock 页面继续清楚标识演示，不代表其已接真实服务。

## 安全与运行

本地账号由受控环境配置；浏览器使用 HttpOnly/SameSite Cookie、写请求 CSRF 与 Origin 检查。每次 API 查询和修改校验服务端项目授权；Viewer 禁写。服务令牌只供受控 CLI，开发代理不向匿名浏览器注入。账号、AI Key 和对象存储凭据均不进入 Git/fixture/普通日志。

文件限20MiB、64 Sheet、每Sheet20000行/512列；ZIP解压上限256MiB、单项64MiB，拒绝路径穿越/重复路径/符号链接。原文件保持不可变对象引用；敏感列和键值布局脱敏，CSV导出防公式注入。部署只运行显式版本迁移，不启动时重建数据库。

开发入口 localhost:5174，Go localhost:8081；原 demo 5180 保留。运行、账号配置和迁移说明见 ../dev/README.md。实际验收见 BOM-REAL-ACCEPTANCE.json；未配置真实 Provider、未人工消歧的实际工作簿不能标通过。

## 标准包兼容性

按 FR-2.1 七CSV目录和现有 OpenAPI BOMItem 字段生成不可变ZIP，manifest含摘要、行数和 schema_version=keel-bom-openapi/v1。仓库没有独立完整“附录A CSV列清单”；不声称已认证未提供的历史列格式，外部既有包兼容需以其真实规范补测。此差异不影响当前真实 API 事务提交和 CMDB 发布。

## 本轮实现补充

逐字段人工修正保存actor、理由、前后值和时间；审批绑定不可变型号版本引用。明确批次编号同摘要幂等、异摘要409；批准模板按唯一表头匹配，可处理列序变化。映射源行不能静默消失，排除必须记录源行与理由。布局采用来源单元格关联的人工落位，不猜测设备高度或U位；位置投影仍引用统一资产。

AI三模式复用候选结构与生成/审核管道；应用字段/Sheet候选必须验证真实源坐标并由人确认，不直接提交。关联建议仍需显式双端确认。WARNING需当前修订确认，INFO不冒充阻断；计数/身份缺失不伪装重复证据。发布自动轮询，故障显示重试状态，旧事件不能回退较新CMDB版本。

实际运行与缺口只以BOM-REAL-ACCEPTANCE.json为准。真实netbom尚未人工确认，不将其标为已正式提交。
