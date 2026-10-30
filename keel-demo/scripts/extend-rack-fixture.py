"""Explicit synthetic observations; never production defaults or invented live readings."""
import json,math
from datetime import datetime,timedelta,timezone
from pathlib import Path
p=Path('fixtures/demo-full-64-b300/demo-seed.json');d=json.loads(p.read_text());c=d['collections'];end=datetime(2026,10,30,tzinfo=timezone.utc)
c['metric_series']=[r for r in c['metric_series'] if not r['id'].startswith('rack-trend-')]
for i,rack in enumerate(c['racks']):
 rack['pod_id']='demo-pod-'+str(i//4+1);rack['group_source']='explicit_synthetic_demo_group_v1'
 sensor=next((s for s in c['rack_environment_sensors'] if s['rack_id']==rack['id']),None)
 if sensor:sensor.update(capabilities={'liquid_flow_lpm':'unsupported','voltage_v':'unknown'},support='supported')
 for key,unit,base in [('environment.inlet','°C',rack.get('temperature_inlet_c')),('environment.humidity','%',sensor.get('humidity_percent') if sensor else None),('power.load','kW',rack.get('power_used_kw'))]:
  if base is None:continue
  for j in range(57):
   c['metric_series'].append({'id':f'rack-trend-{i}-{key}-{j}','project_id':rack['project_id'],'subject_type':'rack','subject_id':rack['id'],'metric_key':key,'unit':unit,'value':None if i==5 and j%7==0 else round(base+math.sin(j*.6+i)*(.7 if unit!='%' else 2),2),'timestamp':(end-timedelta(hours=(56-j)*3)).isoformat().replace('+00:00','Z'),'source':'explicit_synthetic_rack_timeseries_v1','freshness':'stale' if i==5 else 'fresh','support':'supported','is_demo':True})
# A few recent samples make 1h useful; the 7d history stays separate and deterministic.
for i,rack in enumerate(c['racks']):
 for j in range(6):
  c['metric_series'].append({'id':f'rack-trend-{i}-recent-{j}','project_id':rack['project_id'],'subject_type':'rack','subject_id':rack['id'],'metric_key':'environment.inlet','unit':'°C','value':round(rack['temperature_inlet_c']+math.sin(j*.6+i)*.7,2),'timestamp':(end-timedelta(minutes=(5-j)*10)).isoformat().replace('+00:00','Z'),'source':'explicit_synthetic_rack_timeseries_v1','freshness':'stale' if i==5 else 'fresh','support':'supported','is_demo':True})
p.write_text(json.dumps(d,ensure_ascii=False,indent=2)+'\n')
