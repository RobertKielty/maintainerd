# CLAUDE.md

## Overview

The KDP Workspaces Operator is responsible for creating and managing KDP (Kubermatic Development Platform) organizations for CNCF projects and foundation services. This operator bridges the maintainer-d CRDs with KDP workspace (kcp workspaces of type *kdp-organization*) provisioning.

We always work with two clusters:

- **KDP Cluster** - the target cluster where the workspaces (type *kdp-organization*) live
- **Service Cluster** - where the KDP Workspaces Operator runs and where the maintainer-d CRDs reside

### Purpose

The operator serves as the foundation for a self-service portal for CNCF project maintainers by:

1. **Organization Creation** - Automatically creates KDP organizations for:
   - Every CNCF project (service consumers, 240+ projects)
   - Foundation services (service providers and management)

2. **Access Management** - Sets up access for:
   - Project maintainers (as KDP organization admins)
   - Collaborators (as read-only users)
   - CNCF Staff and support team members (as admins via `kdp:owner` ClusterRole)

3. **CRD Integration** - Consumes CRDs from the maintainer-d operator:
   - `projects.maintainer-d.cncf.io` - source for project organizations
   - `maintainers.maintainer-d.cncf.io` - for admin membership
   - `collaborators.maintainer-d.cncf.io` - for read-only access
   - `staffmembers.maintainer-d.cncf.io` - for CNCF staff access across all workspaces

4. **Workspace Hierarchy** - Manages KDP workspace structure using kcp:
   - v1 uses a flat hierarchy under root workspace
   - KDP organization names derived from GitHub org slugs
   - Delegates cross-workspace service setup to separate operators

### Architecture

The operator uses a **multi-controller architecture** for separation of concerns:

#### Controllers

1. **ProjectReconciler** (`internal/controller/project_controller.go`)
   - Watches `projects.maintainer-d.cncf.io` resources on Service Cluster
   - Creates and manages KDP workspaces (type: `kdp-organization`) on KDP Cluster
   - Updates Project annotations with workspace status and metadata
   - Single responsibility: workspace lifecycle management

2. **StaffMemberReconciler** (`internal/controller/staffmember_controller.go`)
   - Watches `staffmembers.maintainer-d.cncf.io` resources on Service Cluster
   - Watches `projects.maintainer-d.cncf.io` for workspace readiness events
   - Creates/updates `ClusterRoleBinding/cncf-staff-access` in all managed workspaces
   - Grants `kdp:owner` role to all staff members across all workspaces
   - Updates StaffMember annotations with sync status
   - Single responsibility: staff access management

#### Staff Access Flow

1. StaffMember or Project change triggers reconciliation
2. Controller fetches ALL StaffMembers from `maintainerd` namespace
3. Lists all managed workspaces (annotation: `managed-by=kdp-ws-operator`)
4. For each Ready workspace:
   - Builds subjects list: `oidc:<primaryEmail>` for each staff member
   - Creates/updates `ClusterRoleBinding/cncf-staff-access` with:
     - Subjects: all staff members with OIDC prefix
     - RoleRef: `ClusterRole/kdp:owner`
     - Annotations: last-synced, staff-count, managed-by
5. Updates StaffMember annotations with sync results

### Design Reference

See `PLAN_STAFF_MEMBERS_SUPPORT.md` for detailed implementation plan.

## Testing Operator

### Verify Workspace Creation (ProjectReconciler)

#### On Service Cluster
```bash
kubectx context-cdv2c4jfn5q

# Check Project status and annotations
kubectl get projects.maintainer-d.cncf.io -n maintainerd
kubectl get projects.maintainer-d.cncf.io <project-name> -n maintainerd -o yaml

# Verify workspace annotations on Project
kubectl get project <project-name> -n maintainerd \
  -o jsonpath='{.metadata.annotations}' | jq .
# Expected: kdp-workspaces.cncf.io/workspace-name, workspace-phase, workspace-url
```

