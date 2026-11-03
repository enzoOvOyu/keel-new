"""Run one targeted pre-Agent integration check in a disposable database/bucket."""
import json, os, pathlib, subprocess, uuid
from urllib.parse import quote
root=pathlib.Path(__file__).resolve().parents[2]
def configuration(container):
 data=json.loads(subprocess.check_output(['docker','inspect',container]))[0]
 values=dict(v.split('=',1) for v in data['Config']['Env'] if '=' in v)
 return values,data['NetworkSettings']['Ports']
pg,ports=configuration('keel-dev-postgres-1');s3,s3ports=configuration('keel-dev-minio-1')
name='keel_engineering_kickoff_'+uuid.uuid4().hex[:10]
user=pg.get('POSTGRES_USER','postgres');live=pg.get('POSTGRES_DB','postgres')
def pgexec(*args,**kwargs):return subprocess.run(['docker','exec','-i','keel-dev-postgres-1',*args],check=True,**kwargs)
pgexec('createdb','-U',user,name)
try:
 schema=subprocess.check_output(['docker','exec','keel-dev-postgres-1','pg_dump','-U',user,'--schema-only','--no-owner','--no-privileges',live])
 pgexec('psql','-U',user,'-d',name,'-v','ON_ERROR_STOP=1',input=schema,stdout=subprocess.DEVNULL)
 versions=subprocess.check_output(['docker','exec','keel-dev-postgres-1','psql','-U',user,'-d',live,'-tAc','SELECT version FROM schema_migration ORDER BY version'])
 pgexec('psql','-U',user,'-d',name,'-v','ON_ERROR_STOP=1','-c','COPY schema_migration(version) FROM STDIN',input=versions,stdout=subprocess.DEVNULL)
 env=os.environ.copy()
 env['KEEL_KICKOFF_TEST_DATABASE_URL']=f"postgres://{quote(user)}:{quote(pg.get('POSTGRES_PASSWORD',''))}@127.0.0.1:{ports['5432/tcp'][0]['HostPort']}/{name}?sslmode=disable"
 env['KEEL_ENGINEERING_TEST_DATABASE_URL']=env['KEEL_KICKOFF_TEST_DATABASE_URL']
 env['KEEL_KICKOFF_TEST_S3_ENDPOINT']='127.0.0.1:'+s3ports['9000/tcp'][0]['HostPort']
 env['KEEL_KICKOFF_TEST_S3_USER']=s3.get('MINIO_ROOT_USER','minioadmin');env['KEEL_KICKOFF_TEST_S3_PASSWORD']=s3.get('MINIO_ROOT_PASSWORD','minioadmin')
 env['KEEL_KICKOFF_TEST_ARTIFACT']='/tmp/keel-kickoff-verified-workspace.json'
 subprocess.run(['go','test','./cmd/api','-run','^(TestKickoffBeforeAgent|TestEngineeringPhysicalPublication)$','-count=1','-v'],cwd=root/'dev/backend',env=env,check=True)
finally:pgexec('dropdb','-U',user,'--force',name)
