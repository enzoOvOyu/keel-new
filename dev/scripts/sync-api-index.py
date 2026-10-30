from pathlib import Path
import yaml,shutil
s=yaml.safe_load(Path('docs/openapi.yaml').read_text());text='# 当前 API 操作索引\n\n由当前 openapi.yaml 提取，base URL 为 /api/v1。路径为HTTP路由。BOM真实实施见BOM-REAL-DESIGN；操作存在不等于其他模块已有Go实现。\n\n| 方法 | 路径 | operationId | 状态 |\n|---|---|---|---|\n'
for p,item in s['paths'].items():
 for m,o in item.items():
  if m in {'get','post','patch','put','delete','head','options'}:text+=f"| {m.upper()} | `{p}` | `{o['operationId']}` | {'deprecated' if o.get('deprecated') else '当前契约'} |\n"
Path('docs/API-OPERATION-INDEX.md').write_text(text);shutil.copy2('docs/API-OPERATION-INDEX.md','keel-demo/docs/API-OPERATION-INDEX.md')
