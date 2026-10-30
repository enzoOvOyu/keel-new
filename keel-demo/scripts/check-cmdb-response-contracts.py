import json,warnings
from pathlib import Path
import yaml
warnings.filterwarnings('ignore',category=DeprecationWarning)
from jsonschema import Draft202012Validator,RefResolver
warnings.filterwarnings('ignore',category=DeprecationWarning)
doc=yaml.safe_load(Path('docs/openapi.yaml').read_text());payload=json.loads(Path('/tmp/keel-cmdb-contract-payload.json').read_text());mapping={'catalog':'CmdbDossierCatalogEnvelope','tree':'CmdbDossierTreeEnvelope','sources':'CmdbDossierSourcesEnvelope','config':'CmdbDossierConfigEnvelope','lifecycle':'CmdbDossierLifecycleEnvelope','quality':'CmdbDossierQualityEnvelope','relations':'CmdbDossierRelationsEnvelope','saved':'CmdbDossierSavedViewEnvelope','manual':'CmdbDossierManualEnvelope',**({'manual_evidence':'CmdbDossierEvidenceEnvelope'} if 'manual_evidence' in payload else {})}
failures=[]
for key,schema in mapping.items():
 errors=list(Draft202012Validator(doc['components']['schemas'][schema],resolver=RefResolver.from_schema(doc)).iter_errors(payload[key]));print(key,'PASS' if not errors else 'FAIL')
 for e in errors[:3]:failures.append({'resource':key,'path':'/'.join(map(str,e.absolute_path)),'message':e.message[:700]})
Path('/tmp/keel-cmdb-contract-check.json').write_text(json.dumps({'status':'PASS' if not failures else 'FAIL','responses':len(mapping),'failures':failures},ensure_ascii=False,indent=2)+'\n')
if failures:print(json.dumps(failures,ensure_ascii=False,indent=2));raise SystemExit(1)
