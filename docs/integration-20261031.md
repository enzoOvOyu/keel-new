# Keel Git 整合记录 · 2026-10-31

本次仅将当前工程的开发历史整合到 `enzoOvOyu/keel-new` 主分支，不部署、不运行迁移、不导入数据、不恢复已暂停的开发。独立工作区 `/home/ubuntu/keel-integration`，整合分支 `integrate/all-keel-branches`。

## 范围

| 分支 | 源提交 | 整合结果 |
|---|---|---|
| `audit/intelligent-deployment-p0` | `314126d` | 已包含 |
| `demo/keel-mock-first` | `4492819` | 独立旧工程，保留分支 |
| `deploy/bom-agent-topology-1031` | `148b88d` | 已包含 |
| `deploy/bom-v5-flows-1031` | `138a86c` | 已包含 |
| `deploy/project-delete-1031` | `2d66974` | 已包含 |
| `feat/datacenter-visualization-approved` | `b184645` | 已包含 |
| `feat/flows-redesign` | `95df50f` | 已包含 |
| `feat/job-platform-rebuild` | `2f5ed92` | 已包含 |
| `feat/jobs-kestra-ui` | `327b953` | 已包含 |
| `feat/project-delete` | `1d6464e` | 已包含 |
| `fix/bom-acceptance-root-causes` | `664f4a5` | 已包含 |
| `fix/bom-agent-correctness` | `74ab238` | 已包含 |
| `fix/bom-source-closure` | `a12a522` | 已包含 |
| `keel-new-main` | `74ab238` | 已包含 |

远端 `design/prd-v3.0.3-20261020` 和 `dev/p1-network-bom` 同为旧工程历史，与当前工程无共同祖先，保留远端分支，不做无关工程拼接。所有这些分支仍可单独检出。

## 冲突处理

- 业务基线：最新 BOM / 项目管理 `314126d`。原生作业平台增量 `327b953` 自动合并，后端增量限于 Flows 四个文件；其他后端代码与业务基线一致。
- BOM 历史分支存在 cherry-pick 重复：`efcb6ff/4403c63/e3f6cd3` 补丁ID一致，`ad8918b/9f16065` 补丁ID一致；`67c7f1d/fdffbcb` 的相关 BOM/AI/CMDB 文件一致，`74ab238/9f16065` 相关识别文件一致。保留后续修正，不恢复从端点型号自动制造实物模块等旧行为。
- HANDOFF 保留当前两条任务接续入口，旧验收快照使用报告和提交引用，避免多份历史“当前部署”互相覆盖。部署文档保留双方历史记录。
- 迁移025为Flows游标、026为项目删除，编号不冲突；本次没有应用迁移。

## 必要检查

- Go：Flows、BOM、CMDB包通过。初次AI/API检查缺少Pydantic校验器；使用现有 `KEEL_BOM_PYTHON` 指向的Python环境后，这两个包通过。未跳过或修改断言。
- 契约登记：55个已登记操作及其Schema、来源、文档、部署证据引用检查通过；OpenAPI共390个设计操作，本次不宣称验证全部业务接口。
- 前端 `npm run build` 通过（TypeScript检查 + Vite生产打包，Node堆上限2048MB）。存在上游PURE注释及大分包提示；未改变依赖或为本次合并扩大构建配置。构建日志在本机 `/tmp/keel-integration-build.log`。
- 差异空白检查：原用户任务文件 `docs/flows-redesign/keel-flows-module-build-spec.md` 有7行既存尾随空白，未无关修改原任务内容；整合新增交接文件单独检查。
- 未进行浏览器、数据库、设备或全链路验收；本轮任务是分支整合。原生作业平台仍约20%且已暂停，完整缺口见HANDOFF。

## 环境保护

未切换或重置任何既有工作区，未提交其他窗口的未提交改动。未更换服务目录、端口、配置、凭据或运行二进制。Git主分支更新不等于运行环境发布。
