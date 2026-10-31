import {bomRequest} from './realBom';
export type DeleteProjectResult={id:string;status:'deleted';storage_status:'pending';storage_event_id:string};
export function deleteProject(id:string,confirmName:string){return bomRequest<DeleteProjectResult>('projects/'+encodeURIComponent(id),'DELETE',{confirm_name:confirmName})}

import type {IdentitySnapshot} from './identityBridge';
// Prune only the confirmed deleted scope; the next session request remains
// authoritative for permission revisions and other projects.
export function withoutProject(session:IdentitySnapshot,id:string):IdentitySnapshot {
 const project_permissions={...session.project_permissions};delete project_permissions[id];
 const project_roles={...session.project_roles};delete project_roles[id];
 return {...session,authorized_projects:session.authorized_projects.filter(p=>p.id!==id),project_permissions,project_roles,grants:session.grants.filter(g=>g.project_id!==id)};
}
