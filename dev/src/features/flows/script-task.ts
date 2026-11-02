/** Inline copies freeze saved script content into the next Flow revision.
 * These are Kestra worker tasks; device execution stays in Keel's controlled pipeline.
 */
export function inlineScriptTask(path:string,content:string,existingIDs:string[]){
 const python=/\.py$/i.test(path);
 if(!python&&!/\.(sh|bash)$/i.test(path))throw Error('请选择 Shell 或 Python 脚本');
 const base='run_'+(path.split('/').pop()||'script').replace(/\.[^.]+$/,'').replace(/[^a-zA-Z0-9_]/g,'_');
 let id=base,n=2;while(existingIDs.includes(id))id=base+'_'+n++;
 return {id,type:`io.kestra.plugin.scripts.${python?'python':'shell'}.Script`,script:content};
}
