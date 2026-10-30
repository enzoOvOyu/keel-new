# 项目创建与智能开局 AI 配置

## 当前可用入口

- `/projects`：真实创建空白项目；项目编码3–32位字母/数字/连接符/下划线，大小写不敏感唯一。创建后直接进入该项目 `/p/:projectId/deploy/bom`。
- `/admin/ai-provider`：系统管理 → AI Provider 配置，平台管理员管理；现有 `/ai/providers`复用同一组件和同一配置源。
- BOM 识别诊断：Rules / Mock / Provider；Provider可选择默认服务或指定已启用服务，并能跳转系统管理配置。

本轮不把尚未接入后端的演示项目模板当作正式模板；先建空白项目再导入。其他交付执行仍按既有Mock边界，不因此变成真实硬件执行。

## 项目创建

复用既有project、project_membership、audit_event。事务创建planned项目，赋予创建者项目级delivery_engineer和delivery_lead角色并更新权限版本；不授予network_engineer。角色并集不解除既有高危操作自批禁止规则。重复项目编码由数据库唯一索引拒绝。前端沿用现有项目卡片和创建对话框。

## Provider 与真实调用

平台级ai_provider_config保存名称、协议、模型、规范化Endpoint、受控credential_ref、启用/默认状态、超时、输出预算、Token参数、JSON模式、版本和最近测试结果。任何读取均不返回api_key。编辑时密钥留空保留原引用；写入新密钥产生新受控引用。版本冲突返回409；同一时刻只有一个默认且启用的Provider。

支持两种协议：

1. OpenAI兼容Chat Completions。Base URL需包含服务要求的版本路径；后端补全`/chat/completions`，已填完整路径时保留。按服务能力选择max_tokens或max_completion_tokens；可关闭json_object请求选项，但返回值仍必须通过mapping/v1校验。
2. 现有Keel mapping/v1结构化协议，填写完整接口地址。

[官方Chat Completions接口](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create)用于核对messages、response_format及响应封装；不将特定供应商模型、URL或凭据设为项目常量。

Go后端加载配置和凭据并调用服务；浏览器只请求Keel API。无Provider时Rules和人工映射继续工作。AI任务诊断记录Provider ID、配置版本、协议、模型、Prompt/Schema版本、耗时和可获得的Token使用量，候选必须人工应用和审核；AI不提交正式BOM。

## API 对照

以下路径前缀`/api/v1`，使用既有Session、CSRF、permissions[]及统一错误格式。

| API | 权限 | 行为 |
|---|---|---|
| POST /projects | 全局project:create | 事务创建项目、创建者成员关系和审计 |
| GET /admin/ai-providers | 全局admin:read | 返回脱敏配置及最近测试状态 |
| POST /admin/ai-providers | 全局admin:edit | 保存配置，api_key只写 |
| PUT /admin/ai-providers/{id} | 全局admin:edit | revision并发校验，留空保留密钥 |
| POST /admin/ai-providers/{id}/test | 全局admin:edit | 冻结revision，发送合成字段，返回真实HTTP/JSON结果 |
| GET /ai/provider-options?project_id= | 项目ai:read | 仅启用服务的非敏感选择信息 |
| POST /bom-imports/{id}/ai-tasks | 既有BOM项目权限 | provider_id可选，不传则加载默认服务 |

连接测试不发送原始Excel，可能消耗少量Token；HTTP成功但JSON错误仍报告失败。测试通过不代表真实工作簿识别准确率，不代表BOM已审核。

## 凭据与运行

018、019迁移新增配置、唯一默认约束、Token参数和项目编码约束。当前开发受控凭据存储为backend/.secrets/ai-providers，目录0700、文件0600；PostgreSQL只保存local-secret引用。该目录同时被Git和Docker构建上下文排除。宿主Go进程以backend为工作目录；可通过KEEL_SECRET_DIR指定持久化目录。

Docker API配置了独立ai-secrets持久卷和非root可写目录。宿主进程与Docker卷不是同一目录；切换运行方式时须受控迁移引用对应的凭据文件，不要以普通配置文件或Git传递密钥。本轮实际验证宿主Go进程，没有重建Docker镜像。生产Vault/KMS接入尚未实现，不把本地文件引用宣称为企业密钥管理能力。

## 本轮实际验证

- Go相关AI协议测试与API编译通过，新增标准Chat messages/Token参数/响应封装用例。
- React生产构建通过。
- 浏览器真实创建项目、进入BOM、保存配置、刷新后密钥为空、后端调用本地合成服务、BOM共享配置目录通过，详见PROJECT-AI-CONFIG-CHECK.json。
- 表单布局检查只针对新增页面，未重复运行全平台冒烟。
- 合成Provider和测试凭据已清理。真实外部Provider仍待用户配置和验证，未声称真实模型准确率或外部服务可用性已通过。
