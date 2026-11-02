Keel 智能开局工程工作台

本地入口：http://localhost:5195/p/proj-b300-demo/kickoff/overview
目录：/home/ubuntu/keel-dev/kickoff
方案：OPTIMIZATION.txt
集成说明：CONTRACTS.txt
本地服务：tmux keel-kickoff-workspace
日志：/tmp/keel-kickoff-workspace.log

启动（沿用当前仓库依赖，不安装第二套组件体系）：
  cd /home/ubuntu/keel-dev/kickoff
  ln -s ../dev/node_modules node_modules  # 仅在不存在时执行
  npm run dev

默认5195，不占用已有5180/8081/5081/5181/5194。根路径自动跳转项目工作台。
  npm run check  # 定向TypeScript
  npm run build  # 本原型构建，不构建整个平台

工作区
overview：启动总览、数据流、建议动作、独立下游能力。
sources：资料包、工作表/区域、标准化字段与来源、重复资料摘要识别。
review：设备/组件/端口/连接/落位、筛选、来源抽屉、局部问题确认。
publish：选择范围、关联对象预览、本地确认入库、版本/差异与导出。
models/facility：机房/坐标、机柜、型号尺寸、设备落位。
models/network：管理接入、Fabric/Plane、角色、接口、地址/ASN、邻居、策略。
models/provisioning：启动身份、镜像/磁盘/BIOS/固件策略。
models/cabling：通道节点/路段、机柜接入、线缆/模块、规则与确定性计算。
models/power：功率、机柜容量、PDU/回路、PSU供电关系与负荷计算。

资料可先入库，工程域不是必须依次通过的五步。
顶部和页面使用正式业务文案。实际连接边界放左下“工作区说明”和入库确认处，不在每页反复展示。

原型能力边界
状态保存于当前浏览器 localStorage，键 keel-kickoff-workspace-v1；不改在线项目、CMDB或真实设备。
新资料只登记名称、摘要和待分析状态；标准工程表支持真正XLSX/CSV读取和导出。未接AI不能冒充完成真实资料理解。
计算对本地已填写参数执行确定性算法，能返回局部结果/缺口/过期；不代表完整厂商兼容或工程规则批准。
跨工作区链接打开现有服务，它们未消费本地浏览器版本；正式接入按CONTRACTS.txt完成。
原型合成数据来源及初始问题情景在 scripts/project-fixture.py，保留统一fixture身份。不含真实MM文件或凭据。

修改范围限kickoff及HANDOFF；主应用、Go API和在线数据库不变。

本轮检查
TypeScript/Vite构建通过。Playwright定向检查13项分段完成，记录在validation.json；screenshots保留主要页面与窄屏结果。
覆盖9个路由、例外确认、19台局部入库/20台后续增补、稳定身份与版本、标准XLSX往返、确定性线长、计算过期、电力缺口、新资料待分析及项目隔离。
检查仅使用独立浏览器上下文，不代表真实AI、CMDB或设备验收。后续按变更范围运行 scripts/check-prototype.mjs，不必每次重复全量检查。
