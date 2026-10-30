# Keel 本地开发

当前交付演示项目位于 [`keel-demo/`](keel-demo/README.md)，访问地址为 <http://localhost:5180>。
启动演示：在 `keel-demo/` 目录执行 `docker compose up -d`。
下方是基础开发环境说明。

设计规范见 [docs/README.md](docs/README.md)。当前为前端先行的基础环境，业务页面和 Go API 尚未实现。

1. 可选：`cp .env.example .env`，按需调整本地配置。
2. `docker compose up -d --build` 启动服务；`docker compose ps` 查看状态。
3. 前端：<http://localhost:5174>；MinIO 控制台：<http://localhost:9001>。PostgreSQL、Redis 和 MinIO API 分别绑定本机 5432、6379、9000 端口。
4. `docker compose stop` 停止服务；数据保存在独立 Docker volumes 中。不要使用 `down -v`，除非明确要删除本地数据。

默认凭据仅适用于本机开发，正式环境应使用密钥管理。MinIO bucket 在接入对象存储时创建；目前不自动创建。前端依赖更新后重新执行 `docker compose up -d --build`。可在 web 目录运行 `npm ci && npm run lint && npm run build` 验证。

MinIO 默认使用本机已有的 `keel-minio:2025-10-15-9e49d5e` 镜像；其他机器需提供兼容的 MinIO 镜像并通过 `MINIO_IMAGE` 配置。
