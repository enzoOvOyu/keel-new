# BOM 真实开发实施状态

当前为进行中的真实开发切片，未完成用户要求的全量 BOM 验收。代码位于 dev/；原 demo 保留。

已运行：Go 单元测试（解析脱敏/来源、无效端点、Mock非法输出/失败），真实 netbom.xlsx 全15 Sheet结构读取，真实 MinIO 上传与PostgreSQL任务入库及异步解析；dev 生产构建、浏览器BOM入口检查通过。浏览器无 pageerror。原始结构统计见 NETBOM-STRUCTURE-REPORT.json。未执行真实Provider，不宣称AI识别率。

未完成：完整OpenAPI兼容适配、逐Sheet语义配置与复杂映射、跨Sheet工程关联、已审批型号能力与全面工程校验、权限/会话体系、Provider诊断评测持久化、完整CMDB关系/位置投影、下游正式资产API接入、真实样本完整审核提交验收。当前仅服务令牌单项目dev访问，不能视为生产RBAC。现有结构化审核为基础版本，不等于完整人工工作台。

不将真实Excel复制到Git；未把待审核样本发布为正式资产。运行方式和限制见 ../dev/README.md。
