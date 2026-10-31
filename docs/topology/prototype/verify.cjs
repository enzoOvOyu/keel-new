// Isolated design review only: no login, model calls, database or business API.
const {chromium}=require('./node_modules/playwright');
const fs=require('node:fs');const path=require('node:path');
(async()=>{
 const b=await chromium.launch({headless:true,args:['--no-sandbox']});const p=await b.newPage({viewport:{width:1440,height:900}});
 const errors=[],requests=[],checks=[];p.on('pageerror',e=>errors.push(e.message));p.on('request',r=>{if(new URL(r.url()).pathname.startsWith('/api/'))requests.push(r.url())});
 const out=path.join(__dirname,'screenshots');fs.mkdirSync(out,{recursive:true});
 const base='http://127.0.0.1:5181';
 async function visit(url){await p.goto(base+'/'+url);await p.getByRole('heading',{level:1}).waitFor();}
 async function shot(name){await p.screenshot({path:path.join(out,name+'.png'),fullPage:false});}
 async function check(fn,label){for(let i=0;i<20;i++){if(await fn()){checks.push(label);return}await p.waitForTimeout(100)}throw Error(label)}
 await visit('global-topology');await p.locator('canvas').first().waitFor();await p.waitForTimeout(1800);await shot('global-topology');
 await p.getByPlaceholder('搜索服务器、交换机或 SN').fill('b300-001');await p.locator('.search-results button').first().click();await p.getByRole('heading',{name:'b300-001',exact:true}).waitFor();await check(async()=>(await p.locator('.inspector').innerText()).includes('b300-001'),'全局搜索定位资产');
 await p.getByRole('button',{name:'定位所属机柜'}).click();await p.waitForURL('**/rack-detail?**');await check(async()=>p.url().includes('rack=rack-0001'),'跨页面保留稳定机柜ID');
 await visit('network-architecture');await p.locator('canvas').first().waitFor();await p.waitForTimeout(1500);await shot('network-architecture');await p.getByRole('tab',{name:'连接矩阵',exact:true}).click();await p.locator('.matrix button.filled').first().click();await check(async()=>await p.locator('.table-box tbody tr').count()>0,'Spine/Leaf矩阵下钻真实成员');
 await p.getByRole('tab',{name:'交换机目录'}).click();await p.locator('td .text-link').first().click();await check(async()=>(await p.locator('.inspector').innerText()).includes('交换机详情'),'交换机列表详情联动');
 await visit('physical-cabling?mode=Port+Panel&asset=sw-leaf-0001');await p.getByRole('button',{name:'端口 swp1',exact:true}).click();await check(async()=>(await p.locator('.inspector').innerText()).includes('sw-leaf-0001-port-01'),'端口面板稳定ID选择');await shot('physical-cabling');
 await p.getByRole('tab',{name:'Cable Inventory'}).click();await p.getByLabel('证据筛选').selectOption('incomplete');await check(async()=>await p.locator('.table-box tbody tr').count()===1,'缺端记录保留且可筛选');await p.getByRole('button',{name:'cable-srv-0001-04',exact:true}).click();await check(async()=>(await p.locator('.inspector').innerText()).includes('端点缺失'),'Cable详情显示断点');
 await p.getByRole('button',{name:'追踪 A 端'}).click();await p.waitForURL('**/link-tracing?**');await check(async()=>p.url().includes('link-tracing'),'线缆跳转物理追踪');await check(async()=>(await p.locator('.trace-path').innerText()).includes('未登记 B 端'),'缺端路径不补造终点');
 await visit('link-tracing?port=srv-0001-nic-01-p1');await p.getByRole('button',{name:'开始追踪'}).click();await check(async()=>await p.locator('.path-step').count()===3,'端口→线缆→端口三段展示');await shot('link-tracing');
 await p.getByLabel('追踪模式').selectOption('Logical Trace');await check(async()=>await p.getByRole('button',{name:'开始追踪'}).isDisabled(),'缺FIB证据禁止逻辑追踪');await check(async()=>(await p.locator('.empty-state').first().innerText()).includes('FIB'),'逻辑能力缺口可见');
 await visit('rack-detail?rack=rack-0001&asset=srv-0001');await p.getByRole('tab',{name:'back',exact:true}).click();await check(async()=>await p.locator('svg[aria-label*="背面"]').count()===1,'机柜背面同一身份');await shot('rack-detail');
 await p.getByRole('button',{name:'编辑草稿',exact:true}).click();const server=p.getByRole('button',{name:'b300-001',exact:true});await server.focus();await server.press('ArrowDown');await check(async()=>await p.getByRole('button',{name:'保存展示草稿',exact:true}).isDisabled(),'U位重叠阻止保存');await server.press('ArrowUp');await p.waitForTimeout(80);await server.press('ArrowUp');
 await p.getByRole('button',{name:'保存展示草稿',exact:true}).click();await p.getByRole('button',{name:'确认保存展示草稿',exact:true}).click();await check(async()=>await p.evaluate(()=>JSON.parse(localStorage.getItem('keel.topology.prototype.rack.rack-0001'))['srv-0001']===34),'U位草稿仅本地保存');await p.getByRole('button',{name:'撤销',exact:true}).click();await p.waitForTimeout(80);await p.getByRole('button',{name:'撤销',exact:true}).click();await p.waitForTimeout(80);await p.getByRole('button',{name:'撤销',exact:true}).click();await check(async()=>await p.getByRole('button',{name:'保存展示草稿',exact:true}).isDisabled(),'撤销恢复原始布局');

 await p.getByRole('tab',{name:'Mapping',exact:true}).click();await check(async()=>await p.locator('.table-box tbody tr').count()>0,'Mapping保留设备关联');await p.reload();await check(async()=>p.url().includes('asset=srv-0001')&&p.url().includes('mode=Mapping'),'刷新恢复Mapping上下文');
 await p.getByRole('tab',{name:'电源空间'}).click();await check(async()=>(await p.locator('.rack-info').innerText()).includes('未采集 PDU'),'PDU缺测不虚构');
 await p.getByRole('button',{name:'保存视图',exact:true}).click();await visit('global-topology');await p.getByRole('button',{name:'恢复视图',exact:true}).click();await p.waitForURL('**/rack-detail?**');await check(async()=>p.url().includes('rack-detail'),'保存/恢复本地视图');
 await p.setViewportSize({width:1280,height:900});await visit('physical-cabling?mode=Cable+Inventory');await shot('physical-cabling-1280');await check(async()=>await p.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'1280页面无整体横向溢出');
 await check(async()=>errors.length===0,'浏览器无未捕获异常');await check(async()=>requests.length===0,'无业务API请求');
 fs.writeFileSync(path.join(__dirname,'validation.json'),JSON.stringify({at:new Date().toISOString(),viewport:[1440,900],secondary:[1280,900],checks,errors,businessRequests:requests},null,2));
 console.log(JSON.stringify({checks,errors,businessRequests:requests},null,2));await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
