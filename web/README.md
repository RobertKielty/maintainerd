## Architecture (TL;DR)

- **Framework**: Next.js 16.1.1 (App Router, Turbopack in dev)
- **React**: 19.2.3 / ReactDOM 19.2.3
- **Design system**: `clo-ui` (CNCF UI kit) + Sass + CSS Modules
- **Markdown**: `react-markdown`, `remark-gfm`, `rehype-raw/sanitize`
- **Data path**: Web UI → BFF (Go) → Postgres (prod; sqlite optional local)
- **Auth**: GitHub OAuth handled by BFF. Dev/test mode supports `/auth/test-login?login=<user>` and auto-login via `NEXT_PUBLIC_DEV_AUTH_LOGIN`.
- **Env key**: `NEXT_PUBLIC_BFF_BASE_URL` (e.g., `http://localhost:8001` in dev)

## Page flow

- `/projects/[id]` is a client component that fetches the project via BFF `/api/projects/:id` and renders the project page component.
- Layout:
  - Top row: project header + Project Details card.
  - Bottom row: left vertical menu, right content pane that swaps between:
    - Legacy Data (maintainer-d roster + Project Admin File + diff bar)
    - Proposed dot project.yaml (stub for GitOps export)
    - Service placeholders (license checker, mailing lists, docs, Slack/Discord)
- Modals:
  - Add maintainer from ref (staff-only), supports creating/selecting companies.
- Diff:
  - `ProjectDiffControl` shows matches/missing/ref-only GitHub handles; “re-run diff” triggers refresh.

## BFF (Go) at a glance

- Auth: GitHub OAuth; in dev/test we allow `/auth/test-login`.
- Session: cookie `md_session`, in-memory store.
- DB: Postgres in prod (`MD_DB_DRIVER=postgres`, `MD_DB_DSN=...`); sqlite supported locally.
- Key endpoints used by the web app:
  - `GET /api/projects/:id` (project details + ref status/body + diff data)
  - `PATCH /api/projects/:id` (update maintainerRef URL, staff)
  - `POST /api/maintainers/from-ref` (add maintainer, staff)
  - `GET /api/companies`, `POST /api/companies` (staff)
  - `GET /api/me` (role)

## Dev quickstart (web)

```bash
cd web
NEXT_PUBLIC_BFF_BASE_URL=http://localhost:8001 \
NEXT_PUBLIC_DEV_AUTH_LOGIN=staff-tester \
npm run dev -- --port 3000   # Turbopack hot reload
```

Make sure `web-bff` is running in test mode and pointing at your local DB. For local Postgres via podman, use:
`host=127.0.0.1 port=55432 dbname=maintainerd_local user=rk password=localpass sslmode=disable`

## Dev quickstart (web-bff)

```bash
BFF_ADDR=:8001 \
MD_DB_DRIVER=postgres \
MD_DB_DSN="host=127.0.0.1 port=55432 dbname=maintainerd_local user=rk password=localpass sslmode=disable" \
WEB_APP_BASE_URL=http://localhost:3000 \
SESSION_COOKIE_DOMAIN= \
SESSION_COOKIE_SECURE=false \
BFF_TEST_MODE=true \
LFX_ENRICHMENT_ADMIN_GITHUB_LOGINS=staff-tester \
GITHUB_OAUTH_CLIENT_ID=test \
GITHUB_OAUTH_CLIENT_SECRET=test \
go run ./cmd/web-bff
```

## Testing

- E2E (Playwright/Cucumber): `make web-bdd`
- Lint: `npm run lint`

## Versions (key)

- Next.js: 16.1.1
- React / ReactDOM: 19.2.3
- TypeScript: ^5
- ESLint: ^9
- Playwright: ^1.55.0
