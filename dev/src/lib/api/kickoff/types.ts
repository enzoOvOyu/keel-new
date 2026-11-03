export type Domain = 'facility' | 'network' | 'provisioning' | 'cabling' | 'power';
export type ReviewState = 'ready' | 'blocked' | 'published';
export type Readiness = 'ready' | 'partial' | 'needs_input';
export interface Evidence { sourceId: string; sheet: string; cell: string; method: 'source' | 'mapping' | 'manual' | 'calculation'; note?: string }
export interface Device {
  id: string; name: string; serial: string; type: 'server' | 'switch' | 'device'; model: string;
  rackId: string; startU: number | null; heightU: number | null; role: string;
  managementAddress: string; evidence: Evidence;
}
export interface Port { id: string; deviceId: string; name: string; mac?: string; speed: string; role: string; componentId?: string; evidence: Evidence }
export interface Connection { id: string; a: string; b: string; medium: string; evidence: Evidence }
export interface Component { id: string; deviceId: string; type: string; name: string; model: string; slot: string; evidence: Evidence }
export interface Rack { id: string; name: string; room: string; capacityU: number | null; evidence: Evidence }
export interface SourceRegion { id: string; sheet: string; range: string; category: string; rowCount: number; state: 'extracted' | 'needs_input' | 'unmapped'; note: string }
export interface Source {
  id: string; name: string; size: string; version: number; state: 'recognized' | 'historical' | 'partial' | 'awaiting_analysis' | 'needs_input';
  hash: string; createdAt: string; regions: SourceRegion[]; originalAvailable: boolean;
}
export interface Issue {
  action?: string; id: string; scope: 'identity' | 'placement' | 'configuration'; title: string; description: string;
  entityId: string; field: string; impact: string; resolved: boolean; evidence: Evidence;
}
export interface ModelField { key: string; label: string; kind?: 'number' | 'positive' | 'integer' | 'select'; options?: string[]; hint?: string }
export interface TableDefinition {
  id: string; domain: Domain; name: string; description: string; consumer: string; fields: ModelField[];
}
export type ModelRow = Record<string, string>;
export interface ModelInput { tableId: string; rows: ModelRow[]; version: number; confirmed: boolean; updatedAt: string; source: string }
export interface CalculationRow { connectionId: string; from: string; to: string; route: string[]; pathM: number; leadM: number; slackM: number; calculatedM: number; stockM: number | null }
export interface Calculation {
  stale?: boolean; id: string; domain: 'cabling' | 'power'; inputDigest: string; createdAt: string; status: Readiness;
  rows: CalculationRow[]; issues: string[]; power: { rackId: string; loadKW: number | null; capacityKW: number | null; remainingKW: number | null }[];
}
export interface Publication {
  id: string; revision: number; createdAt: string; actor: string; reason: string; state: 'pending' | 'published' | 'failed'; errorCode?: string; deviceIds: string[];
  facilityIds?: string[]; portIds: string[]; connectionIds: string[]; componentIds: string[]; placementIds: string[];
  modelVersions: Record<string, number>; modelSnapshot: ModelInput[]; rackSnapshot: Rack[]; deviceSnapshot: Device[]; counts: Record<string, number>;
}
export interface Activity { id: string; action: string; detail: string; at: string }
export interface Workspace {
  capabilities?: Capability[];
  totals?: Record<string,number>; page?: {section:string;offset:number;limit:number;total:number;hasMore:boolean};
  schemaVersion: number; projectId: string; projectName: string; revision: number;
  devices: Device[]; ports: Port[]; connections: Connection[]; components: Component[]; racks: Rack[];
  sources: Source[]; issues: Issue[]; models: ModelInput[]; calculations: Calculation[];
  publications: Publication[]; activity: Activity[];
}
export interface Capability { id: string; title: string; detail: string; state: Readiness; available: number; total: number; action: string; href: string }
export interface PublishPreview { devices: Device[]; ports: Port[]; connections: Connection[]; components: Component[]; placements: Device[]; held: Device[]; counts: Record<string, number> }
export interface ImportPreview { rows: ModelRow[]; errors: string[]; unknownHeaders: string[]; sheet: string }
export interface PublicationOption {id:string;state:'new'|'changed'|'unchanged'|'blocked'|'pending';reason:string}
export interface FacilityPublicationOption extends PublicationOption {label:string;kind:string}
export interface PublicationOptions {revision:number;devices:PublicationOption[];facilities?:FacilityPublicationOption[]}
export interface KickoffAPI {
 refresh?():Promise<void>;
 sourceTasks?(sourceId:string,offset?:number,limit?:number):Promise<SourceTaskPage>;
 sourceTask?(sourceId:string,jobId:string):Promise<SourceTaskDetail>;
 adoptionStatus?(sourceId:string):Promise<AdoptionTask|null>;
 startSourceAnalysis?(id:string,domains:AnalysisDomain[]):Promise<{jobId:string;state:string}>;
 sourceAnalysis?(id:string,jobId?:string):Promise<SourceAnalysis>;
 adoptSourceAnalysis?(id:string,jobId:string,tableIds:string[]):Promise<AdoptionTask>;

