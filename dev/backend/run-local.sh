#!/usr/bin/env bash
set -eu
cd "$(dirname "$0")"
set -a
. ./.env
set +a
exec go run ./cmd/api
