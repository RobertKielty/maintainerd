#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
ALLOWLIST_FILE="${ROOT_DIR}/scripts/pii-allowlist.regex"

usage() {
  echo "usage: $0 --staged | --ref-range <from-ref> <to-ref>" >&2
  exit 2
}

if [[ ! -f "${ALLOWLIST_FILE}" ]]; then
  echo "missing allowlist file: ${ALLOWLIST_FILE}" >&2
  exit 2
fi

mode="${1:---staged}"
case "${mode}" in
  --staged)
    changed_files=$(git diff --cached --name-only --diff-filter=ACMR || true)
    ;;
  --ref-range)
    if [[ $# -ne 3 ]]; then
      usage
    fi
    from_ref="$2"
    to_ref="$3"
    changed_files=$(git diff --name-only --diff-filter=ACMR "${from_ref}" "${to_ref}" || true)
    ;;
  *)
    usage
    ;;
esac

if [[ -z "${changed_files}" ]]; then
  exit 0
fi

tmp_dir=$(mktemp -d)
trap 'rm -rf "${tmp_dir}"' EXIT

is_allowed_match() {
  local candidate="$1"
  while IFS= read -r pattern; do
    [[ -z "${pattern}" ]] && continue
    [[ "${pattern}" =~ ^# ]] && continue
    if [[ "${candidate}" =~ ${pattern} ]]; then
      return 0
    fi
  done <"${ALLOWLIST_FILE}"
  return 1
}

render_file() {
  local file="$1"
  local target="$2"
  if [[ "${mode}" == "--staged" ]]; then
    git show ":${file}" >"${target}"
  else
    git show "${to_ref}:${file}" >"${target}"
  fi
}

fail=0
for file in ${changed_files}; do
  rendered_file="${tmp_dir}/$(basename "${file}")"
  if ! render_file "${file}" "${rendered_file}" 2>/dev/null; then
    continue
  fi
  if ! LC_ALL=C grep -Iq . "${rendered_file}"; then
    continue
  fi

  while IFS= read -r match; do
    [[ -z "${match}" ]] && continue
    line_no="${match%%:*}"
    value="${match#*:}"
    if ! is_allowed_match "${value}"; then
      echo "ERROR: possible PII email in ${file}:${line_no}: ${value}" >&2
      fail=1
    fi
  done < <(grep -nEo '[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}' "${rendered_file}" | sort -u || true)

  while IFS= read -r match; do
    [[ -z "${match}" ]] && continue
    line_no="${match%%:*}"
    value="${match#*:}"
    if ! is_allowed_match "${value}"; then
      echo "ERROR: possible secret/token in ${file}:${line_no}: ${value}" >&2
      fail=1
    fi
  done < <(
    grep -nEo \
      'ghp_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|Bearer[[:space:]][A-Za-z0-9._=-]{16,}|sk-[A-Za-z0-9]{16,}|AKIA[0-9A-Z]{16}|-----BEGIN (RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----' \
      "${rendered_file}" | sort -u || true
  )
done

if [[ "${fail}" -ne 0 ]]; then
  echo "Commit blocked. Remove or sanitize PII/secrets, or extend the allowlist for approved fixtures." >&2
  exit 1
fi