 publicationOptions(revision:number):Promise<PublicationOptions>;
 sourceUnderstanding(id:string):Promise<SourceUnderstanding>;
 reuseSourceHistory(id:string):Promise<SourceUnderstanding>;
 sourceRegionPreview(sourceId:string,regionId:string,offset:number,revisionId:string,view:'source'|'normalized'):Promise<SourceRegionPreview>;
  evaluatePublication?(ids:string[],revision:number,facilityIds?:string[]):Promise<{revision:number;counts:Record<string,number>}>;
  loadContext?(id:string):Promise<void>;
  exportReview?(query:URLSearchParams):Promise<ReviewRow[]>;
  reviewPage?(query:URLSearchParams):Promise<ReviewPage>;
  loadSection?(section:string): Promise<void>;
  publicationDetail?(id:string): Promise<Publication>;
  subscribe(listener: () => void): () => void;
  snapshot(): Workspace;
  addSources(files: File[]): Promise<void>;
  removeSource(id: string): Promise<void>;
  analyzeSource(id: string): Promise<void>;
  resolveIssue(id: string, value: string): Promise<void>;
  saveModel(tableId: string, rows: ModelRow[], source: string): Promise<void>;
  previewPublication(ids: string[]): PublishPreview;
  publish(ids: string[], reason: string, expectedRevision: number, facilityIds?:string[]): Promise<Publication>;
  calculate(domain: 'cabling' | 'power'): Promise<Calculation>;
}

export interface ReviewSignal {code:string;label:string;severity:'block'|'warning'|'info';impact:string;action:string}
export interface ReviewFacts {
 type:string;model:string;serial:string;ownerId:string;ownerName:string;rackId:string;rackName:string;room:string;mac:string;speed:string;role:string;localPort:string;peerName:string;peerPort:string;medium:string;slot:string;
 startU:number|null;heightU:number|null;capacityU:number|null;
 portCount:number;connectionCount:number;componentCount:number;
}
export interface ReviewRow {id:string;name:string;description:string;reference:string;location:string;state:string;evidence:Evidence;facts:ReviewFacts;signals:ReviewSignal[]}
export interface ReviewPage {revision:number;rows:ReviewRow[];total:number;offset:number;limit:number;hasMore:boolean;counts:Record<string,number>;facets:Record<'type'|'model'|'rack'|'source',{value:string;label:string}[]>}

export interface SourceUnderstanding {
 sourceId:string; revisionId:string; state:'historical'|'recognized'|'partial'|'unsupported'; origin:'history'|'agent'|'none';
 steps:{id:'structure'|'regions'|'semantics'|'relationships'|'fields';state:'complete'|'partial'|'pending'|'unsupported'|'not_applicable';count:number;completed:number;total:number;note:string}[];
 regions:(SourceRegion & {categories:string[];evidenceCount:number;fieldEvidenceCount:number;objectCount:number;coveredCells:number;totalCells:number;mappings:{field:string;kind:'field'|'object';count:number}[];issues:string[]})[];
 limitations:string[];
}
export interface SourceRegionPreview {
 sourceId:string;revisionId:string;regionId:string;offset:number;limit:number;total:number;
 rows:{row:number;objects:{id:string;kind:string;label:string;fields:{field:string;value:string}[];evidence:number}[];cells:{cell:string;column:number;value:string;formula:string;sensitive:boolean;fields:string[];entityIds:string[]}[]}[];
}

export type AnalysisDomain='equipment'|'facility'|'cabling'|'network'|'power'|'provisioning';
export interface AgentNormalizationEvidence {rule:string;sheet?:string;column?:number;mapping_bounds?:{start_row:number;end_row:number;start_column:number;end_column:number};region_bounds?:{start_row:number;end_row:number;start_column:number;end_column:number};ticks?:{sheet:string;cell:string;value:string;u:number}[];unit_headers?:{sheet:string;cell:string;value:string}[];start_u?:string;height_u?:string;normalized_value?:string}
export interface AgentFieldEvidence {row:number;region:string;sheet:string;sourceRow:number;sourceColumn:number;fields:Record<string,{sheet:string;cell:string;formula:boolean;relationshipEvidence?:{sheet:string;cell:string}[];relationshipReason?:string;normalizationEvidence?:AgentNormalizationEvidence;calculationRunId?:string;lookup?:{sheet:string;key_column:number;value_column:number;start_row:number;end_row:number};normalization?:string;classification?:string}>;missing:string[]}
export interface AgentDraft {tableId:string;domain:AnalysisDomain;rowCount:number;previewLimit:number;rows:ModelRow[];issues:{row:number;field:string;code:string;message:string}[];baseVersion:number;evidence:AgentFieldEvidence[]}
export interface AdoptionTask {jobId:string;state:'pending'|'running'|'succeeded'|'failed';errorCode?:string;result?:{revision:number;adoptedTableIds:string[];skippedTableIds:string[]}}
export interface SourceAnalysis {adoption?:AdoptionTask;coverageTotal?:number;missingTotal?:number;coverageTruncated?:boolean;missingTruncated?:boolean;jobId:string|null;state:'idle'|'pending'|'running'|'succeeded'|'failed';errorCode?:string;domains:AnalysisDomain[];coverage:{region:string;step:number;state:string;rows:number;reason:string}[];missing:string[];drafts:AgentDraft[]}

export interface SourceTask {id:string;kind:'analysis'|'adoption';state:string;createdAt:string;updatedAt:string;errorCode?:string;analysisJobId?:string}
export interface SourceTaskPage {items:SourceTask[];offset:number;limit:number;total:number}
export interface SourceTaskDetail {task:SourceTask;steps:{sequence:number;tool:string;state:string;detail:string;createdAt:string}[];understanding?:SourceUnderstanding}
