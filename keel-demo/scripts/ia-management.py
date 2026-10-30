import json
from pathlib import Path
root=Path(__file__).resolve().parents[1];p='proj-b300-demo';stamp='2026-10-29T14:00:00Z'
def row(id,**kw):return dict(id=id,project_id=p,revision=1,**kw)
c={
'ai_providers':[row('ai-provider-demo',name='Offline BOM semantic adapter',model='deterministic-mock-v1',api_url='https://ai.demo.invalid/v1',credential_ref='vault://demo/ai/provider',enabled=True,timeout_seconds=30,max_attempts=2,tokens_used=2400,status='mock_only')],
'ai_jobs':[row('ai-parse-demo',provider_id='ai-provider-demo',import_id='bom-import-demo-001',status='review_required',attempt=1,tokens_used=2400,source_refs=['bom-import-demo-001'],error_code=None,created_at=stamp)],
'knowledge_sources':[row('kb-bom',name='脱敏BOM映射规则',source_ref='docs/EXCEL-IMPORT-PROFILE.md',status='indexed',version='mock-v1'),row('kb-issues',name='T1–T7 固定排障案例',source_ref='fixtures/demo-64-b300/expanded-seed.json',status='indexed',version='mock-v1')],
'ai_feedback':[row('feedback-seed',task_id='ai-parse-demo',field='hostname',original='synthetic header',corrected='hostname',reason='人工确认合成字段语义',status='reviewed')],
'notification_settings':[row('notify-critical',name='严重告警与执行失败',event_types=['alert.critical','execution.failed'],channel='in_app',enabled=True)],
'api_tokens':[row('token-demo-metadata',name='Demo read-only integration',scopes=['cmdb:read'],expires_at='2026-12-31T00:00:00Z',status='active',token_prefix='SIMULATION_ONLY_')],
'ssh_trust':[row('ssh-trust-demo',device_id='srv-0001',fingerprint='SHA256:DEMO-NON-KEY-FINGERPRINT',status='pending',source='synthetic_demo',observed_at=stamp)],
'collection_policies':[row('collection-bmc',name='维护期 IPMI 观测',protocol='ipmi',scope='maintained',interval_seconds=60,timeout_seconds=15,max_attempts=2,status='draft',notice='Demo interval only; production policy not approved'),row('collection-standard',name='交付期采集',protocol='agent',scope='delivery',interval_seconds=300,timeout_seconds=30,max_attempts=2,status='draft',notice='No maintained targets; no real collector')]
}
(root/'fixtures/demo-64-b300/ia-v24-management.json').write_text(json.dumps({'meta':{'notice':'Synthetic UI contract fixtures. No real keys, endpoints, fingerprints or production defaults.'},'collections':c},ensure_ascii=False,indent=2)+'\n')
