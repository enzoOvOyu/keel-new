import {bomRequest} from './realBom';
export type DeleteProjectResult={id:string;status:'deleted';storage_status:'pending';storage_event_id:string};
export function deleteProject(id:string,confirmName:string){return bomRequest<DeleteProjectResult>('projects/'+encodeURIComponent(id),'DELETE',{confirm_name:confirmName})}
