# CLAUDE.md

This file provides guidance to Claude Code when working with the code-scanners operator.

## Overview

A Kubernetes operator built with Kubebuilder v4 that manages code scanning integrations for CNCF projects. It defines two CRDs (`CodeScannerFossa` and `CodeScannerSnyk`) that create ConfigMaps with lineage tracking via owner references and annotations.

| Attribute | Value |
|-----------|-------|
| API Group | `maintainer-d.cncf.io` |
| API Version | `v1alpha1` |
| Go Version | 1.24+ |
| Controller-Runtime | v0.22.4 |

## Directory Structure

```
code-scanners/
├── api/v1alpha1/           # CRD type definitions (Spec, Status, markers)
├── internal/controller/    # Reconciliation logic and tests
├── cmd/main.go             # Manager entry point
├── config/
│   ├── crd/bases/          # Generated CRD YAMLs
│   ├── rbac/               # RBAC roles and bindings
│   ├── manager/            # Deployment manifests
│   └── samples/            # Example CR YAMLs
├── test/e2e/               # End-to-end tests (Ginkgo + Kind)
└── hack/                   # Build utilities
```

## Development Commands

```bash
# Generate CRDs and RBAC from kubebuilder markers
make manifests generate

# Build and test
make build                  # Compile to bin/manager
make test                   # Unit tests with envtest
make lint                   # Run golangci-lint

# Run locally against current kubeconfig
make run

# Deploy to cluster
make install                # Install CRDs only
make deploy IMG=<image>     # Deploy controller

# E2E testing (creates Kind cluster)
make test-e2e
```

## Architecture

### Reconciliation Flow

1. Fetch CR → return nil if not found (deleted)
2. Build ConfigMap with `CodeScanner` and `ProjectName` data keys
3. Set owner reference (`controller: true`, `blockOwnerDeletion: true`)
4. Create or update ConfigMap
5. Add `maintainer-d.cncf.io/configmap-ref` annotation to CR
6. Update CR status with `configMapRef`

### Key Files

| File | Purpose |
|------|---------|
| `api/v1alpha1/*_types.go` | CRD definitions with kubebuilder markers |
| `internal/controller/*_controller.go` | Reconciler implementations |
| `internal/controller/constants.go` | Shared constants (annotation keys, scanner types) |
| `internal/controller/suite_test.go` | envtest setup for unit tests |

## Conventions

### Kubebuilder Markers

```go
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:validation:Required
// +kubebuilder:printcolumn:name="Project",type=string,JSONPath=`.spec.projectName`
```

### Error Handling

Always return empty `ctrl.Result{}` when returning an error:

```go
// Correct
if err != nil {
    return ctrl.Result{}, err
}

// Wrong - don't combine RequeueAfter with error
if err != nil {
    return ctrl.Result{RequeueAfter: time.Second}, err
}
```

### Testing

- Unit tests use envtest (in-process etcd + API server)
- Use standard Go testing, NOT Ginkgo/Gomega for controller tests
- E2E tests use Ginkgo + Kind cluster

```go
// Correct - standard Go test
func TestReconciler_CreatesConfigMap(t *testing.T) {
    if got != want {
        t.Errorf("got %v, want %v", got, want)
    }
}
```

### Logging

Use structured logging from context:

```go
log := logf.FromContext(ctx)
log.Info("Reconciliation complete", "configMap", configMapRef)
log.Error(err, "Failed to create ConfigMap")
```

## CRD Structure

```yaml
apiVersion: maintainer-d.cncf.io/v1alpha1
kind: CodeScannerFossa  # or CodeScannerSnyk
metadata:
  name: zot-fossa
  namespace: code-scanners
  annotations:
    maintainer-d.cncf.io/configmap-ref: code-scanners/zot-fossa  # Added by controller
spec:
  projectName: zot  # Required, min length 1
status:
  configMapRef: code-scanners/zot-fossa  # Updated by controller
  conditions: []  # Standard metav1.Condition
```

## KDP Sync Agent Configuration

The `hack/kdp-syncagent/` directory contains configuration for deploying the kcp API sync agent, which enables the code-scanners CRDs to be available as a Kubermatic Developer Platform (KDP) service.

### Directory Structure

```
hack/kdp-syncagent/
├── code-scanners_api_syncagent_values.yaml   # Helm values for sync agent
├── code-scanners_published_resources.yaml    # PublishedResource CRs
└── rbac/                                      # RBAC manifests (Kustomize)
    ├── kustomization.yaml
    ├── cluster-role-aggregated.yaml
    ├── cluster-role-binding.yaml
    ├── cluster-role-configmap.yaml
    └── cluster-role-resources.yaml
```

### Configuration Files

| File | Purpose |
|------|---------|
| `code-scanners_api_syncagent_values.yaml` | Helm values for `kcp/api-syncagent` chart |
| `code-scanners_published_resources.yaml` | Defines `CodeScannerFossa` and `CodeScannerSnyk` as published resources with related ConfigMap references |
| `rbac/` | Kustomize-based RBAC granting sync agent permissions for CRDs and ConfigMaps |

### Deployment Commands

```bash
# Create namespace and kubeconfig secret
kubectl create namespace code-scanners
kubectl create secret generic syncagent-code-scanner-svc-kubeconfig \
    --from-file=kubeconfig=tmp/code-scanners.maintainer-d.cncf.io-kubeconfig

# Deploy sync agent via Helm
helm repo add kcp https://kcp-dev.github.io/helm-charts
helm install kcp-api-syncagent kcp/api-syncagent \
    --values hack/kdp-syncagent/code-scanners_api_syncagent_values.yaml \
    --version=0.2.0 \
    --namespace code-scanners

# Apply RBAC
kubectl kustomize hack/kdp-syncagent/rbac | kubectl apply -f -

# Publish resources
kubectl apply -f hack/kdp-syncagent/code-scanners_published_resources.yaml
```

For detailed setup instructions including KDP service creation, see `hack/kdp_syncagent.md`.

## Implementation Status

Track progress in `PLAN.md`. Current phases:
- Phase 1-2: Complete (scaffolding, API types)
- Phase 3: Complete (controller implementation)
- Phase 4: In progress (unit testing)
- Phase 5: Pending (deployment validation)
