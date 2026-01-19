# Code Scanning Operator - Implementation Plan

This document outlines the iterative implementation plan for the Code Scanning Operator. The operator manages two CRDs (`CodeScannerFossa` and `CodeScannerSnyk`) and creates corresponding ConfigMaps with lineage tracking.

## Overview

| Attribute | Value |
|-----------|-------|
| API Group | `maintainer-d.cncf.io` |
| API Version | `v1alpha1` |
| Namespace | `code-scanners` |
| Implementation Directory | `code-scanners/` |
| Kubebuilder Version | v4.x (latest) |
| Go Version | 1.22+ |

## Phase 1: Project Scaffolding

**Goal:** Initialize the Kubebuilder project structure with proper configuration.

### 1.1 Install Kubebuilder (if needed)

```bash
curl -L -o kubebuilder "https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)"
chmod +x kubebuilder && sudo mv kubebuilder /usr/local/bin/
kubebuilder version  # Verify installation
```

### 1.2 Initialize Project

```bash
cd code-scanners
kubebuilder init --domain maintainer-d.cncf.io --repo github.com/cncf/maintainer-d/code-scanners
```

### 1.3 Create API Scaffolds

```bash
# Create CodeScannerFossa API
kubebuilder create api --group "" --version v1alpha1 --kind CodeScannerFossa --resource --controller

# Create CodeScannerSnyk API
kubebuilder create api --group "" --version v1alpha1 --kind CodeScannerSnyk --resource --controller
```

### 1.4 Remove Ginkgo/Gomega (use standard Go tests)

After scaffolding, replace `internal/controller/suite_test.go` with standard Go test setup using `envtest`.

### Deliverables
- [x] Initialized project with `PROJECT` file
- [x] Basic directory structure
- [x] `go.mod` and `go.sum`
- [x] Scaffolded API and controller files

---

## Phase 2: API Types Definition

**Goal:** Define the CRD types with proper spec and status fields.

### 2.1 CodeScannerFossa Types

**File:** `api/v1alpha1/codescannerfossa_types.go`

```go
package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CodeScannerFossaSpec defines the desired state of CodeScannerFossa
type CodeScannerFossaSpec struct {
    // ProjectName is the name of the CNCF project to scan
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:MinLength=1
    ProjectName string `json:"projectName"`
}

// CodeScannerFossaStatus defines the observed state of CodeScannerFossa
type CodeScannerFossaStatus struct {
    // ConfigMapRef is the namespace/name reference to the created ConfigMap
    // +optional
    ConfigMapRef string `json:"configMapRef,omitempty"`

    // Conditions represent the latest available observations of the resource's state
    // +optional
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Project",type=string,JSONPath=`.spec.projectName`
// +kubebuilder:printcolumn:name="ConfigMap",type=string,JSONPath=`.status.configMapRef`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CodeScannerFossa is the Schema for the codescannerfossas API
type CodeScannerFossa struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   CodeScannerFossaSpec   `json:"spec,omitempty"`
    Status CodeScannerFossaStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CodeScannerFossaList contains a list of CodeScannerFossa
type CodeScannerFossaList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []CodeScannerFossa `json:"items"`
}

func init() {
    SchemeBuilder.Register(&CodeScannerFossa{}, &CodeScannerFossaList{})
}
```

### 2.2 CodeScannerSnyk Types

**File:** `api/v1alpha1/codescannersnyk_types.go`

```go
package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CodeScannerSnykSpec defines the desired state of CodeScannerSnyk
type CodeScannerSnykSpec struct {
    // ProjectName is the name of the CNCF project to scan
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:MinLength=1
    ProjectName string `json:"projectName"`
}

// CodeScannerSnykStatus defines the observed state of CodeScannerSnyk
type CodeScannerSnykStatus struct {
    // ConfigMapRef is the namespace/name reference to the created ConfigMap
    // +optional
    ConfigMapRef string `json:"configMapRef,omitempty"`

    // Conditions represent the latest available observations of the resource's state
    // +optional
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Project",type=string,JSONPath=`.spec.projectName`
// +kubebuilder:printcolumn:name="ConfigMap",type=string,JSONPath=`.status.configMapRef`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CodeScannerSnyk is the Schema for the codescannersnyk API
type CodeScannerSnyk struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   CodeScannerSnykSpec   `json:"spec,omitempty"`
    Status CodeScannerSnykStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CodeScannerSnykList contains a list of CodeScannerSnyk
type CodeScannerSnykList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []CodeScannerSnyk `json:"items"`
}

func init() {
    SchemeBuilder.Register(&CodeScannerSnyk{}, &CodeScannerSnykList{})
}
```

