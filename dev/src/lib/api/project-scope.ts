import {entries,isConstructionPath} from '../../app/registry';
import type {ProjectScope} from './index';

// Resolve the existing provider's scope before legacy data is read. No parallel context.
export function initialProjectScope(previous:ProjectScope,authorized:string[],pathname:string,search:string):ProjectScope{
 const route=pathname.match(/^\/p\/([^/]+)\/(?:deploy|kickoff)(?:\/|$)/);
 const query=new URLSearchParams(search);
 const queryKey=query.has('project_id')?'project_id':query.has('project')?'project':null;
 const requested=route?decodeURIComponent(route[1]):queryKey?query.get(queryKey):null;
 if(requested&&!authorized.includes(requested))throw Error('403 · 未授权项目');
 const projectId=requested|| (authorized.includes(previous.projectId)?previous.projectId:authorized[0]||'');
 const filter=route?projectId:queryKey?requested||null:previous.filter&&authorized.includes(previous.filter)?previous.filter:null;
 return {projectId,mode:route?'project':'cross',filter};
}

// Unknown pages need no legacy data; registered real workspaces load their own APIs.
export function requiresLegacyFixtures(pathname:string):boolean{
 if(isConstructionPath(pathname))return false;
 const parts=pathname.split('/').filter(Boolean);
 const offset=parts[0]==='p'&&parts[2]==='deploy'?3:0;
 const entry=entries.find(e=>e.id===parts[offset]);
 if(!entry||['projects','kickoff','cmdb','topology','racks','flows'].includes(entry.id))return false;
 const view=parts[offset+1];
 if(entry.id==='admin'&&['users','members','roles','sessions','logins','audit','approvals','ssh','ai-provider'].includes(view||'users'))return false;
 if(entry.id==='ai'&&view==='providers')return false;
 return !view||!entry.views||entry.id==='flows'||entry.views.some(v=>v.id===view);
}
