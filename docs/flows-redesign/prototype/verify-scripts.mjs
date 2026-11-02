import {chromium} from 'playwright';
import YAML from 'yaml';
import fs from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
const dir=path.dirname(fileURLToPath(import.meta.url));const checks=[],errors=[],apiRequests=[];
const browser=await chromium.launch();const page=await browser.newPage({viewport:{width:1600,height:1000}});
page.on('pageerror',e=>errors.push(e.message));page.on('request',r=>{if(new URL(r.url()).pathname.startsWith('/api/'))apiRequests.push(r.url())});
const toolbar=()=>page.locator('.jp-editor .tabs-actions');const dialog=()=>page.locator('.kel-dialog:visible');
async function focusPanel(uid,label){await page.locator('.jp-editor .main-editor').waitFor();const tab=page.locator('.editor-tab[data-tab-id="'+uid+'"]');if(await tab.count())await tab.click();else await page.getByText(label,{exact:true}).first().click()}
async function flowDocument(){await focusPanel('code','YAML');await page.locator('.native-code-editor .monaco-editor').waitFor();return YAML.parse(await page.locator('.native-code-editor [data-testid="monaco-editor-hidden-synced-textarea"]').inputValue())}
try{
 await page.goto('http://127.0.0.1:5181/jobs-preview/index.html#editor/gpu-preflight');await page.locator('.leaf-block-card').first().waitFor();
 await toolbar().getByRole('button',{name:'新增脚本',exact:true}).click();await dialog().getByLabel('语言',{exact:true}).selectOption('python');await dialog().getByPlaceholder('scripts/check_gpu.py').fill('scripts/new_gpu_check.py');await dialog().getByRole('button',{name:'创建并编辑',exact:true}).click();
 await page.locator('.jp-script-code .monaco-editor').waitFor({timeout:45000});
 if(!await page.getByRole('button',{name:'在 Flow 中使用',exact:true}).isDisabled())throw Error('Unsaved script can be bound');
 await page.locator('.jp-script-code .monaco-editor').click({position:{x:120,y:80}});await page.keyboard.press('Control+End');await page.keyboard.insertText('\nprint("prototype-script-edit")\n');
 await page.locator('.jp-script-filebar').getByRole('button',{name:'保存',exact:true}).click();await page.getByRole('button',{name:'在 Flow 中使用',exact:true}).click();await dialog().getByLabel('引用方式',{exact:true}).selectOption('inline');await dialog().getByRole('button',{name:'确认关联',exact:true}).click();
 const inline=await flowDocument();const task=inline.tasks.find(t=>t.id==='run_new_gpu_check');if(task?.type!=='io.kestra.plugin.scripts.python.Script'||!task.script.includes('prototype-script-edit')||task.namespaceFiles)throw Error('Inline task did not preserve saved script');checks.push('New Python script, real Monaco keyboard edit, save, and inline task binding');
 await page.getByRole('button',{name:'Revisions',exact:true}).click();await page.getByRole('button',{name:'Builder',exact:true}).click();await focusPanel('files','Files / 脚本');await page.locator('.jp-script-tree>button').filter({hasText:'new_gpu_check.py'}).click();if(!(await page.locator('.jp-script-filebar').innerText()).includes('已保存到原型'))throw Error('Saved file lost when switching Flow tabs');checks.push('Saved files survive Flow context-tab unmount/remount');
 const chooserPromise=page.waitForEvent('filechooser');await toolbar().getByRole('button',{name:'导入脚本',exact:true}).click();const chooser=await chooserPromise;await chooser.setFiles([{name:'uploaded.sh',mimeType:'text/plain',buffer:Buffer.from('#!/usr/bin/env bash\nprintf "synthetic import\\n"\n')},{name:'uploaded.py',mimeType:'text/plain',buffer:Buffer.from('print("synthetic import")\n')}]);
 await dialog().getByRole('button',{name:'导入并编辑',exact:true}).click();await page.locator('.jp-script-tree>button').filter({hasText:'uploaded.sh'}).waitFor();await page.locator('.jp-script-tree>button').filter({hasText:'uploaded.py'}).waitFor();
 await page.locator('.jp-script-filebar').getByRole('button',{name:'保存',exact:true}).click();await page.getByRole('button',{name:'在 Flow 中使用',exact:true}).click();await dialog().getByLabel('引用方式',{exact:true}).selectOption('namespace');await dialog().getByLabel('目标任务',{exact:true}).selectOption('new');await dialog().getByRole('button',{name:'确认关联',exact:true}).click();
 const bound=await flowDocument();const command=bound.tasks.find(t=>t.id==='run_uploaded');if(command?.type!=='io.kestra.plugin.scripts.shell.Commands'||command.namespaceFiles?.include?.[0]!=='scripts/uploaded.sh'||command.commands?.[0]!=="bash 'scripts/uploaded.sh'")throw Error('Namespace binding invalid');checks.push('Multi-file import retains both files; saved Shell file binds via namespaceFiles/commands');
 await focusPanel('files','Files / 脚本');await page.locator('.jp-script-tree>button').filter({hasText:'new_gpu_check.py'}).click();await page.locator('.kel-splitter-panel').filter({has:page.locator('.jp-script-files')}).last().getByRole('button',{name:'最大化面板',exact:true}).click();await page.waitForTimeout(250);await page.screenshot({path:path.join(dir,'screenshots/scripts-files.png')});
 const oversized=await page.locator('svg').evaluateAll(items=>items.map(n=>n.getBoundingClientRect()).filter(r=>r.width>40&&r.height>40&&r.width&&r.height));if(oversized.length)throw Error('Oversized icon appeared in script workspace');
 if(errors.length||apiRequests.length)throw Error(JSON.stringify({errors,apiRequests}));checks.push('No oversized icons, page errors, business API or device calls');
 const result={status:'PASS',checks,errors,apiRequests,scope:'Synthetic standalone script workflow only; files in memory, no real Namespace persistence or execution'};fs.writeFileSync(path.join(dir,'scripts-validation.json'),JSON.stringify(result,null,2)+'\n');console.log(JSON.stringify(result,null,2));
}finally{await browser.close()}
