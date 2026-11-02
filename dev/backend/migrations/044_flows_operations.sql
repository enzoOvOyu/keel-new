-- Durable business-operation progress, not a second workflow scheduler or asset store.
CREATE TABLE flow_operation_job (
 id uuid PRIMARY KEY,project_id text NOT NULL REFERENCES project(id),owner_id uuid NOT NULL REFERENCES app_user(id),
 kind text NOT NULL CHECK(kind IN('bootstrap','backfill')),state text NOT NULL CHECK(state IN('PENDING','RUNNING','PAUSED','COMPLETED','PARTIAL_FAILURE','CANCELLED')),
 request jsonb NOT NULL,request_hash text NOT NULL,idempotency_key text NOT NULL,revision bigint NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),UNIQUE(project_id,owner_id,idempotency_key)
);
CREATE TABLE flow_operation_item (
 job_id uuid NOT NULL REFERENCES flow_operation_job(id),ordinal integer NOT NULL,asset_id text REFERENCES cmdb_entity(item_uid),scheduled_at timestamptz,
 state text NOT NULL CHECK(state IN('PENDING','RUNNING','SUCCEEDED','BLOCKED','UNKNOWN','CANCELLED')),execution_id uuid REFERENCES flow_execution_binding(id),
 result jsonb NOT NULL DEFAULT '{}',error_code text,worker_id uuid,lease_until timestamptz,started_at timestamptz,ended_at timestamptz,
 PRIMARY KEY(job_id,ordinal)
);
CREATE INDEX flow_operation_queue ON flow_operation_job(state,created_at);
CREATE INDEX flow_operation_lease ON flow_operation_item(state,lease_until);
CREATE TABLE flow_agent_certificate_rotation (
 id uuid PRIMARY KEY,agent_id uuid NOT NULL REFERENCES flow_agent(id),old_serial text NOT NULL,new_serial text NOT NULL UNIQUE,
 public_key_hash text NOT NULL,certificate text NOT NULL,ca text NOT NULL,expires_at timestamptz NOT NULL,
 state text NOT NULL CHECK(state IN('pending','active','superseded')),created_at timestamptz NOT NULL DEFAULT now(),activated_at timestamptz,
 UNIQUE(agent_id,public_key_hash)
);
CREATE UNIQUE INDEX flow_agent_one_pending_rotation ON flow_agent_certificate_rotation(agent_id) WHERE state='pending';
INSERT INTO schema_migration(version) VALUES(44);
