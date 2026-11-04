import {build} from 'esbuild';
import {mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {fileURLToPath,pathToFileURL} from 'node:url';
const temporary=await mkdtemp(tmpdir()+'/keel-revision-');
const output=temporary+'/provider.mjs';
import assert from 'node:assert/strict';
await build({entryPoints:[fileURLToPath(new URL('../src/lib/api/kickoff/http.ts',import.meta.url))],bundle:true,platform:'node',format:'esm',outfile:output});
const {HttpKickoffProvider}=await import(pathToFileURL(output).href);
const api=new HttpKickoffProvider('synthetic','synthetic');
let version=6,posts=0,reads=0;
const workspace=rev=>({projectId:'synthetic',revision:rev});
globalThis.fetch=async(url,options)=>{
 if(url.endsWith('/workspace'))return Response.json({data:workspace(version)});
 if(url.includes('/review?')){reads++;const rev=new URL(url,'http://local').searchParams.get('revision');return Number(rev)!==version?Response.json({error:{code:'REVISION_CONFLICT'}},{status:409}):Response.json({data:{revision:version,rows:[]}});}
 if(options.method==='POST'){posts++;assert.equal(JSON.parse(options.body).expectedRevision,version);return Response.json({data:{jobId:'adopt',state:'pending'}});}
 if(url.endsWith('/analysis/adopt'))return Response.json({data:{jobId:'adopt',state:'succeeded',result:{revision:version}}});
 throw Error('Unexpected request');
};
await api.refresh();version=7;
assert.equal((await api.reviewPage(new URLSearchParams())).revision,7);assert.equal(reads,2);assert.equal(api.snapshot().revision,7);
version=8;await api.adoptSourceAnalysis('source','job',['devices']);assert.equal(posts,1);
version=9;await api.adoptionStatus('source');assert.equal(api.snapshot().revision,9);
let release;globalThis.fetch=()=>new Promise(resolve=>{release=resolve});const oldRefresh=api.refresh();
globalThis.fetch=async()=>Response.json({data:workspace(11)});await api.refresh();release(Response.json({data:workspace(10)}));await oldRefresh;assert.equal(api.snapshot().revision,11);
console.log('PASS: stale read recovers once; adoption uses current revision; completed task synchronizes; late refresh cannot roll back version.');

await rm(temporary,{recursive:true,force:true});
