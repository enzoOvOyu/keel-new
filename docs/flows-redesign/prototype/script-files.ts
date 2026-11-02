export type ScriptFile={content:string;saved:string|null;revision:number};
export type ScriptUse={path:string;mode:'namespace'|'inline';target:'new'|'selected'};
export function languageFor(path:string):'shell'|'python'|'plaintext'{return /\.(?:sh|bash)$/i.test(path)?'shell':/\.py$/i.test(path)?'python':'plaintext'}
export function scriptTemplate(kind:string){return kind==='python'?'#!/usr/bin/env python3\n\n# 在此编写 Python 脚本\n':'#!/usr/bin/env bash\nset -euo pipefail\n\n# 在此编写 Shell 脚本\n'}
export function validatePath(path:string){if(!path||path.startsWith('/')||path.split('/').some(p=>!p||p==='.'||p==='..'))throw Error('请使用相对文件路径，例如 scripts/check_gpu.py');}
export function scriptTask(path:string,mode:'namespace'|'inline',content:string,id:string){
 const kind=languageFor(path);if(kind==='plaintext')throw Error('目前任务引用演示支持 .sh、.bash 和 .py 脚本，其他文本可编辑与下载');
 if(mode==='inline')return {id,type:`io.kestra.plugin.scripts.${kind==='python'?'python':'shell'}.Script`,script:content};
 const quoted="'"+path.replaceAll("'","'\\''")+"'";
 return {id,type:`io.kestra.plugin.scripts.${kind==='python'?'python':'shell'}.Commands`,namespaceFiles:{enabled:true,include:[path]},commands:[`${kind==='python'?'python':'bash'} ${quoted}`]};
}