### 2.3 Generate CRDs

```bash
make manifests generate
```

### Deliverables
- [x] `codescannerfossa_types.go` with spec and status
- [x] `codescannersnyk_types.go` with spec and status
- [x] Generated CRD YAMLs in `config/crd/bases/`
- [x] Generated deep copy functions

---

## Phase 3: Controller Implementation

**Goal:** Implement reconciliation logic that creates ConfigMaps and adds lineage annotations.

### 3.1 Constants and Annotations

Define shared constants for annotation keys:

```go
const (
    // AnnotationConfigMapRef is the annotation key for ConfigMap reference
    AnnotationConfigMapRef = "maintainer-d.cncf.io/configmap-ref"

    // ConfigMapKeyCodeScanner is the ConfigMap data key for scanner type
    ConfigMapKeyCodeScanner = "CodeScanner"

    // ConfigMapKeyProjectName is the ConfigMap data key for project name
    ConfigMapKeyProjectName = "ProjectName"

    // ScannerTypeFossa identifies the Fossa scanner
    ScannerTypeFossa = "Fossa"

    // ScannerTypeSnyk identifies the Snyk scanner
    ScannerTypeSnyk = "Snyk"
)
```

### 3.2 CodeScannerFossa Controller

**File:** `internal/controller/codescannerfossa_controller.go`

```go
package controller

import (
    "context"
    "fmt"

    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/log"

    maintainerdv1alpha1 "github.com/cncf/maintainer-d/code-scanners/api/v1alpha1"
)

// CodeScannerFossaReconciler reconciles a CodeScannerFossa object
type CodeScannerFossaReconciler struct {
    client.Client
    Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=maintainer-d.cncf.io,resources=codescannerfossas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=maintainer-d.cncf.io,resources=codescannerfossas/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=maintainer-d.cncf.io,resources=codescannerfossas/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

func (r *CodeScannerFossaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := log.FromContext(ctx)

    // 1. Fetch the CodeScannerFossa instance
    fossa := &maintainerdv1alpha1.CodeScannerFossa{}
    if err := r.Get(ctx, req.NamespacedName, fossa); err != nil {
        if errors.IsNotFound(err) {
            log.Info("CodeScannerFossa resource not found, ignoring")
            return ctrl.Result{}, nil
        }
        log.Error(err, "Failed to get CodeScannerFossa")
        return ctrl.Result{}, err
    }

    // 2. Build the ConfigMap
    configMap := r.configMapForFossa(fossa)

    // 3. Set owner reference for garbage collection
    if err := ctrl.SetControllerReference(fossa, configMap, r.Scheme); err != nil {
        log.Error(err, "Failed to set owner reference on ConfigMap")
        return ctrl.Result{}, err
    }

    // 4. Create or Update the ConfigMap
    existingCM := &corev1.ConfigMap{}
    err := r.Get(ctx, client.ObjectKeyFromObject(configMap), existingCM)
    if err != nil && errors.IsNotFound(err) {
        log.Info("Creating ConfigMap", "name", configMap.Name, "namespace", configMap.Namespace)
        if err := r.Create(ctx, configMap); err != nil {
            log.Error(err, "Failed to create ConfigMap")
            return ctrl.Result{}, err
        }
    } else if err != nil {
        log.Error(err, "Failed to get ConfigMap")
        return ctrl.Result{}, err
    } else {
        // Update existing ConfigMap
        existingCM.Data = configMap.Data
        if err := r.Update(ctx, existingCM); err != nil {
            log.Error(err, "Failed to update ConfigMap")
            return ctrl.Result{}, err
        }
    }

    // 5. Add lineage annotation to CR
    configMapRef := fmt.Sprintf("%s/%s", configMap.Namespace, configMap.Name)
    if fossa.Annotations == nil {
        fossa.Annotations = make(map[string]string)
    }
    if fossa.Annotations[AnnotationConfigMapRef] != configMapRef {
        fossa.Annotations[AnnotationConfigMapRef] = configMapRef
        if err := r.Update(ctx, fossa); err != nil {
            log.Error(err, "Failed to update CodeScannerFossa annotation")
            return ctrl.Result{}, err
        }
    }

    // 6. Update status
    fossa.Status.ConfigMapRef = configMapRef
    if err := r.Status().Update(ctx, fossa); err != nil {
        log.Error(err, "Failed to update CodeScannerFossa status")
        return ctrl.Result{}, err
    }

    log.Info("Reconciliation complete", "configMap", configMapRef)
    return ctrl.Result{}, nil
}

func (r *CodeScannerFossaReconciler) configMapForFossa(fossa *maintainerdv1alpha1.CodeScannerFossa) *corev1.ConfigMap {
    return &corev1.ConfigMap{
        ObjectMeta: metav1.ObjectMeta{
            Name:      fossa.Name,
            Namespace: fossa.Namespace,
        },
        Data: map[string]string{
            ConfigMapKeyCodeScanner: ScannerTypeFossa,
            ConfigMapKeyProjectName: fossa.Spec.ProjectName,
        },
    }
}

func (r *CodeScannerFossaReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&maintainerdv1alpha1.CodeScannerFossa{}).
        Owns(&corev1.ConfigMap{}).
        Named("codescannerfossa").
        Complete(r)
}
```

