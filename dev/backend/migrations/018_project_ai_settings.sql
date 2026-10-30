CREATE UNIQUE INDEX project_code_unique ON project(lower(code));
CREATE TABLE ai_provider_config(
 id uuid PRIMARY KEY,name text NOT NULL,protocol text NOT NULL CHECK(protocol IN('openai-chat','keel-mapping')),endpoint text NOT NULL,model text NOT NULL,
 credential_ref text NOT NULL DEFAULT '',timeout_seconds int NOT NULL CHECK(timeout_seconds BETWEEN 1 AND 120),token_budget int NOT NULL CHECK(token_budget BETWEEN 128 AND 32768),json_mode boolean NOT NULL DEFAULT true,
 enabled boolean NOT NULL DEFAULT false,is_default boolean NOT NULL DEFAULT false,revision bigint NOT NULL DEFAULT 1,
 last_test_status text NOT NULL DEFAULT 'not_tested',last_test_code text,last_test_http_status int,last_test_at timestamptz,last_test_ms bigint,
 created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),CHECK(NOT is_default OR enabled)
);
CREATE UNIQUE INDEX ai_provider_one_default ON ai_provider_config(is_default) WHERE is_default;
INSERT INTO schema_migration(version) VALUES(18);
