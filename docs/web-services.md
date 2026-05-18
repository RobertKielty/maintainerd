# Web Services (Frontend + BFF)

This repo now includes a separate web frontend and a Go BFF layer.

## Components
- **Next.js SSR app**: `web/`
- **Go BFF**: `cmd/web-bff/`
- **API-only maintainer-d**: `main.go` (webhook + health only for now)

## BFF env vars
- `BFF_ADDR` (default `:8000`)
- `MD_DB_DRIVER` (default `sqlite`, use `postgres` for managed DB)
- `MD_DB_DSN` (required when `MD_DB_DRIVER=postgres`)
- `GITHUB_OAUTH_CLIENT_ID` (required)
- `GITHUB_OAUTH_CLIENT_SECRET` (required)
- `GITHUB_OAUTH_REDIRECT_URL` (default `http://localhost:8000/auth/callback`)
- GitHub OAuth requests `read:user` and `public_repo` so staff can submit dot-project PRs from project-prefixed forks under their own GitHub account.
- `WEB_APP_BASE_URL` (default `http://localhost:3000`)
- `SESSION_COOKIE_NAME` (default `md_session`)
- `SESSION_COOKIE_DOMAIN` (optional, e.g., `github-events.cncf.io`)
- `SESSION_COOKIE_SECURE` (optional, `true` forces Secure cookies)
- `SESSION_TTL` (optional, default `8h`)
- `OAUTH_STATE_COOKIE_NAME` (optional, default `md_oauth_state`)

## Next steps
- Implement GitHub OIDC login and callback in the BFF.
- Add session storage (cookie + server-side store).
- Add API proxy routes in the BFF for the web app.
- Integrate `clo-ui` into the Next.js app.

## Kubernetes manifests
Manifests are provided for the web app and BFF:
- `deploy/manifests/web-deployment.yaml`
- `deploy/manifests/web-service.yaml`
- `deploy/manifests/web-bff-deployment.yaml`
- `deploy/manifests/web-bff-service.yaml`

Expected Secrets:
- `maintainerd-web-bff-env` should include:
  - `GITHUB_OAUTH_CLIENT_ID`
  - `GITHUB_OAUTH_CLIENT_SECRET`
  - `GITHUB_OAUTH_REDIRECT_URL`
  - `WEB_APP_BASE_URL`
  - `SESSION_COOKIE_DOMAIN`
  - `SESSION_COOKIE_SECURE`
- `maintainerd-db-env` should include:
  - `MD_DB_DRIVER`
  - `MD_DB_DSN`
- `maintainerd-web-env` should include:
  - `NEXT_PUBLIC_BFF_BASE_URL`

Note: `NEXT_PUBLIC_*` values are embedded at build time for client bundles.
Expose the services via your ingress controller or a dedicated LoadBalancer as needed.

## Local dev example
```
export GITHUB_OAUTH_CLIENT_ID=your_client_id
export GITHUB_OAUTH_CLIENT_SECRET=your_client_secret
export GITHUB_OAUTH_REDIRECT_URL=http://localhost:8000/auth/callback
export WEB_APP_BASE_URL=http://localhost:3000
export SESSION_COOKIE_SECURE=false
```

For the web app:
```
export NEXT_PUBLIC_BFF_BASE_URL=http://localhost:8000
```
