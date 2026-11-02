import type {Definition} from './definition';
/** Presentation lineage lives in the same versioned Flow document, never in a second execution model. */
export type ScriptSource=import('@/lib/schemas/openapi').components['schemas']['FlowsScriptSource'];
export type FileIntent={action:string;nonce:number;path?:string;revisionId?:string;content?:string};
export function scriptSources(doc:Definition):Record<string,ScriptSource>{return doc.variables?.keel_script_sources||{}}
export function setScriptSource(doc:Definition,id:string,source?:ScriptSource){doc.variables||={};const sources={...scriptSources(doc)};if(source)sources[id]=source;else delete sources[id];if(Object.keys(sources).length)doc.variables.keel_script_sources=sources;else delete doc.variables.keel_script_sources;}
