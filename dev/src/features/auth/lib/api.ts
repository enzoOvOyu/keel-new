import {bomRequest} from '../../../lib/api/realBom';
export type Session=import('../../../lib/schemas/openapi').components['schemas']['IdentitySession'];
export class ApiError extends Error{constructor(public status:number,public code:string,public retryAfter=0,public traceId=''){super(code)}}
export async function getAuthProviders(signal?:AbortSignal){const r=await fetch('/api/v1/auth/providers',{signal});if(!r.ok)throw new ApiError(r.status,'AUTH_UNAVAILABLE');return (await r.json()).data as {status:string;methods:string[];environment:string;ldap_name?:string;oidc_providers?:{id:string;name:string}[]}}
export async function getCSRF(){return{csrf_token:''}}
export async function login(body:{method:string;username:string;password:string},_:string):Promise<Session>{try{return await bomRequest<Session>('auth/login','POST',body)}catch(e:any){throw new ApiError(e.status||401,e.message,e.retryAfter||0)}}
export function safeNext(next:string|undefined,s:Session){if(!next||!next.startsWith('/')||next.startsWith('//')||next.startsWith('/login'))return '/projects';const m=next.match(/^\/p\/([^/]+)/);if(m&&!s.authorized_projects.some(p=>p.id===decodeURIComponent(m[1])))return '/projects';return next}
