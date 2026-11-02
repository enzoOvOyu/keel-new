import {createContext,useContext} from 'react';import type {FabricProvider} from '../../lib/api/fabric/mock';import type {FabricState} from '../../lib/api/fabric/types';
export interface DialogRequest {kind:string;id?:string;ids?:string[]}
export interface Workspace {p:FabricProvider;s:FabricState;run:(label:string,fn:()=>unknown)=>Promise<boolean>;open:(kind:string,id?:string,ids?:string[])=>void;inspect:(kind:string,id:string)=>void;go:(page:string)=>void;plane:string;setPlane:(id:string)=>void;mode:'physical'|'logical'|'planes';setMode:(m:'physical'|'logical'|'planes')=>void}
export const WorkspaceContext=createContext<Workspace>(null!);export const useWorkspace=()=>useContext(WorkspaceContext);
