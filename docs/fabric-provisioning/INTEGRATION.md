# Fabric 原型工程化接入 · 第一切片

分支feat/fabric-rebuild，工作区/home/ubuntu/keel-fabric-dev。主应用路由/p/:projectId/deploy/fabric/{overview,blueprint,onboarding,configuration,validation,deployments,assurance,settings}。不是新应用，不新增createRoot或BrowserRouter，不改变部署/运维/系统三个平面。

## 视觉与交互

直接迁入5194 React原型Pages、Dialogs、Topology、ui及其CSS；没有用旧Fabric/NetworkWorkspace替代。原独立App Shell由现有Shell/Sidebar/Header承接，八个页面登记为Fabric子菜单。沿用原页面标题、布局、表格、过滤、Graph/Tabs、Inspector、Drawer、Dialog与状态链。CSS在构建时限定.fabric-workspace，Portal容器也在作用域内；host.css仅调整Fabric内容区几何，其他页面不受影响。

当前尚未找到FABRIC-DESIGN-CONTRACT.md。已向用户询问路径；没有自行编造同名文件或宣称通过该文档验收。source-manifest记录原型源码摘要；现有5194为当前可读取的唯一视觉源码，不假设其等同尚未提供的另一份Astra文件。

## 数据边界

Mock/Http共用FabricFixture/FabricState等页面DTO，统一位于src/lib/api/fabric；页面不直接读取fixture。Mock显式VITE_FABRIC_PROVIDER=mock，用项目隔离sessionStorage；主应用身份来自Session，无模拟身份下拉。其他项目上的Mock数据是明确合成切片，不能当作这些项目的真实资产。

默认Http模式只读取本切片真实GET输入API，将Kickoff已发布范围与授权CMDB投影为同一页面模型；不猜测型号/端口/Plane/NOS/实际配置/LLDP。真实写操作尚未接入时明确不可用，绝不回退Mock。下一切片再实现输入采用及Blueprint修订持久化，其后按产物→校验审批→受控执行→回采推进，不一次性重写全后端。

## Kickoff依赖

用户指定另一窗口的新Kickoff作为唯一发布上游。依赖其published状态、publication_object、publication_input和CMDB回执；不复制未提交开发，不重复建模或导入。检查时用户所述5191未监听；当前已知原型5195、集成页5196。端口不是数据契约，正式读取后端API。

上游迁移049未部署时API返回WAITING_FOR_KICKOFF；未发布/failed/pending范围不出现在输入列表。新API本身不创建workspace，不运行AI，不写数据库或调用设备。

## 文件映射

- src/features/fabric/Workspace.tsx：复用Shell/ProjectContext/Session/Query，注册原型WorkspaceContext。
- Pages.tsx / Dialogs.tsx / Topology.tsx / ui.tsx：保留原型内容组件；仅导入路径、异步命令、Portal与真实/模拟来源标识适配。
- styles.css：原型CSS；host.css：Fabric独占内容区几何。
- src/lib/api/fabric/{types,mock,adapters,http,provider}.ts：统一页面模型、原型演示状态机、真实只读Adapter与显式Provider选择。
- cmd/api/fabric.go：getFabricPublishedInputs，复用CMDB身份及engineeringModel读取。
- docs/openapi.yaml / 生成类型 / docs/api/fabric.md / API台账：当前只读切片契约。

## 验证范围

integration/result.json记录8页主应用加载、无重复Shell、发布输入Dialog/Mock采用、Sidebar/Blueprint Commit、配置Dialog；零pageerror，所有API拦截synthetic，无后端或设备写入。截图只能证明当前Mock接入，不能代替设计契约逐项/像素批准。

Go针对摘要与项目拒绝的检查通过；真实Kickoff/CMDB联合数据与数据库分页尚未验证，不宣称只读API已经在线可用。本分支未部署5180/8081，也未迁移在线数据库。

最终TS复核与生产构建通过（Vite1m23s）；Go API构建和2项定向检查通过；123项API台账引用通过。当前主应用隔离预览5197返回200，tmux keel-fabric-integrated，显式Mock；不是5180发布。
