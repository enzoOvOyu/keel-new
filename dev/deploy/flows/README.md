# Flows 本地运行

当前分支 `feat/flows-redesign`，独立工作区 `/home/ubuntu/keel-flows-dev`。包内要求真实 Kestra/Go/Agent；本地实例不替换原5180/8081，也不合并main。所有密码/PKI放忽略路径，不复制原环境业务数据。

## 依赖与配置

固定 Kestra `v2.0.7`、PostgreSQL17.6。复用项目Node/Go依赖。复制 `.env.example` 为 `.env.local`，`chmod 600`，设置独立凭据。Kestra要求强密码且SECRET输入必须配置encryption key；key丢失无法解密历史SECRET。生产应使用正式PKI与HTTPS，不使用此开发CA。

在工作区根目录启动引擎：

```bash
docker compose --env-file dev/deploy/flows/.env.local -f dev/deploy/flows/compose.yaml up -d
```

引擎8095仅监听localhost。storage-init仅初始化命名卷所有权；不挂Docker socket。

应用需要已有Keel schema的独立数据库；本次仅复制原数据库结构至`keel_flows_dev`，没有复制原数据。迁移021…024已应用在该隔离库，不能再重复执行。新建完整环境应按现有迁移顺序001…024执行并记录schema_migration。**现有backend/migrate.sh写死原容器，不可用于本隔离环境**；使用显式DATABASE_URL/psql与版本检查。不要自动删表重建。Kestra库与Keel业务库是两个库。

生成开发PKI（仅首次，工具拒绝覆盖）：

```bash
cd dev/backend
go run ./cmd/flows-pki --dir ../deploy/flows/pki --host localhost
go build -o /tmp/keel-flows-api ./cmd/api
go build -o /tmp/keel-flows-agent ./cmd/agent
```

API使用私有环境变量（数据库、现有对象存储等）及`.env.local`，前者只复用对象存储配置，后者必须覆盖DATABASE_URL。环境文件不可输出到日志。`KEEL_WORKER_CALLBACK_URL`必须从Kestra容器可达，本机实际绑定Docker bridge地址172.22.0.1:8096；示例localhost绑定需按环境调整，不能直接照抄为生产网络设置。

本机已配置环境的API重启命令（确认专属旧PID已退出后运行，不输出配置）：

```bash
cd /home/ubuntu/keel-flows-dev
set -a
source /home/ubuntu/keel/dev/backend/.env
source dev/deploy/flows/.env.local
set +a
cd dev/backend
/tmp/keel-flows-api
```

以上使用Bash。不要把此命令用于重启原8081，也不要在已有隔离API进程旁重复启动。

前端（`dev/`）：

```bash
KEEL_API_URL=http://172.22.0.1:8096 ./node_modules/.bin/vite --host 127.0.0.1 --port 5190 --strictPort
```

仅本机地址5190，远程使用端口转发。本隔离admin密码读取本地`.env.local`的`KEEL_BOOTSTRAP_PASSWORD`，与原5180账号无关，未更改原密码。

## Agent

从Agents页面对已授权CMDB主机签发单次Token，通过本实例HTTPS8098注册；未知/歧义机器身份待人工核验。Token保存0600临时文件，注册后删除。端程序必须非root：

```bash
SSL_CERT_FILE=/ABSOLUTE_PRIVATE_PATH/pki/ca.pem /tmp/keel-flows-agent --gateway localhost:8097 --enroll-url https://localhost:8098 --token-file /PRIVATE/token --state-dir /PRIVATE/agent-state
```

注册后重启只需gateway/state-dir，不生成第二身份。实际运行状态与PID/log路径见HANDOFF。任务journal持久化防止重连重复执行；丢失结果保持RECONCILING。

Bootstrap已实现严格SSH通道和固定helper部署流程，但需要运维预置`keel-agent-bootstrap`为root所有/不可由登录用户修改，配置仅该helper的sudo授权、`/etc/keel-agent/package.sha256`可信包摘要、HTTPS包分发及证书信任。helper用非root service user及systemd隔离运行Agent。缺少前提返回503，不能把按钮当成安装完成。此轮未对真实主机安装或升级。

## 定向检查

```bash
cd dev/backend
go test ./internal/flows ./cmd/agent ./cmd/api
# TestKestraSSHIntegration默认跳过，仅显式FLOWS_TEST_DATABASE_URL指向keel_flows_dev时运行。
cd ..
npm run build
```

浏览器证据见`docs/flows-redesign/ui-validation.json`，运行证据`runtime-validation.json`。已有检查不重复跑全平台。停止本实例只使用专属PID/Compose项目；不要停止或重启原Keel环境。


## 开发补齐与升级

当前Agent0.2.0新增`job.pipeline`能力；旧Agent可继续单任务，不能冒充支持多步骤。迁移023包含Schedule消费、步骤结果、持久事件、文件版本；024绑定审批单次执行。两者已应用到本隔离库，不可重复执行；未应用原业务库。

Cron由Kestra执行，Go不是另一个调度器；当前合成验证计划已停用。新增二进制资源上限1MiB、每目标32步、步骤结果/输入绑定32KiB。Bootstrap上线需要发布0.2.0包并预置对应可信摘要，不使用旧0.1.0包假称支持pipeline。
