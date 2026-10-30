CREATE TABLE IF NOT EXISTS import_draft_revision(import_id uuid NOT NULL REFERENCES bom_import_batch(id),revision int NOT NULL,draft jsonb NOT NULL,profiles jsonb NOT NULL,reason text NOT NULL,actor text NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),PRIMARY KEY(import_id,revision));
CREATE TABLE IF NOT EXISTS device_model_revision(id uuid PRIMARY KEY,model text NOT NULL,category text NOT NULL,capabilities jsonb NOT NULL,approved_by text NOT NULL,approval_reason text NOT NULL,created_at timestamptz NOT NULL DEFAULT now());
CREATE UNIQUE INDEX IF NOT EXISTS device_model_definition ON device_model_revision(model,category,capabilities);
INSERT INTO schema_migration(version) VALUES(3) ON CONFLICT DO NOTHING;
CREATE OR REPLACE FUNCTION preserve_import_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.revision <> OLD.revision THEN
  INSERT INTO import_draft_revision(import_id,revision,draft,profiles,reason,actor)
  VALUES(OLD.id,OLD.revision,OLD.draft,OLD.profiles,'Immutable prior revision; correction reason linked by audit_event','local-developer') ON CONFLICT DO NOTHING;
 END IF;
 RETURN NEW;
END $$;
CREATE OR REPLACE TRIGGER preserve_import_revision BEFORE UPDATE ON bom_import_batch FOR EACH ROW EXECUTE FUNCTION preserve_import_revision();
