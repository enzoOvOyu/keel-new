import assert from 'node:assert/strict';
import {readFileSync,writeFileSync,mkdirSync} from 'node:fs';
import {fileURLToPath} from 'node:url';
import ts from '../topology/prototype/node_modules/typescript/lib/typescript.js';
import {chromium} from '../topology/prototype/node_modules/playwright/index.mjs';
const root=fileURLToPath(new URL('.',import.meta.url));
const results=[];function pass(name){results.push({name,status:'PASS'});console.log('PASS',name)}
const source=readFileSync(root+'../topology/prototype/rack/geometry.ts','utf8');
const js=ts.transpileModule(source,{compilerOptions:{target:ts.ScriptTarget.ES2022,module:ts.ModuleKind.ES2022}}).outputText;
const {layoutRack,unitY}=await import('data:text/javascript;base64,'+Buffer.from(js).toString('base64'));
const item=(id,u,height,face='front',extra={})=>({id,u,height,face,fullDepth:null,depth:null,...extra});
for(const h of [42,45,48]){
  const g=layoutRack(h,[item('half',1,.5),item('top',h,1),item('pdu',null,0)]);
  assert.equal(g.occupied,1.5);assert.equal(g.zeroU.length,1);assert.equal(g.issues.length,0);assert.equal(unitY(h,h,1,15,40),40);
}
assert.equal(layoutRack(42,[item('bad',42,2)]).issues[0].kind,'range');
assert.equal(layoutRack(42,[item('a',1,2),item('b',2,1)]).issues[0].kind,'overlap');
assert.equal(layoutRack(42,[item('a',1,2,'unknown'),item('b',1,2,'rear')]).issues[0].kind,'uncertain');
assert.equal(layoutRack(42,[item('a',1,2,'front',{fullDepth:true}),item('b',1,2,'rear')]).issues[0].kind,'overlap');
const shallow=layoutRack(42,[item('a',1,2,'front',{fullDepth:false,depth:200}),item('b',1,2,'rear',{fullDepth:false,depth:200})],600);
assert.equal(shallow.issues.length,0);assert.equal(shallow.occupied,2);
pass('几何：42/45/48U、0.5U、0U、越界、同面/全深度冲突、未知面与双侧共用并集');
const browser=await chromium.launch({headless:true,args:['--no-sandbox']});
const page=await browser.newPage({viewport:{width:1600,height:1040}});const errors=[],requests=[];
page.on('pageerror',e=>errors.push(e.message));page.on('request',r=>{if(new URL(r.url()).pathname.startsWith('/api/'))requests.push(r.url())});
const base='http://localhost:5181/rack-detail';const shot=async name=>page.screenshot({path:root+'screenshots/'+name+'.png',fullPage:true});
try{
  await page.goto(base);await page.locator('.rv-rack-card').first().waitFor();assert.equal(await page.locator('.rv-rack-card').count(),16);assert.match(await page.locator('.rv-summary').innerText(),/430 \/ 672/);await shot('gallery');
  await page.getByLabel('Pod 筛选').selectOption('demo-pod-1');await page.locator('.rv-rack-card').nth(4).waitFor({state:'detached'});assert.equal(await page.locator('.rv-rack-card').count(),4);
  await page.getByLabel('Pod 筛选').selectOption('');await page.getByPlaceholder('搜索机柜、设备、型号、SN').fill('srv-0064');await page.locator('.rv-rack-card').nth(1).waitFor({state:'detached'});assert.equal(await page.locator('.rv-rack-card').count(),1);await page.locator('.rv-rack-card').click();await page.locator('.rv-dual').waitFor();assert.equal(await page.locator('.rv-svg-device.is-selected').count(),2);
  pass('Gallery 16 柜、范围数量、Pod 筛选、设备搜索定位与双面同步选择');
  await page.goto(base+'?view=dual&rack=rack-0001');await page.locator('.rv-dual').waitFor();
  const geometry=await page.locator('.rv-dual .rv-elevation').evaluateAll(svgs=>svgs.map(s=>[...s.querySelectorAll('[data-device]')].map(d=>[d.getAttribute('data-device'),d.getAttribute('data-u'),d.getAttribute('data-height')])));
  assert.deepEqual(geometry[0],geometry[1]);assert.equal(geometry[0].length,7);assert.match(await page.locator('.rv-detail-summary').innerText(),/29.*13/s);
  await page.locator('.rv-dual [data-face="front"] [data-device="srv-0001"]').click();await page.locator('.rv-svg-device.is-selected').first().waitFor();assert.equal(await page.locator('.rv-svg-device.is-selected').count(),2);assert.match(await page.locator('.rv-inspector').innerText(),/未记录/);await shot('dual-asset');
  await page.getByRole('tab',{name:'计算资源',exact:true}).click();await page.locator('.overlay-compute').first().waitFor();assert.match(await page.locator('[data-face="front"] [data-device="srv-0001"]').textContent(),/8 GPU/);await shot('dual-compute');
  await page.getByRole('tab',{name:'空间',exact:true}).click();await page.locator('.rv-svg-empty').first().waitFor();assert.ok(await page.locator('.rv-svg-empty').count()>0);await shot('dual-capacity');
  await page.reload();await page.locator('.rv-dual').waitFor();assert.equal(await page.locator('.rv-svg-device.is-selected').count(),2);assert.equal(await page.getByRole('tab',{name:'空间',exact:true}).getAttribute('aria-selected'),'true');
  pass('双面同身份/U、29U 并集、13U 未占用、Compute/Capacity、URL 恢复');
  await page.getByRole('tab',{name:'Mapping',exact:true}).click();await page.locator('.rv-port-index button').first().click();await page.locator('.rv-connection-list>button').first().click();assert.doesNotMatch(await page.locator('.rv-port-index').innerText(),/GG/);await shot('mapping');await page.getByRole('button',{name:'追踪线缆'}).click();await page.waitForURL('**/link-tracing?**');assert.match(page.url(),/link-tracing.*cable=/);
  pass('Mapping：设备 → 登记端口 → 线缆 → 链路追踪（稳定 ID）');
  await page.goto(base+'?view=compare&rack=rack-0001');await page.locator('.rv-compare').waitFor();assert.equal(await page.getByRole('button',{name:'生成差异'}).isDisabled(),true);await shot('compare-prerequisites');await page.getByRole('button',{name:'查看数据依据'}).click();assert.match(await page.getByRole('dialog').innerText(),/88 \/ 92/);await page.getByRole('button',{name:'返回工作台'}).click();
  pass('规划/实装缺口禁用比较、能力审计数据可查看、无虚构实时覆盖层');
  await page.goto(base+'?view=dual&rack=rack-0001&asset=srv-0001');await page.getByRole('button',{name:'展示草稿',exact:true}).click();await page.getByRole('button',{name:'退出草稿'}).waitFor();const device=page.locator('.rv-dual [data-face="front"] [data-device="srv-0001"]');await device.focus();await device.press('ArrowDown');await page.getByRole('alert').waitFor();assert.ok(await page.getByRole('alert').count()>0);assert.equal(await page.getByRole('button',{name:'保存草稿',exact:true}).isDisabled(),true);await page.getByLabel('撤销草稿').click();await page.locator('[data-face="front"] [data-device="srv-0001"][data-u="33"]').waitFor();await device.focus();await device.press('ArrowUp');await page.locator('[data-face="front"] [data-device="srv-0001"][data-u="33.5"]').waitFor();assert.equal(await device.getAttribute('data-u'),'33.5');await page.getByRole('button',{name:'保存草稿',exact:true}).click();await page.getByRole('button',{name:'确认保存展示草稿'}).click();await page.getByRole('button',{name:'退出草稿'}).click();await page.locator('[data-face="front"] [data-device="srv-0001"][data-u="33"]').waitFor();assert.equal(await device.getAttribute('data-u'),'33');
  pass('草稿 0.5U 键盘移动、冲突阻断保存、撤销、显式保存、退出恢复事实');
  const downloadPromise=page.waitForEvent('download');await page.getByRole('button',{name:'SVG',exact:true}).click();const download=await downloadPromise;const path=await download.path();const svg=readFileSync(path,'utf8');assert.match(svg,/<svg/);assert.match(svg,/FRONT/);assert.match(svg,/REAR/);assert.match(svg,/SYNTHETIC/);
  pass('双面 SVG 导出包含数据性质与两面');
  await page.setViewportSize({width:1440,height:960});await page.goto(base+'?view=dual&rack=rack-0001&asset=srv-0001');await page.locator('.rv-inspector').waitFor();await shot('dual-1440');assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth),true);
  assert.deepEqual(errors,[]);assert.deepEqual(requests,[]);pass('1440px 无横向溢出；浏览器无异常；0 条业务 API 请求');
  writeFileSync(root+'validation.json',JSON.stringify({at:new Date().toISOString(),scope:'isolated rack prototype only',viewport:[1600,1040],results,page_errors:errors,business_api_requests:requests},null,2)+'\n');
}finally{await browser.close()}
