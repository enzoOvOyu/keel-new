import type {Engine} from '@/lib/api/flows';
/** Only explicit stable identifiers are accepted; display names never establish scope. */
export function taskRunAsset(run:Engine):string|null {
 if(typeof run.asset_id==='string')return run.asset_id;
 if(typeof run.value==='object'&&run.value!==null){const value=run.value as Record<string,unknown>;if(typeof value.asset_id==='string')return value.asset_id;}
 if(typeof run.value==='string'){try{const value=JSON.parse(run.value);if(value&&typeof value.asset_id==='string')return value.asset_id}catch{}return run.value}
 return null;
}
export function taskRunsForAsset(runs:Engine[],asset:string){return runs.filter(run=>taskRunAsset(run)===asset)}
export function safeJobsReturn(value:string|null,fallback='/flows') {return value&&/^\/flows(?:\?|\/|$)/.test(value)&&!value.includes('://')&&!/[\\\r\n]/.test(value)?value:fallback}
