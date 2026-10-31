# 项目彻底删除

用户确认范围：删除测试项目及其专属数据，保留审计和共享型号。

实现位置：项目列表的“删除项目”按钮；需要 project:delete（当前默认仅管理员），输入完整项目名确认。DELETE /api/v1/projects/{project_id} 使用现有 Session/CSRF。返回202表示数据库事务已提交、文件清理已持久化排队，不表示文件已清空。

同事务删除项目成员、BOM导入及候选、提交/版本、项目资产关联、专属CMDB实体/关系/来源。共享资产保留，解除待删除项目版本的可空引用；共享型号、用户、审计、审批及签署保留。迁移025允许审批/Flow审计保留历史项目ID，并防止已授权的并发上传重新写入删除的项目。

源文件、包、AI输出按严格 project_id/ 前缀使用现有Outbox删除；存储失败保留待处理事件并重试，错误为 PROJECT_STORAGE_CLEANUP_FAILED。工作目录里的原始XLSX不属于上传对象，不删除。

限制：存在待处理/执行中BOM任务时返回409；存在Flow绑定、执行绑定或注册Agent时返回409 PROJECT_FLOWS_REQUIRE_RETIREMENT，需先完成外部系统退役。这次未实现外部Kestra或Agent的级联销毁。项目清理事务会短暂锁定相关业务表；不是大规模并发删除方案。

实际检查：
- API及identity Go包检查通过（第一次实现时）。
- OpenAPI类型生成通过，TypeScript检查通过。
- 未授权删除用例通过。
- PostgreSQL回滚验证：原测试项目专属数据删除通过；增加共享资产场景后暴露 cmdb_entity.revision_id 外键阻断，现已修正引用处理。
- 用户随后要求停止测试。修正后的共享资产回滚用例、对象存储失败重试用例未执行，不声称通过。
- 未进行浏览器、冒烟、回归或真实项目删除；未部署，未执行迁移025。

独立工作树 /home/ubuntu/keel-project-delete-dev，分支 feat/project-delete。不修改当前运行前端或其他开发者分支。
