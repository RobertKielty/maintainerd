# kdp-workspace operator support for adding access to staffmembers

Goal: add staff members to the workspaces (that the operator creates already) with the `kdp:owner` role

Provide 2 alternative solutions, if any information is missing ask, and document the answer in @PLAN_STAFF_MEMBERS_SUPPORT.md (this file). One of them must be multi-controller.

## Input

CRD to observe:

```bash
kubectx context-cdv2c4jfn5q

kubectl get staffmembers.maintainer-d.cncf.io -n maintainerd

# notice the email
kubectl get staffmembers.maintainer-d.cncf.io -n maintainerd  wojciech.barczynski-kubermatic.com 
```

## Output

1. We want to give access `kdp:owner` in every workspace that kdp-workspace-operator created:

```bash
KUBECONFIG=$(git rev-parse --show-toplevel)/tmp/kdp-cluster-cncf/kubeconfig-admin k get clusterroles.rbac.authorization.k8s.io  "kdp:owner" -o yaml
```

2. We will do it by creating a clusterBinding inside a given workspace, e.g., `zod` (` KUBECONFIG=$(git rev-parse --show-toplevel)/tmp/kdp-cluster-cncf/kubeconfig-admin k get ws zot`):

```yaml
                  apiVersion: rbac.authorization.k8s.io/v1
                  kind: ClusterRoleBinding
                  metadata:
                    name: cncf-staff-access # <- propose a better name
                  subjects:
                    - kind: User
                      name: oidc:staff-one@example.test
                    - kind: User
                      name: oidc:staff-two@example.test
                    - kind: User
                      name: oidc:staff-three@example.test # notice oidc: prefix, we should always have "odic"
                    - kind: User
                      name: oidc:staff-four@example.test
                  roleRef:
                    apiGroup: rbac.authorization.k8s.io
                    kind: ClusterRole
                    name: kdp:owner
```

---

## Requirements Clarification (Q&A)

The following questions were asked and answered to clarify implementation details:

1. **Email field to use**: `spec.primaryEmail` from StaffMember resources
2. **ClusterRole name**: `kdp:owner` (with colon, as mentioned in the goal)
3. **Handle StaffMember deletion**: Yes, remove the user from all ClusterRoleBindings when a StaffMember is deleted
4. **Timing**: Only create/update ClusterRoleBindings in workspaces that are in Ready phase

---

## Annotations and Status Tracking

The operator will add annotations to track synchronization status and metadata. We use two complementary annotation strategies:

### StaffMember Annotations (Option 2)

Annotations added to StaffMember resources after successful reconciliation:

```go
const (
    // AnnotationStaffLastSynced tracks when this staff member was last synced to workspaces
    AnnotationStaffLastSynced = "kdp-workspaces.cncf.io/last-synced"

    // AnnotationStaffSyncStatus tracks sync status: "success", "partial", "error"
    AnnotationStaffSyncStatus = "kdp-workspaces.cncf.io/sync-status"

    // AnnotationStaffWorkspaceCount tracks number of workspaces this member has access to
    AnnotationStaffWorkspaceCount = "kdp-workspaces.cncf.io/workspace-count"
)
```

**Example after successful reconciliation:**

```yaml
apiVersion: maintainer-d.cncf.io/v1alpha1
kind: StaffMember
metadata:
  name: wojciech.barczynski-kubermatic.com
  namespace: maintainerd
  annotations:
    kdp-workspaces.cncf.io/last-synced: "2025-01-14T10:30:45Z"
    kdp-workspaces.cncf.io/sync-status: "success"
    kdp-workspaces.cncf.io/workspace-count: "247"
spec:
  displayName: "Wojciech Barczynski"
  primaryEmail: "staff-one@example.test"
```

**Update Logic in StaffMemberReconciler:**

```go
func (r *StaffMemberReconciler) updateStaffMemberAnnotations(ctx context.Context,
    staffMemberName string, workspaceCount int, syncStatus string) error {

    // Fetch the StaffMember
    staffMember := &unstructured.Unstructured{}
    staffMember.SetGroupVersionKind(schema.GroupVersionKind{
        Group: "maintainer-d.cncf.io", Version: "v1alpha1", Kind: "StaffMember"})

    if err := r.Get(ctx, client.ObjectKey{
        Name: staffMemberName, Namespace: r.StaffMemberNamespace}, staffMember); err != nil {
        return err
    }

    // Create patch
    patch := client.MergeFrom(staffMember.DeepCopy())

    // Update annotations
    annotations := staffMember.GetAnnotations()
    if annotations == nil {
        annotations = make(map[string]string)
    }
    annotations[AnnotationStaffLastSynced] = time.Now().Format(time.RFC3339)
    annotations[AnnotationStaffSyncStatus] = syncStatus
    annotations[AnnotationStaffWorkspaceCount] = fmt.Sprintf("%d", workspaceCount)
    staffMember.SetAnnotations(annotations)

    // Apply patch
    return r.Patch(ctx, staffMember, patch)
}
```

**Benefits:**
- Visibility into sync status per staff member via `kubectl get staffmembers -o yaml`
- Debugging: can identify which staff members have stale syncs
- Observability: detect when staff members fail to sync to workspaces

**Trade-offs:**
- Operator writes to resources it doesn't strictly own (shared with maintainer-d)
- Extra API calls after each reconciliation
- Annotations may not capture full many-to-many relationship complexity

### ClusterRoleBinding Annotations (Option 3)

Annotations added to ClusterRoleBindings created in each workspace:

```go
const (
    // Annotations for ClusterRoleBindings
    AnnotationBindingLastSynced = "kdp-workspaces.cncf.io/last-synced"
    AnnotationBindingStaffCount = "kdp-workspaces.cncf.io/staff-count"
    AnnotationBindingManagedBy  = "kdp-workspaces.cncf.io/managed-by"
    AnnotationBindingSourceNamespace = "kdp-workspaces.cncf.io/source-namespace"

    // Labels
    LabelManagedBy = "managed-by"
)
```

**Example ClusterRoleBinding with annotations:**

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cncf-staff-access
  annotations:
    kdp-workspaces.cncf.io/last-synced: "2025-01-14T10:30:45Z"
    kdp-workspaces.cncf.io/staff-count: "12"
    kdp-workspaces.cncf.io/managed-by: "kdp-ws-operator"
    kdp-workspaces.cncf.io/source-namespace: "maintainerd"
  labels:
    managed-by: kdp-ws-operator
