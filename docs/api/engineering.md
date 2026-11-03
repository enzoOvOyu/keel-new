# 保留工程接口 · 兼容与复用边界

当前产品入口已是[Kickoff](../modules/kickoff.md)。早期engineering-imports/工程模型/发布接口仍在Go后端，旧BOM前端退役没有删除源文件、候选、工程版本、Outbox及审计。保留接口不是恢复旧五步产品的要求。

## 当前用途

- Kickoff复用eng_entity、类型化事实、来源证据、工程发布/CMDB服务；历史复用读取同项目保存的候选。
- [注册表](real-api-registry.json)中`module=engineering_legacy`的47项保留原Handler/Source及当时发布/验证信息；它们的历史revision不是当前主环境revision。
- 唯一机器契约仍是[OpenAPI](../openapi.yaml)。需要修改具体旧操作时，先查operationId、Handler与服务调用链，不能仅据本页判断所有旧链路可用。
- 旧Agent/MM理解审计只证明旧版本输入与检查；新Kickoff/analyze明确503，不能以旧Agent记录声称新Agent完成。

## 仍须遵守

项目授权/CSRF、版本冲突与幂等、候选人工审核、正式发布对象闭包、事务Outbox、回执和历史不可变继续生效。文件上传/分析/采用/发布具有不同副作用；读取接口不能触发AI或设备配置。不得用名称近似、AI置信度、缺失值补默认参数来替代业务确认。

物理端口/连接、逻辑网络、落位、功率与观测分模型。维护工程接口时保留来源、稳定资产ID及原始资料引用；不复制真实原件/凭据到文档或Git。

当前开发默认使用[Kickoff API](kickoff.md)，而不是旧workspace/采用/交接门槛。旧操作详细语义仅在维护兼容实现时按需读[历史契约](../archive/2026-11-03/previous/docs/api/engineering.md)。源路径、权限、错误、加载/缓存、部署和验证逐项以注册表及受影响Handler为核查入口，不把历史集成报告扩大成当前逐端点验收。
