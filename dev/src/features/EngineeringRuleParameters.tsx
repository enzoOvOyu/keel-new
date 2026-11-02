import {useState} from 'react';
import {useQuery} from '@tanstack/react-query';
import {Button,Badge} from '../components/ui';
import {createEngineeringRule,engineeringRules,reviewEngineeringRule,type CalculationRule,type RuleExpression} from '../lib/api/engineering';

const literal=(value:string,unit:string):RuleExpression=>({operator:'literal',literal:{value,unit},arguments:[]});
const length:RuleExpression={operator:'input',input:'length',arguments:[]};
const named=(value:string)=>({name:'length',value:{kind:'decimal' as const,decimal:value,unit:'m'}});
function parameterValues(rule:CalculationRule){
 const e=rule.expression;
 if(e.operator==='literal')return {algorithm:'lead',value:e.literal?.value||'',name:'端部引入长度'};
 if(e.operator==='select_ceiling')return {algorithm:'stock',value:e.arguments.slice(1).map(a=>a.literal?.value).filter(Boolean).join(', '),name:'标准长度向上匹配'};
 return {algorithm:'slack',value:e.arguments[1]?.arguments[1]?.literal?.value||'',name:'布线余量百分比'};
}
export function EngineeringRuleParameters({project,canEdit,canReview}:{project:string;canEdit:boolean;canReview:boolean}){
 const rules=useQuery({queryKey:['engineering-rules',project],queryFn:({signal})=>engineeringRules(project,signal)});
 const [algorithm,setAlgorithm]=useState('lead'),[value,setValue]=useState(''),[testInput,setTestInput]=useState(''),[expected,setExpected]=useState(''),[reason,setReason]=useState(''),[error,setError]=useState(''),[busy,setBusy]=useState(false),[base,setBase]=useState<CalculationRule|null>(null);
 async function run(action:()=>Promise<unknown>){setBusy(true);setError('');try{await action();await rules.refetch()}catch(e){setError((e as Error).message)}finally{setBusy(false)}}
 async function save(){
  let expression:RuleExpression;
  if(algorithm==='lead')expression=literal(value,'m');
  else if(algorithm==='slack')expression={operator:'multiply',arguments:[length,{operator:'add',arguments:[literal('1','1'),literal(value,'%')]}]};
  else expression={operator:'select_ceiling',arguments:[length,...value.split(/[,，\s]+/).filter(Boolean).map(v=>literal(v,'m'))]};
  const rule:CalculationRule={id:base?.id||crypto.randomUUID(),version:(base?.version||0)+1,domain:'connectivity_cabling',scope_type:'project',scope_id:project,inputs:algorithm==='lead'?[]:[{name:'length',value_type:'decimal',unit:'m',required:true}],outputs:[{name:'length',value_type:'decimal',unit:'m',required:true}],expression,dependencies:[],validation_cases:[{id:crypto.randomUUID(),inputs:algorithm==='lead'?[]:[named(testInput)],expected:[named(expected)],tolerance:'0'}],approval_status:'draft'};
  await createEngineeringRule(project,{expected_version:base?.version||0,rule,reason});setBase(null);
 }
 return <section className="es-rule-parameters"><h4>布线算法与参数</h4><p>在明确允许的通道上计算最短路径。两端引入、余量和可采购长度分别使用经核对的规则；参数不预填样本值。</p>
 <div className="es-rule-form"><label>规则用途<select value={algorithm} disabled={busy} onChange={e=>{setAlgorithm(e.target.value);setBase(null);setValue('')}}><option value="lead">端部引入长度</option><option value="slack">布线余量百分比</option><option value="stock">标准长度向上匹配</option></select></label><label>{algorithm==='lead'?'引入长度 (m)':algorithm==='slack'?'余量 (%)':'可采购长度 (m，逗号分隔)'}<input value={value} onChange={e=>setValue(e.target.value)} disabled={busy||!canEdit}/></label>{algorithm!=='lead'&&<label>独立参考算例：输入长度 (m)<input value={testInput} onChange={e=>setTestInput(e.target.value)} disabled={busy||!canEdit}/></label>}<label>独立参考算例：预期结果 (m)<input value={expected} onChange={e=>setExpected(e.target.value)} disabled={busy||!canEdit}/></label><label>依据 / 审核说明<input value={reason} onChange={e=>setReason(e.target.value)} disabled={busy}/></label></div>
 <Button disabled={!canEdit||busy||!value.trim()||!expected.trim()||!reason.trim()||algorithm!=='lead'&&!testInput.trim()} onClick={()=>void run(save)}>{base?'保存规则新版本':'保存并验证规则'}</Button>{base&&<Button onClick={()=>setBase(null)}>取消新版本</Button>}
 {[error,rules.error?.message].filter(Boolean).map((e,i)=><p role="alert" key={i}>{e}</p>)}
 <div className="table-wrap"><table><thead><tr><th>用途与参数</th><th>规则引用</th><th>输入 → 输出</th><th>状态</th><th>操作</th></tr></thead><tbody>{rules.data?.map(v=><tr key={v.rule.id+v.rule.version}><td>{parameterValues(v.rule).name}<small className="engineering-code">{parameterValues(v.rule).value} {parameterValues(v.rule).algorithm==='slack'?'%':'m'}</small></td><td><code>{v.rule.id}@{v.rule.version}</code></td><td>{v.rule.inputs.map(i=>i.name+' ('+i.unit+')').join(', ')||'明确参数'} → {v.rule.outputs.map(o=>o.name+' ('+o.unit+')').join(', ')}</td><td><Badge value={v.rule.approval_status}/></td><td><Button disabled={!canReview||busy||v.rule.approval_status!=='validated'||!reason.trim()} onClick={()=>void run(()=>reviewEngineeringRule(project,v.rule.id,v.rule.version,{expected_digest:v.digest,decision:'approve',reason}))}>审核采用</Button>{v.rule.domain==='connectivity_cabling'&&<Button disabled={!canEdit||busy} onClick={()=>{const p=parameterValues(v.rule);setBase(v.rule);setAlgorithm(p.algorithm);setValue(p.value);setTestInput(v.rule.validation_cases[0]?.inputs[0]?.value.decimal||'');setExpected(v.rule.validation_cases[0]?.expected[0]?.value.decimal||'')}}>新版本</Button>}</td></tr>)}</tbody></table></div><p className="muted">在下方规则引用字段中选择已批准规则，机柜接入可选择各自的引入规则。不同机柜可使用各自的引入规则；调整参数产生新版本，旧计算保留。</p></section>
}
