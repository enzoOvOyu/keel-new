# Keel dev — BOM 智能开局配置

前端从 keel-demo 复制到 dev，原 demo 5180 保留。BOM 六步工作台接真实 Go/PostgreSQL/MinIO；其他域仍明确使用原合成 Mock，不冒充真实设备后端。

## 启动

1. 保持根 compose 的 PostgreSQL、MinIO 运行。
2. 配置 `backend/.env`（参考 `.env.example`）；秘密配置不进入 Git。
3. 从仓库根目录运行 `./dev/backend/migrate.sh`，显式执行尚未应用的版本迁移。
4. 运行 `./dev/backend/run-local.sh`，Go 默认监听 localhost:8081。
5. 在 dev 运行 `npm ci`、`npm run dev`，前端默认端口5174。旧 `keel-dev-web-1` 占用该端口时先停止它。
6. BOM入口：`http://localhost:5174/p/proj-b300-demo/deploy/bom`。

账号为受控配置中的 `KEEL_LOGIN_USER`（本地默认developer），密码在 `backend/.env` 的 `KEEL_LOGIN_PASSWORD`；只读账号viewer使用独立 `KEEL_VIEWER_PASSWORD`。浏览器通过登录建立HttpOnly会话；开发代理不注入服务令牌。不要将密码粘入Git、日志或评测样本。

首次依赖安装后，可按 backend/Dockerfile 构建镜像。可选 compose 使用当前 Ubuntu 的 host 网络访问已准备的本机 PostgreSQL/MinIO，并读取受控 backend/.env；本轮实际运行验证使用本地 Go 进程，没有宣称验证过 Docker 镜像构建。

## 能力与边界

完整实现链路说明见 `../docs/BOM-REAL-DESIGN.md`。真实文件解析、人工配置、候选/关联校验、版本审核、事务BOM提交、Outbox和CMDB投影均使用真实服务与存储。真实Provider通过服务端HTTPS适配器配置；未配置时Rules仍可完整运行，Mock错误可诊断，不修改正式数据。

当前本地真实账号绑定一个配置项目，所有API服务端校验该授权；支持管理员/只读账号。企业SSO和全平台多项目账号管理属于其他系统模块，不借本次BOM任务假称完成。外部AI供应商实际接入与真实网络/设备操作未验收。实际netbom的身份/物理连接仍必须人工确认，不能用合成样本通过结果替代。

## 相关验证

- `cd backend && go test ./...`
- `go run ./cmd/testworkbook /tmp/keel-api-acceptance.xlsx`
- 仓库根目录：`python3 dev/scripts/bom-api-check.py`、`python3 dev/scripts/check-bom-responses.py`
- `npm run build`

真实Excel和凭据文件被Git忽略。验收样本由testworkbook命令生成，全部身份带synthetic标记，不能当成真实客户物料。
