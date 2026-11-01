# Keel 开发入口

后续开发统一使用 **Git `dev` 分支**，应用代码位于 **`dev/`**。默认工作区 `/home/ubuntu/keel-dev`；继续任务先读 [HANDOFF](HANDOFF.md) 和 [执行约束](AGENTS.md)。`main` 为稳定整合分支，未经用户要求不自动合并或部署。

## 当前应用与依赖

- 前端：`dev/`，React + TypeScript + Vite；安装及构建为 `cd dev && npm ci && npm run build`。
- 后端：`dev/backend/`，Go；配置、启动和显式迁移见 [部署说明](docs/DEPLOYMENT.md)。凭据和原始Excel不入Git。
- 根 `compose.yaml` 只提供PostgreSQL、Redis、MinIO。按需启动：`docker compose up -d postgres redis minio`；不再提供旧Web/Demo入口。不要为整理Git执行down或删除数据卷。
- 当前服务仍为5180前端、8081 API、5181独立设计原型；运行工作区及二进制独立于Git分支，本次整理没有重启或迁移它们。新的dev工作区不会自动成为已部署环境。
- 统一契约与真实实现范围见 [真实API入口](docs/REAL-API-CONTRACTS.md)。未接真实后端的领域继续使用dev中明确标记的Mock Adapter及现有fixtures。

## 旧演示归档

旧 `keel-demo/` 与 `web/` 已从当前应用树移除；有效的 `dev/`、`docs/`、`fixtures/` 保留。

- `archive/keel-mock-first-20261101`：独立旧Demo分支及其完整历史。
- `archive/pre-dev-cleanup-20261101`：本次清理前完整main快照，包含旧应用目录。

归档仅供查阅，需要查看时在单独目录检出对应Tag，禁止合并到dev或启动旧演示覆盖现有端口。整理详情见 [交接记录](HANDOFF.md)。
