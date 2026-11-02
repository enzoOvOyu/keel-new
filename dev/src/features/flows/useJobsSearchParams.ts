import {useSearchParams} from 'react-router-dom';
/** Router updaters are not React state queues. Read the current browser URL so
 * consecutive Vue events cannot overwrite a preceding task/attempt update. */
export function useJobsSearchParams(){
 const [params,setParams]=useSearchParams();
 const update:typeof setParams=(next,options)=>setParams(typeof next==='function'?next(new URLSearchParams(window.location.search)):next,options);
 return [params,update] as const;
}
