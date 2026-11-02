export type Domain = 'facility' | 'network' | 'provisioning' | 'cabling' | 'power';
export type ReviewState = 'ready' | 'blocked' | 'published';
export type Readiness = 'ready' | 'partial' | 'needs_input';
export interface Evidence { sourceId: string; sheet: string; cell: string; method: 'source' | 'mapping' | 'manual' | 'calculation'; note?: string }
export interface Device {
  id: string; name: string; serial: string; type: 'server' | 'switch'; model: string;
  rackId: string; startU: number | null; heightU: number | null; role: string;
  managementAddress: string; evidence: Evidence;
}
export interface Port { id: string; deviceId: string; name: string; mac?: string; speed: string; role: string; componentId?: string; evidence: Evidence }
export interface Connection { id: string; a: string; b: string; medium: string; evidence: Evidence }
export interface Component { id: string; deviceId: string; type: string; name: string; model: string; slot: string; evidence: Evidence }
export interface Rack { id: string; name: string; room: string; capacityU: number | null; evidence: Evidence }
export interface SourceRegion { id: string; sheet: string; range: string; category: string; rowCount: number; state: 'extracted' | 'needs_input' | 'unmapped'; note: string }
export interface Source {
  id: string; name: string; size: string; version: number; state: 'recognized' | 'awaiting_analysis' | 'needs_input';
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
  id: string; domain: 'cabling' | 'power'; inputDigest: string; createdAt: string; status: Readiness;
  rows: CalculationRow[]; issues: string[]; power: { rackId: string; loadKW: number | null; capacityKW: number | null; remainingKW: number | null }[];
}
export interface Publication {
  id: string; revision: number; createdAt: string; actor: string; reason: string; state: 'pending' | 'published' | 'failed'; errorCode?: string; deviceIds: string[];
  portIds: string[]; connectionIds: string[]; componentIds: string[]; placementIds: string[];
  modelVersions: Record<string, number>; modelSnapshot: ModelInput[]; rackSnapshot: Rack[]; deviceSnapshot: Device[]; counts: Record<string, number>;
}
export interface Activity { id: string; action: string; detail: string; at: string }
export interface Workspace {
  schemaVersion: number; projectId: string; projectName: string; revision: number;
  devices: Device[]; ports: Port[]; connections: Connection[]; components: Component[]; racks: Rack[];
  sources: Source[]; issues: Issue[]; models: ModelInput[]; calculations: Calculation[];
  publications: Publication[]; activity: Activity[];
}
export interface Capability { id: string; title: string; detail: string; state: Readiness; available: number; total: number; action: string; href: string }
export interface PublishPreview { devices: Device[]; ports: Port[]; connections: Connection[]; components: Component[]; placements: Device[]; held: Device[]; counts: Record<string, number> }
export interface ImportPreview { rows: ModelRow[]; errors: string[]; unknownHeaders: string[]; sheet: string }
export interface KickoffAPI {
  subscribe(listener: () => void): () => void;
  snapshot(): Workspace;
  addSources(files: File[]): Promise<void>;
  removeSource(id: string): Promise<void>;
  analyzeSource(id: string): Promise<void>;
  resolveIssue(id: string, value: string): Promise<void>;
  saveModel(tableId: string, rows: ModelRow[], source: string): Promise<void>;
  previewPublication(ids: string[]): PublishPreview;
  publish(ids: string[], reason: string, expectedRevision: number): Promise<Publication>;
  calculate(domain: 'cabling' | 'power'): Promise<Calculation>;
}
