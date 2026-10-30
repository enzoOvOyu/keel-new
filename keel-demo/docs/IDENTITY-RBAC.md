# 身份认证与项目授权（真实实现）

## 范围与来源

当前 dev 使用 Go/PostgreSQL 的本地身份与 RBAC。BOM 使用真实 API；其余交付/设备操作继续显示 SIMULATION_ONLY。身份登录统一在 `/login`，不再在 BOM 内重复登录。

登录页面直接迁入用户仓库 `enzoOvOyu/keel` 的 `web/src/features/auth/LoginPage.tsx`，来源提交 `a2dcb508648330b223ae94151875953c287a8f0a`。保留 JSX 布局、Brand、KeelStructure、Preferences、字体、设计令牌及中英文文案；仅适配现有 React Router、shadcn 导入与当前 API。样式限定在 login-shell，避免改变控制台。生命周期文案沿用当前六阶段。

## 唯一权限定义

默认角色和权限集合以 `dev/backend/internal/identity/catalog.go` 为权威，数据库保存 Role、Permission、RolePermission。前端只消费真实会话的 permissions、grants、authorized_projects，不自行推导授权。历史 DE/DL/NE/SRE/Viewer 只在一个兼容适配器中用于旧 Mock 展示，不能成为真实授权依据。

| 稳定 ID | 范围与职责 |
|---|---|
| viewer | 授权项目只读 |
| delivery_engineer | 交付执行、BOM、装机/验机/测试/清理；可担任独立 DE 签署人 |
| delivery_lead | 项目成员管理、交付审批与豁免，不自动获得执行权限 |
| developer_sre | Flow、自动化、研发工具；不是平台管理员 |
| network_engineer | 网络诊断、配置、BOM 网络资料和处置；不继承装机/清理权限 |
| admin | 平台级治理与权限管理；不自动具备 NE/DE/DL 业务签署资格 |

普通角色仅通过 ProjectMembership 获得项目权限；一个项目多个角色按权限并集计算。资源授权附 resource_type/resource_id，只有匹配资源可使用。平台角色仅 UserGlobalRole(admin)，不可将项目角色变为全局权限。有效期与撤销在每次请求重新查询，默认拒绝。权限版本供 UI 刷新，不作为服务端授权缓存。

## 数据与会话

复用已有 api_session、audit_event，新增 app_user、role、permission、role_permission、user_global_role、project、project_membership、login_event、approval_record、approval_signature。版本迁移 010–013；旧无 user_id 会话失效。Session 对外 ID 是独立 UUID，原始 Cookie Token 不进入响应、普通日志或审计。

密码使用 Argon2id（64 MiB、3 次、并行度 2、独立随机盐）；本地密码长度 12–256 字节。Session 为 8 小时 HttpOnly、SameSite=Lax Cookie；生产 APP_ENV=production 启用 Secure，需 HTTPS。写接口必须同时通过 Cookie、CSRF 与来源校验。密码/Session 不放 localStorage，只有语言和主题偏好可保存在本地。

失败登录按账号 5 次锁定 10 分钟，来源 IP 10 分钟 20 次失败限流。管理员可解锁；禁用、改密、重置密码撤销旧 Session。显式撤销 Session 立即生效。授权撤销不依赖浏览器刷新，下一次 API 请求即重新判定。未配置 LDAP/OIDC 时 providers 仅返回 local，不出现假入口；尚未接入或验证外部身份服务。

## 项目与前端

ProjectSwitcher、项目目录和所有部署请求共用 ProjectContext；后端从资源所属项目或 URL 解析范围，拒绝 query project_id 与资源所属项目不一致。会话仅返回授权项目。跨项目列表继续使用授权集合与可清除筛选，不把普通项目权限当作全局权限。各标签页内存维护可变上下文，切换取消旧查询。

管理入口：`/admin/users|members|roles|sessions|logins|audit|approvals`；账户安全 `/account/security`。项目组长通过用户菜单进入成员管理；无管理权限页面显示 403。实际写接口再次授权。管理列表使用紧凑搜索/排序/分页及详情对话框；授权变更和会话管理均调用真实 API。

## 审批与权限分离

审批申请冻结 project_id、resource_type、resource_id、snapshot_version、target_ids、有效期和申请人。申请人不能签署自身操作，同一人不能兼任两个签署职责。普通网络变更必须不同 NE + DE；BIOS、固件、BMC 电源、重装和清理的当前审批策略使用独立 DL，真实设备执行尚未接入。

Critical 网络仍要求 NE + DE + NH。PRD 已有 NH 业务职责，但六个默认身份角色没有 NH 资格绑定；记录 DECISIONS，不将 NH 等同 admin。此类申请进入 blocked_policy，执行校验拒绝。执行前复核签署人仍有效及冻结范围一致；配置版本/目标改变必须重申。`/check` 只返回授权/审批结果并明确 device_execution=not_performed，不执行设备操作，也不替代维护态、门禁和后续设备适配器安全检查。

## API 与审计

HTTP 详情和 Schema 只以 openapi.yaml 的 Identity* 定义为准。统一 `/api/v1`、data/request_id、错误码、X-CSRF-Token；安全主体使用后端 cookieSession。读取当前用户、角色目录、授权项目、成员、会话、登录和权限审计；创建/禁用/解锁/改密/重置、成员授予/撤销、平台管理员授予/撤销、审批申请/签署/校验均为真实实现。

权限变更及 before/after 审计在同一事务提交；快照只包含非敏感账户属性、授权和活动会话数，不含密码哈希、CSRF、Cookie。登录记录独立保存结果与来源。当前管理接口最多返回最近/前 200 条，前端在此范围内搜索和分页；大规模服务端检索/保留周期尚未实现，不以此宣称企业审计归档完成。

## 验收依据

`dev/scripts/identity-check.py` 使用独立合成账号验证六角色、跨项目、直接越权、撤销、账号禁用、Session、改密、限流、职责分离和审计，结束后禁用合成账号。结果见 IDENTITY-ACCEPTANCE.json。身份单元测试覆盖密码与项目/资源权限边界。实际HTTP验收16项，63个成功响应通过OpenAPI Schema校验；浏览器也检查撤销当前会话后返回统一登录。真实 BOM 浏览器回归覆盖统一登录后上传、映射、修正、校验、审核、提交及 CMDB 发布；不代表真实设备或外部 LDAP/OIDC 已验证。
