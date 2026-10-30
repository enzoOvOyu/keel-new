"""Concrete CMDB registry, version/change and adapter contracts; preserve existing dossier wire shapes."""
from pathlib import Path
import yaml,copy,re
p=Path('docs/openapi.yaml');raw=p.read_text();s=yaml.safe_load(raw);old=copy.deepcopy(s);S=s['components']['schemas']
class D(yaml.SafeDumper):
 def ignore_aliases(self,data):return True
st={'type':'string'};integer={'type':'integer'};number={'type':'number'};boolean={'type':'boolean'};nullable={'type':['string','null']};date={'type':'string','format':'date-time'}
def ref(n):return {'$ref':'#/components/schemas/'+n}
def arr(v):return {'type':'array','items':v}
def obj(fields,required=None,extra=False):return {'type':'object','properties':fields,'required':list(fields) if required is None else required,'additionalProperties':extra}
S['CmdbJsonValue']={'description':'JSON-serialized value; attribute semantics are validated against the versioned registry. Relationships remain typed foreign-key records.', 'oneOf':[{'type':'null'},st,number,boolean,{'type':'array','items':{}},{'type':'object','additionalProperties':True}]}
attrs={'type':'object','additionalProperties':ref('CmdbJsonValue'),'description':'Read/write attributes are validated against the referenced CMDB Schema version. Core identity and typed relationships remain relational; this is not an unvalidated generic entity.'}
S['CmdbAttributeDefinition']=obj(dict(name=st,label=st,type={'type':'string','enum':['Text','Number','Boolean','Enum','IP','MAC','DateTime','Reference','Array','Object']},unit=st,required=boolean,unique=boolean,editable=boolean,deprecated=boolean,sources=arr({'enum':['bom','agent','bmc','switch','manual','derived']}),models=arr(st),group=st,enum=arr(st),rules=obj(dict(max_length=integer,pattern=st,minimum=number,maximum=number,integer=boolean),[])),['name','label','type','required','unique','editable','sources','models','group','rules'])
S['CmdbDisplayProfile']=obj(dict(catalog=boolean,columns=arr(st),groups=arr(st)),['columns','groups'])
S['CmdbAssetTypeDefinition']=obj(dict(name=st,label=st,extends=st,attributes=arr(ref('CmdbAttributeDefinition')),allow_extensions=boolean,display=ref('CmdbDisplayProfile')))
S['CmdbRelationshipDefinition']=obj(dict(name=st,label=st,kind={'enum':['position','composition','physical','logical','business','dependency','reference']},source_types=arr(st),target_types=arr(st),direction={'enum':['outbound','inbound','bidirectional']},max_targets={'type':'integer','minimum':0},max_sources={'type':'integer','minimum':0},sources=arr(st)))
S['CmdbSchemaDocument']=obj(dict(version=st,types=arr(ref('CmdbAssetTypeDefinition')),relationships=arr(ref('CmdbRelationshipDefinition'))))
S['CmdbSchemaProblem']=obj(dict(code=st,asset_id=st,field=st,reason=st))
S['CmdbSchemaCommand']=obj(dict(schema=ref('CmdbSchemaDocument'),expected_version=st))
S['CmdbSchemaVersionRecord']=obj(dict(version=st,content_hash=st,status={'enum':['active','retired']},created_by=st,created_at=date))
S['CmdbSchemaSummary']=obj(dict(version=st,asset_types=arr(obj(dict(name=st,label=st,catalog=boolean)))))
S['CmdbInventoryRecord']=obj(dict(id=st,project_id=st,category=st,cmdb_asset_type=st,revision=integer,hostname=st,name=st,serial=nullable,model=nullable,lifecycle_status=st,health_status=st,freshness=st,rack_id=nullable,rack_u={'type':['number','null']},source=st),['id','project_id','category'],ref('CmdbJsonValue'))
S['CmdbRackPlacementRecord']=obj(dict(id=st,item_id=st,rack_id=st,u_start=number,u_height=number,face=st,diagram_revision_id=nullable,status=st,source=st,project_id=st,revision=integer),['id','item_id','rack_id','u_start','u_height'],ref('CmdbJsonValue'))
S['CmdbInventorySnapshot']=obj({k:arr(ref('CmdbInventoryRecord')) for k in ['servers','switches','gpus','nics','ports','switch_ports','optics','cables','racks']}|{'rack_placements':arr(ref('CmdbRackPlacementRecord'))})
S['CmdbCanonicalInventoryEnvelope']=obj(dict(data=ref('CmdbInventorySnapshot'),schema_version=st,source=st,truncated=boolean,request_id=st))
S['CmdbDossierAsset']['properties']['asset_type']=dict(type='string',description='Registry-defined type; existing server/switch/gpu/nic/port/optic/cable/rack identifiers are preserved.')
S['CmdbDossierAsset']['properties'].update(dict(lifecycle_state=st,schema_version=st))
S['CmdbDossierRelation']['properties']['kind']['enum']=list(dict.fromkeys(S['CmdbDossierRelation']['properties']['kind']['enum']+['dependency','reference']))
S['CmdbDossierRelation']['properties']['fact']['enum']=['planned','observed','recorded','baseline','intended','actual','operational']
S['CmdbDossierQuality']['properties']['project_id']=st
S['CmdbDossierRelation']['properties'].update(dict(source_type=st,direction=st,cardinality=st,valid_from=date,valid_to={'type':['string','null']},relationship_status=st,definition_version=nullable))
S['CmdbDossierSource']['properties'].update(dict(source_type=st,source_ref=st,source_location=obj(dict(import_id=nullable,document_id=nullable,sheet=nullable,cell=nullable,row=integer,column=integer,artifact_ref=st,resource=st,record_id=st),[]),fact_layer=st,operator=st,updated_at=date,confidence={'type':['number','null']}))
S['CmdbDossierConfigVersion']['properties']['source_ref']=st
S['CmdbDossierEvent']['properties']['source_ref']=st
S['CmdbDossierCatalogEnvelope']['properties']['counts']['additionalProperties']=integer
S['CmdbDossierCatalogEnvelope']['properties'].update(dict(saved_views=arr(ref('CmdbDossierSavedView')),schema_summary=ref('CmdbSchemaSummary')))
S['CmdbDossierTreeEnvelope']['properties']['device_records']=arr(ref('CmdbInventoryRecord'))
S['CmdbAssetDetailsEnvelope']=obj(dict(data=ref('CmdbDossierAsset'),attributes=attrs,owner_device={'oneOf':[ref('CmdbInventoryRecord'),{'type':'null'}]},request_id=st))
S['CmdbWriteSource']=obj(dict(source_type=st,source_ref=st,fact_layer=st,operator=st,observed_at={'type':['string','null']},source_location={'oneOf':[attrs,{'type':'null'}]},approval_status=st,freshness=st,support=st,evidence_refs={'oneOf':[arr(st),{'type':'null'}]}))
S['CmdbRelationCommand']=obj(dict(id=st,relationship_type=st,source_asset_id=st,target_asset_id=st,project_id=st,expected_revision=integer),['relationship_type','source_asset_id','target_asset_id'])
S['CmdbRelationCommand']['properties']['source']=dict(ref('CmdbWriteSource'),readOnly=True)
S['CmdbChange']=obj(dict(op={'enum':['asset.create','asset.update','asset.retire','asset.replace','relationship.upsert','relationship.end']},asset_id=st,asset_type=st,expected_revision=integer,attributes=attrs,relationship=ref('CmdbRelationCommand'),relationship_id=st,replacement_id=st,before={'oneOf':[attrs,{'type':'null'}]}),['op','asset_id','expected_revision'])
S['CmdbChangeSetCommand']=obj(dict(project_id=st,title=st,reason=st,schema_version=st,changes=arr(ref('CmdbChange')),branch_ref=st,base_ref=st),['project_id','title','reason','schema_version','changes'])
S['CmdbChangeSet']=obj(dict(id=st,project_id=st,title=st,reason=st,status={'enum':['Draft','Review','Approved','Published','Rejected']},version=integer,schema_version=st,changes=arr(ref('CmdbChange')),requester_id=st,reviewer_id=nullable,review_reason=nullable,error_code=nullable,created_at=date,published_at={'type':['string','null']}))
S['CmdbChangeSetAction']=obj(dict(expected_revision=integer,reason=st))
S['CmdbRevision']=obj(dict(id=st,version=integer,schema_version=st,attributes=attrs,before={'oneOf':[attrs,{'type':'null'}]},source=st,source_ref=st,operator=st,created_at=date))
S['CmdbRevisionDiff']=obj(dict(asset_id=st,from_=integer,to=integer,changes=arr(obj(dict(field=st,before=ref('CmdbJsonValue'),after=ref('CmdbJsonValue'))))))
S['CmdbRevisionDiff']['properties']['from']=S['CmdbRevisionDiff']['properties'].pop('from_');S['CmdbRevisionDiff']['required'].remove('from_');S['CmdbRevisionDiff']['required'].append('from')
S['CmdbChangeSetDiff']=obj(dict(id=st,changes=arr(obj(dict(op=st,asset_id=st,expected_revision=integer,before={'oneOf':[attrs,{'type':'null'}]},after={'oneOf':[attrs,{'type':'null'}]},relationship={'oneOf':[ref('CmdbRelationCommand'),{'type':'null'}]},replacement_id=st))),frozen=boolean))
S['CmdbDrawerResources']={'type':'object','properties':{k:arr(ref('CmdbInventoryRecord')) for k in ['gpus','nics','ports','switch_ports','optics','bmc_inventory','bmc_sensors','firmware_inventory','server_config_baselines','server_observed_configs','mapping_pairs','server_lldp_observations','lldp_neighbors','metrics','metric_series','alerts','server_runtime','bmc_sel_events','network_baselines','push_jobs','switch_config_baselines','switch_running_configs','switch_config_diffs','parameters','audit_events','switch_runtime','fdb_entries','arp_entries','switch_bgp_neighbors','switch_interfaces','monitoring_sources','network_assertions','lossless_baselines','lossless_evidence','lossless_checks','lossless_profiles','lossless_raw_outputs','lossless_host_evidence','ddm_samples','execution_logs','switch_logs']},'additionalProperties':False}
# Domain snapshots have resource-specific ownership and IDs; not all snapshots are assets.
for k in S['CmdbDrawerResources']['properties']:S['CmdbDrawerResources']['properties'][k]=arr(obj(dict(id=st,project_id=st),['id'],ref('CmdbJsonValue')))
S['CmdbDrawerEnvelope']=obj(dict(data={'oneOf':[ref('IAServerTab'),ref('IANetworkTab')]},resources=ref('CmdbDrawerResources'),device=ref('CmdbInventoryRecord'),request_id=st))
# Keep raw evidence unions resource-specific for existing and newly linked source classes.
for resource in ['servers','switches','gpus','nics','ports','switch_ports','cables','racks','optics','handover_records','server_runtime','switch_runtime','switch_config_baselines','delivery_stage_records']:
 name='Full'+''.join(p.title() for p in resource.split('_'))
 if name in S and ref(name) not in S['CmdbDossierEvidenceEnvelope']['properties']['data']['oneOf']:S['CmdbDossierEvidenceEnvelope']['properties']['data']['oneOf'].append(ref(name))
