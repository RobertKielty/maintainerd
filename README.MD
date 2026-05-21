# maintainer-d

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FRobertKielty%2Fmaintainerd.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2FRobertKielty%2Fmaintainerd?ref=badge_shield)

## Overview

maintainer-d is a database-backed application that tracks Maintainers that work on CNCF Projects using multiple data sources.

Right now this is very much a CNCF-internal project that we are developing in the open to satisfy the day-to-day administrative needs of both the CNCF Project Team and the CNCF Events Team. The internal nature of the project reflects the fact that we are iterating on this within the CNCF Staff teams.

At this time, we are *not looking for external contributors*.

If you are interested in contributing to one of the 240+ CNCF Projects please visit 

https://contribute.cncf.io/ 

to learn more about joining our amazing community. 

You can find a project to work on the [CNCF Landscape](https://landscape.cncf.io/?view-mode=card&classify=maturity&sort-by=name&sort-direction=asc#maturity--graduated) (that link sets up the view to allow you to navigate our projects by Project Maturity) 

## Rationale

The success that the CNCF has in attracting new projects to the foundation means that we had to improve both our internal tooling for administering maintainer details and the processes around maintainer operations. This project is only a part of the ongoing effort to improving processes in this area.

The plan, at this time, is as follows:

1. For CNCF Staff only, the web app will provide a portal to allow staff to easily access and audit CNCF Project maintainer details. 

2. For CNCF Maintainers, CNCF Staff intend to /generate/ project YAML files using the data we have internally combined with your existing maintainer declarations in your repos. We will eventually provide PRs into the repos you currently use for governance to make it easy for you to review and merge the new standardized project metadata format. Then those project YAML files will be the single source of truth for the project and it should simplify the process informing the CNCF of changes to maintainer lineups, company affiliations, email address changes etc. etc. the majority of the specification work pertaining to maintainer management has taken place over on the cncf/automation. There we are working on defining the format of the .project.yaml files to store essential metadata for projects.


## Current processes
At present, a Project Maintainer registers with the CNCF by
 - sending an email triggering a manual add to an internal Google Worksheet
 - requesting access to services via a Service Desk Request

## Proposed Maintainer and Project registration methods
 - project yaml files will formally list out the maintainers for a project
 - changes will be picked up by maintainer-d
 - admin web front end for CNCF Staff 
 - eventually we will look at integrattion with OpenProfile.dev

# Stack and

## Database
The maintainer-d backend is a database implemented using GORM, a Go object relational mapping tool. We started out with SQLite and have recently migrated to Postgres. We have a repeatable process to lift data from our old internal worksheet, but we hope to stop using the worksheet moving forward once the web front end is good enough to use full time instead of the old internal worksheet.

## maintainer-d server

Listens for specific changes on the cncf/sandbox repo to initiate onboarding workflows for new CNCF Projects. At this time, there is a FOSSA workflow for projects that choose to use FOSSA for License Scanning.

## Service Plugins

A plugin will reconcile the list of maintainers for a project and ensure that they are registered with their chosen services.

## Custom Resource Definitions

We have CRDs for projects, maintainers and companies that are periodically synchronized from data in the DB. These CRDs could form the backbone of k8s based tooling to retrieve CNCF Project data.

## Operations Guide

Operational commands, deployment steps, and Make targets live in [OPS.MD](./OPS.MD).

## Developer: Run the Web App Locally

Prereqs:
- Node.js 20+
- Go 1.24+

1) Seed a local DB for the web-bff:
```
go run ./cmd/web-bff-seed -db ./testdata/maintainerd_test.db
```

2) Start the web-bff (API server):
```
MD_DB_PATH=./testdata/maintainerd_test.db \
WEB_APP_BASE_URL=http://localhost:3000 \
GITHUB_OAUTH_REDIRECT_URL=http://localhost:8000/auth/callback \
GITHUB_OAUTH_CLIENT_ID=dev-client \
GITHUB_OAUTH_CLIENT_SECRET=dev-secret \
BFF_TEST_MODE=true \
go run ./cmd/web-bff
```

3) Start the web app:
```
cd web
NEXT_PUBLIC_BFF_BASE_URL=http://localhost:8000 \
npm install
npm run dev
```

Notes:
- `BFF_TEST_MODE=true` enables `/auth/test-login?login=<github>` for local sign-in.
- `NEXT_PUBLIC_BFF_BASE_URL` should match the web-bff address.

## Web BDD With Microcks

For the web BDD test setup that mocks the FOSSA API with Microcks, see
[web/tests/MICROCKS.MD](https://github.com/cncf/maintainer-d/blob/main/web/tests/MICROCKS.MD).

## License
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FRobertKielty%2Fmaintainerd.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2FRobertKielty%2Fmaintainerd?ref=badge_large)
