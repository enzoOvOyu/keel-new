Keel Fabric Studio · 本地高保真交互原型
入口：http://localhost:5194/p/proj-b300-demo/fabric/overview
运行目录：/home/ubuntu/keel-dev/dev
启动：./node_modules/.bin/vite --config prototypes/fabric-studio/vite.config.ts
本地服务：tmux keel-fabric-prototype；日志 /tmp/keel-fabric-prototype.log。

设计与研究：docs/fabric-provisioning/{DESIGN.txt,REFERENCES.txt,CONTRACTS.txt,lifecycle.mmd}
源码：contracts.ts / provider.ts / adapters.ts / App.tsx / Pages.tsx / Dialogs.tsx / Topology.tsx / ui.tsx / style.css。
共8个工作区，使用既有React、shadcn Button/Radix Dialog、TanStack Table；SVG拓扑由统一数据投影。

A–G操作顺序见总览“交互指南”。初始身份为网络工程师；B/E切换交付工程师，D/F签署切换交付负责人。
初始18个对象有合成初始化证据，leaf-07/08需要确认身份后演示ZTP。可只选leaf-01/02完成短路径，但最终交付范围也仅这两台。
状态只在浏览器localStorage持久化。顶部角色选择是原型模拟身份，不是生产登录。
所有输入发布、任务、配置和证据均为DEMO；无真实DHCP、SSH、NVUE、DMS、Kestra或监控调用。

定向检查：
  ./node_modules/.bin/tsc --noEmit -p prototypes/fabric-studio/tsconfig.json
  node prototypes/fabric-studio/tests/domain-check.mjs
  node prototypes/fabric-studio/tests/browser-check.mjs
按需运行；不是每次修改后都重复执行。用户要求不运行全平台冒烟，不扩大测试范围。
检查结果记录于docs/fabric-provisioning/，截图仅证明当时原型范围。
