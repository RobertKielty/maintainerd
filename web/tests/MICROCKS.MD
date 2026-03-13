# Web BDD With Microcks

This document describes how to run the web BDD suite against a mocked FOSSA API
using Microcks.

## Why Microcks

The web BDD suite exercises FOSSA-backed flows such as:

- choosing FOSSA for a project
- listing FOSSA team state on the License Checker page
- refreshing maintainer service state
- sending FOSSA invites
- reconciling missing FOSSA team memberships

Those flows are more reliable under test when the FOSSA API is mocked.

## Mock Artifact

The mock contract lives at:

- [`microcks/fossa-api-mock.yaml`](/home/rk/cncf/gh/maintainer-d/microcks/fossa-api-mock.yaml)

The supporting scripts are:

- [`scripts/microcks-up.sh`](/home/rk/cncf/gh/maintainer-d/scripts/microcks-up.sh)
- [`scripts/microcks-down.sh`](/home/rk/cncf/gh/maintainer-d/scripts/microcks-down.sh)

## Run The Web BDD Suite

The simplest path is:

```bash
WEB_BDD_USE_MICROCKS=true make test-web
```

This flow will:

- reset and seed the local test database
- start Microcks automatically
- import the FOSSA mock artifact
- point `web-bff` at the Microcks mock base URL
- build and start the web app
- run the Playwright/Cucumber BDD suite
- stop Microcks during cleanup

## Run A Single Feature

To run only one feature file:

```bash
WEB_BDD_USE_MICROCKS=true \
BDD_FEATURE=../features/web/maintainer_profile.feature \
make test-web
```

Example for the project License Checker feature:

```bash
WEB_BDD_USE_MICROCKS=true \
BDD_FEATURE=../features/web/license_checker.feature \
make test-web
```

## Manual Local Use

If you want Microcks available outside the Make target:

```bash
scripts/microcks-up.sh
```

Microcks UI:

```text
http://localhost:8585
```

The mock base URL used by `web-bff` is:

```text
http://localhost:8585/rest/FOSSA%20API/4.32.3
```

When finished:

```bash
scripts/microcks-down.sh
```

## Notes

- The mock reflects observed live FOSSA behavior for `PUT /teams/{id}/users`,
  including the currently required `users[].id` request field.
- The web BDD seed data is created by
  [`cmd/web-bff-seed/main.go`](/home/rk/cncf/gh/maintainer-d/cmd/web-bff-seed/main.go).
- The Cucumber step definitions live under
  [`web/tests/steps`](/home/rk/cncf/gh/maintainer-d/web/tests/steps).
