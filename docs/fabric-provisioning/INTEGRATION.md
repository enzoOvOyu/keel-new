# Fabric 原型工程化接入 · 真实持久化切片

分支 feat/fabric-rebuild，工作区 /home/ubuntu/keel-fabric-dev。设计入口为本目录 CONTRACTS.txt 与 IMPLEMENTATION.txt；用户已明确，不再以缺少 FABRIC-DESIGN-CONTRACT.md 作为阻塞。

## 页面

八个已迁入原型页面继续使用当前 Shell / Sidebar / Header / ProjectContext / Session，路由 /p/:projectId/deploy/fabric/{overview,blueprint,onboarding,configuration,validation,deployments,assurance,settings}。保留原型页面结构、表格、Topology、Dialog/Drawer；真实数据的未知状态按原样式表达，不用旧页面替换。

主应用中的模拟 Provider、设备/作业/观测模拟 Adapter 已删除。没有 VITE_FABRIC_PROVIDER 开关，没有 fixture/localStorage/sessionStorage 回退。模拟结果选择、漂移注入和虚构默认时间窗已移除；原型中的假设备数/800G、Plane 和网络参数不进入真实展示。真实大范围拓扑按 18 个设备一组显示，保留缩放和角色/Plane 过滤；未确认角色独立展示，不推断 Leaf/Spine。

## 后端

合入 dev 已提交的最新 Kickoff 发布链；不是复制另一窗口未提交代码。只读 publication + CMDB scope，按稳定 ID 去重，保留设备/端口/Cable 身份。迁移 052 新增 Fabric 工作区与幂等回执，资产不复制。增量迁移前 Schema 备份保存在本机私有 .local-fabric 目录，不提交 Git。

真实采用/Blueprint/意图产物/预检持久化及统一审批路径见 docs/api/fabric.md。编译器仅生成明确类型的结构化意图，缺少原生 Renderer 和设备能力时不可执行。ZTP、实际下发、真实独立回采和回滚没有假成功路径。

现有发布数据不含 NetworkPlan；任务目录没有可用的 Fabric 任务声明。配置预检必须如实阻断；物理布线展示不等于网络方案已批准。

## 运行隔离

前端 5197，tmux keel-fabric-integrated，代理真实 Go 8100（tmux keel-fabric-real-api）。使用已有受控环境配置和同一开发数据库，关闭后台通用 Worker，未绑定重复 Agent 网关/注册端口。5180/8081 与独立5194原型不变。该分支预览不是现网部署。

## 验证记录

真实 API 检查已通过：稳定ID去重、旧版本409、证据缺失阻断、PostgreSQL刷新恢复、项目拒绝；8页真实接口渲染及配置Dialog打开，0 pageerror。使用已发布开发库22,492条对象，967台服务器/交换机，0个NetworkPlan，0个适配Fabric的任务声明；没有设备调用。首次执行另已验证同key重放只增加一次版本。4项Go定向检查通过。TypeScript + 生产构建通过（Vite 1m32s）；仅保留既有依赖注释和大chunk告警，未做无关依赖重构。136项真实API台账引用检查通过。历史 integration/result.json 的合成 API 拦截只证明此前原型接入，不作为本切片真实 API 证据。真实检查不调用设备，不模拟网络执行。

真实检查采用现有数据库记录，不证明现场设备或 NOS 兼容。只读目视检查 overview/configuration 截图；实际截图保存在 /tmp/keel-fabric-real-check，未提交真实资产截图。后续调整了未知角色的紧凑分行及DEV标识，需以最终源代码为准，不宣称像素100%验收。
