from pathlib import Path
import runpy,json
root=Path(__file__).resolve().parents[1]/'backend/internal/ai/prompts'
ns=runpy.run_path(str(root/'validate_engineering.py'))
schema=ns['Action'].model_json_schema(); props=schema['properties']; defs=schema['$defs']
args={
'inspect_workbook':[], 'inspect_sheet':['sheet'], 'inspect_region':['sheet','bounds'], 'read_range':['sheet','bounds'], 'inspect_formula':['sheet','cell'], 'trace_formula_dependencies':['sheet','cell'], 'resolve_cross_sheet_references':['sheet','cell'],
'classify_information':['proposal'], 'map_semantic_fields':['proposal'], 'propose_engineering_model':['sheet','proposal'], 'validate_model':['proposal'],'analyze_validation_issues':['proposal'],'propose_calculation_rule':['rule'],'run_calculation':['rule_ref','cells','inputs']}
tools=[]
for name,fields in args.items():
 p={k:props[k] for k in fields}
 # Omission carries absence; don't advertise null tool arguments.
 for k,v in p.items():
  if 'anyOf' in v:p[k]=next(x for x in v['anyOf'] if x.get('type')!='null')
 used={}
 def refs(v):
  if isinstance(v,dict):
   if '$ref' in v:
    key=v['$ref'].split('/')[-1]
    if key not in used: used[key]=defs[key];refs(defs[key])
   for x in v.values():refs(x)
  elif isinstance(v,list):
   for x in v:refs(x)
 refs(p)
 required=fields if name not in ('trace_formula_dependencies','resolve_cross_sheet_references','run_calculation') else (['sheet'] if name!='run_calculation' else [])
 params={'type':'object','properties':p,'required':required,'additionalProperties':False}
 if used: params['$defs']=used
 description={'propose_engineering_model':'Finish ONE inspected sheet: propose all its engineering regions and explicitly retain unknown information. Persist this sheet checkpoint and move on. No formal writes.','inspect_sheet':'Read structural regions, merged headers and bounded representative source rows. Use content, not the sheet name, to understand semantics.','map_semantic_fields':'Validate a partial region mapping before finishing the sheet.','inspect_workbook':'List workbook structure; no source interpretation.'}.get(name,name.replace('_',' '))
 tools.append({'type':'function','function':{'name':name,'description':description,'parameters':params}})
(root/'engineering-tools.json').write_text(json.dumps(tools,separators=(',',':'))+'\n')
