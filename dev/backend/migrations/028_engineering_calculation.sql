BEGIN;
CREATE TABLE eng_rule (
 project_id text NOT NULL REFERENCES project(id) ON DELETE CASCADE,
 id uuid NOT NULL, version integer NOT NULL CHECK(version>0), domain text NOT NULL,
 scope_type text NOT NULL CHECK(scope_type IN('project','customer','domain')), scope_id text NOT NULL,
 expression jsonb NOT NULL CHECK(jsonb_typeof(expression)='object'),
 approval_status text NOT NULL CHECK(approval_status IN('draft','validated','approved','retired')),
 review_id uuid, actor text NOT NULL, reason text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(project_id,id,version)
);
CREATE TABLE eng_rule_io (
 project_id text NOT NULL, rule_id uuid NOT NULL, version integer NOT NULL,
 direction text NOT NULL CHECK(direction IN('input','output')), position integer NOT NULL,
 name text NOT NULL, value_type text NOT NULL, unit text NOT NULL, required boolean NOT NULL,
 PRIMARY KEY(project_id,rule_id,version,direction,name),
 FOREIGN KEY(project_id,rule_id,version) REFERENCES eng_rule(project_id,id,version) ON DELETE CASCADE
);
CREATE TABLE eng_rule_dependency (
 project_id text NOT NULL, rule_id uuid NOT NULL, version integer NOT NULL,
 dependency_id uuid NOT NULL, dependency_version integer NOT NULL, dependency_digest text NOT NULL,
 PRIMARY KEY(project_id,rule_id,version,dependency_id),
 FOREIGN KEY(project_id,rule_id,version) REFERENCES eng_rule(project_id,id,version) ON DELETE CASCADE,
 FOREIGN KEY(project_id,dependency_id,dependency_version) REFERENCES eng_rule(project_id,id,version)
);
CREATE TABLE eng_rule_case (
 project_id text NOT NULL, rule_id uuid NOT NULL, version integer NOT NULL,
 id text NOT NULL, tolerance numeric NOT NULL CHECK(tolerance>=0),
 PRIMARY KEY(project_id,rule_id,version,id),
 FOREIGN KEY(project_id,rule_id,version) REFERENCES eng_rule(project_id,id,version) ON DELETE CASCADE
);
CREATE TABLE eng_rule_case_value (
 project_id text NOT NULL, rule_id uuid NOT NULL, version integer NOT NULL, case_id text NOT NULL,
 direction text NOT NULL CHECK(direction IN('input','expected')), name text NOT NULL,
 value jsonb NOT NULL CHECK(jsonb_typeof(value)='object'),
 PRIMARY KEY(project_id,rule_id,version,case_id,direction,name),
 FOREIGN KEY(project_id,rule_id,version,case_id) REFERENCES eng_rule_case(project_id,rule_id,version,id) ON DELETE CASCADE
);
-- Human decisions are retained when project-owned calculations/rules are removed.
CREATE TABLE eng_rule_review (
 id uuid PRIMARY KEY, project_id text NOT NULL, rule_id uuid NOT NULL, version integer NOT NULL,
 digest text NOT NULL, decision text NOT NULL CHECK(decision IN('approve','reject','retire')),
 actor text NOT NULL, reason text NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE eng_calculation_run (
 id uuid PRIMARY KEY, project_id text NOT NULL REFERENCES project(id) ON DELETE CASCADE,
 source_id uuid NOT NULL REFERENCES eng_source(id) ON DELETE CASCADE,
 revision_id uuid NOT NULL REFERENCES eng_revision(id) ON DELETE CASCADE,
 module_instance_id text NOT NULL, provider_id text NOT NULL, provider_version text NOT NULL DEFAULT '',
 rule_id uuid, rule_version integer, source_sha256 text NOT NULL, input_digest text NOT NULL,
 status text NOT NULL CHECK(status IN('pending','running','succeeded','partial','failed','unsupported')),
 compatibility text NOT NULL DEFAULT 'unknown', actor text NOT NULL,
 requested_cells jsonb NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(requested_cells)='array'),
 claim_id uuid, created_at timestamptz NOT NULL DEFAULT now(), started_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(), finished_at timestamptz,
 FOREIGN KEY(project_id,rule_id,rule_version) REFERENCES eng_rule(project_id,id,version)
);
CREATE INDEX eng_calculation_source ON eng_calculation_run(source_id,created_at DESC);
CREATE UNIQUE INDEX eng_calculation_active ON eng_calculation_run(source_id,input_digest) WHERE status IN('pending','running');
CREATE TABLE eng_calculation_value (
 run_id uuid NOT NULL REFERENCES eng_calculation_run(id) ON DELETE CASCADE,
 direction text NOT NULL CHECK(direction IN('input','output')), name text NOT NULL,
 value jsonb NOT NULL CHECK(jsonb_typeof(value)='object'), PRIMARY KEY(run_id,direction,name)
);
CREATE TABLE eng_calculation_formula (
 run_id uuid NOT NULL REFERENCES eng_calculation_run(id) ON DELETE CASCADE,
 sheet text NOT NULL, cell text NOT NULL, expression text NOT NULL,
 cached_value jsonb, recalculated_value jsonb, dependency_status text NOT NULL, compatibility text NOT NULL,
 error_code text, PRIMARY KEY(run_id,sheet,cell)
);
CREATE TABLE eng_calculation_dependency (
 run_id uuid NOT NULL, sheet text NOT NULL, cell text NOT NULL, position integer NOT NULL,
 to_sheet text NOT NULL, to_range text NOT NULL, kind text NOT NULL,
 PRIMARY KEY(run_id,sheet,cell,position),
 FOREIGN KEY(run_id,sheet,cell) REFERENCES eng_calculation_formula(run_id,sheet,cell) ON DELETE CASCADE
);
CREATE TABLE eng_calculation_error (
 run_id uuid NOT NULL REFERENCES eng_calculation_run(id) ON DELETE CASCADE,
 position integer NOT NULL, code text NOT NULL, message text NOT NULL, sheet text, cell text,
 PRIMARY KEY(run_id,position)
);
ALTER TABLE eng_evidence ADD COLUMN calculation_run_id uuid REFERENCES eng_calculation_run(id);
INSERT INTO schema_migration(version) VALUES(28);
COMMIT;
