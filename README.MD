# maintainerd

maintainerd is a database-backed application that tracks Maintainers that work on CNCF Projects using multiple datasources.

At present, a Project Maintainer registers with the CNCF by
 - sending an email triggering a manual add to an internal Google Worksheet
 - requesting access to services via a Service Desk Request

## Proposed Maintainer and Project registration methods
 - maintainer.yaml formally lists out the maintainers for a project
 - openprofile.dev
 - admin front end for CNCF Project Team

# Components
## Database
The maintainerd backend is a database implemented using GORM, a golang object relational mapping tool.

Building and running the resultant db executable loads data from the internal worksheet.

```
cd db
go build
./db
2025/06/10 05:06:34 maintainerd: backend: database successfully seeded demo.db (SQLite)
```
Data is stored in a file called demo.db
MD_WORKSHEET needs to contain the ID of the Goolge Worksheet being read
credentials.json needs to contain the Google Service Account that is allowed to read the
worksheet.

## Service Plugins

A plugin will reconcile the list of maintainers for a project and ensure that they are registered
with their chosen services.