#### On KDP Cluster (https://services.cncf.io/)
```bash
export KUBECONFIG=$(git rev-parse --show-toplevel)/tmp/kdp-cluster-cncf/kubeconfig-admin

# List workspaces (type: kdp-organization)
kubectl get ws

# Describe specific workspace
kubectl describe ws <project-name>

# Verify workspace is managed by operator
kubectl get ws <project-name> -o jsonpath='{.metadata.annotations.managed-by}'
# Expected: kdp-ws-operator
```

### Verify Staff Access (StaffMemberReconciler)

#### On Service Cluster
```bash
kubectx context-cdv2c4jfn5q

# List all staff members
kubectl get staffmembers.maintainer-d.cncf.io -n maintainerd

# Check specific staff member with annotations
kubectl get staffmember <staff-name> -n maintainerd -o yaml

# Verify staff member sync annotations
kubectl get staffmember <staff-name> -n maintainerd \
  -o jsonpath='{.metadata.annotations}' | jq .
# Expected annotations:
# - kdp-workspaces.cncf.io/last-synced: "2025-01-14T10:30:45Z"
# - kdp-workspaces.cncf.io/sync-status: "success" | "partial" | "error"
# - kdp-workspaces.cncf.io/workspace-count: "247"

# Find staff members with failed syncs
kubectl get staffmembers -n maintainerd -o json | \
  jq '.items[] | select(.metadata.annotations["kdp-workspaces.cncf.io/sync-status"] != "success")'
```

#### On KDP Cluster
```bash
export KUBECONFIG=$(git rev-parse --show-toplevel)/tmp/kdp-cluster-cncf/kubeconfig-admin

# Verify staff access binding exists in a workspace
kubectl get clusterrolebinding cncf-staff-access -o yaml

# Check binding has OIDC subjects
kubectl get clusterrolebinding cncf-staff-access \
  -o jsonpath='{.subjects[*].name}' | tr ' ' '\n' | grep oidc

# Verify binding annotations
kubectl get clusterrolebinding cncf-staff-access \
  -o jsonpath='{.metadata.annotations}' | jq .
# Expected annotations:
# - kdp-workspaces.cncf.io/last-synced: "2025-01-14T10:30:45Z"
# - kdp-workspaces.cncf.io/managed-by: "kdp-ws-operator"
# - kdp-workspaces.cncf.io/source-namespace: "maintainerd"
# - kdp-workspaces.cncf.io/staff-count: "12"

# List all managed staff bindings across workspaces
kubectl get clusterrolebinding -A -l managed-by=kdp-ws-operator

# Verify RoleRef points to kdp:owner
kubectl get clusterrolebinding cncf-staff-access \
  -o jsonpath='{.roleRef}'
# Expected: {"apiGroup":"rbac.authorization.k8s.io","kind":"ClusterRole","name":"kdp:owner"}
```

### Debug Operator Issues

```bash
kubectx context-cdv2c4jfn5q

# Check operator logs (both controllers)
kubectl logs -n kdp-workspaces-system deployment/kdp-workspaces-controller-manager -f

# Filter logs for specific controller
kubectl logs -n kdp-workspaces-system deployment/kdp-workspaces-controller-manager -f | \
  grep "StaffMemberReconciler"

# Check operator events
kubectl get events -n kdp-workspaces-system --sort-by='.lastTimestamp'

# Verify operator RBAC permissions
kubectl auth can-i list staffmembers.maintainer-d.cncf.io -n maintainerd \
  --as=system:serviceaccount:kdp-workspaces-system:kdp-workspaces-controller-manager

# Check KCP connection config
kubectl get configmap kdp-workspaces -n kdp-workspaces-system -o yaml
kubectl get secret kdp-workspaces -n kdp-workspaces-system -o yaml
```

## Configuration

### Operator Flags

The operator accepts the following flags:

