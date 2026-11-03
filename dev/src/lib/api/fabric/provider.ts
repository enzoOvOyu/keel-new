import type {IdentitySnapshot} from '../identityBridge';
import {HttpFabricProvider} from './http';
export type StudioProvider=HttpFabricProvider;
export const loadFabricProvider=(project:string,session:IdentitySnapshot,signal:AbortSignal)=>HttpFabricProvider.load(project,session,signal);