def env(name,data):S[name]=obj(dict(data=data,request_id=st));return ref(name)
def op(path,method,name,data,body=None,params=None):
 item=s['paths'].setdefault(path,{})
 item['parameters']=[{'in':'path','name':x[1:-1],'required':True,'schema':st} for x in path.split('/') if x.startswith('{')]
 endpoint={'tags':['CMDB'],'operationId':name,'description':'Real PostgreSQL CMDB. Existing presentation contract is retained. Asset writes publish through a versioned, independently approved ChangeSet. Unknown data remains unknown.','security':[{'cookieSession':[]}],'responses':{'200':{'description':'Success','content':{'application/json':{'schema':env(name+'Envelope',data)}}}}}
 endpoint['parameters']=[{'in':'query','name':'project_id','required':False,'schema':st}]+(params or [])
 for code,message in [('400','Invalid query or bounded traversal'),('401','Session expired'),('403','Project/resource permission or source ownership denied'),('404','Resource not found in authorized scope'),('409','Version, identity or idempotency conflict'),('422','Schema, relation cardinality, validation or approval condition not satisfied')]:endpoint['responses'][code]={'description':message}
 if method!='get':endpoint['parameters'] +=[{'in':'header','name':'X-CSRF-Token','required':True,'schema':st},{'in':'header','name':'Idempotency-Key','required':False,'schema':st}]
 if body:endpoint['requestBody']={'required':True,'content':{'application/json':{'schema':ref(body)}}}
 item[method]=endpoint
