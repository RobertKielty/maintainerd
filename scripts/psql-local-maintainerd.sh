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

DB_HOST="${MD_DB_HOST:-127.0.0.1}"
DB_PORT="${MD_DB_PORT:-55432}"
DB_NAME="${MD_DB_NAME:-maintainerd_local}"
DB_USER="${MD_DB_USER:-rk}"
DB_PASSWORD="${MD_DB_PASSWORD:-localpass}"
DB_SSLMODE="${MD_DB_SSLMODE:-disable}"

if ! command -v psql >/dev/null 2>&1; then
  echo "psql is not installed or not on PATH."
  exit 1
fi

CONN_STR="host=${DB_HOST} port=${DB_PORT} user=${DB_USER} dbname=${DB_NAME} sslmode=${DB_SSLMODE} connect_timeout=5"

if [ "$INTERACTIVE" = true ]; then
  PGPASSWORD="${DB_PASSWORD}" \
    TERM="${TERM:-xterm}" \
    PAGER=cat \
    PSQL_PAGER=off \
    exec psql -X -w "${PSQL_ARGS[@]}" "${CONN_STR}"
elif [ -n "$SQL_FILE" ]; then
  PGPASSWORD="${DB_PASSWORD}" psql -X -w "${PSQL_ARGS[@]}" "${CONN_STR}" -f "$SQL_FILE"
else
  PGPASSWORD="${DB_PASSWORD}" psql -X -w "${PSQL_ARGS[@]}" "${CONN_STR}" -c "$SQL"
fi
