-- Extend existing execution/namespace domains, never duplicate CMDB inventory.
CREATE TABLE flow_schedule (
 flow_id uuid PRIMARY KEY REFERENCES flow_binding(id), owner_id uuid NOT NULL REFERENCES app_user(id),
 engine_flow_id text NOT NULL, cron text NOT NULL, timezone text NOT NULL, enabled boolean NOT NULL,
 request jsonb NOT NULL, revision bigint NOT NULL DEFAULT 1, last_error text, updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE flow_schedule_fire (
 engine_execution_id text PRIMARY KEY, flow_id uuid NOT NULL REFERENCES flow_binding(id),
 state text NOT NULL, execution_id uuid REFERENCES flow_execution_binding(id), error_code text, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE flow_step_result (
 execution_id uuid NOT NULL, asset_id text NOT NULL, step_id text NOT NULL, state text NOT NULL,
 exit_code integer, stdout text NOT NULL DEFAULT '', stderr text NOT NULL DEFAULT '',
 PRIMARY KEY(execution_id,asset_id,step_id), FOREIGN KEY(execution_id,asset_id) REFERENCES flow_execution_target(execution_id,asset_id)
);
CREATE TABLE flow_event (
 id bigserial PRIMARY KEY, execution_id uuid NOT NULL REFERENCES flow_execution_binding(id),
 event_type text NOT NULL, payload jsonb NOT NULL, fingerprint text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(execution_id,fingerprint)
);
CREATE INDEX flow_event_replay ON flow_event(execution_id,id);
CREATE TABLE flow_file_revision (
 id uuid PRIMARY KEY, project_id text NOT NULL REFERENCES project(id), namespace text NOT NULL, path text NOT NULL,
 revision bigint NOT NULL, sha256 text NOT NULL, size bigint NOT NULL, content bytea NOT NULL,
 actor_id uuid NOT NULL REFERENCES app_user(id), created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(namespace,path,revision)
);
INSERT INTO schema_migration(version) VALUES(23);