### 3.3 CodeScannerSnyk Controller

**File:** `internal/controller/codescannersnyk_controller.go`

Follow the same pattern as the Fossa controller, replacing:
- `CodeScannerFossa` → `CodeScannerSnyk`
- `ScannerTypeFossa` → `ScannerTypeSnyk`
- `codescannerfossa` → `codescannersnyk`

### 3.4 RBAC Configuration

Ensure the controller has permissions for ConfigMaps:

```yaml
# config/rbac/role.yaml (auto-generated from markers)
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

### Deliverables
- [x] `codescannerfossa_controller.go` with reconciliation logic
- [x] `codescannersnyk_controller.go` with reconciliation logic
- [x] RBAC rules for ConfigMap access
- [x] Owner references for garbage collection

---

## Phase 4: Unit Testing

**Goal:** Implement comprehensive tests using standard Go testing (no Ginkgo/Gomega).

### 4.1 Test Setup with envtest

**File:** `internal/controller/suite_test.go`

```go
package controller

import (
    "context"
    "os"
    "path/filepath"
    "testing"

    "k8s.io/client-go/kubernetes/scheme"
    "k8s.io/client-go/rest"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/envtest"
    "sigs.k8s.io/controller-runtime/pkg/log/zap"

    maintainerdv1alpha1 "github.com/cncf/maintainer-d/code-scanners/api/v1alpha1"
)

var (
    cfg       *rest.Config
    k8sClient client.Client
    testEnv   *envtest.Environment
    ctx       context.Context
    cancel    context.CancelFunc
)

func TestMain(m *testing.M) {
    ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

    ctx, cancel = context.WithCancel(context.Background())

    testEnv = &envtest.Environment{
        CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
        ErrorIfCRDPathMissing: true,
    }

    var err error
    cfg, err = testEnv.Start()
    if err != nil {
        panic(err)
    }

    err = maintainerdv1alpha1.AddToScheme(scheme.Scheme)
    if err != nil {
        panic(err)
    }

    k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
    if err != nil {
        panic(err)
    }

    code := m.Run()

    cancel()
    _ = testEnv.Stop()
    os.Exit(code)
}
```

### 4.2 CodeScannerFossa Controller Tests

**File:** `internal/controller/codescannerfossa_controller_test.go`

```go
package controller

import (
    "context"
    "testing"
    "time"

    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/types"
    "sigs.k8s.io/controller-runtime/pkg/client"

    maintainerdv1alpha1 "github.com/cncf/maintainer-d/code-scanners/api/v1alpha1"
)

