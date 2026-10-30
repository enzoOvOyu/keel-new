"""Isolated synthetic outbox event fault/recovery; no schema changes or asset deletion."""
import subprocess,pathlib,uuid,time,json
imp=str(uuid.UUID(pathlib.Path('/tmp/keel-bom-acceptance-import').read_text().strip()));event=str(uuid.uuid4());missing=str(uuid.uuid4())
def sql(query):
 r=subprocess.run(['docker','exec','-i','keel-dev-postgres-1','psql','-U','keel','-d','keel','-qAt','-v','ON_ERROR_STOP=1'],input=query,text=True,capture_output=True,check=True);return r.stdout.strip()
revision=sql(f"SELECT id FROM bom_revision WHERE import_id='{imp}';");uuid.UUID(revision)
before=json.loads(sql("SELECT COALESCE(jsonb_object_agg(e.item_uid::text,r.created_at),'{}'::jsonb) FROM cmdb_entity e JOIN bom_revision r ON r.id=e.revision_id;"))
sql(f"INSERT INTO outbox_event(id,project_id,event_type,payload) SELECT '{event}',r.project_id,'bom.committed',jsonb_build_object('import_id',r.import_id::text,'revision_id','{missing}','items',(SELECT jsonb_agg(payload) FROM bom_item_revision WHERE revision_id=r.id),'synthetic_failure_test',true) FROM bom_revision r WHERE r.id='{revision}';")
try:
 for _ in range(20):
  count=sql(f"SELECT retry_count FROM outbox_event WHERE id='{event}';")
  if int(count)>0:break
  time.sleep(1)
 assert int(count)>0,'Projection did not record failure'
 assert sql(f"SELECT published_at IS NULL FROM outbox_event WHERE id='{event}';")=='t'
 assert sql(f"SELECT count(*) FROM cmdb_entity WHERE revision_id='{missing}';")=='0','Failed projection partially changed entities'
 assert sql(f"SELECT count(*) FROM cmdb_relationship WHERE revision_id='{missing}';")=='0','Failed projection partially changed relationships'
finally:
 sql(f"UPDATE outbox_event SET payload=jsonb_set(payload,'{{revision_id}}',to_jsonb('{revision}'::text)),next_attempt_at=now() WHERE id='{event}';")
for _ in range(20):
 done=sql(f"SELECT published_at IS NOT NULL FROM outbox_event WHERE id='{event}';")
 if done=='t':break
 time.sleep(1)
assert done=='t','Outbox recovery did not finish'
after=json.loads(sql("SELECT COALESCE(jsonb_object_agg(e.item_uid::text,r.created_at),'{}'::jsonb) FROM cmdb_entity e JOIN bom_revision r ON r.id=e.revision_id;"))
from datetime import datetime
assert all(k in after and datetime.fromisoformat(after[k])>=datetime.fromisoformat(v) for k,v in before.items()),'Replayed older revision regressed CMDB'
print(json.dumps({'outbox_fault_recovery':'PASS','failure_recorded':True,'failed_projection_atomic':True,'repaired_event_replayed':True,'cmdb_version_did_not_regress':True,'scope':'synthetic event only'}))
