"""Normalize supplied *demo* execution snapshots without inventing target device identities."""
import json
from collections import Counter
from pathlib import Path
root=Path(__file__).resolve().parents[1];p=root/'fixtures/demo-full-64-b300/demo-seed.json';d=json.loads(p.read_text());c=d['collections'];report={'source':'supplied full demo execution_targets','repairs':[],'blocked_snapshots':[]};c.setdefault('task_evidence_archive',[])
if not c['task_evidence_archive']:c['task_evidence_archive']=c['task_runs']
new_tasks=[];devices={r['id']:r for r in c['servers']+c['switches']};flows={r['id']:r for r in c['flows']}
for e in c['executions']:
 targets=[t for t in c['execution_targets'] if t['execution_id']==e['id']];flow=flows[e['flow_id']];invalid=[t['target_id'] for t in targets if t['target_id'] not in devices or devices[t['target_id']]['category'] not in flow['target_types']]
 report['repairs'].append({'id':e['id'],'reported_target_count':e.get('reported_target_count',e['target_count']),'provided_target_snapshot_count':len(targets),'provided_task_evidence_count':sum(t['execution_id']==e['id'] for t in c['task_evidence_archive']),'method':'use explicit supplied target IDs, preserve original declaration, derive per-target result projection'})
 e.setdefault('reported_target_count',e['target_count']);e.setdefault('reported_counts',e['counts']);e['target_count']=len(targets);e['target_ids']=[t['target_id'] for t in targets];e['counts']=dict(Counter(t['status'] for t in targets));e['snapshot_scope_notice']='Repaired demo scope uses only provided explicit targets; original aggregate retained as reported_*; not a real execution claim';e['snapshot_complete']=not invalid;e['data_quality']='degraded' if invalid else 'repaired_demo_scope'
 if invalid:e.setdefault('reported_status',e['status']);e['status']='result_unknown';e['snapshot_error']='FLOW_TARGET_TYPE_MISMATCH';report['blocked_snapshots'].append({'id':e['id'],'targets':invalid,'reason':'server targets in a switch-only Flow; no device ID substitution or automatic retry'})
 for t in targets:
  new_tasks.append({'id':'full-target-task-'+t['id'],'project_id':e['project_id'],'execution_id':e['id'],'task_key':'snapshot_result','target_id':t['target_id'],'status':'result_unknown' if invalid else t['status'],'attempt_count':1,'started_at':t['started_at'],'finished_at':t['finished_at'],'duration_ms':0,'exit_code':t['exit_code'],'error_summary':'Provided target type does not match Flow' if invalid else None,'retryable':False,'execution_mode':'mock','source':'execution_targets_projection','source_target_record_id':t['id']})
for log in c['execution_logs']:
 task=next((t for t in new_tasks if t['execution_id']==log['execution_id'] and t['target_id']==log['target_id']),None)
 if task:log.setdefault('source_task_run_id',log['task_run_id']);log['task_run_id']=task['id']
c['task_runs']=new_tasks;d['meta']['counts']={k:len(v) for k,v in c.items()};d['meta']['collection_counts']=d['meta']['counts'];p.write_text(json.dumps(d,ensure_ascii=False,indent=2)+'\n');(root/'docs/IA24-EXECUTION-REPAIR.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print('projected',len(new_tasks),'target result tasks; quarantined',len(report['blocked_snapshots']),'invalid historical snapshots')
