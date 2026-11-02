import fixtureUrl from '../../../../../fixtures/demo-full-64-b300/fabric-studio.json?url';
import type {IdentitySnapshot} from '../identityBridge';
import {FabricProvider,initialState} from './mock';
import type {FabricFixture,FabricState,Role} from './types';
import {HttpFabricProvider} from './http';
export type StudioProvider=FabricProvider&{mode:'mock'|'real'};
export function setIdentity(fixture:FabricFixture,state:FabricState,project:string,session:IdentitySnapshot){
 const roles=[...(session.global_roles||[]),...(session.project_roles[project]||[])];const role=(roles.includes('admin')?'admin':roles[0]||'viewer') as Role;
 fixture.actors=[{id:session.user.id,name:session.user.display_name,role,projectIds:[project]}];state.actorId=session.user.id;
}
export async function loadFabricProvider(project:string,session:IdentitySnapshot,signal:AbortSignal):Promise<StudioProvider>{
 if(import.meta.env.VITE_FABRIC_PROVIDER!=='mock')return HttpFabricProvider.load(project,session,signal);
 const response=await fetch(fixtureUrl,{signal});if(!response.ok)throw Error('演示数据加载失败，请重试');const fixture:FabricFixture=await response.json();
 // This entire provider is explicitly synthetic; never relabel real CMDB assets.
 fixture.projectId=project;fixture.snapshots.forEach(s=>s.projectId=project);
 const key='keel-fabric-mock-v1:'+project;const raw=sessionStorage.getItem(key);const saved:FabricState=raw?JSON.parse(raw):initialState(fixture);saved.projectId=project;setIdentity(fixture,saved,project,session);
 return Object.assign(new FabricProvider(fixture,saved,s=>sessionStorage.setItem(key,JSON.stringify(s))),{mode:'mock' as const});
}
