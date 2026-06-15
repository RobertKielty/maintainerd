#!/usr/bin/env bash
set -euo pipefail

SQL=""
SQL_FILE=""
INTERACTIVE=false
PSQL_ARGS=()

usage() {
  echo "Usage: $0 [-i] [-v name=value] [\"<sql>\"]"
  echo "Example: $0 \"select count(*) from projects;\""
  echo "Or: $0 -f /path/to/query.sql"
  echo "Or: $0 -v apply=true -f /path/to/query.sql"
  echo "Or: $0 -i"
}

while getopts ":f:iv:" opt; do
  case "$opt" in
    f)
      SQL_FILE="$OPTARG"
      ;;
    i)
      INTERACTIVE=true
      ;;
    v)
      PSQL_ARGS+=("-v" "$OPTARG")
      ;;
    :)
      usage
      exit 1
      ;;
    \?)
      usage
      exit 1
      ;;
  esac
done

shift $((OPTIND - 1))

if [ -n "$SQL_FILE" ] && [ "$INTERACTIVE" = true ]; then
  echo "Use either -f or -i, not both."
  exit 1
fi

if [ -n "$SQL_FILE" ]; then
  if [ ! -f "$SQL_FILE" ]; then
    echo "SQL file not found: $SQL_FILE"
    exit 1
  fi
elif [ "$INTERACTIVE" = false ]; then
  SQL="${1:-}"
  if [ -z "$SQL" ]; then
    usage
    exit 1
  fi
fi

DB_HOST="${MD_DB_HOST:-10.0.10.121}"
DB_PORT="${MD_DB_PORT:-5432}"
DB_NAME="${MD_DB_NAME:-maintainerd}"
DB_USER="${MD_DB_USER:-admin}"
DB_PASSWORD="${MD_DB_PASSWORD:-}"
POD_NAME=""
CONN_STR="host=${DB_HOST} port=${DB_PORT} user=${DB_USER} dbname=${DB_NAME} sslmode=require connect_timeout=5"

if [ -z "$DB_PASSWORD" ]; then
  echo "Set MD_DB_PASSWORD before running."
  exit 1
fi

cleanup_pod() {
  if [ -n "$POD_NAME" ]; then
    kubectl -n maintainerd delete pod "$POD_NAME" --ignore-not-found --wait=false >/dev/null 2>&1 || true
  fi
}

if [ "$INTERACTIVE" = true ]; then
  POD_NAME="psql-client-${USER:-user}-$$"
  trap cleanup_pod EXIT INT TERM

  kubectl -n maintainerd run "$POD_NAME" --restart=Never \
    --image=postgres:16-alpine \
    --env="PGPASSWORD=${DB_PASSWORD}" \
    --command -- sleep infinity >/dev/null

  kubectl -n maintainerd wait --for=condition=Ready "pod/${POD_NAME}" --timeout=60s >/dev/null

  kubectl -n maintainerd exec -it "$POD_NAME" -- \
    sh -lc "export TERM='${TERM:-xterm}' PAGER=cat PSQL_PAGER=off; exec psql -X -w ${PSQL_ARGS[*]} \"${CONN_STR}\""
elif [ -n "$SQL_FILE" ]; then
  POD_NAME="psql-client-${USER:-user}-$$"
  trap cleanup_pod EXIT INT TERM

  kubectl -n maintainerd run "$POD_NAME" --restart=Never \
    --image=postgres:16-alpine \
    --env="PGPASSWORD=${DB_PASSWORD}" \
    --command -- sleep infinity >/dev/null

  kubectl -n maintainerd wait --for=condition=Ready "pod/${POD_NAME}" --timeout=60s >/dev/null

  kubectl -n maintainerd exec -i "$POD_NAME" -- \
    psql -X -w "${PSQL_ARGS[@]}" "${CONN_STR}" < "$SQL_FILE"
else
  POD_NAME="psql-client-${USER:-user}-$$"
  trap cleanup_pod EXIT INT TERM

  kubectl -n maintainerd run "$POD_NAME" --restart=Never \
    --image=postgres:16-alpine \
    --env="PGPASSWORD=${DB_PASSWORD}" \
    --command -- sleep infinity >/dev/null

  kubectl -n maintainerd wait --for=condition=Ready "pod/${POD_NAME}" --timeout=60s >/dev/null

  kubectl -n maintainerd exec -i "$POD_NAME" -- \
    psql -X -w "${PSQL_ARGS[@]}" "${CONN_STR}" -c "$SQL"
fi
