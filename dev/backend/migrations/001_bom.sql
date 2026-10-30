CREATE TABLE IF NOT EXISTS schema_migration(version integer PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS bom_import_batch (
 id uuid PRIMARY KEY, project_id text NOT NULL, source_document_id uuid NOT NULL, object_key text NOT NULL UNIQUE,
 filename text NOT NULL, content_sha256 text NOT NULL, status text NOT NULL,
 revision integer NOT NULL DEFAULT 1, workbook jsonb NOT NULL DEFAULT '{}',
 draft jsonb NOT NULL DEFAULT '[]', issues jsonb NOT NULL DEFAULT '[]',
 reviewed_revision integer, committed_revision uuid, error_code text,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS bom_item (
 item_uid uuid PRIMARY KEY, identity_key text NOT NULL UNIQUE, category text NOT NULL,
 serial text, parent_uid uuid REFERENCES bom_item(item_uid), payload jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS bom_revision(id uuid PRIMARY KEY, import_id uuid NOT NULL UNIQUE REFERENCES bom_import_batch(id), project_id text NOT NULL, digest text NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS project_asset(project_id text NOT NULL,item_uid uuid NOT NULL REFERENCES bom_item(item_uid),revision_id uuid NOT NULL REFERENCES bom_revision(id),PRIMARY KEY(project_id,item_uid,revision_id));
CREATE TABLE IF NOT EXISTS outbox_event(id uuid PRIMARY KEY,project_id text NOT NULL,event_type text NOT NULL,payload jsonb NOT NULL,published_at timestamptz,created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS cmdb_entity(item_uid uuid PRIMARY KEY REFERENCES bom_item(item_uid), payload jsonb NOT NULL, updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS audit_event(id uuid PRIMARY KEY,project_id text NOT NULL,actor text NOT NULL,action text NOT NULL,resource_id text NOT NULL,details jsonb NOT NULL DEFAULT '{}',created_at timestamptz NOT NULL DEFAULT now());
INSERT INTO schema_migration(version) VALUES(1) ON CONFLICT DO NOTHING;
