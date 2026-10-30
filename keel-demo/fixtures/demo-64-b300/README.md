# 64 x B300 deterministic demo fixture

- Dataset: `keel-demo-b300-64-v1`
- 64 synthetic B300 nodes, 512 GPUs, 640 NICs, 16 racks, 24 switches, 704 cable records.
- Every ID, hostname, MAC, serial and IP is synthetic. Hostnames use `.invalid`; IPv4 uses documentation ranges. No source workbook credentials or real serial/IP/MAC values are copied.
- This dataset is a coherent demo profile, not a claim that the source Excel's real project contains exactly 64 nodes or the same rack allocation. The real workbook contains multiple deployment scales and templates; the demo profile deliberately scopes one 64-node slice.
- `demo-seed.json`: canonical fixture data used by the mock provider.
- `seed-demo.sql`: loads the same records into the isolated PostgreSQL `keel_demo` schema as JSONB payloads; this is a demo adapter schema, not the production normalized domain schema.
- Re-run is idempotent by `(resource_type, resource_id)`, relation ID, and event ID.
- Use a disposable demo database only. SQL refuses to run unless `PGOPTIONS="-c keel.allow_demo_seed=true"` is set. Do not grant this schema permission to connect to real switch/host executors.
- Example: `PGOPTIONS="-c keel.allow_demo_seed=true" psql "$KEEL_DEMO_DATABASE_URL" -v ON_ERROR_STOP=1 -f fixtures/demo-64-b300/seed-demo.sql`