```bash
# KCP cluster connection
--kcp-configmap-name=kdp-workspaces         # ConfigMap name for KCP configuration
--kcp-configmap-namespace=kdp-workspaces-system
--kcp-secret-name=kdp-workspaces            # Secret name for KCP kubeconfig
--kcp-secret-namespace=kdp-workspaces-system

# Staff member configuration
--staff-namespace=maintainerd               # Namespace to watch StaffMember CRDs

# Standard controller flags
--leader-elect=true                         # Enable leader election
--metrics-bind-address=:8080
--health-probe-bind-address=:8081
```

### KCP Connection ConfigMap

Format expected in ConfigMap:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kdp-workspaces
  namespace: kdp-workspaces-system
data:
  workspace-path: "root"                    # Workspace path in KCP cluster
  workspace-type: "kdp-organization"        # Type of workspaces to create
```

### KCP Connection Secret

Format expected in Secret:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: kdp-workspaces
  namespace: kdp-workspaces-system
type: Opaque
data:
  kubeconfig: <base64-encoded-kubeconfig>   # KCP cluster kubeconfig
```

## Annotations Reference

### StaffMember Annotations (Service Cluster)

Added by StaffMemberReconciler to track sync status:

| Annotation | Value | Description |
|------------|-------|-------------|
| `kdp-workspaces.cncf.io/last-synced` | RFC3339 timestamp | When this staff member was last synced to workspaces |
| `kdp-workspaces.cncf.io/sync-status` | `success` \| `partial` \| `error` | Status of last sync operation |
| `kdp-workspaces.cncf.io/workspace-count` | Integer | Number of workspaces this member has access to |

**Example:**
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

### Project Annotations (Service Cluster)

Added by ProjectReconciler to track workspace status:

| Annotation | Value | Description |
|------------|-------|-------------|
| `kdp-workspaces.cncf.io/workspace-name` | String | Name of the created workspace |
| `kdp-workspaces.cncf.io/workspace-url` | URL | Access URL for the workspace |
| `kdp-workspaces.cncf.io/workspace-phase` | `Ready` \| `Pending` \| etc. | Current phase of workspace |

### ClusterRoleBinding Annotations (KDP Cluster)

Added by StaffMemberReconciler to track binding metadata:

| Annotation | Value | Description |
|------------|-------|-------------|
| `kdp-workspaces.cncf.io/last-synced` | RFC3339 timestamp | When this binding was last updated |
| `kdp-workspaces.cncf.io/staff-count` | Integer | Number of staff members in this binding |
| `kdp-workspaces.cncf.io/managed-by` | `kdp-ws-operator` | Identifies operator managing this binding |
| `kdp-workspaces.cncf.io/source-namespace` | `maintainerd` | Source namespace for StaffMembers |

**Labels:**
- `managed-by=kdp-ws-operator` - enables efficient querying of managed bindings

**Example:**
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cncf-staff-access
  annotations:
    kdp-workspaces.cncf.io/last-synced: "2025-01-14T10:30:45Z"
    kdp-workspaces.cncf.io/managed-by: "kdp-ws-operator"
    kdp-workspaces.cncf.io/source-namespace: "maintainerd"
    kdp-workspaces.cncf.io/staff-count: "12"
  labels:
    managed-by: kdp-ws-operator
subjects:
  - kind: User
    name: oidc:staff-one@example.test
  - kind: User
    name: oidc:staff-two@example.test
  # ... (more staff members)
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kdp:owner
```

## Manual Testing Scenarios

### Test Staff Member Addition

```bash
# 1. Create a test staff member on Service Cluster
kubectx context-cdv2c4jfn5q

kubectl apply -f - <<EOF
apiVersion: maintainer-d.cncf.io/v1alpha1
kind: StaffMember
metadata:
  name: test-staff-cncf.io
  namespace: maintainerd
spec:
  displayName: "Test Staff"
  primaryEmail: "test-staff@example.test"
EOF

# 2. Wait for reconciliation (check logs)
kubectl logs -n kdp-workspaces-system deployment/kdp-workspaces-controller-manager -f | \
  grep "test-staff"

