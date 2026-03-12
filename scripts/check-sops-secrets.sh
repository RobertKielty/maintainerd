#!/usr/bin/env bash
set -euo pipefail

mode="${1:---staged}"

case "${mode}" in
  --staged)
    changed_files=$(git diff --cached --name-only --diff-filter=ACMR | rg '^deploy/secrets/.*\.ya?ml$' || true)
    ;;
  --ref-range)
    if [[ $# -ne 3 ]]; then
      echo "usage: $0 --staged | --ref-range <from-ref> <to-ref>" >&2
      exit 2
    fi
    from_ref="$2"
    to_ref="$3"
    changed_files=$(git diff --name-only --diff-filter=ACMR "${from_ref}" "${to_ref}" | rg '^deploy/secrets/.*\.ya?ml$' || true)
    ;;
  *)
    echo "usage: $0 --staged | --ref-range <from-ref> <to-ref>" >&2
    exit 2
    ;;
esac

if [[ -z "${changed_files}" ]]; then
  exit 0
fi

tmp_dir=$(mktemp -d)
trap 'rm -rf "${tmp_dir}"' EXIT

fail=0
for file in ${changed_files}; do
  rendered_file="${tmp_dir}/$(basename "${file}")"
  if [[ "${mode}" == "--staged" ]]; then
    git show ":${file}" >"${rendered_file}"
  else
    git show "${to_ref}:${file}" >"${rendered_file}"
  fi

  if ! rg -q '^sops:' "${rendered_file}"; then
    echo "ERROR: ${file} is missing sops metadata." >&2
    fail=1
    continue
  fi

  while IFS= read -r line; do
    [[ "${line}" =~ ^[[:space:]]*# ]] && continue
    [[ "${line}" =~ ^[[:space:]]*$ ]] && continue

    if [[ "${line}" =~ ^[[:space:]]*([A-Za-z0-9_]*(TOKEN|SECRET|PASSWORD|KEY)[A-Za-z0-9_]*)[[:space:]]*:[[:space:]]*(.+)$ ]]; then
      value="${BASH_REMATCH[3]}"
      if [[ "${value}" != ENC[* ]]; then
        echo "ERROR: ${file} contains unencrypted value for ${BASH_REMATCH[1]}." >&2
        fail=1
      fi
    fi
  done <"${rendered_file}"
done

if [[ "${fail}" -ne 0 ]]; then
  echo "Commit blocked. Encrypt secrets with SOPS before committing." >&2
  exit 1
fi
