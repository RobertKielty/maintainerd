#!/usr/bin/env bash
set -euo pipefail

SQL=""
SQL_FILE=""

if [ "${1:-}" = "-f" ]; then
  SQL_FILE="${2:-}"
  if [ -z "$SQL_FILE" ]; then
    echo "Usage: $0 -f <path-to-sql-file>"
    exit 1
  fi
  if [ ! -f "$SQL_FILE" ]; then
    echo "SQL file not found: $SQL_FILE"
    exit 1
  fi
else
  SQL="${1:-}"
  if [ -z "$SQL" ]; then
    echo "Usage: $0 \"<sql>\""
    echo "Example: $0 \"select count(*) from projects;\""
    echo "Or: $0 -f /path/to/query.sql"
    exit 1
  fi
fi

DB_HOST="${MD_DB_HOST:-10.0.10.121}"
DB_PORT="${MD_DB_PORT:-5432}"
DB_NAME="${MD_DB_NAME:-maintainerd}"
DB_USER="${MD_DB_USER:-admin}"
DB_PASSWORD="${MD_DB_PASSWORD:-}"

if [ -z "$DB_PASSWORD" ]; then
  echo "Set MD_DB_PASSWORD before running."
  exit 1
fi

if [ -n "$SQL_FILE" ]; then
  kubectl -n maintainerd run psql-client --rm -it --restart=Never \
    --image=postgres:16-alpine \
    --env="PGPASSWORD=${DB_PASSWORD}" -- \
    psql "host=${DB_HOST} port=${DB_PORT} user=${DB_USER} dbname=${DB_NAME} sslmode=require" \
    -f "$SQL_FILE"
else
  kubectl -n maintainerd run psql-client --rm -it --restart=Never \
    --image=postgres:16-alpine \
    --env="PGPASSWORD=${DB_PASSWORD}" -- \
    psql "host=${DB_HOST} port=${DB_PORT} user=${DB_USER} dbname=${DB_NAME} sslmode=require" \
    -c "$SQL"
fi
