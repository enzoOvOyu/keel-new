import {z} from 'zod';
export const cmdbAssetTypes=['server','switch','gpu','nic','port','optic','cable','rack'] as const;
export const cmdbCatalogRequest=z.object({project_id:z.string().optional(),q:z.string().default(''),type:z.enum(cmdbAssetTypes).optional(),status:z.string().optional(),rack_id:z.string().optional(),cursor:z.string().default('0'),limit:z.number().int().min(1).max(100).default(20),sort:z.enum(['name','-name','type','-type']).default('name')});
export const cmdbManualRequest=z.object({project_id:z.string(),asset_id:z.string(),expected_revision:z.number().int().min(0),operator_notes:z.string().max(4000)}).strict();
export type CmdbAsset={id:string;project_id:string;asset_type:typeof cmdbAssetTypes[number];name:string;serial:string|null;model:string|null;address:string|null;rack_id:string|null;parent_id:string|null;delivery_stage:string|null;maintenance_state:string;health:string;freshness:string;completeness:string;source:string;revision:number};
export type CmdbRelation={id:string;source:string;target:string;kind:'position'|'composition'|'physical'|'logical'|'business';label:string;fact:'planned'|'observed'|'recorded';source_ref:string;revision:string};
export type CmdbNode={id:string;label:string;asset_type:string;project_id:string;href:string|null};
export type CmdbSource={id:string;asset_id:string;field:string;value_json:string;source:string;revision:string;observed_at:string|null;freshness:string;support:string;evidence_refs:string[];import_id:string|null;sheet:string|null;cell:string|null;status:string;href:string|null};
export type CmdbQuality={id:string;asset_id:string;field:string;code:string;summary:string;source:string;severity:'warning'|'critical';href:string;status:string};