subjects:
  - kind: User
    name: oidc:staff-one@example.test
  - kind: User
    name: oidc:staff-two@example.test
  # ... (10 more staff members)
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kdp:owner
```

**Update Logic in CreateOrUpdateStaffBinding:**

```go
func (c *Client) CreateOrUpdateStaffBinding(ctx context.Context,
    workspaceName string, subjects []rbacv1.Subject) error {

    // Build ClusterRoleBinding with annotations
    binding := &rbacv1.ClusterRoleBinding{
        ObjectMeta: metav1.ObjectMeta{
            Name: "cncf-staff-access",
            Annotations: map[string]string{
                AnnotationBindingLastSynced:      time.Now().Format(time.RFC3339),
                AnnotationBindingStaffCount:      fmt.Sprintf("%d", len(subjects)),
                AnnotationBindingManagedBy:       "kdp-ws-operator",
                AnnotationBindingSourceNamespace: "maintainerd",
            },
            Labels: map[string]string{
                LabelManagedBy: "kdp-ws-operator",
            },
        },
        Subjects: subjects,
        RoleRef: rbacv1.RoleRef{
            APIGroup: "rbac.authorization.k8s.io",
            Kind:     "ClusterRole",
            Name:     "kdp:owner",
        },
    }

    // Create or update in workspace...
    return nil
}
```

**Benefits:**
- ✅ Metadata on resources we create and own
- ✅ Tracks per-workspace sync status
- ✅ No conflicts with maintainer-d operator
- ✅ Label selector for finding all managed bindings: `managed-by=kdp-ws-operator`
- ✅ Can verify each workspace has current staff list

**Use Cases:**
- Debug: "When was this workspace's staff access last updated?"
- Audit: "Which workspaces have staff access configured?"
- Verification: "Does this workspace have all 12 staff members?"

### Combined Strategy

Using both annotation strategies provides comprehensive observability:

1. **StaffMember annotations** → "Is this staff member synced? To how many workspaces?"
2. **ClusterRoleBinding annotations** → "When was this workspace's staff access updated? How many staff?"

**Example debugging workflow:**

```bash
# Check if staff member is synced
kubectl get staffmember wojciech.barczynski-kubermatic.com -n maintainerd -o jsonpath='{.metadata.annotations}'

# Output:
# {
#   "kdp-workspaces.cncf.io/last-synced": "2025-01-14T10:30:45Z",
#   "kdp-workspaces.cncf.io/sync-status": "success",
#   "kdp-workspaces.cncf.io/workspace-count": "247"
# }

# Check specific workspace's staff binding
KUBECONFIG=./tmp/kdp-cluster-cncf/kubeconfig-admin \
  kubectl get clusterrolebinding cncf-staff-access -o yaml

# Find all workspaces with staff access
KUBECONFIG=./tmp/kdp-cluster-cncf/kubeconfig-admin \
  kubectl get clusterrolebinding -A -l managed-by=kdp-ws-operator
```

---

## Current Architecture Analysis

### Existing Operator Structure

The kdp-workspace operator currently consists of:

**Components:**
- `ProjectReconciler` (kdp-workspaces/internal/controller/project_controller.go:52) - watches Project CRDs on Service Cluster
- KCP Client (kdp-workspaces/internal/kcp/client.go) - manages connection to KDP cluster
- Workspace Manager (kdp-workspaces/internal/kcp/workspace.go) - creates/manages workspaces

**Current Flow:**
1. ProjectReconciler watches `projects.maintainer-d.cncf.io` resources in `maintainerd` namespace
2. On reconcile, loads KCP config from ConfigMap and Secret
3. Creates workspace of type `kdp-organization` in KDP cluster
4. Waits for workspace to reach Ready phase
5. Updates Project annotations with workspace name, URL, and phase

**Workspace Creation:**
- Workspace name derived from Project name (lowercased)
- Annotations added: `managed-by=kdp-ws-operator`, `organization-namespace`, `organization-name`
- Workspace type: `kdp-organization` at path `root` (configurable)

### StaffMembers CRD Structure

Located in: apis/maintainers/v1alpha1/types.go:91

```go
type StaffMemberSpec struct {
    DisplayName   string
    PrimaryEmail  string              // ← We'll use this for OIDC subjects
    GitHubAccount string
    GitHubEmail   string
    FoundationRef *ResourceReference
    RegisteredAt  *metav1.Time
    ExternalIDs   map[string]string
}
```

StaffMembers are cluster-scoped resources in the `maintainerd` namespace on the Service Cluster.

---

## Solution 1: Single Controller Extension (Simpler)

### Overview

Extend the existing `ProjectReconciler` to also manage staff access by creating ClusterRoleBindings after workspace creation.

### Approach

**Modified Reconciliation Flow:**
1. ProjectReconciler creates/gets workspace (existing logic)
2. Wait for workspace to reach Ready phase (existing logic)
3. **NEW:** Fetch all StaffMembers from `maintainerd` namespace
4. **NEW:** Build ClusterRoleBinding with oidc-prefixed subjects
5. **NEW:** Create/update ClusterRoleBinding `cncf-staff-access` in the workspace
6. Update Project status (existing logic)

### Implementation Details

**File Changes:**

1. **kdp-workspaces/internal/controller/project_controller.go**
   - Add `StaffMemberNamespace` field to ProjectReconciler config
   - After workspace ready (line 206), call new method `r.reconcileStaffAccess(ctx, kcpClient, workspaceName)`
   - Add RBAC marker: `// +kubebuilder:rbac:groups=maintainer-d.cncf.io,resources=staffmembers,verbs=get;list;watch`

2. **kdp-workspaces/internal/kcp/rbac.go** (NEW FILE)
   ```go
   // Constants for ClusterRoleBinding annotations and labels
   const (
       AnnotationBindingLastSynced = "kdp-workspaces.cncf.io/last-synced"
       AnnotationBindingStaffCount = "kdp-workspaces.cncf.io/staff-count"
       AnnotationBindingManagedBy  = "kdp-workspaces.cncf.io/managed-by"
       AnnotationBindingSourceNamespace = "kdp-workspaces.cncf.io/source-namespace"
       LabelManagedBy = "managed-by"
       StaffAccessBindingName = "cncf-staff-access"
   )

   // CreateOrUpdateStaffBinding creates/updates ClusterRoleBinding in workspace
   func (c *Client) CreateOrUpdateStaffBinding(ctx context.Context,
       workspaceName string, subjects []rbacv1.Subject) error
   ```

3. **kdp-workspaces/cmd/main.go**
   - Add flag: `--staff-namespace` (default: `maintainerd`)
   - Pass to ProjectReconciler initialization

**Reconcile Logic:**

```go
func (r *ProjectReconciler) reconcileStaffAccess(ctx context.Context,
    kcpClient *kcp.Client, workspaceName string) error {

    // List all StaffMembers
    staffList := &unstructured.UnstructuredList{}
    staffList.SetGroupVersionKind(schema.GroupVersionKind{
        Group: "maintainer-d.cncf.io", Version: "v1alpha1", Kind: "StaffMemberList"})

    if err := r.List(ctx, staffList,
        client.InNamespace(r.StaffMemberNamespace)); err != nil {
        return fmt.Errorf("failed to list staff members: %w", err)
    }

    // Build subjects list
    subjects := []rbacv1.Subject{}
    for _, staff := range staffList.Items {
        email := staff.Object["spec"].(map[string]any)["primaryEmail"].(string)
        if email != "" {
            subjects = append(subjects, rbacv1.Subject{
                Kind: "User",
                Name: fmt.Sprintf("oidc:%s", email),
            })
        }
    }

    // Create/update binding in workspace
    return kcpClient.CreateOrUpdateStaffBinding(ctx, workspaceName, subjects)
}
```

