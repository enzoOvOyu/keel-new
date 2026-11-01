from pathlib import Path
import re,yaml
p=Path(__file__).resolve().parents[2]/'docs/openapi.yaml';raw=p.read_text();d=yaml.safe_load(raw);s=d['components']['schemas']['IDECablePlan'];s['properties']['segment_index']={'type':'integer','minimum':1,'description':'One-based external physical segment; required for paths containing patch panels.'}
yaml.SafeDumper.ignore_aliases=lambda self,data:True
block=''.join('    '+line for line in yaml.safe_dump({'IDECablePlan':s},sort_keys=False,allow_unicode=True,width=120).splitlines(True))
raw,n=re.subn(r'^    IDECablePlan:\n.*?(?=^    [A-Za-z0-9_-]+:)',lambda _:block,raw,count=1,flags=re.M|re.S);assert n==1;p.write_text(raw)
