# Keel dev — BOM real implementation in progress

The existing frontend was copied from `keel-demo` into this independent dev workspace. Other domains retain their explicit synthetic Mock provider; BOM uses the Go service. The original demo is preserved.

## Run locally

1. Keep PostgreSQL and MinIO from the root compose environment running.
2. Apply versioned SQL deliberately: `docker exec -i keel-dev-postgres-1 psql -U keel -d keel -v ON_ERROR_STOP=1 < dev/backend/migrations/001_bom.sql` from repository root. This creates domain tables; it never drops existing tables.
3. Configure backend `.env` from `.env.example` using local controlled credentials; run `./dev/backend/run-local.sh`.
4. Configure `dev/.env` with the same local API token and `KEEL_API_URL=http://127.0.0.1:8081`. The Vite proxy injects the credential server-side; it is not a `VITE_*` browser variable.
5. Run `npm ci` then `npm run dev -- --port 5174` in dev. Stop the old `keel-dev-web-1` first if it owns that port.

Backend: localhost:8081. Frontend: localhost:5174. Original demo: localhost:5180.

## Current boundary

This is not yet the requested full BOM delivery. Actual file upload, hash, immutable generated object keys, PostgreSQL import jobs, background structural parsing, basic candidate correction, validation, review, transactional BOM/outbox and a CMDB entity projector are implemented. No real AI call was made. Authentication currently uses an explicitly configured development service token restricted to one project, not a production browser session/RBAC implementation.

The existing standard contract still requires full adapter conformance. Human mapping for arbitrary Sheet layouts, source-authoritative transformations, approved model/template binding, complete cross-Sheet correlation, topology/engineering rules, component/port/placement relationship projections, Provider diagnostics/evaluation, idempotency payload reconciliation and production permissions remain incomplete. Never treat this branch as production-ready or import the real sample into formal assets without completing those checks.

Raw `netbom.xlsx` and credential files are Git-ignored. The structure report includes only counts and Sheet metadata, never raw asset rows.
