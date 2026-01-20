# KDP configuration

## Create (in-active) Service (KDP)

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

3. Download the _kubeconfig_ (`code-scanners.maintainer-d.cncf.io-kubeconfig`) for the service and save it to _tmp/_ directory.

## Configuring (kdp/kcp) Sync Agent (Service Cluster)

1.On the service cluster:

    ```bash
    kubectx context-cdv2c4jfn5q

    # if does not exist
    kubectl create namespace code-scanners
    ```

    ```bash
    kubectl create secret generic \
        syncagent-code-scanner-svc-kubeconfig \
        --from-file=kubeconfig=tmp/code-scanners.maintainer-d.cncf.io-kubeconfig
    ```

    ```bash
    helm repo add kcp https://kcp-dev.github.io/helm-charts
    helm repo update

    helm install kcp-api-syncagent kcp/api-syncagent \
      --values hack/kdp-syncagent/code-scanners_api_syncagent_values.yaml \
      --version=0.2.0 \
      --namespace code-scanners
    ```

2.
