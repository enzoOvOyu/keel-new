import { createContext, useContext } from 'react';
import type { Evidence, KickoffAPI, Workspace } from './lib/api';
export interface WorkspaceContextValue {
  api: KickoffAPI; state: Workspace; go: (route:string)=>void;
  notify: (message:string, error?:boolean)=>void;
  inspect: (kind:'device'|'port'|'connection'|'component'|'evidence', id:string, evidence?:Evidence)=>void;
  run: (message:string, action:()=>unknown)=>boolean;
}
export const WorkspaceContext=createContext<WorkspaceContextValue|null>(null);
export function useWorkspace(){const value=useContext(WorkspaceContext);if(!value)throw Error('工作区未初始化');return value;}
