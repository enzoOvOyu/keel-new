// V3 design prototype only. No business API, database, model calls or rack retesting.
const {chromium}=require('./node_modules/playwright');
const fs=require('node:fs');const path=require('node:path');
(async()=>{
 const b=await chromium.launch({headless:true,args:['--no-sandbox']});
 const p=await b.newPage({viewport:{width:1440,height:900}});
 p.setDefaultTimeout(7000);
 const errors=[],requests=[],checks=[];
 p.on('pageerror',e=>errors.push(e.message));
 p.on('request',r=>{if(new URL(r.url()).pathname.startsWith('/api/'))requests.push(r.url())});
 const out=path.join(__dirname,'screenshots-v3');fs.mkdirSync(out,{recursive:true});
 async function visit(url){await p.goto('http://127.0.0.1:5181/'+url);await p.locator('.viz-v3').waitFor();}
 async function shot(name){await p.screenshot({path:path.join(out,name+'.png'),fullPage:false});}
 async function check(fn,label){for(let i=0;i<20;i++){if(await fn()){checks.push(label);return}await p.waitForTimeout(100)}throw Error(label)}
 await visit('global-topology');await check(async()=>await p.locator('.cluster-container').count()===4,'全局4个Pod容器含局部矩阵');await shot('global-topology');
 await p.getByRole('button',{name:'展开 Pod 1',exact:true}).click();await check(async()=>await p.locator('.v-matrix tbody tr').count()===16,'Pod展开16台服务器，非全量端口图');
 await p.getByPlaceholder('搜索服务器、交换机或 SN').fill('b300-001');await p.locator('.v-search-results button').first().click();await check(async()=>p.url().includes('asset=srv-0001')&&(await p.locator('.inspector').innerText()).includes('b300-001'),'搜索定位稳定资产ID');await p.reload();await check(async()=>await p.locator('.inspector').count()===1,'刷新恢复资产与范围');await p.keyboard.press('Escape');await check(async()=>await p.locator('.inspector').count()===0,'Escape关闭证据栏');
 await visit('network-architecture');await shot('network-architecture');await check(async()=>await p.locator('.flow-hit').count()===20,'175条完整记录聚合成20束');
 await p.getByRole('button',{name:'leaf-01 到 RACK-01 39 条',exact:true}).click();await p.getByRole('button',{name:'展开成员',exact:true}).click();await check(async()=>await p.locator('.flow-strand').count()===24,'束展开限制24线，未丢弃39条成员');await shot('network-selection');
 await p.getByRole('tab',{name:'Bipartite',exact:true}).click();await check(async()=>await p.locator('.bipartite-row').count()===12,'二分图按真实Cable配对并分页12条');await shot('bipartite');
 await p.locator('.v-inline-control').getByRole('button',{name:'下一组'}).click();await check(async()=>await p.locator('.v-inline-control').innerText().then(x=>x.includes('2/4')),'二分图后续成员可查');
 await visit('physical-cabling');await shot('physical-cabling');await check(async()=>await p.getByRole('tab',{name:'Connection Matrix'}).getAttribute('aria-selected')==='true','物理布线默认矩阵');
 await p.getByRole('button',{name:'b300-001 × leaf-01：9 条记录',exact:true}).click();await check(async()=>p.url().includes('row=srv-0001')&&p.url().includes('col=sw-leaf-0001'),'矩阵单元格绑定精确设备ID');await check(async()=>(await p.locator('.v-ledger .v-figure-head').innerText()).includes('9 条'),'矩阵9条成员，不把缺端第10条伪装完整');
 await p.getByRole('button',{name:'展开端口关系'}).click();await check(async()=>await p.locator('.bio-cable').count()===9&&await p.locator('.bio-terminal').count()===18,'BioFabric每列一个Cable，每条完整记录恰好两个端点');await shot('biofabric');
 await p.getByRole('button',{name:'线缆列 cable-srv-0001-01',exact:true}).click();await check(async()=>(await p.locator('.inspector').innerText()).includes('NIC1/P1')&&(await p.locator('.inspector').innerText()).includes('swp1'),'BioFabric选择对应真实双端');
 await p.reload();await check(async()=>p.url().includes('mode=BioFabric')&&await p.locator('.inspector').count()===1,'BioFabric及线缆选择刷新恢复');
 await visit('physical-cabling');await p.locator('.v-incomplete-tray button').click();await check(async()=>(await p.locator('.inspector').innerText()).includes('端点缺失'),'矩阵排除的缺端记录仍有入口');
 await p.getByRole('button',{name:'追踪 A 端'}).click();await p.waitForURL('**/link-tracing?**');await check(async()=>await p.locator('.metro-station.missing').count()===1,'缺端追踪显示断点，不补造终点');await shot('trace-breakpoint');
 await visit('link-tracing');await shot('link-tracing');await check(async()=>await p.locator('.metro-station').count()===3&&await p.locator('.tree-branch').count()===10,'Metro物理段与独立端接树同时显示');
 await p.locator('.tree-branch').filter({hasText:'NIC4/P1'}).click();await check(async()=>p.url().includes('port=srv-0001-nic-04-p1')&&await p.locator('.metro-station.missing').count()===1,'点击关联支路重新追踪该端口');
 await p.getByLabel('追踪模式').selectOption('Logical Trace');await check(async()=>await p.getByRole('button',{name:'开始追踪'}).isDisabled()&&(await p.locator('.v-capability').innerText()).includes('FIB'),'逻辑证据缺失，不启用虚假追踪');
 await p.setViewportSize({width:1280,height:900});await visit('physical-cabling');await shot('physical-cabling-1280');await check(async()=>await p.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'1280宽度无整体横向溢出');
 await check(async()=>errors.length===0,'浏览器无未捕获异常');await check(async()=>requests.length===0,'无业务API请求');
 const result={at:new Date().toISOString(),scope:'four isolated design prototypes; rack unchanged',viewport:[1440,900],secondary:[1280,900],checks,errors,businessRequests:requests};
 fs.writeFileSync(path.join(__dirname,'validation-v3.json'),JSON.stringify(result,null,2));console.log(JSON.stringify(result,null,2));await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
