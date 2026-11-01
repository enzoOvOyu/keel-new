# Keel dev — BOM 智能开局配置

本目录是唯一应用开发入口，后续统一在Git dev分支开发；旧Demo已归档，不再独立运行。下文保留历史BOM实施说明，最新状态以根HANDOFF为准。BOM 六步工作台接真实 Go/PostgreSQL/MinIO；其他域仍明确使用原合成 Mock，不冒充真实设备后端。

## 启动

1. 保持根 compose 的 PostgreSQL、MinIO 运行。
2. 配置 `backend/.env`（参考 `.env.example`）；秘密配置不进入 Git。
3. 从仓库根目录运行 `./dev/backend/migrate.sh`，显式执行尚未应用的版本迁移。
4. 运行 `./dev/backend/run-local.sh`，Go 默认监听 localhost:8081。
5. 在 dev 运行 `npm ci`、`npm run dev`，前端默认端口5174。旧 `keel-dev-web-1` 占用该端口时先停止它。
6. BOM入口：`http://localhost:5174/p/proj-b300-demo/deploy/bom`。

统一登录入口为 `http://localhost:5174/login`。首次空库通过受控 `KEEL_BOOTSTRAP_PASSWORD` 初始化 `admin`，现有账号不在启动时重置；BOM不再出现独立登录表单。实际本地密码由交付消息提供，不写入Git。

首次依赖安装后，可按 backend/Dockerfile 构建镜像。可选 compose 使用当前 Ubuntu 的 host 网络访问已准备的本机 PostgreSQL/MinIO，并读取受控 backend/.env；本轮实际运行验证使用本地 Go 进程，没有宣称验证过 Docker 镜像构建。

## 能力与边界

完整实现链路说明见 `../docs/BOM-REAL-DESIGN.md`。真实文件解析、人工配置、候选/关联校验、版本审核、事务BOM提交、Outbox和CMDB投影均使用真实服务与存储。真实Provider通过服务端HTTPS适配器配置；未配置时Rules仍可完整运行，Mock错误可诊断，不修改正式数据。

身份与 RBAC、项目成员、Session、审计和审批已真实接入，见 `../docs/IDENTITY-RBAC.md`。ProjectSwitcher只展示获授权项目；其他设备域继续Mock，LDAP/OIDC和真实设备未验证。Critical NH签署资格尚待产品确认，不等同Admin。

## 相关验证

- `cd backend && go test ./...`
- `go run ./cmd/testworkbook /tmp/keel-api-acceptance.xlsx`
- 仓库根目录：`python3 dev/scripts/bom-api-check.py`、`python3 dev/scripts/check-bom-responses.py`
- `npm run build`

真实Excel和凭据文件被Git忽略。验收样本由testworkbook命令生成，全部身份带synthetic标记，不能当成真实客户物料。

身份定向检查：`python3 dev/scripts/identity-check.py`；`cd dev/backend && go test ./internal/identity ./cmd/api`。
