-- Flows business bindings only; Kestra owns definitions and execution states.
CREATE TABLE flow_binding (
 id uuid PRIMARY KEY, project_id text NOT NULL REFERENCES project(id), namespace text NOT NULL,
 flow_id text NOT NULL, name text NOT NULL, description text NOT NULL DEFAULT '', target_type text NOT NULL,
 revision integer NOT NULL DEFAULT 1, created_by uuid NOT NULL REFERENCES app_user(id),
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(project_id,flow_id), UNIQUE(namespace,flow_id)
);
CREATE TABLE flow_execution_binding (
 id uuid PRIMARY KEY, project_id text NOT NULL REFERENCES project(id), flow_id uuid REFERENCES flow_binding(id),
 kestra_id text UNIQUE, namespace text NOT NULL, engine_flow_id text NOT NULL, flow_revision integer NOT NULL,
 source_module text NOT NULL, mode text NOT NULL DEFAULT 'REAL' CHECK(mode='REAL'),
 requester_id uuid NOT NULL REFERENCES app_user(id), parent_id uuid REFERENCES flow_execution_binding(id),
 idempotency_key text NOT NULL, request_hash text NOT NULL, request jsonb NOT NULL, error_code text,
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(project_id,idempotency_key)
);
CREATE TABLE flow_execution_target (
 execution_id uuid NOT NULL REFERENCES flow_execution_binding(id), asset_id text NOT NULL REFERENCES cmdb_entity(item_uid),
 snapshot jsonb NOT NULL, selected_transport text NOT NULL, actual_transport text,
 state text NOT NULL DEFAULT 'PENDING', attempt integer NOT NULL DEFAULT 0,
 agent_task_id uuid, ssh_profile_id uuid, exit_code integer, stdout text NOT NULL DEFAULT '', stderr text NOT NULL DEFAULT '',
 started_at timestamptz, ended_at timestamptz, command_hash text NOT NULL,
 PRIMARY KEY(execution_id,asset_id)
);
CREATE TABLE flow_ssh_trust (
 id uuid PRIMARY KEY, project_id text NOT NULL REFERENCES project(id), asset_id text NOT NULL REFERENCES cmdb_entity(item_uid),
 host text NOT NULL, port integer NOT NULL DEFAULT 22 CHECK(port>0 AND port<65536), username text NOT NULL,
 secret_ref text NOT NULL, known_hosts_ref text NOT NULL, approved_by uuid NOT NULL REFERENCES app_user(id),
 status text NOT NULL CHECK(status IN('approved','revoked')), revision bigint NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(project_id,asset_id)
);
CREATE TABLE flow_agent (
 id uuid PRIMARY KEY, project_id text NOT NULL REFERENCES project(id), asset_id text NOT NULL REFERENCES cmdb_entity(item_uid),
 identity jsonb NOT NULL, version text NOT NULL, capabilities jsonb NOT NULL DEFAULT '[]',
 state text NOT NULL CHECK(state IN('pending_verification','enrolled','quarantined','revoked')),
 certificate_serial text, certificate_expires_at timestamptz, last_heartbeat timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(project_id,asset_id)
);
CREATE TABLE flow_enrollment (
 id uuid PRIMARY KEY, project_id text NOT NULL REFERENCES project(id), asset_id text NOT NULL REFERENCES cmdb_entity(item_uid),
 token_hash text NOT NULL UNIQUE, expires_at timestamptz NOT NULL, consumed_at timestamptz,
 created_by uuid NOT NULL REFERENCES app_user(id), created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE flow_agent_task (
 id uuid PRIMARY KEY, agent_id uuid NOT NULL REFERENCES flow_agent(id), execution_id uuid NOT NULL REFERENCES flow_execution_binding(id),
 asset_id text NOT NULL, state text NOT NULL, command_hash text NOT NULL, result jsonb NOT NULL DEFAULT '{}',
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(execution_id,asset_id)
);
CREATE TABLE flow_dispatch_ticket (
 hash text PRIMARY KEY, execution_id uuid NOT NULL REFERENCES flow_execution_binding(id), expires_at timestamptz NOT NULL
);
CREATE TABLE flow_audit (
 id uuid PRIMARY KEY, project_id text NOT NULL REFERENCES project(id), actor text NOT NULL, action text NOT NULL,
 resource_id text NOT NULL, details jsonb NOT NULL DEFAULT '{}', created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX flow_execution_project_created ON flow_execution_binding(project_id,created_at DESC);
CREATE INDEX flow_agent_project ON flow_agent(project_id,last_heartbeat);
INSERT INTO schema_migration(version) VALUES(21);