# 3. Verify annotations added to StaffMember
kubectl get staffmember test-staff-cncf.io -n maintainerd -o yaml | grep -A5 annotations

# 4. Verify staff added to all workspace bindings on KDP Cluster
export KUBECONFIG=$(git rev-parse --show-toplevel)/tmp/kdp-cluster-cncf/kubeconfig-admin

kubectl get clusterrolebinding cncf-staff-access -o yaml | grep "test-staff@example.test"

# 5. Verify staff-count annotation incremented
kubectl get clusterrolebinding cncf-staff-access \
  -o jsonpath='{.metadata.annotations.kdp-workspaces\.cncf\.io/staff-count}'
```

### Test Staff Member Removal

```bash
# 1. Delete staff member on Service Cluster
kubectx context-cdv2c4jfn5q

kubectl delete staffmember test-staff-cncf.io -n maintainerd

# 2. Wait for reconciliation
kubectl logs -n kdp-workspaces-system deployment/kdp-workspaces-controller-manager -f

# 3. Verify staff removed from workspace bindings on KDP Cluster
export KUBECONFIG=$(git rev-parse --show-toplevel)/tmp/kdp-cluster-cncf/kubeconfig-admin

kubectl get clusterrolebinding cncf-staff-access -o yaml | grep "test-staff@example.test"
# Should return no results

# 4. Verify staff-count annotation decremented
kubectl get clusterrolebinding cncf-staff-access \
  -o jsonpath='{.metadata.annotations.kdp-workspaces\.cncf\.io/staff-count}'
```

### Test New Workspace Gets Staff Access

```bash
# 1. Create a new Project on Service Cluster
kubectx context-cdv2c4jfn5q

kubectl apply -f - <<EOF
apiVersion: maintainer-d.cncf.io/v1alpha1
kind: Project
metadata:
  name: test-project
  namespace: maintainerd
spec:
  displayName: "Test Project"
  # ... other spec fields
EOF

# 2. Wait for workspace to become Ready
kubectl get project test-project -n maintainerd \
  -o jsonpath='{.metadata.annotations.kdp-workspaces\.cncf\.io/workspace-phase}'
# Wait until: Ready

# 3. Trigger staff reconciliation (happens automatically when workspace becomes Ready)
# Check logs to see the project watch trigger reconciliation
kubectl logs -n kdp-workspaces-system deployment/kdp-workspaces-controller-manager -f | \
  grep "Project's workspace is ready"

# 4. Verify staff binding created in new workspace on KDP Cluster
export KUBECONFIG=$(git rev-parse --show-toplevel)/tmp/kdp-cluster-cncf/kubeconfig-admin

kubectl get clusterrolebinding cncf-staff-access -o yaml

# 5. Verify all staff members are in the binding
kubectl get clusterrolebinding cncf-staff-access \
  -o jsonpath='{.subjects[*].name}' | tr ' ' '\n'
```

## Error Handling

### Reconciliation Errors

The StaffMemberReconciler implements robust error handling:

1. **Partial Failures**: If some workspaces fail to update, the controller:
   - Logs each failed workspace with details
   - Aggregates errors using `errors.Join`
   - Updates StaffMember annotation with `sync-status: partial`
   - Returns aggregated error for controller-runtime to requeue

2. **Complete Failures**: If all workspaces fail:
   - Updates StaffMember annotation with `sync-status: error`
   - Uses exponential backoff for retries
   - Logs comprehensive failure information

3. **Network Issues**: KCP connection failures trigger:
   - Requeue with backoff
   - Error logged with context
   - No StaffMember annotation update (preserves last successful sync time)

### Checking for Errors

```bash
# Find staff members with sync errors
kubectl get staffmembers -n maintainerd -o json | \
  jq '.items[] | select(.metadata.annotations["kdp-workspaces.cncf.io/sync-status"] != "success") | {name: .metadata.name, status: .metadata.annotations["kdp-workspaces.cncf.io/sync-status"], lastSynced: .metadata.annotations["kdp-workspaces.cncf.io/last-synced"]}'

