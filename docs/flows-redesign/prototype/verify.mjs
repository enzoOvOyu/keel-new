import {chromium} from 'playwright';
import fs from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
const dir=path.dirname(fileURLToPath(import.meta.url));
const screenshots=path.join(dir,'screenshots');fs.mkdirSync(screenshots,{recursive:true});
const browser=await chromium.launch({headless:true});
const page=await browser.newPage({viewport:{width:1600,height:1000},deviceScaleFactor:1});
const errors=[],checks=[],apiRequests=[];
page.on('pageerror',e=>errors.push(e.message));
page.on('request',r=>{if(new URL(r.url()).pathname.startsWith('/api/'))apiRequests.push(r.url())});
const base='http://127.0.0.1:5181/jobs-preview/index.html';
async function shot(name){await page.waitForTimeout(250);await page.screenshot({path:path.join(screenshots,name+'.png'),fullPage:false})}
try{
 await page.goto(base+'#flows');await page.locator('.jp-catalog .kel-table').waitFor();
 await shot('01-flows');
 const search=page.getByPlaceholder('搜索 Flow 名称、ID 或 Namespace');await search.fill('NCCL');await page.getByRole('button',{name:'NCCL 集群验证',exact:true}).waitFor();
 if(await page.locator('.jp-catalog .jp-row-title').count()!==1)throw Error('Flow filter mismatch');checks.push('Flow search filters independent fixture');await search.fill('');
 await page.getByRole('button',{name:'GPU 交付前置检查',exact:true}).click();await page.locator('.jp-editor .leaf-block-card').first().waitFor();await page.locator('.jp-topology-node').first().waitFor();
 await shot('02-builder');
 const message=page.getByRole('textbox',{name:'Task message'});await message.fill('Prototype property edit');await message.blur();
 await page.getByText('YAML',{exact:true}).first().click();await page.locator('.monaco-editor').first().waitFor({timeout:45000});checks.push('Native multi-panel and Monaco mount; selected task properties editable');
 await page.goto(base+'#execution');await page.locator('.jp-gantt-row').first().waitFor();await shot('03-execution');
 await page.locator('.jp-gantt-row').first().click();if(!(await page.locator('.jp-task-detail h3').innerText()).includes('resolve_targets'))throw Error('Task selection did not update inspector');checks.push('Gantt selection updates task detail');
 await page.getByRole('button',{name:'Targets',exact:true}).click();await page.locator('.jp-target-matrix>button').first().waitFor();await page.locator('.jp-target-matrix>button.failed').first().click();await shot('04-targets');checks.push('Target matrix selects failed device');
 await page.getByRole('button',{name:'Logs',exact:true}).click();await page.locator('.jp-logs-view select').first().selectOption('ERROR');if(await page.locator('.jp-log-line.expanded').count()!==2)throw Error('Log level filter mismatch');checks.push('Logs filter by severity');
 await page.getByRole('button',{name:'Variables & Outputs',exact:true}).click();await page.locator('.jp-variable-json').waitFor();await shot('05-outputs');checks.push('Native variable sidebar with selected JSON');
 await page.goto(base+'#flows');await page.getByRole('button',{name:'Quick Run',exact:true}).click();await page.getByRole('button',{name:'下一步',exact:true}).click();await page.getByRole('textbox',{name:'Quick Run script'}).fill('# Synthetic preview only\necho preview');await page.getByRole('button',{name:'下一步',exact:true}).click();await shot('06-run-confirmation');await page.getByRole('button',{name:'启动演示执行',exact:true}).click();await page.locator('.jp-detail-heading').waitFor();if(!await page.getByRole('button',{name:'暂停',exact:true}).count())throw Error('Run did not create running preview');
 await page.getByRole('button',{name:'暂停',exact:true}).click();await page.getByRole('button',{name:'确认暂停执行',exact:true}).click();await page.getByRole('button',{name:'继续',exact:true}).waitFor();checks.push('Quick Run wizard creates isolated execution; pause updates visible lifecycle');
 await page.goto(base+'#agents');await page.locator('.jp-main .kel-table').waitFor();await page.locator('.jp-main .jp-row-title').first().click();await page.locator('.jp-agent-drawer').waitFor();await shot('07-agents');checks.push('Agent row opens scoped detail drawer');await page.getByRole('button',{name:'关闭Agent详情',exact:true}).click();
 await page.getByRole('button',{name:'切换主题',exact:true}).click();await page.goto(base+'#flows');await page.locator('.jp-catalog .jp-row-title').first().waitFor();await shot('08-light');checks.push('Light/dark theme switch');await page.getByRole('button',{name:'切换主题',exact:true}).click();await page.setViewportSize({width:1440,height:900});await page.locator('.jp-catalog .jp-row-title').first().waitFor();await shot('09-flows-1440');
 if(apiRequests.length)throw Error('Prototype unexpectedly requested a business API');
 if(errors.length)throw Error(errors.join('\n'));
 console.log(JSON.stringify({status:'PASS',checks,errors,apiRequests},null,2));
 fs.writeFileSync(path.join(dir,'validation.json'),JSON.stringify({status:'PASS',checks,errors,apiRequests,scope:'Synthetic design prototype only; no real API, database, device execution or production UI verification'},null,2)+'\n');
}finally{await browser.close()}
