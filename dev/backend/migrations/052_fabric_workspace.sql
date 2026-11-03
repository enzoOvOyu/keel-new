BEGIN;
-- Fabric-owned configuration/review history only. No copies of CMDB assets or executions.
CREATE TABLE fabric_workspace (
 project_id text PRIMARY KEY REFERENCES project(id) ON DELETE CASCADE,
 revision bigint NOT NULL DEFAULT 0,
 state jsonb NOT NULL,
 input_digest text NOT NULL DEFAULT '',
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE fabric_command_receipt (
 project_id text NOT NULL REFERENCES fabric_workspace(project_id) ON DELETE CASCADE,
 idempotency_key text NOT NULL,request_hash text NOT NULL,result jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),PRIMARY KEY(project_id,idempotency_key)
);
INSERT INTO schema_migration(version) VALUES(52);
COMMIT;