func TestCodeScannerFossaReconciler_CreatesConfigMap(t *testing.T) {
    ctx := context.Background()
    namespace := "test-ns"
    name := "test-fossa"
    projectName := "zot"

    // Create namespace
    ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
    if err := k8sClient.Create(ctx, ns); err != nil {
        t.Fatalf("Failed to create namespace: %v", err)
    }
    defer k8sClient.Delete(ctx, ns)

    // Create CodeScannerFossa
    fossa := &maintainerdv1alpha1.CodeScannerFossa{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: namespace,
        },
        Spec: maintainerdv1alpha1.CodeScannerFossaSpec{
            ProjectName: projectName,
        },
    }
    if err := k8sClient.Create(ctx, fossa); err != nil {
        t.Fatalf("Failed to create CodeScannerFossa: %v", err)
    }
    defer k8sClient.Delete(ctx, fossa)

    // Wait for ConfigMap to be created
    var configMap corev1.ConfigMap
    key := types.NamespacedName{Name: name, Namespace: namespace}

    // Poll for ConfigMap creation
    deadline := time.Now().Add(10 * time.Second)
    for time.Now().Before(deadline) {
        if err := k8sClient.Get(ctx, key, &configMap); err == nil {
            break
        }
        time.Sleep(100 * time.Millisecond)
    }

    // Verify ConfigMap
    if configMap.Data[ConfigMapKeyCodeScanner] != ScannerTypeFossa {
        t.Errorf("Expected CodeScanner=%s, got %s", ScannerTypeFossa, configMap.Data[ConfigMapKeyCodeScanner])
    }
    if configMap.Data[ConfigMapKeyProjectName] != projectName {
        t.Errorf("Expected ProjectName=%s, got %s", projectName, configMap.Data[ConfigMapKeyProjectName])
    }

    // Verify annotation on CR
    var updatedFossa maintainerdv1alpha1.CodeScannerFossa
    if err := k8sClient.Get(ctx, key, &updatedFossa); err != nil {
        t.Fatalf("Failed to get updated CodeScannerFossa: %v", err)
    }

    expectedRef := namespace + "/" + name
    if updatedFossa.Annotations[AnnotationConfigMapRef] != expectedRef {
        t.Errorf("Expected annotation %s=%s, got %s", AnnotationConfigMapRef, expectedRef, updatedFossa.Annotations[AnnotationConfigMapRef])
    }
}
```

### 4.3 Test Cases to Implement

| Test Case | Description |
|-----------|-------------|
| `TestCreatesConfigMap` | Verify ConfigMap is created with correct data |
| `TestSetsOwnerReference` | Verify ConfigMap has owner reference to CR |
| `TestAddsLineageAnnotation` | Verify CR is annotated with ConfigMap reference |
| `TestUpdatesConfigMap` | Verify ConfigMap is updated when spec changes |
| `TestDeletesCascade` | Verify ConfigMap is deleted when CR is deleted |
| `TestIdempotent` | Verify reconcile is idempotent (no duplicate resources) |
| `TestHandlesNotFound` | Verify graceful handling when CR is deleted |

### Deliverables
- [ ] `suite_test.go` with envtest setup
- [ ] `codescannerfossa_controller_test.go` with comprehensive tests
- [ ] `codescannersnyk_controller_test.go` with comprehensive tests
- [ ] All tests passing with `go test ./...`

---

## Phase 5: Deployment & Validation

**Goal:** Deploy the operator to the development cluster and validate functionality.

### 5.1 Update Namespace Configuration

**File:** `config/default/kustomization.yaml`

```yaml
namespace: code-scanners
```

### 5.2 Build and Deploy

```bash
# Generate manifests and build
make manifests generate fmt vet

# Install CRDs
make install

# Build and push image (if using remote cluster)
make docker-build docker-push IMG=<registry>/code-scanners:v0.1.0

# Deploy controller
make deploy IMG=<registry>/code-scanners:v0.1.0
```

### 5.3 Create Sample Resources

**File:** `config/samples/v1alpha1_codescannerfossa.yaml`

```yaml
apiVersion: maintainer-d.cncf.io/v1alpha1
kind: CodeScannerFossa
metadata:
  name: zot-fossa
  namespace: code-scanners
spec:
  projectName: zot
