import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import YAML from 'yaml';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const read = p => fs.readFileSync(path.join(root, p), 'utf8');
const registry = JSON.parse(read('docs/api/real-api-registry.json'));
const api = YAML.parse(read(registry.schema_authority));
const deployment = JSON.parse(read(registry.deployment_evidence));
const errors = [];
const check = (condition, message) => { if (!condition) errors.push(message); };
const exists = p => fs.existsSync(path.join(root, p));
const seen = new Set();
const ids = new Set();
check(registry.deployed_revision === deployment.code_revision, 'Deployment revision mismatch');
for (const entry of registry.operations) {
  const key = `${entry.method} ${entry.path}`;
  check(!seen.has(key), `Duplicate operation: ${key}`); seen.add(key);
  check(!ids.has(entry.operation_id), `Duplicate operationId: ${entry.operation_id}`); ids.add(entry.operation_id);
  const operation = api.paths?.[entry.path]?.[entry.method];
  check(operation?.operationId === entry.operation_id, `OpenAPI operation mismatch: ${key}`);
  const response = operation?.responses?.[entry.response_status || '200']?.content?.[entry.response_media_type || 'application/json']?.schema;
  if (entry.response_schema) check(response?.$ref === `#/components/schemas/${entry.response_schema}` || response?.properties?.data?.$ref === `#/components/schemas/${entry.response_schema}`, `Response schema mismatch: ${key}`);
  else check(!!response, `Response media schema missing: ${key}`);
  if (entry.request_schema) check(operation?.requestBody?.content?.['application/json']?.schema?.$ref === `#/components/schemas/${entry.request_schema}`, `Request schema mismatch: ${key}`);
  for (const field of ['handler', 'source', 'documentation']) check(exists(entry[field]), `Missing ${field}: ${entry[field]}`);
  if (entry.verification === 'startup_http_200') check(deployment.api_startup_checks.checks.some(c => c.endpoint === entry.path && c.status === 200), `Missing startup evidence: ${key}`);
}
for (const doc of ['HANDOFF.md', 'docs/REAL-API-CONTRACTS.md', 'docs/api/visualization.md', 'docs/api/flows.md']) {
  for (const match of read(doc).matchAll(/\]\(([^)]+)\)/g)) {
    const target = match[1].split('#')[0];
    if (!target || /^[a-z]+:/i.test(target)) continue;
    check(exists(path.join(path.dirname(doc), target)), `Broken link in ${doc}: ${target}`);
  }
}
if (errors.length) {
  process.stderr.write(errors.join('\n') + '\n');
  process.exitCode = 1;
} else {
  const methods = new Set(['get', 'post', 'put', 'patch', 'delete', 'head', 'options']);
  const total = Object.values(api.paths).reduce((n, item) => n + Object.keys(item).filter(k => methods.has(k)).length, 0);
  console.log(`PASS: ${registry.operations.length} registered operations, schema/source/documentation references and deployment evidence consistent.`);
  console.log(`OpenAPI design operations: ${total}; other operations were not audited by this registry. No HTTP, database or AI calls performed.`);
}
