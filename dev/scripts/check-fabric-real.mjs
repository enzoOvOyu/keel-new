// Opt-in real API verification: Fabric metadata writes only; no CMDB or device writes.
import {request,chromium} from 'playwright';
import assert from 'node:assert/strict';
import fs from 'node:fs';
const base=process.env.FABRIC_CHECK_URL||'http://127.0.0.1:5197';
const project=process.env.FABRIC_CHECK_PROJECT;
if(!project||!process.env.KEEL_BOOTSTRAP_PASSWORD)throw Error('Explicit project and private login configuration required');
const output=process.env.FABRIC_CHECK_OUTPUT||'/tmp/keel-fabric-real-check';fs.mkdirSync(output,{recursive:true,mode:0o700});
const api=await request.newContext({baseURL:base});
const auth=await api.post('/api/v1/auth/login',{data:{username:process.env.FABRIC_CHECK_USER||'admin',password:process.env.KEEL_BOOTSTRAP_PASSWORD}});assert.equal(auth.status(),200,'real authentication');
const session=(await auth.json()).data;const root='/api/v1/projects/'+encodeURIComponent(project)+'/fabric';const headers={'X-CSRF-Token':session.csrf_token};const checks=[];
const read=async()=>{const r=await api.get(root+'/workspace');assert.equal(r.status(),200);return (await r.json()).data};
let data=await read();assert(data.inputs.inputs.length,'existing published Kickoff required');
const latest=data.inputs.inputs[0];assert.equal(new Set(latest.assets.map(a=>a.id)).size,latest.assets.length,'deduplicated CMDB identities');checks.push('real published assets use unique stable IDs');
const command=async(action,values={},status=200)=>{const response=await api.post(root+'/commands',{headers:{...headers,'Idempotency-Key':crypto.randomUUID()},data:{action,expectedRevision:data.revision,...values}});assert.equal(response.status(),status,action+' status');const body=await response.json();if(status===200)data=body.data;return body};
if(data.state.inputId!==latest.id){
 const body={action:'adopt',expectedRevision:data.revision,id:latest.id},key=crypto.randomUUID();
 for(let i=0;i<2;i++){const response=await api.post(root+'/commands',{headers:{...headers,'Idempotency-Key':key},data:body});assert.equal(response.status(),200);data=(await response.json()).data}
 assert.equal(data.revision,body.expectedRevision+1);checks.push('adoption persisted; identical idempotency key increments once');
}
const conflict=await api.post(root+'/commands',{headers:{...headers,'Idempotency-Key':crypto.randomUUID()},data:{action:'commitBlueprint',expectedRevision:-1}});assert.equal(conflict.status(),409);checks.push('stale workspace version rejected');
if(data.state.blueprints.at(-1)?.status==='DRAFT')await command('commitBlueprint');
const target=latest.assets.find(a=>a.type==='switch');assert(target,'real switch source');
if(!data.state.builds.length)await command('compile',{domain:'SWITCH',ids:[target.id]});
if(!data.state.validations.length)await command('validate');
assert.equal(data.state.builds.at(-1).status,'INVALID','unsupported native renderer is not executable');assert.equal(data.state.validations.at(-1).status,'BLOCKED');
await command('createChange',{task:'unbound',strategy:'SERIAL',purpose:'verification',window:'verification-only'},422);checks.push('missing native capability/evidence blocks approval and execution');
const persisted=await read();assert.equal(persisted.revision,data.revision);assert.equal(persisted.state.blueprints.at(-1).id,data.state.blueprints.at(-1).id);checks.push('reload restores PostgreSQL workspace and reports');
const forbidden=await api.get('/api/v1/projects/not-authorized-project/fabric/workspace');assert.equal(forbidden.status(),403);checks.push('project access denied server-side');
const browser=await chromium.launch({headless:true});const context=await browser.newContext({viewport:{width:1600,height:1000},storageState:await api.storageState()});const page=await context.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));
try{
 for(const view of ['overview','blueprint','onboarding','configuration','validation','deployments','assurance','settings']){
  await page.goto(base+'/p/'+project+'/deploy/fabric/'+view);await page.locator('.fabric-workspace .page-heading').waitFor({timeout:60000});
  if(view==='configuration'){await page.getByRole('button',{name:'生成设备配置',exact:true}).first().click();await page.getByRole('dialog').waitFor();assert.equal(await page.getByText('此次模拟结果',{exact:true}).count(),0);await page.keyboard.press('Escape')}
  await page.screenshot({path:output+'/'+view+'.png'});
 }
 assert.equal(errors.length,0,errors.join('\n'));checks.push('8 real API pages render; configuration dialog opens; no page errors');
 const summary={checks,errors,assetCount:latest.assets.length,taskCount:data.tasks.length,networkPlans:latest.networkPlans.length,revision:data.revision,scope:'Real session/API/PostgreSQL. No mocked API, CMDB write, device execution or telemetry claim.'};fs.writeFileSync(output+'/result.json',JSON.stringify(summary,null,2));console.log(JSON.stringify(summary));
}finally{await browser.close();await api.dispose()}
