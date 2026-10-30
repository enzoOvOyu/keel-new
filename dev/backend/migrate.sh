#!/usr/bin/env bash
set -eu
cd "$(dirname "$0")"
for migration in migrations/*.sql; do
 version=$(basename "$migration" | cut -d_ -f1 | sed 's/^0*//')
 applied=$(docker exec keel-dev-postgres-1 psql -U keel -d keel -tAc "SELECT EXISTS(SELECT 1 FROM schema_migration WHERE version=$version)" 2>/dev/null || true)
 if [ "$applied" != "t" ]; then docker exec -i keel-dev-postgres-1 psql -U keel -d keel -v ON_ERROR_STOP=1 --single-transaction < "$migration"; fi
done
