INSERT INTO project(id,name,code) VALUES('proj-gb300-phase2','GB300 Phase 2 运维演示项目','GB300-PHASE2'),('proj-b300-lab','B300 Lab 运维演示项目','B300-LAB') ON CONFLICT(id) DO NOTHING;
DELETE FROM project p WHERE p.id IN ('proj-a100-demo','proj-lab-demo') AND NOT EXISTS(SELECT 1 FROM project_membership m WHERE m.project_id=p.id) AND NOT EXISTS(SELECT 1 FROM bom_import_batch b WHERE b.project_id=p.id);
INSERT INTO schema_migration(version) VALUES(11) ON CONFLICT DO NOTHING;
