-- Project deletion must not scan every other project's versions per entity.
CREATE INDEX IF NOT EXISTS eng_entity_version_project_entity_idx
 ON eng_entity_version(project_id, entity_id);
CREATE INDEX IF NOT EXISTS eng_evidence_source_idx
 ON eng_evidence(source_id);
