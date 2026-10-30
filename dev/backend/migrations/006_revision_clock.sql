CREATE OR REPLACE FUNCTION preserve_import_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 NEW.updated_at=now();
 IF NEW.revision <> OLD.revision THEN
  INSERT INTO import_draft_revision(import_id,revision,draft,profiles,reason,actor)
  VALUES(OLD.id,OLD.revision,OLD.draft,OLD.profiles,'Immutable prior revision; correction reason linked by audit_event','local-developer') ON CONFLICT DO NOTHING;
 END IF;
 RETURN NEW;
END $$;
INSERT INTO schema_migration(version) VALUES(6) ON CONFLICT DO NOTHING;
