import {z} from 'zod';
export const resourceRef=z.object({id:z.string().min(1),project_id:z.string().optional()});
export const flowInput=z.object({name:z.string().min(3,'至少输入 3 个字符'),description:z.string().min(5,'请填写至少 5 个字符的用途')});
export const placementInput=z.object({u_start:z.number().int().min(1).max(42),u_height:z.number().int().positive(),rack_id:z.string()}).refine(p=>p.u_start+p.u_height-1<=42,'超出机柜边界');