### Pros

- ✅ Simpler architecture (single controller)
- ✅ Atomic creation: workspace + staff access together
- ✅ Easier to understand and maintain
- ✅ Fewer moving parts

### Cons

- ❌ Couples workspace creation with staff management
- ❌ Project reconciliation triggers even when only staff changes
- ❌ Staff changes require touching ProjectReconciler
- ❌ Less flexible (can't update staff access independently)
- ❌ Harder to test staff logic separately

### When to Use

Choose this solution if:
- Simplicity is paramount
- Staff list changes infrequently
- You want atomic guarantees (workspace + access together)
- Team size is small

---

## Solution 2: Multi-Controller Architecture (Recommended)

### Overview

Create a separate `StaffMemberReconciler` controller that independently manages staff access across all workspaces created by the operator.

### Approach

**Two Independent Controllers:**

1. **ProjectReconciler** (existing) - creates workspaces
2. **StaffMemberReconciler** (NEW) - manages staff access across all workspaces

**StaffMemberReconciler Reconciliation Flow:**
1. Watch StaffMember create/update/delete events
2. On any change, list all workspaces with annotation `managed-by=kdp-ws-operator`
3. Filter to Ready workspaces only
4. For each workspace:
   - Fetch current list of ALL StaffMembers
   - Build complete subjects list with oidc: prefix
   - Create/update ClusterRoleBinding `cncf-staff-access`

### Implementation Details

**File Changes:**

1. **kdp-workspaces/internal/controller/staffmember_controller.go** (NEW FILE)
   ```go
   type StaffMemberReconciler struct {
       client.Client
       Scheme                *runtime.Scheme
       KCPConfigMapName      string
       KCPConfigMapNamespace string
       KCPSecretName         string
       KCPSecretNamespace    string
       StaffMemberNamespace  string  // default: maintainerd
   }

   func (r *StaffMemberReconciler) Reconcile(ctx context.Context,
       req ctrl.Request) (ctrl.Result, error)
   ```

2. **kdp-workspaces/internal/kcp/rbac.go** (NEW FILE)
   - `CreateOrUpdateStaffBinding()` - create/update ClusterRoleBinding in workspace
   - `DeleteStaffBindingSubject()` - remove specific subject from binding
   - Uses dynamic client with cluster path to target specific workspace

3. **kdp-workspaces/internal/kcp/workspace.go**
   - Add `ListManagedWorkspaces(ctx) ([]*WorkspaceInfo, error)` method
   - Lists all workspaces, filters by annotation `managed-by=kdp-ws-operator`
   - Returns only Ready workspaces

4. **kdp-workspaces/cmd/main.go**
   - Add flag: `--staff-namespace` (default: `maintainerd`)
   - Register StaffMemberReconciler with manager (similar to ProjectReconciler)
   - Add RBAC markers for StaffMember watch permissions

**Reconcile Logic:**

```go
func (r *StaffMemberReconciler) Reconcile(ctx context.Context,
    req ctrl.Request) (ctrl.Result, error) {

    logger := log.FromContext(ctx)
    logger.Info("Reconciling StaffMember change", "staffmember", req.Name)

    // Load KCP config and client
    kcpConfig, err := kcp.LoadConfigFromCluster(ctx, r.Client,
        r.KCPConfigMapName, r.KCPSecretName, r.KCPConfigMapNamespace)
    if err != nil {
        return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
    }

    kcpClient, err := kcp.NewClient(kcpConfig)
    if err != nil {
        return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
    }

    // List all managed workspaces
    workspaces, err := kcpClient.ListManagedWorkspaces(ctx)
    if err != nil {
        return ctrl.Result{RequeueAfter: 30 * time.Second}, err
    }

    // Get current list of ALL staff members
    staffList := &unstructured.UnstructuredList{}
    staffList.SetGroupVersionKind(schema.GroupVersionKind{
        Group: "maintainer-d.cncf.io", Version: "v1alpha1", Kind: "StaffMemberList"})

    if err := r.List(ctx, staffList,
        client.InNamespace(r.StaffMemberNamespace)); err != nil {
        return ctrl.Result{}, fmt.Errorf("failed to list staff members: %w", err)
    }

    // Build subjects list from ALL staff members
    subjects := []rbacv1.Subject{}
    for _, staff := range staffList.Items {
        spec := staff.Object["spec"].(map[string]any)
        if email, ok := spec["primaryEmail"].(string); ok && email != "" {
            subjects = append(subjects, rbacv1.Subject{
                Kind: "User",
                Name: fmt.Sprintf("oidc:%s", email),
            })
        }
    }

    // Update binding in all workspaces
    var reconcileErrors []error
    successCount := 0
    for _, ws := range workspaces {
        if err := kcpClient.CreateOrUpdateStaffBinding(ctx, ws.Name, subjects); err != nil {
            logger.Error(err, "Failed to update staff binding", "workspace", ws.Name)
            reconcileErrors = append(reconcileErrors, err)
        } else {
            successCount++
        }
    }

    // Update StaffMember annotations with sync status
    // Note: This updates the triggering StaffMember, but on reconcile we process ALL staff
    // So we update the annotation for the StaffMember that triggered this reconciliation
    syncStatus := "success"
    if len(reconcileErrors) > 0 {
        syncStatus = "partial"
        if successCount == 0 {
            syncStatus = "error"
        }
    }

    // Update the StaffMember that triggered this reconciliation
    if err := r.updateStaffMemberAnnotations(ctx, req.Name,
        successCount, syncStatus); err != nil {
        logger.Error(err, "Failed to update StaffMember annotations",
            "staffmember", req.Name)
        // Don't fail reconciliation if annotation update fails
    }

    if len(reconcileErrors) > 0 {
        return ctrl.Result{RequeueAfter: 30 * time.Second},
            fmt.Errorf("failed to update %d workspaces", len(reconcileErrors))
    }

    return ctrl.Result{}, nil
}

// updateStaffMemberAnnotations updates sync metadata annotations on a StaffMember
func (r *StaffMemberReconciler) updateStaffMemberAnnotations(ctx context.Context,
    staffMemberName string, workspaceCount int, syncStatus string) error {

    // Fetch the StaffMember
    staffMember := &unstructured.Unstructured{}
    staffMember.SetGroupVersionKind(schema.GroupVersionKind{
        Group: "maintainer-d.cncf.io", Version: "v1alpha1", Kind: "StaffMember"})

    if err := r.Get(ctx, client.ObjectKey{
        Name: staffMemberName, Namespace: r.StaffMemberNamespace}, staffMember); err != nil {
        return err
    }

    // Create patch
    patch := client.MergeFrom(staffMember.DeepCopy())

    // Update annotations
    annotations := staffMember.GetAnnotations()
    if annotations == nil {
        annotations = make(map[string]string)
    }
    annotations[AnnotationStaffLastSynced] = time.Now().Format(time.RFC3339)
    annotations[AnnotationStaffSyncStatus] = syncStatus
    annotations[AnnotationStaffWorkspaceCount] = fmt.Sprintf("%d", workspaceCount)
    staffMember.SetAnnotations(annotations)

    // Apply patch
    return r.Patch(ctx, staffMember, patch)
}
```

**KCP RBAC Helper (kdp-workspaces/internal/kcp/rbac.go):**

```go
const (
    // Annotations for ClusterRoleBindings
    AnnotationBindingLastSynced = "kdp-workspaces.cncf.io/last-synced"
    AnnotationBindingStaffCount = "kdp-workspaces.cncf.io/staff-count"
    AnnotationBindingManagedBy  = "kdp-workspaces.cncf.io/managed-by"
    AnnotationBindingSourceNamespace = "kdp-workspaces.cncf.io/source-namespace"

    // Labels
    LabelManagedBy = "managed-by"

    // ClusterRoleBinding name
    StaffAccessBindingName = "cncf-staff-access"
)

func (c *Client) CreateOrUpdateStaffBinding(ctx context.Context,
    workspaceName string, subjects []rbacv1.Subject) error {

    // Get workspace path (e.g., root:projectname)
    workspacePath := logicalcluster.NewPath(c.config.WorkspacePath).Join(workspaceName)

    // Create dynamic client scoped to workspace
    restConfig, _ := clientcmd.RESTConfigFromKubeConfig(c.config.KubeconfigData)
    dynamicClient, _ := dynamic.NewForConfig(restConfig)

    // Build ClusterRoleBinding with annotations
    binding := &rbacv1.ClusterRoleBinding{
        ObjectMeta: metav1.ObjectMeta{
            Name: StaffAccessBindingName,
            Annotations: map[string]string{
                AnnotationBindingLastSynced:      time.Now().Format(time.RFC3339),
                AnnotationBindingStaffCount:      fmt.Sprintf("%d", len(subjects)),
                AnnotationBindingManagedBy:       "kdp-ws-operator",
                AnnotationBindingSourceNamespace: "maintainerd",
            },
            Labels: map[string]string{
                LabelManagedBy: "kdp-ws-operator",
            },
        },
        Subjects: subjects,
        RoleRef: rbacv1.RoleRef{
            APIGroup: "rbac.authorization.k8s.io",
            Kind:     "ClusterRole",
            Name:     "kdp:owner",
        },
    }

    // Create or update using dynamic client with cluster path
    // ... implementation using dynamicClient.Cluster(workspacePath).Resource(...)

    return nil
}

func (c *Client) ListManagedWorkspaces(ctx context.Context) ([]*WorkspaceInfo, error) {
    workspacePath := logicalcluster.NewPath(c.config.WorkspacePath)
    clusterClient := c.GetClusterClient()

    wsList, err := clusterClient.Cluster(workspacePath).
        TenancyV1alpha1().Workspaces().List(ctx, metav1.ListOptions{})
    if err != nil {
        return nil, err
    }

    var result []*WorkspaceInfo
    for _, ws := range wsList.Items {
        // Filter by annotation and Ready phase
        if ws.Annotations["managed-by"] == "kdp-ws-operator" &&
           ws.Status.Phase == corev1alpha1.LogicalClusterPhaseReady {
            result = append(result, &WorkspaceInfo{
                Name:  ws.Name,
                URL:   ws.Spec.URL,
                Phase: string(ws.Status.Phase),
                Ready: true,
            })
        }
    }

    return result, nil
}
```

### RBAC Permissions

Add to kdp-workspaces/internal/controller/staffmember_controller.go:

```go
// +kubebuilder:rbac:groups=maintainer-d.cncf.io,resources=staffmembers,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
```

### Pros

- ✅ **Separation of concerns** - each controller has single responsibility
- ✅ **Independent reconciliation** - staff changes don't affect workspace creation
- ✅ **Scalable** - efficiently handles 240+ projects
- ✅ **Flexible** - can handle staff additions/removals without touching workspace logic
- ✅ **Testable** - staff logic isolated and easier to test
- ✅ **Kubernetes-native** - follows standard multi-controller patterns
- ✅ **Eventual consistency** - workspaces get staff access soon after creation
- ✅ **Fault tolerant** - staff reconciliation failures don't block workspace creation

### Cons

- ❌ More complex architecture (two controllers)
- ❌ Eventual consistency gap (staff access comes after workspace creation)
- ❌ Need to list all workspaces on each StaffMember change
- ❌ More code to maintain

### When to Use

Choose this solution if:
- ✅ You want proper separation of concerns
- ✅ Staff list changes frequently
- ✅ You have many workspaces (scalability matters)
- ✅ You want to follow Kubernetes best practices
- ✅ **REQUIRED: User specifically asked for multi-controller solution**

---

## Recommendation

**Solution 2 (Multi-Controller)** is strongly recommended because:

1. **Meets Requirements** - User explicitly requested a multi-controller solution
2. **Better Architecture** - Proper separation of concerns (SRP)
3. **Scalability** - Handles 240+ CNCF projects efficiently
4. **Maintainability** - Easier to test, debug, and extend
5. **Flexibility** - Staff changes don't trigger workspace reconciliation
6. **Kubernetes Patterns** - Follows standard controller design patterns

The eventual consistency gap (workspace created → staff access added shortly after) is acceptable and mirrors how other Kubernetes operators work.

---

## Implementation Plan (Solution 2)

**Implementation Strategy:** Build and test the MVP implementation (Phases 1-5) first, then optimize (Phase 6).

The initial implementation uses a "rebuild-all" reconciliation strategy for simplicity and correctness. This ensures the system works correctly before adding complexity. Performance optimizations will be added in Phase 6 after the MVP is validated in production.

---

## Implementation Status

| Phase | Status | Commit | Description |
|-------|--------|--------|-------------|
| Phase 1 | ✅ **COMPLETED** | 542c2e1 | KCP RBAC extensions (rbac.go, ListManagedWorkspaces) |
| Phase 2 | ✅ **COMPLETED** | 542c2e1 | StaffMemberReconciler controller implementation |
| Phase 3 | ✅ **COMPLETED** | 542c2e1 | Integration into main.go with --staff-namespace flag |
| Phase 4 | ✅ **COMPLETED** | ae33fc2, caea7aa | Unit tests + E2E testing guide |
| Phase 5 | 🔄 **IN PROGRESS** | - | Documentation updates (CLAUDE.md) |
| Phase 6 | ⏳ **PENDING** | - | Post-MVP optimizations (after production validation) |

**Version:** 0.0.2 (Commit 7f58765)

---

### Phase 1: KCP Client Extensions ✅ COMPLETED

**Tasks:**
1. Create `kdp-workspaces/internal/kcp/rbac.go`
2. Implement `CreateOrUpdateStaffBinding()` method
3. Implement `ListManagedWorkspaces()` in workspace.go
4. Add tests for new KCP client methods

**Files Modified:**
- kdp-workspaces/internal/kcp/rbac.go (NEW)
- kdp-workspaces/internal/kcp/workspace.go (add ListManagedWorkspaces)

### Phase 2: StaffMember Controller ✅ COMPLETED

**Tasks:**
1. Create `kdp-workspaces/internal/controller/staffmember_controller.go`
2. Implement StaffMemberReconciler struct
3. Add annotation constants for StaffMember tracking:
   ```go
   const (
       AnnotationStaffLastSynced = "kdp-workspaces.cncf.io/last-synced"
       AnnotationStaffSyncStatus = "kdp-workspaces.cncf.io/sync-status"
       AnnotationStaffWorkspaceCount = "kdp-workspaces.cncf.io/workspace-count"
   )
   ```
4. Implement Reconcile() method with logic above
5. Implement updateStaffMemberAnnotations() helper method
6. Add SetupWithManager() method
7. Add RBAC markers

**Files Modified:**
- kdp-workspaces/internal/controller/staffmember_controller.go (NEW)

### Phase 3: Main Integration ✅ COMPLETED

**Tasks:**
1. Update `kdp-workspaces/cmd/main.go`
2. Add `--staff-namespace` flag
3. Register StaffMemberReconciler with manager
4. Update RBAC manifests via kubebuilder

**Files Modified:**
- kdp-workspaces/cmd/main.go

### Phase 4: Testing ✅ COMPLETED

**Tasks:**
1. ✅ Unit tests for KCP RBAC methods
2. ✅ Unit tests for StaffMemberReconciler
3. ✅ E2E testing guide with isolated namespace (manual tests documented)
4. ✅ Test value analysis via subagent (high-value tests identified)

**Files Modified:**
- kdp-workspaces/internal/kcp/rbac_test.go (NEW) - Commit ae33fc2
- kdp-workspaces/internal/controller/staffmember_controller_test.go (NEW) - Commit ae33fc2

**Note:** E2E tests require actual KCP cluster access and are documented in the Testing Operator sections of AGENTS.md and CLAUDE.md. Automated E2E tests deferred until CI/CD infrastructure with KCP available.

### Phase 5: Documentation 🔄 IN PROGRESS

**Tasks:**
1. 🔄 Update kdp-workspaces/CLAUDE.md with new controller info
2. ⏳ Update kdp-workspaces/README.md (if exists)
3. ⏳ Add operator deployment docs

**Status:** CLAUDE.md update in progress

---

## Testing Strategy

### Unit Tests

1. **KCP RBAC Tests** (kcp/rbac_test.go)
   - Test CreateOrUpdateStaffBinding with empty subjects
   - Test CreateOrUpdateStaffBinding with multiple subjects
   - Test ListManagedWorkspaces filters correctly

2. **Controller Tests** (controller/staffmember_controller_test.go)
   - Test reconcile with no workspaces
   - Test reconcile with multiple workspaces
   - Test reconcile handles KCP client errors
   - Test reconcile builds correct subjects list
   - Test StaffMember annotations updated correctly after successful sync
   - Test StaffMember annotations reflect partial sync on errors
   - Test annotation update failures don't block reconciliation

### E2E Tests

1. **Workspace Creation + Staff Access**
   - Create Project CRD
   - Create StaffMember CRDs
   - Verify workspace created
   - Verify ClusterRoleBinding exists in workspace
   - Verify subjects list matches staff members
   - Verify ClusterRoleBinding has correct annotations (last-synced, staff-count, managed-by)
   - Verify StaffMember annotations updated (last-synced, sync-status, workspace-count)

2. **Staff Member Addition**
   - Create new StaffMember
   - Verify ClusterRoleBinding updated in all workspaces
   - Verify new StaffMember has annotations with sync status
   - Verify ClusterRoleBinding staff-count incremented

3. **Staff Member Removal**
   - Delete StaffMember
   - Verify ClusterRoleBinding updated (subject removed)
   - Verify ClusterRoleBinding staff-count decremented
   - Verify remaining StaffMembers have updated annotations

### Manual Testing

```bash
# 1. Deploy operator
kubectl apply -k kdp-workspaces/config/default

# 2. Create test staff members
kubectl apply -f - <<EOF
apiVersion: maintainer-d.cncf.io/v1alpha1
kind: StaffMember
metadata:
  name: test-staff
  namespace: maintainerd
spec:
  displayName: "Test Staff"
  primaryEmail: "test-staff@example.test"
EOF

# 3. Verify binding in workspace
KUBECONFIG=$(git rev-parse --show-toplevel)/tmp/kdp-cluster-cncf/kubeconfig-admin \
  kubectl get clusterrolebinding cncf-staff-access -o yaml

# 4. Check subjects include oidc:test-staff@example.test and verify annotations
KUBECONFIG=$(git rev-parse --show-toplevel)/tmp/kdp-cluster-cncf/kubeconfig-admin \
  kubectl get clusterrolebinding cncf-staff-access -o jsonpath='{.metadata.annotations}'

# Expected output:
# {
#   "kdp-workspaces.cncf.io/last-synced": "2025-01-14T10:30:45Z",
#   "kdp-workspaces.cncf.io/managed-by": "kdp-ws-operator",
#   "kdp-workspaces.cncf.io/source-namespace": "maintainerd",
#   "kdp-workspaces.cncf.io/staff-count": "1"
# }

# 5. Verify StaffMember annotations updated
kubectl get staffmember test-staff -n maintainerd -o jsonpath='{.metadata.annotations}'

# Expected output:
# {
#   "kdp-workspaces.cncf.io/last-synced": "2025-01-14T10:30:45Z",
#   "kdp-workspaces.cncf.io/sync-status": "success",
#   "kdp-workspaces.cncf.io/workspace-count": "247"
# }

# 6. Find all workspaces with staff access managed by operator
KUBECONFIG=$(git rev-parse --show-toplevel)/tmp/kdp-cluster-cncf/kubeconfig-admin \
  kubectl get clusterrolebinding -A -l managed-by=kdp-ws-operator
```

---

## Migration Path

If currently deployed:

1. Deploy new operator version with StaffMemberReconciler
2. StaffMemberReconciler will discover existing workspaces
3. Creates ClusterRoleBindings in all Ready workspaces
4. No downtime or data migration needed

---

## Phase 6: Post-MVP Optimizations ⏳ PENDING

**⚠️ IMPORTANT:** These optimizations should ONLY be implemented AFTER the MVP (Phases 1-5) is deployed, tested, and validated in production.

**Status:** Awaiting MVP production deployment and validation

### Why Optimize Later?

The initial implementation (Phases 1-5) uses a "rebuild-all" reconciliation strategy. While less efficient, this approach is:
- ✅ **Simpler** to implement and reason about
- ✅ **Self-healing** - rebuilds complete state every time
- ✅ **Correct** - always converges to desired state
- ✅ **Easier to test** - fewer code paths and edge cases

After validating correctness in production, the following optimizations improve efficiency and responsiveness:

| Optimization | Addresses | Complexity | Impact |
|--------------|-----------|------------|--------|
| **A: Incremental Updates** | Inefficiency (O(staff × workspaces) → O(workspaces)) | High (3 code paths + finalizers) | 500× faster for staff changes |
| **B: Workspace Watch** | Critical gap (new workspaces don't get access) | Medium (cross-cluster watch) | Eliminates gap entirely |

### Trade-offs Summary

**MVP Approach (Phases 1-5):**
- Simple, correct, self-healing
- ❌ Inefficient for large deployments (500 staff × 240 workspaces)
- ❌ Gap: new workspaces wait for next StaffMember change

**Optimized Approach (Phase 6):**
- ✅ 500× more efficient
- ✅ No gap - immediate workspace access
- ❌ More complex (6 code paths vs 1)
- ❌ More edge cases to handle

### Optimization A: Incremental Subject Updates

**Current MVP Behavior:**
- Every StaffMember change triggers fetching ALL staff members and rebuilding complete subject lists for ALL workspaces
- For 500 staff members and 240 workspaces, this is O(500 × 240) = 120,000 operations per change

**Optimization Goal:**
- Update only the changed staff member's subject in each workspace
- Reduces to O(240) operations = 500× improvement

**Implementation Details:**

1. **Detect Event Type in Reconcile:**
   ```go
   func (r *StaffMemberReconciler) Reconcile(ctx context.Context,
       req ctrl.Request) (ctrl.Result, error) {

       logger := log.FromContext(ctx)

       // Try to fetch the StaffMember
       staffMember := &maintainerv1alpha1.StaffMember{}
       err := r.Get(ctx, req.NamespacedName, staffMember)

       if err != nil {
           if errors.IsNotFound(err) {
               // DELETE event - staff member was deleted
               return r.handleStaffMemberDelete(ctx, req.Name)
           }
           return ctrl.Result{}, err
       }

       // CREATE or UPDATE event - staff member exists
       return r.handleStaffMemberUpsert(ctx, staffMember)
   }
   ```

2. **Handle Create/Update (Add/Update Single Subject):**
   ```go
   func (r *StaffMemberReconciler) handleStaffMemberUpsert(ctx context.Context,
       staffMember *maintainerv1alpha1.StaffMember) (ctrl.Result, error) {

       logger := log.FromContext(ctx)
       email := staffMember.Spec.PrimaryEmail
       if email == "" {
           logger.Info("StaffMember has no email, skipping", "name", staffMember.Name)
           return ctrl.Result{}, nil
       }

       subject := rbacv1.Subject{
           Kind: "User",
           Name: fmt.Sprintf("oidc:%s", email),
       }

       // Get KCP client (cached in reconciler)
       kcpClient := r.kcpClient

       // List managed workspaces
       workspaces, err := kcpClient.ListManagedWorkspaces(ctx)
       if err != nil {
           return ctrl.Result{RequeueAfter: 30 * time.Second}, err
       }

       // Add/update this subject in each workspace
       var reconcileErrors []error
       for _, ws := range workspaces {
           if err := kcpClient.AddSubjectToStaffBinding(ctx, ws.Name, subject); err != nil {
               logger.Error(err, "Failed to add subject", "workspace", ws.Name)
               reconcileErrors = append(reconcileErrors, err)
           }
       }

       // Update StaffMember annotations
       syncStatus := "success"
       if len(reconcileErrors) > 0 {
           syncStatus = "partial"
       }
       r.updateStaffMemberAnnotations(ctx, staffMember.Name, len(workspaces), syncStatus)

       if len(reconcileErrors) > 0 {
           return ctrl.Result{RequeueAfter: 30 * time.Second},
               errors.NewAggregate(reconcileErrors)
       }

       return ctrl.Result{}, nil
   }
   ```

3. **Handle Delete (Remove Single Subject):**
   ```go
   func (r *StaffMemberReconciler) handleStaffMemberDelete(ctx context.Context,
       staffMemberName string) (ctrl.Result, error) {

       logger := log.FromContext(ctx)

       // Reconstruct email from name (assumes name format: email-domain.com)
       // OR: Use a finalizer to cache the email before deletion
       email := reconstructEmailFromName(staffMemberName)
       if email == "" {
           logger.Info("Cannot determine email from deleted StaffMember", "name", staffMemberName)
           return ctrl.Result{}, nil
       }

       subject := rbacv1.Subject{
           Kind: "User",
           Name: fmt.Sprintf("oidc:%s", email),
       }

       // Get KCP client
       kcpClient := r.kcpClient

       // List managed workspaces
       workspaces, err := kcpClient.ListManagedWorkspaces(ctx)
       if err != nil {
           return ctrl.Result{RequeueAfter: 30 * time.Second}, err
       }

       // Remove this subject from each workspace
       var reconcileErrors []error
       for _, ws := range workspaces {
           if err := kcpClient.RemoveSubjectFromStaffBinding(ctx, ws.Name, subject); err != nil {
               logger.Error(err, "Failed to remove subject", "workspace", ws.Name)
               reconcileErrors = append(reconcileErrors, err)
           }
       }

       if len(reconcileErrors) > 0 {
           return ctrl.Result{RequeueAfter: 30 * time.Second},
               errors.NewAggregate(reconcileErrors)
       }

       return ctrl.Result{}, nil
   }
   ```

4. **New KCP Client Methods (kdp-workspaces/internal/kcp/rbac.go):**
   ```go
   // AddSubjectToStaffBinding adds or updates a single subject in the ClusterRoleBinding
   func (c *Client) AddSubjectToStaffBinding(ctx context.Context,
       workspaceName string, subject rbacv1.Subject) error {

       // Get existing binding
       binding, err := c.GetStaffBinding(ctx, workspaceName)
       if err != nil {
           if errors.IsNotFound(err) {
               // Binding doesn't exist, create it with this subject
               return c.CreateOrUpdateStaffBinding(ctx, workspaceName, []rbacv1.Subject{subject})
           }
           return err
       }

       // Check if subject already exists
       found := false
       for i, s := range binding.Subjects {
           if s.Kind == subject.Kind && s.Name == subject.Name {
               binding.Subjects[i] = subject // Update in case fields changed
               found = true
               break
           }
       }

       if !found {
           binding.Subjects = append(binding.Subjects, subject)
       }

       // Update annotations
       binding.Annotations[AnnotationBindingLastSynced] = time.Now().Format(time.RFC3339)
       binding.Annotations[AnnotationBindingStaffCount] = fmt.Sprintf("%d", len(binding.Subjects))

       // Patch the binding
       return c.patchStaffBinding(ctx, workspaceName, binding)
   }

   // RemoveSubjectFromStaffBinding removes a single subject from the ClusterRoleBinding
   func (c *Client) RemoveSubjectFromStaffBinding(ctx context.Context,
       workspaceName string, subject rbacv1.Subject) error {

       // Get existing binding
       binding, err := c.GetStaffBinding(ctx, workspaceName)
       if err != nil {
           if errors.IsNotFound(err) {
               // Binding doesn't exist, nothing to remove
               return nil
           }
           return err
       }

       // Filter out the subject
       newSubjects := []rbacv1.Subject{}
       for _, s := range binding.Subjects {
           if !(s.Kind == subject.Kind && s.Name == subject.Name) {
               newSubjects = append(newSubjects, s)
           }
       }

       binding.Subjects = newSubjects

       // Update annotations
       binding.Annotations[AnnotationBindingLastSynced] = time.Now().Format(time.RFC3339)
       binding.Annotations[AnnotationBindingStaffCount] = fmt.Sprintf("%d", len(binding.Subjects))

       // Patch the binding
       return c.patchStaffBinding(ctx, workspaceName, binding)
   }
   ```

5. **Finalizer for Delete Handling:**
   ```go
   const StaffMemberFinalizer = "kdp-workspaces.cncf.io/staff-access"

   func (r *StaffMemberReconciler) Reconcile(ctx context.Context,
       req ctrl.Request) (ctrl.Result, error) {

       staffMember := &maintainerv1alpha1.StaffMember{}
       err := r.Get(ctx, req.NamespacedName, staffMember)

       if err != nil {
           if errors.IsNotFound(err) {
               // Already deleted, nothing to do
               return ctrl.Result{}, nil
           }
           return ctrl.Result{}, err
       }

       // Check if being deleted
       if !staffMember.DeletionTimestamp.IsZero() {
           // Handle deletion
           if containsString(staffMember.Finalizers, StaffMemberFinalizer) {
               // Remove from all workspaces
               if err := r.removeStaffFromAllWorkspaces(ctx, staffMember); err != nil {
                   return ctrl.Result{}, err
               }

               // Remove finalizer
               staffMember.Finalizers = removeString(staffMember.Finalizers, StaffMemberFinalizer)
               if err := r.Update(ctx, staffMember); err != nil {
                   return ctrl.Result{}, err
               }
           }
           return ctrl.Result{}, nil
       }

       // Add finalizer if not present
       if !containsString(staffMember.Finalizers, StaffMemberFinalizer) {
           staffMember.Finalizers = append(staffMember.Finalizers, StaffMemberFinalizer)
           if err := r.Update(ctx, staffMember); err != nil {
               return ctrl.Result{}, err
           }
       }

       // Normal reconciliation
       return r.handleStaffMemberUpsert(ctx, staffMember)
   }
   ```

**Benefits:**
- 500× efficiency improvement for large deployments
- Faster response time to staff changes
- Reduced API server load

**Considerations:**
- More complex code paths (create/update/delete)
- Need to handle edge cases (binding doesn't exist yet)
- Requires finalizer management for clean deletion

---

### Optimization B: Watch for New Workspaces

**Current MVP Behavior:**
- StaffMemberReconciler only triggers on StaffMember changes
- New workspaces created by ProjectReconciler don't immediately get staff access
- Gap exists until next StaffMember change triggers reconciliation

**Optimization Goal:**
- Immediately add staff access to new workspaces when they're created
- Eliminate the gap between workspace creation and staff access

**Implementation Details:**

1. **Add Workspace Watch to StaffMemberReconciler:**
   ```go
   func (r *StaffMemberReconciler) SetupWithManager(mgr ctrl.Manager) error {
       // Watch StaffMember resources on Service Cluster
       staffMember := &maintainerv1alpha1.StaffMember{}

       // Create mapper function to enqueue reconcile requests when workspaces change
       workspaceToRequests := func(ctx context.Context, obj client.Object) []reconcile.Request {
           // When a workspace is created/updated, trigger reconciliation
           // We use a dummy request since we'll fetch all staff members anyway
           return []reconcile.Request{{
               NamespacedName: types.NamespacedName{
                   Name:      "workspace-trigger",
                   Namespace: r.StaffMemberNamespace,
               },
           }}
       }

       // Setup watches
       return ctrl.NewControllerManagedBy(mgr).
           For(staffMember).
           // Watch workspaces in KCP cluster
           Watches(
               &tenancyv1alpha1.Workspace{},
               handler.EnqueueRequestsFromMapFunc(workspaceToRequests),
           ).
           Complete(r)
   }
   ```

2. **Configure Manager for Cross-Cluster Watching:**
   ```go
   // In cmd/main.go

   // Create manager for Service Cluster (default)
   mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
       Scheme: scheme,
       // ... other options
   })

   // Load KCP config
   kcpConfig, err := kcp.LoadConfigFromCluster(
       context.Background(),
       mgr.GetClient(),
       kcpConfigMapName,
       kcpSecretName,
       kcpConfigMapNamespace,
   )

   // Create KCP client
   kcpClient, err := kcp.NewClient(kcpConfig)

   // Add KCP workspace type to scheme
   tenancyv1alpha1.AddToScheme(mgr.GetScheme())

   // Create cache for KCP cluster
   kcpCache, err := cache.New(kcpClient.GetRESTConfig(), cache.Options{
       Scheme: mgr.GetScheme(),
       // Only watch Workspace resources
       DefaultNamespaces: map[string]cache.Config{
           kcpConfig.WorkspacePath: {},
       },
   })

   // Add KCP cache to manager
   if err := mgr.Add(kcpCache); err != nil {
       setupLog.Error(err, "unable to add KCP cache to manager")
       os.Exit(1)
   }
   ```

3. **Enhanced Reconcile Logic for Workspace Events:**
   ```go
   func (r *StaffMemberReconciler) Reconcile(ctx context.Context,
       req ctrl.Request) (ctrl.Result, error) {

       logger := log.FromContext(ctx)

       // Check if this is a workspace-triggered reconciliation
       if req.Name == "workspace-trigger" {
           logger.Info("Reconciling due to workspace change")
           return r.reconcileAllWorkspaces(ctx)
       }

       // Otherwise, this is a StaffMember change
       staffMember := &maintainerv1alpha1.StaffMember{}
       err := r.Get(ctx, req.NamespacedName, staffMember)
       // ... handle StaffMember reconciliation
   }

   func (r *StaffMemberReconciler) reconcileAllWorkspaces(ctx context.Context) (ctrl.Result, error) {
       // Fetch ALL staff members
       staffList := &maintainerv1alpha1.StaffMemberList{}
       if err := r.List(ctx, staffList,
           client.InNamespace(r.StaffMemberNamespace)); err != nil {
           return ctrl.Result{}, err
       }

       // Build complete subjects list
       subjects := []rbacv1.Subject{}
       for _, staff := range staffList.Items {
           if email := staff.Spec.PrimaryEmail; email != "" {
               subjects = append(subjects, rbacv1.Subject{
                   Kind: "User",
                   Name: fmt.Sprintf("oidc:%s", email),
               })
           }
       }

       // List all managed workspaces
       workspaces, err := r.kcpClient.ListManagedWorkspaces(ctx)
       if err != nil {
           return ctrl.Result{RequeueAfter: 30 * time.Second}, err
       }

       // Create/update bindings in all workspaces
       var reconcileErrors []error
       for _, ws := range workspaces {
           if err := r.kcpClient.CreateOrUpdateStaffBinding(ctx, ws.Name, subjects); err != nil {
               reconcileErrors = append(reconcileErrors, err)
           }
       }

       if len(reconcileErrors) > 0 {
           return ctrl.Result{RequeueAfter: 30 * time.Second},
               errors.NewAggregate(reconcileErrors)
       }

       return ctrl.Result{}, nil
   }
   ```

4. **Filter Workspaces by Annotation:**
   ```go
   // In workspace watch mapper function
   workspaceToRequests := func(ctx context.Context, obj client.Object) []reconcile.Request {
       workspace, ok := obj.(*tenancyv1alpha1.Workspace)
       if !ok {
           return nil
       }

       // Only trigger if workspace is managed by our operator
       if workspace.Annotations["managed-by"] != "kdp-ws-operator" {
           return nil
       }

       // Only trigger if workspace is Ready
       if workspace.Status.Phase != corev1alpha1.LogicalClusterPhaseReady {
           return nil
       }

       return []reconcile.Request{{
           NamespacedName: types.NamespacedName{
               Name:      "workspace-trigger",
               Namespace: r.StaffMemberNamespace,
           },
       }}
   }
   ```

**Benefits:**
- ✅ Eliminates gap - new workspaces immediately get staff access
- ✅ Proper event-driven architecture
- ✅ No reliance on StaffMember changes to propagate access

**Considerations:**
- More complex setup (cross-cluster watches)
- Need to configure manager with KCP cache
- Potential for watch connection issues between clusters
- Need proper RBAC for watching workspaces in KCP cluster

**Alternative Simpler Approach:**
Instead of cross-cluster watches, have `ProjectReconciler` explicitly notify or trigger the StaffMemberReconciler after creating a workspace:

```go
// In ProjectReconciler after workspace is ready
if err := r.updateWorkspaceStatus(ctx, project, readyInfo); err != nil {
    return ctrl.Result{}, err
}

// Trigger staff reconciliation by creating an event or annotation
// Option 1: Add annotation to a dummy StaffMember
// Option 2: Use a channel or in-memory queue
// Option 3: Create a custom event resource
```

---

### Migration Plan for Optimizations

**Step 1: Deploy Optimization A (Incremental Updates)**
1. Implement `AddSubjectToStaffBinding` and `RemoveSubjectFromStaffBinding`
2. Add finalizer support to StaffMemberReconciler
3. Update reconcile logic to handle create/update/delete differently
4. Test with gradual rollout (shadow mode: log what would be done)
5. Enable incrementalmode via feature flag

**Step 2: Deploy Optimization B (Workspace Watch)**
1. Implement cross-cluster watch configuration
2. Add workspace event handler
3. Test watch reliability and reconnection logic
4. Deploy with monitoring for watch failures
5. Keep fallback to periodic sync if watch fails

**Step 3: Combined Optimizations**
- Optimization A handles StaffMember changes incrementally
- Optimization B ensures new workspaces get immediate full sync
- Together they provide both efficiency and completeness

**Estimated Impact:**
- Current: O(staff × workspaces) per StaffMember change = ~120,000 ops
- With A: O(workspaces) per StaffMember change = ~240 ops (500× improvement)
- With B: O(staff) per new workspace = ~500 ops vs ~0 (eliminates gap)

---

## Summary of Annotations

This implementation uses a **dual-annotation strategy** for comprehensive observability:

### StaffMember Resource Annotations

Located on: `staffmembers.maintainer-d.cncf.io` in `maintainerd` namespace (Service Cluster)

| Annotation | Format | Purpose |
|------------|--------|---------|
| `kdp-workspaces.cncf.io/last-synced` | RFC3339 timestamp | When this staff member was last synced to workspaces |
| `kdp-workspaces.cncf.io/sync-status` | `success` \| `partial` \| `error` | Status of last sync operation |
| `kdp-workspaces.cncf.io/workspace-count` | Integer | Number of workspaces this member has access to |

**Query Examples:**
```bash
# Check if specific staff member is synced
kubectl get staffmember wojciech.barczynski-kubermatic.com -n maintainerd \
  -o jsonpath='{.metadata.annotations}'

# Find staff members with failed syncs
kubectl get staffmembers -n maintainerd -o json | \
  jq '.items[] | select(.metadata.annotations["kdp-workspaces.cncf.io/sync-status"] != "success")'
```

### ClusterRoleBinding Annotations

Located on: `ClusterRoleBinding/cncf-staff-access` in each workspace (KDP Cluster)

| Annotation | Format | Purpose |
|------------|--------|---------|
| `kdp-workspaces.cncf.io/last-synced` | RFC3339 timestamp | When this binding was last updated |
| `kdp-workspaces.cncf.io/staff-count` | Integer | Number of staff members in this binding |
| `kdp-workspaces.cncf.io/managed-by` | `kdp-ws-operator` | Identifies operator that manages this binding |
| `kdp-workspaces.cncf.io/source-namespace` | `maintainerd` | Source namespace for StaffMembers |

**Label:** `managed-by=kdp-ws-operator` (for efficient queries)

**Query Examples:**
```bash
# Find all managed staff bindings
KUBECONFIG=./tmp/kdp-cluster-cncf/kubeconfig-admin \
  kubectl get clusterrolebinding -A -l managed-by=kdp-ws-operator

# Check when workspace's staff access was last updated
KUBECONFIG=./tmp/kdp-cluster-cncf/kubeconfig-admin \
  kubectl get clusterrolebinding cncf-staff-access \
    -o jsonpath='{.metadata.annotations.kdp-workspaces\.cncf\.io/last-synced}'
```

### Why Both?

1. **StaffMember annotations** answer: "Is this person synced? Where?"
2. **ClusterRoleBinding annotations** answer: "When was this workspace updated? By what?"

This provides observability from both **user perspective** (staff members) and **infrastructure perspective** (workspaces).

---

## Open Questions / Future Enhancements

1. **Caching**: Should we cache workspace list to reduce KCP API calls?
2. **Metrics**: Add Prometheus metrics for staff reconciliation success/failure?
3. **~~Status~~**: ✅ IMPLEMENTED - Using annotations on both StaffMembers and ClusterRoleBindings
4. **Periodic Sync**: Add periodic reconciliation to detect/fix drift?
5. **~~Binding Name~~**: ✅ TO BE IMPLEMENTED - Make configurable via `--staff-access-binding-name` flag
6. **Annotation Cleanup**: Should we remove annotations from deleted ClusterRoleBindings?
7. **Annotation Conflicts**: How to handle if maintainer-d also tries to set annotations on StaffMembers?
8. **~~Incremental Updates~~**: ✅ PLANNED - See Phase 6: Optimization A for incremental subject updates
9. **~~Workspace Watch~~**: ✅ PLANNED - See Phase 6: Optimization B for immediate workspace access
10. **Typed Resources**: Should we use typed StaffMember structs instead of unstructured? (See feedback)
11. **KCP Client Caching**: Cache KCP client in reconciler instead of creating on each reconcile (See feedback)
