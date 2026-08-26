# Operations Guide (Goal-Oriented)

This guide is organized by outcomes. Use it as a checklist when operating maintainer-d in production or development.

## Import data from the legacy worksheet (bootstrap)

Goal: Load or refresh projects/maintainers from the Google worksheet into Postgres.

Production (SOPS secrets):
1) Ensure required secrets are applied:
```
make sops-apply-bootstrap-secrets
make apply-creds
```

2) Run the bootstrap job:
```
make bootstrap-run
```

Development (local .envrc):
1) Generate and apply the dev bootstrap secret:
```
make env
make apply-env
```

2) Run the bootstrap job:
```
make bootstrap-run-dev
```

Notes:
- `maintainerd-bootstrap-env` must include `MD_WORKSHEET`, `FOSSA_API_TOKEN`, and `WORKSPACE_CREDENTIALS_FILE` (internal worksheet credentials).
- `maintainerd-db-env` must include `MD_DB_DRIVER=postgres` and `MD_DB_DSN=...` for production.
- Database extensions required by the schema (e.g., `unaccent`, `pg_trgm`) must be available in the Postgres image and created before restore/migrations. Update local DB scripts in `scripts/` if requirements change.

## Deploy the maintainerd server

Goal: Deploy the API server and supporting resources.

1) Apply namespace, registry secret, and SOPS secrets:
```
make ensure-ns
make apply-ghcr-secret
make sops-apply-web-secrets
make sops-apply-bootstrap-secrets
make apply-creds
```

2) Apply Kubernetes manifests:
```
make manifests-apply
```

3) Verify:
```
kubectl -n maintainerd get pvc,deploy,svc,ing
```

## Run schema migrations

Goal: Apply database schema changes safely without re-importing data.

1) Build and push the migrate image:
```
make migrate-image-build
make migrate-image-push
```

2) Apply the migrate job manifest:
```
kubectl -n maintainerd apply -f deploy/manifests/maintainerd-migrate-schema-job.yaml
```

3) Wait for completion:
```
kubectl -n maintainerd wait --for=condition=complete job/maintainerd-migrate-schema --timeout=5m
```

4) Check logs:
```
kubectl -n maintainerd logs job/maintainerd-migrate-schema
```

5) Cleanup (optional):
```
kubectl -n maintainerd delete job/maintainerd-migrate-schema
```

## Take a copy of the data in the database

Goal: Export a snapshot of the production Postgres database.

Run a one-off `pg_dump` from a pod:
```
PGPASSWORD="<password>" \
kubectl -n maintainerd run pgdump --rm -i --restart=Never \
  --image=postgres:16-alpine \
  --env="PGPASSWORD=$PGPASSWORD" -- \
  pg_dump "host=<db-host> port=5432 user=<db-user> dbname=maintainerd sslmode=require" \
  --format=custom --no-owner --no-privileges > maintainerd-prod.dump
```

## Deploy sanitize

Goal: Run/enable the sanitize job that archives maintainers missing from project admin files.

1) Build and push the image:
```
make sanitize-image-build
make sanitize-image-push
```

2) Apply or update the CronJob manifest:
```
kubectl -n maintainerd apply -f deploy/manifests/cronjob-sanitize.yaml
```

## Deploy the web app for staff

Goal: Deploy web + BFF to production.

1) Build/push images:
```
make web-image-build
make web-image-push
make web-bff-image-build
make web-bff-image-push
```

2) Apply manifests:
```
make deploy-web
```

3) (Optional) pin the web image to a tag:
```
make web-image-set TAG=<tag>
```

## Manage secrets (dev vs production)

### Dev secrets (local .envrc)
- `.envrc` is dev-only.
- Generate and apply dev bootstrap secrets:
```
make env
make apply-env
make apply-creds
```

### Production secrets (SOPS)
We store encrypted secrets in `deploy/secrets/`.

1) Create plaintext secrets (once) and encrypt:
```
mkdir -p deploy/secrets
cp deploy/templates/maintainerd-web-env.yaml.tmpl deploy/secrets/maintainerd-web-env.yaml
cp deploy/templates/maintainerd-web-bff-env.yaml.tmpl deploy/secrets/maintainerd-web-bff-env.yaml
cp deploy/templates/maintainerd-db-env.yaml.tmpl deploy/secrets/maintainerd-db-env.yaml
cp deploy/templates/maintainerd-bootstrap-env.yaml.tmpl deploy/secrets/maintainerd-bootstrap-env.yaml
# fill in values, then:
make sops-encrypt-web-secrets SOPS_AGE_KEY=age1...
make sops-encrypt-bootstrap-secrets SOPS_AGE_KEY=age1...
```

2) Apply encrypted secrets:
```
make sops-apply-web-secrets SOPS_AGE_KEY_FILE=~/.config/sops/age/keys.txt
make sops-apply-bootstrap-secrets SOPS_AGE_KEY_FILE=~/.config/sops/age/keys.txt
make apply-creds
```

## License
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FRobertKielty%2Fmaintainerd.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2FRobertKielty%2Fmaintainerd?ref=badge_large)
