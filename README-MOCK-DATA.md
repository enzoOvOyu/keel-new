# keel v3.4 Mock-first 演示数据包

先看 `docs/MOCK-DATA-CONTRACT.md`。

- `fixtures/demo-64-b300/demo-seed.json`: 全平台固定合成数据源，适用于前端 Mock Provider。
- `fixtures/demo-64-b300/seed-demo.sql`: 把同一批数据装入专用 PostgreSQL `keel_demo` schema，供演示 API adapter 读取。
- 64 台 B300、512 GPU、640 NIC、16 个机柜、24 台交换机、704 条线缆；另有执行、日志、CMDB、门禁、监控、告警、工单、机柜图、介质、参数、用户和审计数据。
- SQL 要求显式 `PGOPTIONS="-c keel.allow_demo_seed=true"`，只能对一次性演示库运行。
- 所有设备标识和地址均为合成值；不能连接真实设备。
