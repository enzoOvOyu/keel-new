import {chromium} from 'playwright';
import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import ts from 'typescript';
const root=path.resolve(import.meta.dirname,'../..');
const state=JSON.parse(await fs.readFile('/tmp/keel-kickoff-verified-workspace.json','utf8'));
const project=state.projectId;
const source=await fs.readFile(path.join(root,'dev/src/lib/api/kickoff/catalog.ts'),'utf8');
await fs.writeFile('/tmp/kickoff-ui-catalog.mjs',ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText);
const {tables}=await import('/tmp/kickoff-ui-catalog.mjs');
const registry=await fs.readFile(path.join(root,'dev/src/app/registry.ts'),'utf8');
const permissions=[...new Set([...registry.matchAll(/permission:'([^']+)'/g)].map(m=>m[1]).concat(['bom:read','bom:edit','bom:import','bom:review','bom:execute']))];
const session={actor:'kickoff-ui-check',user:{id:'kickoff-ui-check',username:'kickoff-ui-check',display_name:'检查'},role:'admin',project_id:project,csrf_token:'synthetic-ui-token',permissions,ai_configured:false,global_permissions:permissions,global_roles:['admin'],project_permissions:{},project_roles:{},grants:[],authorized_projects:[{id:project,name:state.projectName,code:'kickoff-check',status:'Active'}],permission_revision:1};
const base='http://127.0.0.1:5196';const browser=await chromium.launch({headless:true});const page=await browser.newPage({viewport:{width:1600,height:1050}});page.setDefaultTimeout(12000);page.setDefaultNavigationTimeout(60000);
const checked=[],errors=[];const out=path.join(root,'docs/kickoff/screenshots');await fs.mkdir(out,{recursive:true});let workspaceReads=0,edited=false;
page.on('pageerror',e=>errors.push(e.message));
await page.route('**/api/v1/**',async route=>{const url=new URL(route.request().url()),req=route.request();let data;
 if(url.pathname.endsWith('/auth/session'))data=session;
 else if(url.pathname.endsWith('/cmdb/inventory'))data={};
 else if(url.pathname.endsWith('/kickoff/workspace')){workspaceReads++;data=state;}
 else if(url.pathname.includes('/kickoff/models/')&&req.method()==='PUT'){const id=url.pathname.split('/').at(-1),body=req.postDataJSON();const previous=state.models.find(m=>m.tableId===id);const input={tableId:id,rows:body.rows,version:(previous?.version??0)+1,confirmed:true,updatedAt:new Date().toISOString(),source:body.source};state.models=state.models.filter(m=>m.tableId!==id).concat(input);state.revision++;data={version:input.version,revision:state.revision};edited=true;}
 else if(url.pathname.endsWith('/analyze')){return route.fulfill({status:503,json:{error:{code:'KICKOFF_AGENT_NOT_CONFIGURED'}}});}
 else {errors.push('unexpected API '+req.method()+' '+url.pathname);return route.fulfill({status:500,json:{error:{code:'UNEXPECTED_TEST_REQUEST'}}});}
 return route.fulfill({status:200,json:{data,request_id:'synthetic-ui-check'}});
});
const goto=async route=>{await page.goto(base+'/p/'+project+'/kickoff/'+route,{waitUntil:'domcontentloaded'});await page.locator('.kickoff-workspace h1').waitFor({timeout:45000});};
try {
 for(const route of ['overview','sources','review','publish','models/facility','models/network','models/provisioning','models/cabling','models/power']){await goto(route);assert.equal(await page.locator('.shell > aside.sidebar').count(),1);assert.equal(await page.locator('.kickoff-workspace .app-shell').count(),0);await page.screenshot({path:path.join(out,route.replace('/','-')+'.png'),fullPage:true});}
 checked.push('nine prototype pages integrated into one existing shell/router');
 // Compare content metrics against the untouched prototype; shell styling is intentionally owned by Keel.
 await goto('overview');const reference=await browser.newPage({viewport:{width:1600,height:1050}});await reference.goto('http://127.0.0.1:5195/p/proj-b300-demo/kickoff/overview',{waitUntil:'domcontentloaded'});await reference.locator('h1').waitFor();
 const metrics=async(p,selector)=>p.locator(selector).first().evaluate(e=>{const s=getComputedStyle(e);return Object.fromEntries(['fontSize','fontWeight','lineHeight','paddingTop','paddingRight','paddingBottom','paddingLeft','borderTopWidth','borderBottomWidth','borderRadius','boxShadow'].map(k=>[k,s[k]]));});
 const comparisons=[];for(const selector of ['h1','.stat','.button.primary','.heading']){const prototype=await metrics(reference,selector),integrated=await metrics(page,'.kickoff-workspace '+selector);assert.deepEqual(integrated,prototype,selector+' differs from prototype');comparisons.push({selector,prototype,integrated});}
 await goto('models/facility?table=rooms');await reference.goto('http://127.0.0.1:5195/p/proj-b300-demo/kickoff/models/facility?table=rooms',{waitUntil:'domcontentloaded'});await reference.locator('tbody td').first().waitFor();const prototypeTable=await metrics(reference,'tbody td'),integratedTable=await metrics(page,'.kickoff-workspace tbody td');assert.deepEqual(integratedTable,prototypeTable);comparisons.push({selector:'tbody td',prototype:prototypeTable,integrated:integratedTable});await reference.close();await fs.writeFile(path.join(root,'docs/kickoff/STYLE-CHECK.json'),JSON.stringify({status:'PASS',comparisons},null,2)+'\n');checked.push('prototype typography, stat strip, buttons and table density match');
 for(const table of tables){await goto('models/'+table.domain+'?table='+table.id);await page.getByRole('button',{name:'编辑输入',exact:true}).click();const dialog=page.getByRole('dialog');await dialog.waitFor();assert.ok((await dialog.innerText()).includes(table.name));await dialog.getByRole('button',{name:'取消',exact:true}).click();}
 checked.push('all 23 engineering input editors open and close');
 await goto('review');await page.locator('.kickoff-workspace tbody tr').first().click();await page.getByRole('dialog').waitFor();assert.equal(await page.locator('.kickoff-workspace .drawer').count(),1);await page.screenshot({path:path.join(out,'drawer.png'),fullPage:true});await page.getByRole('dialog').getByRole('button',{name:'关闭',exact:true}).first().click();checked.push('original evidence drawer opens inside scoped portal');
 await goto('models/facility?table=rooms');await page.getByRole('button',{name:'编辑输入',exact:true}).click();await page.getByLabel('机房名称 第1行').fill('Room UI verified');await page.getByRole('button',{name:'应用输入',exact:true}).click();await page.getByRole('dialog').waitFor({state:'hidden'});assert.ok(edited);await page.getByText('Room UI verified',{exact:true}).waitFor();checked.push('async input save waits for provider readback');
 await goto('models/cabling');await page.getByText('待重新计算',{exact:true}).waitFor({state:'hidden',timeout:1000});assert.ok((await page.locator('.calculation-section').innerText()).includes('确定性布线计算'));checked.push('persisted server calculation displayed without spurious stale state');
 await goto('sources');await page.getByRole('button',{name:'开始识别',exact:true}).click();await page.getByRole('alert').filter({hasText:'Agent 尚未接入'}).waitFor();checked.push('deferred Agent reports unavailable and preserves sources');
 await goto('overview');const before=workspaceReads;await page.goto(base+'/p/unauthorized/kickoff/overview');await page.getByText('403 · 无权访问此项目',{exact:true}).waitFor();assert.equal(workspaceReads,before);checked.push('ProjectContext denies unauthorized project before workspace request');
 await page.goto(base+'/projects');await page.getByRole('heading',{name:'部署项目',exact:true}).waitFor();checked.push('existing projects route remains available');
 session.global_permissions=permissions.filter(p=>!['bom:edit','bom:import','bom:review','bom:execute'].includes(p));session.permissions=session.global_permissions;session.permission_revision++;
 await goto('models/facility?table=rooms');assert.equal(await page.getByRole('button',{name:'编辑输入',exact:true}).isDisabled(),true);checked.push('read-only RBAC disables model mutations');
 assert.deepEqual(errors,[]);
 await fs.writeFile(path.join(root,'docs/kickoff/UI-CHECK.json'),JSON.stringify({status:'PASS',scope:'Main app UI with captured isolated-backend snapshot and transport replay; no live writes, no Agent',checked,errors},null,2)+'\n');console.log(JSON.stringify({status:'PASS',checked}));
} catch(e){await page.screenshot({path:path.join(out,'failure.png'),fullPage:true});await fs.writeFile(path.join(root,'docs/kickoff/UI-CHECK.json'),JSON.stringify({status:'FAIL',checked,errors,failure:String(e)},null,2)+'\n');throw e;} finally{await browser.close();}