# Check operator logs for error details
kubectl logs -n kdp-workspaces-system deployment/kdp-workspaces-controller-manager | \
  grep -i error | tail -20

# Look for specific workspace failures
kubectl logs -n kdp-workspaces-system deployment/kdp-workspaces-controller-manager | \
  grep "Failed to update staff binding"
```

## Implementation Details

### Key Files

- `internal/controller/project_controller.go` - ProjectReconciler implementation
- `internal/controller/staffmember_controller.go` - StaffMemberReconciler implementation
- `internal/kcp/client.go` - KCP cluster client wrapper
- `internal/kcp/workspace.go` - Workspace management functions
- `internal/kcp/rbac.go` - RBAC (ClusterRoleBinding) management functions
- `cmd/main.go` - Operator entry point with controller registration

### Reconciliation Strategy

**Current (MVP)**: "Rebuild-all" strategy
- Every StaffMember or Project change triggers full sync
- Fetches ALL StaffMembers and rebuilds complete subject list
- Updates ALL managed workspaces with complete binding
- Simple, correct, self-healing
- Trade-off: Less efficient for large deployments (O(staff × workspaces))

**Future Optimization** (documented in PLAN_STAFF_MEMBERS_SUPPORT.md Phase 6):
- Incremental updates: only add/remove changed subject
- Workspace watch: immediate access on new workspace creation
- Efficiency improvement: O(workspaces) instead of O(staff × workspaces)

### Cross-Cluster Watch

The StaffMemberReconciler watches resources on both clusters:

1. **Service Cluster** (primary):
   - Watches `StaffMember` resources (primary trigger)
   - Watches `Project` resources (for workspace readiness)
   - Uses controller-runtime manager's default client

2. **KDP Cluster** (via KCP client):
   - Lists workspaces (no watch in MVP)
   - Creates/updates ClusterRoleBindings
   - Uses separate dynamic client with workspace path

## Troubleshooting

### Staff Access Not Working

1. **Verify StaffMember has email**:
   ```bash
   kubectl get staffmember <name> -n maintainerd -o jsonpath='{.spec.primaryEmail}'
   ```

2. **Check sync status**:
   ```bash
   kubectl get staffmember <name> -n maintainerd -o jsonpath='{.metadata.annotations}'
   ```

3. **Verify binding exists in workspace**:
   ```bash
   KUBECONFIG=... kubectl get clusterrolebinding cncf-staff-access
   ```

4. **Check subject format** (must have `oidc:` prefix):
   ```bash
   KUBECONFIG=... kubectl get clusterrolebinding cncf-staff-access \
     -o jsonpath='{.subjects[*].name}'
   ```

### Workspace Not Getting Staff Access

1. **Verify workspace is Ready**:
   ```bash
   kubectl get project <name> -n maintainerd \
     -o jsonpath='{.metadata.annotations.kdp-workspaces\.cncf\.io/workspace-phase}'
   ```

2. **Verify workspace is managed by operator**:
   ```bash
   KUBECONFIG=... kubectl get ws <name> -o jsonpath='{.metadata.annotations.managed-by}'
   ```
   Expected: `kdp-ws-operator`

3. **Check if Project watch triggered reconciliation**:
   ```bash
   kubectl logs -n kdp-workspaces-system deployment/kdp-workspaces-controller-manager | \
     grep "Project's workspace is ready"
   ```

4. **Manually trigger reconciliation** by updating any StaffMember:
   ```bash
   kubectl annotate staffmember <any-staff> -n maintainerd \
     trigger-sync="$(date +%s)" --overwrite
   ```

## References

- Implementation Plan: `PLAN_STAFF_MEMBERS_SUPPORT.md`
- Controller Pattern: Multi-controller architecture with independent reconciliation loops
```