```

**File:** `config/samples/v1alpha1_codescannersnyk.yaml`

```yaml
apiVersion: maintainer-d.cncf.io/v1alpha1
kind: CodeScannerSnyk
metadata:
  name: zot-snyk
  namespace: code-scanners
spec:
  projectName: zot
```

### 5.4 Validation Commands

```bash
# Connect to dev cluster
kubectx context-cdv2c4jfn5q

# Create namespace
kubectl create namespace code-scanners

# Apply sample resources
kubectl apply -f config/samples/

# Verify CRDs
kubectl get crds | grep maintainer-d

# Verify resources created
kubectl get codescannerfossas -n code-scanners
kubectl get codescannersnyk -n code-scanners

# Verify ConfigMaps
kubectl get configmaps -n code-scanners

# Check ConfigMap content
kubectl get configmap zot-fossa -n code-scanners -o yaml

# Check CR annotations
kubectl get codescannerfossa zot-fossa -n code-scanners -o jsonpath='{.metadata.annotations}'

# Check controller logs
kubectl logs -n code-scanners -l control-plane=controller-manager
```

### 5.5 Expected Output

**ConfigMap:**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: zot-fossa
  namespace: code-scanners
  ownerReferences:
  - apiVersion: maintainer-d.cncf.io/v1alpha1
    kind: CodeScannerFossa
    name: zot-fossa
    uid: <uuid>
    controller: true
    blockOwnerDeletion: true
data:
  CodeScanner: Fossa
  ProjectName: zot
```

**CodeScannerFossa with annotation:**
```yaml
apiVersion: maintainer-d.cncf.io/v1alpha1
kind: CodeScannerFossa
metadata:
  name: zot-fossa
  namespace: code-scanners
  annotations:
    maintainer-d.cncf.io/configmap-ref: code-scanners/zot-fossa
spec:
  projectName: zot
status:
  configMapRef: code-scanners/zot-fossa
```

### Deliverables
- [ ] CRDs installed on cluster
- [ ] Controller deployed and running
- [ ] Sample resources created
- [ ] ConfigMaps created with correct data
- [ ] Lineage annotations present on CRs
- [ ] Owner references set for garbage collection

---

## Summary: Implementation Checklist

### Phase 1: Project Scaffolding
- [x] Install Kubebuilder v4.x
- [x] Initialize project in `code-scanners/`
- [x] Create API scaffolds
- [x] Remove Ginkgo/Gomega

### Phase 2: API Types
- [x] Define CodeScannerFossa types
- [x] Define CodeScannerSnyk types
- [x] Add validation markers
- [x] Generate CRDs

### Phase 3: Controllers
- [x] Implement Fossa reconciler
- [x] Implement Snyk reconciler
- [x] Set up RBAC
- [x] Configure owner references

### Phase 4: Testing
- [ ] Set up envtest
- [ ] Write Fossa controller tests
- [ ] Write Snyk controller tests
- [ ] Achieve test coverage

### Phase 5: Deployment
- [ ] Configure namespace
- [ ] Deploy to dev cluster
- [ ] Validate ConfigMap creation
- [ ] Validate lineage annotations

---

## Notes

### Error Handling Convention

Always return empty `ctrl.Result{}` when returning an error:

```go
// ✅ Correct
if err != nil {
    return ctrl.Result{}, err
}

// ❌ Wrong
if err != nil {
    return ctrl.Result{RequeueAfter: 30 * time.Second}, err
}
```

### Testing Convention

Use standard Go tests, NOT Ginkgo/Gomega:

```go
// ✅ Correct
func TestReconciler_CreatesConfigMap(t *testing.T) {
    if got != want {
        t.Errorf("got %v, want %v", got, want)
    }
}

// ❌ Wrong (Ginkgo)
var _ = Describe("Reconciler", func() {
    It("creates ConfigMap", func() {
        Expect(got).To(Equal(want))
    })
})
```

### Native metav1 Types

Use `metav1.Condition` for status conditions (not custom condition types):

```go
// ✅ Correct
Conditions []metav1.Condition `json:"conditions,omitempty"`

// ❌ Wrong
Conditions []CustomCondition `json:"conditions,omitempty"`
```
