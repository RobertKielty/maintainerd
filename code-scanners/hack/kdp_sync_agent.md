# KDP configuration

1. Go to [services.cncf.io](https://services.cncf.io/)), select the "staff" workspace.

2. Add service to KDP and get kubeconfig neccesary for api sync agent (see [docs](https://docs.kubermatic.com/developer-platform/service-providers)). **Do not activate it yet**

   Through webui:
   - Go to Services
   - Create Service
   - Fill the values:
     - Title: Code Scanning
     - Name (Uniqe): code-scanners.maintainer-d.cncf.io
     - Short Description: Code Scanning with Fossa or Snyk
     - Category: Other
     - Documentation Url: _leave empty_
     - API SyncAgent KubeConfig:
       - code-scanners.maintainer-d.cncf.io (RFC 1123)
       - Namespace: default
   - Logo: TBA

   Programatically:
   - TBA

3. Download the _kubeconfig_ for the service and save it to _tmp/sync_agent_kubeconfig_:

4.
