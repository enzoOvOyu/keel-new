// Targeted prototype UI audit. No business API, login, database or device calls.
import {chromium} from 'playwright';
import fs from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
const dir=path.dirname(fileURLToPath(import.meta.url));
const browser=await chromium.launch();
const page=await browser.newPage({viewport:{width:1440,height:900}});
const errors=[],apiRequests=[],checks=[],measurements=[];
page.on('pageerror',e=>errors.push(e.message));
page.on('request',r=>{if(new URL(r.url()).pathname.startsWith('/api/'))apiRequests.push(r.url())});
const base='http://127.0.0.1:5181/jobs-preview/index.html';
async function audit(label){
 await page.waitForTimeout(150);
 const result=await page.evaluate(()=>({
  oversized:[...document.querySelectorAll('svg')].filter(n=>!n.closest('.empty-state')).map(n=>{const r=n.getBoundingClientRect();return {width:r.width,height:r.height,html:n.parentElement.outerHTML.slice(0,160),visible:r.bottom>0&&r.top<innerHeight&&r.right>0&&r.left<innerWidth}}).filter(n=>n.visible&&(n.width>40||n.height>40)),
  overflow:document.documentElement.scrollWidth>innerWidth+2,
  carets:[...document.querySelectorAll('.kel-icon svg')].map(n=>({width:n.getBoundingClientRect().width,height:n.getBoundingClientRect().height})),
 }));
 measurements.push({label,...result});
 if(result.oversized.length)throw Error(label+': oversized control icon '+JSON.stringify(result.oversized));
 if(result.overflow)throw Error(label+': document overflows horizontally');
}
async function route(hash){await page.goto(base+'#'+hash);await page.locator('.jp-main').waitFor()}
try{
 await route('flows');await page.locator('.jp-catalog .jp-row-title').first().waitFor();
 if(await page.locator('.jp-job-nav>button').count()!==3||await page.locator('.jp-sidebar button:disabled').count())throw Error('Sidebar has unrelated navigation');
 checks.push('Sidebar only contains Flows, Executions and Agents');
 await page.getByLabel('项目切换',{exact:true}).selectOption('所有授权项目');
 await page.locator('.jp-catalog .kel-pagination').waitFor();await audit('flows with pagination');
 await page.locator('.jp-catalog .btn-next').click();await page.getByRole('button',{name:'POD-B 交付前置检查',exact:true}).waitFor();
 if(await page.locator('.jp-catalog .jp-row-title').count()!==1)throw Error('Next page did not display correct slice');
 await page.locator('.jp-catalog .btn-prev').click();await page.getByRole('button',{name:'GPU 交付前置检查',exact:true}).waitFor();checks.push('Flow previous/next changes page and rows');
 await page.locator('.jp-catalog .kel-checkbox').nth(1).click();await page.locator('.jp-selection').waitFor();await page.getByRole('button',{name:'停用所选',exact:true}).click();await audit('batch selection');checks.push('Native checkbox and batch action are usable');
 const search=page.getByPlaceholder('搜索 Flow 名称、ID 或 Namespace');await search.fill('no-match-for-ui-audit');await page.locator('.jp-catalog .empty-state').waitFor();await audit('empty result');await search.fill('');
 await page.screenshot({path:path.join(dir,'screenshots/ui-fix-flows.png')});
 await route('editor/gpu-preflight');await page.locator('.jp-editor .leaf-block-card').first().waitFor();await audit('editor');
 await page.getByText('YAML',{exact:true}).first().click();await page.locator('.monaco-editor').first().waitFor({timeout:45000});await audit('Monaco panel');
 for(const panel of ['Files / 脚本','Docs']){await page.getByText(panel,{exact:true}).first().click();await audit(panel+' panel')}
 await page.getByRole('button',{name:'Revisions',exact:true}).click();await page.locator('.jp-revisions').waitFor();await audit('revisions');
 await page.getByRole('button',{name:'Triggers',exact:true}).click();await page.getByRole('button',{name:'添加 Schedule',exact:true}).click();await page.locator('.kel-dialog:visible').waitFor();await audit('schedule dialog');await page.locator('.kel-dialog:visible .kel-dialog__headerbtn').click();
 checks.push('Editor, Monaco, Files, Docs, Revisions and Schedule dialog checked');
 await route('execution');await page.locator('.jp-gantt-row').first().waitFor();await audit('execution Gantt');
 await page.getByRole('button',{name:'Topology',exact:true}).click();await audit('execution topology');
 await page.getByRole('button',{name:'Targets',exact:true}).click();await page.getByRole('button',{name:'列表',exact:true}).click();await page.locator('.jp-targets-view .kel-pagination').waitFor();
 await page.locator('.jp-targets-view .btn-next').click();await page.locator('.jp-targets-view .kel-table__body').getByText('gpu-a11',{exact:true}).waitFor();
 await page.locator('.jp-targets-view .kel-pagination__sizes .kel-select').click();await page.getByRole('option',{name:'25 per page',exact:true}).waitFor();await audit('teleported page-size menu');
 await page.getByRole('option',{name:'25 per page',exact:true}).click();await page.locator('.jp-targets-view .kel-table__body').getByText('gpu-a25',{exact:true}).waitFor();await audit('target page size');
 await page.screenshot({path:path.join(dir,'screenshots/ui-fix-pagination.png')});checks.push('Target table next page, size selection, and page reset work');
 for(const tab of ['Logs','Variables & Outputs','Audit / Context']){await page.getByRole('button',{name:tab,exact:true}).click();await audit('execution '+tab)}
 await route('agents');await page.locator('.jp-main .jp-row-title').first().waitFor();await page.locator('.jp-main .btn-next').click();await page.getByRole('button',{name:'gpu-a11',exact:true}).waitFor();
 if(await page.locator('.jp-main .jp-row-title').count()!==2)throw Error('Agent pagination did not change rows');
 await page.locator('.jp-main .jp-row-title').first().click();await page.locator('.jp-agent-drawer').waitFor();await audit('Agent drawer');await page.getByRole('button',{name:'关闭Agent详情',exact:true}).click();checks.push('Agent pagination and drawer checked');
 await route('flows');await page.getByRole('button',{name:'Quick Run',exact:true}).click();await page.locator('.kel-dialog:visible').waitFor();await audit('Quick Run targets');
 await page.getByRole('button',{name:'下一步',exact:true}).click();await audit('Quick Run script');await page.getByRole('button',{name:'下一步',exact:true}).click();await audit('Quick Run confirmation');await page.locator('.kel-dialog:visible .kel-dialog__headerbtn').click();
 checks.push('Quick Run three steps and dialog close checked');
 await page.getByRole('button',{name:'切换主题',exact:true}).click();await page.getByLabel('项目切换',{exact:true}).selectOption('所有授权项目');await page.locator('.jp-catalog .kel-pagination').waitFor();await audit('light theme pagination');
 await page.screenshot({path:path.join(dir,'screenshots/ui-fix-light.png')});checks.push('Light/dark primitive styles stay scoped to standalone preview');
 if(errors.length||apiRequests.length)throw Error(JSON.stringify({errors,apiRequests}));
 fs.writeFileSync(path.join(dir,'ui-audit.json'),JSON.stringify({status:'PASS',checks,measurements,errors,apiRequests,scope:'Standalone synthetic preview; no production UI or backend tests'},null,2)+'\n');
 console.log(JSON.stringify({status:'PASS',checks,scannedViews:measurements.length,errors,apiRequests},null,2));
}finally{await browser.close()}