op('/cmdb/schemas','get','getCMDBRegistry',ref('CmdbSchemaDocument'))
op('/cmdb/schemas','post','installCMDBSchema',obj(dict(version=st,status=st,historical_assets_validated=boolean)),'CmdbSchemaCommand')
op('/cmdb/schemas/validate','post','validateCMDBSchema',obj(dict(compatible=boolean,problems=arr(ref('CmdbSchemaProblem')))),'CmdbSchemaCommand')
op('/cmdb/schemas/versions','get','listCMDBSchemaVersions',arr(ref('CmdbSchemaVersionRecord')))
op('/cmdb/schemas/{schema_version}','get','getCMDBSchemaVersion',ref('CmdbSchemaDocument'))
op('/cmdb/inventory','get','getCanonicalCMDBInventory',ref('CmdbInventorySnapshot'));s['paths']['/cmdb/inventory']['get']['responses']['200']['content']['application/json']['schema']=ref('CmdbCanonicalInventoryEnvelope')
op('/cmdb/dossier/assets/{asset_id}','get','getCMDBDossierAsset',ref('CmdbDossierAsset'));s['paths']['/cmdb/dossier/assets/{asset_id}']['get']['responses']['200']['content']['application/json']['schema']=ref('CmdbAssetDetailsEnvelope')
op('/cmdb/dossier/assets/{asset_id}/drawer/{tab}','get','getCMDBDeviceDrawer',ref('CmdbDrawerEnvelope'));s['paths']['/cmdb/dossier/assets/{asset_id}/drawer/{tab}']['get']['responses']['200']['content']['application/json']['schema']=ref('CmdbDrawerEnvelope')
op('/cmdb/dossier/assets/{asset_id}/revisions','get','listCMDBAssetRevisions',arr(ref('CmdbRevision')))
op('/cmdb/dossier/assets/{asset_id}/diff','get','diffCMDBAssetRevisions',ref('CmdbRevisionDiff'),params=[{'in':'query','name':n,'required':True,'schema':integer} for n in ['from','to']])
S['CmdbRelationshipRevision']=obj(dict(id=st,relationship_id=st,version=integer,attributes=attrs,source_ref=st,created_at=date))
op('/cmdb/relationships/{relationship_id}/revisions','get','listCMDBRelationshipRevisions',arr(ref('CmdbRelationshipRevision')))
op('/cmdb/changesets','get','listCMDBChangeSets',arr(ref('CmdbChangeSet')),params=[{'in':'query','name':'asset_id','schema':st}])
op('/cmdb/changesets','post','createCMDBChangeSet',obj(dict(id=st,status=st,version=integer),['id','status']),'CmdbChangeSetCommand')
op('/cmdb/changesets/{changeset_id}/diff','get','previewCMDBChangeSet',ref('CmdbChangeSetDiff'))
for action in ['submit','approve','reject','publish']:op('/cmdb/changesets/{changeset_id}/'+action,'post',action+'CMDBChangeSet',obj(dict(id=st,status=st,version=integer)),'CmdbChangeSetAction')
def patch_section(raw,marker,level,before,after):
 start=raw.index(marker)+len(marker);endmatch=re.search(r'(?m)^\S' if level==2 else r'(?m)^(?:\S|  \S)',raw[start:]);end=start+endmatch.start() if endmatch else len(raw);text=raw[start:end]
 for key,value in after.items():
  if before.get(key)==value:continue
  block=''.join(' '*level+line if line.strip() else line for line in yaml.dump({key:value},Dumper=D,allow_unicode=True,sort_keys=False,width=160).splitlines(True));pattern=r'(?m)^'+' '*level+re.escape(key)+r':[^\n]*\n(?:^(?:'+' '*(level+1)+r'[^\n]*|[ \t]*)\n)*';match=re.search(pattern,text)
  if match:text=text[:match.start()]+block+text[match.end():]
  else:text+=block
 return raw[:start]+text+raw[end:]
raw=patch_section(raw,'paths:\n',2,old['paths'],s['paths']);raw=patch_section(raw,'  schemas:\n',4,old['components']['schemas'],S);assert yaml.safe_load(raw)['paths']==s['paths'];assert yaml.safe_load(raw)['components']['schemas']==S;p.write_text(raw)
