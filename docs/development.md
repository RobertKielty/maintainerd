# Development

## Git hooks

This repo uses a local git hooks directory to prevent committing unencrypted secrets
and likely PII in staged files.

Install hooks for this repo:

```bash
make githooks-install
```

The pre-commit hook currently runs two checks:

- `scripts/check-sops-secrets.sh --staged`
  - blocks commits if any `deploy/secrets/*.yaml` files are missing SOPS metadata
    or contain plaintext values for secret-like keys
- `scripts/check-pii.sh --staged`
  - scans staged text files for likely personal email addresses and obvious tokens
  - permits approved fixture/example values via
    [`scripts/pii-allowlist.regex`](/home/rk/cncf/gh/maintainer-d/scripts/pii-allowlist.regex)

If the PII check flags a legitimate fixture, either sanitize it further or add a
targeted allowlist pattern rather than disabling the hook.
