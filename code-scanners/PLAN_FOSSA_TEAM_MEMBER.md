# FOSSA Team Membership Implementation Plan

**Last Updated**: 2026-02-04
**Status**: Draft
**Depends On**: PLAN_FOSSA_EMAIL.md (completed)

## Overview

Implement automatic FOSSA team membership management for the `CodeScannerFossa` controller. When users accept FOSSA organization invitations, the controller will automatically add them to the appropriate project team and track their membership status.

**Prerequisite**: The FOSSA email invitation functionality from PLAN_FOSSA_EMAIL.md must be implemented first.

## Problem Statement

Currently, the controller:
1. ✅ Invites users to FOSSA organization via email
2. ✅ Tracks invitation status (Pending, Accepted, Failed, Expired)
3. ❌ **Does NOT add accepted users to the project team**

When a user accepts a FOSSA organization invitation, they become a member of the organization but are **not automatically added to any team**. They must be explicitly added to the project team to access project resources.

## FOSSA API Analysis

### Available Client Methods (from plugins/fossa/client.go)

| Method | Line | Purpose | Returns |
|--------|------|---------|---------|
| `AddUserToTeamByEmail(teamID, email, roleID)` | 286-346 | Add user to team by email | `error` (idempotent) |
| `findUserIDByEmail(email)` | 348-372 | Resolve email to FOSSA user ID | `int, error` |
| `FetchTeamUserEmails(teamID)` | 248-281 | Get current team members | `[]string, error` |
| `FetchUsers()` | 68-115 | Get all org users (paginated) | `[]User, error` |

### Team Membership API Flow

```
User State Transitions:
┌─────────────┐   Accept      ┌─────────────┐   AddUserToTeam   ┌─────────────┐
│  Invited    │─ Invitation ─>│ Org Member  │───── API Call ───>│Team Member  │
│  (Pending)  │               │ (Accepted)  │                   │(AddedToTeam)│
└─────────────┘               └─────────────┘                   └─────────────┘
```

### AddUserToTeamByEmail Implementation Details

**Endpoint**: `PUT /api/teams/{teamID}/users`

**Request Body**:
```json
{
  "users": [
    {
      "id": 12345  // User ID resolved from email
    }
  ],
  "action": "add"
}
```

**Key Behaviors** (from client.go:286-346):
1. Resolves email → user ID via `findUserIDByEmail()` (line 291)
2. Searches all users with normalized email matching (case-insensitive)
3. Returns `ErrUserAlreadyMember` (code 2001) if user already on team - **IDEMPOTENT**
4. Returns 200/201/204 on success
5. No explicit roleID in current implementation (uses team default)

### Team Member Retrieval

**Endpoint**: `GET /api/teams/{teamID}/members`

**Response Schema** (line 533-543):
```go
type TeamMembers struct {
    Results []struct {
        UserID   int    `json:"userId"`
        RoleID   int    `json:"roleId"`
        Username string `json:"username"`
        Email    string `json:"email"`
    } `json:"results"`
    PageSize   int `json:"pageSize"`
    Page       int `json:"page"`
    TotalCount int `json:"totalCount"`
}
```

## Architecture

### Status Progression Model

```
FossaUserInvitation Status Lifecycle:

Pending ──┬──> Accepted ──> AddedToTeam ─┐
          │                                ├──> (stable state)
          │                                │
          ├──> Expired                     │
          │      │                         │
          │      └──> Pending (resend) ────┘
          │
          └──> Failed

AlreadyMember ──> AddedToTeam (verify team membership)
```

### Status Field Extension

Extend `FossaUserInvitation` struct:

```go
type FossaUserInvitation struct {
    // Email is the user's email address
    Email string `json:"email"`

    // Status is the current invitation status
    // Valid values: Pending, Accepted, AddedToTeam, AlreadyMember, Failed, Expired
    Status string `json:"status"`

    // Message provides additional context about the status
    // +optional
    Message string `json:"message,omitempty"`

    // InvitedAt is when the invitation was sent
    // +optional
    InvitedAt *metav1.Time `json:"invitedAt,omitempty"`

    // AcceptedAt is when the user accepted the invitation
    // +optional
    AcceptedAt *metav1.Time `json:"acceptedAt,omitempty"`

    // AddedToTeamAt is when the user was added to the FOSSA team
    // +optional
    AddedToTeamAt *metav1.Time `json:"addedToTeamAt,omitempty"`  // NEW FIELD
}
```

### Reconciliation Logic Flow

```
┌────────────────────────────────────────────────────────────────┐
│ ensureUserInvitations() - EXISTING FUNCTION                    │
│ - Check if user is org member (FetchUsers)                     │
│ - Send invitation if needed                                    │
│ - Update status: Pending/Accepted/Failed/Expired               │
└────────────────────────────────────────────────────────────────┘
                           ↓
┌────────────────────────────────────────────────────────────────┐
│ ensureTeamMembership() - NEW FUNCTION                          │
│ FOR each invitation with status "Accepted" or "AlreadyMember": │
│   1. Check if user is on team (FetchTeamUserEmails)            │
│   2. If NOT on team: AddUserToTeamByEmail(teamID, email, 0)    │
│   3. Update status to "AddedToTeam"                            │
│   4. Set AddedToTeamAt timestamp                               │
│   5. Handle ErrUserAlreadyMember gracefully (idempotent)       │
└────────────────────────────────────────────────────────────────┘
```

## Implementation Phases

### Phase 1: Extend CRD Types

**Goal**: Add `AddedToTeamAt` field to track when users join the team

**Tasks**:

1. Update `FossaUserInvitation` struct in `api/v1alpha1/codescannerfossa_types.go`:
   ```go
   // AddedToTeamAt is when the user was added to the FOSSA team
   // +optional
   AddedToTeamAt *metav1.Time `json:"addedToTeamAt,omitempty"`
   ```

2. Add new invitation status constant in `internal/controller/constants.go`:
   ```go
   InvitationStatusAddedToTeam = "AddedToTeam"
   ```

3. Add new condition reason for team membership:
   ```go
   ReasonTeamMembershipProcessed = "TeamMembershipProcessed"
   ```

4. Regenerate CRDs:
   ```bash
   make generate manifests
   ```

**Files to modify**:
- `code-scanners/api/v1alpha1/codescannerfossa_types.go`
- `code-scanners/internal/controller/constants.go`

**Files generated**:
- `code-scanners/config/crd/bases/maintainer-d.cncf.io_codescannerfossas.yaml`

---

### Phase 2: Extend FossaClient Interface

**Goal**: Add team membership methods to the controller's `FossaClient` interface

**Tasks**:

1. Extend `FossaClient` interface in `codescannerfossa_controller.go`:
   ```go
   type FossaClient interface {
       CreateTeam(name string) (*fossa.Team, error)
       FetchTeam(name string) (*fossa.Team, error)
       // User invitation methods (existing)
       SendUserInvitation(email string) error
       HasPendingInvitation(email string) (bool, error)
       FetchUsers() ([]fossa.User, error)
       // Team membership methods (new)
       AddUserToTeamByEmail(teamID int, email string, roleID int) error
       FetchTeamUserEmails(teamID int) ([]string, error)
   }
   ```

2. Verify interface implementation matches real client signature

**Files to modify**:
- `code-scanners/internal/controller/codescannerfossa_controller.go` (line ~44-51)

---

### Phase 3: Implement Team Membership Logic

**Goal**: Add automatic team membership for accepted users

**Tasks**:

#### 3.1: Create `ensureTeamMembership()` helper

```go
// ensureTeamMembership ensures that users who have accepted invitations
// are added to the FOSSA team.
// It updates the invitation status to "AddedToTeam" when successful.
func (r *CodeScannerFossaReconciler) ensureTeamMembership(
    ctx context.Context,
    fossaClient FossaClient,
    teamID int,
    invitations []maintainerdcncfiov1alpha1.FossaUserInvitation,
) ([]maintainerdcncfiov1alpha1.FossaUserInvitation, error) {
    log := logf.FromContext(ctx)

    // Fetch current team members for comparison
    teamEmails, err := fossaClient.FetchTeamUserEmails(teamID)
    if err != nil {
        log.Error(err, "Failed to fetch team members")
        return invitations, fmt.Errorf("failed to fetch team members: %w", err)
    }

    // Build a set of team member emails (lowercase for comparison)
    teamMemberSet := make(map[string]bool)
    for _, email := range teamEmails {
        teamMemberSet[strings.ToLower(email)] = true
    }

    var updated []maintainerdcncfiov1alpha1.FossaUserInvitation
    var addedCount, alreadyMemberCount int

    for _, inv := range invitations {
        emailLower := strings.ToLower(inv.Email)

        // Only process users who have accepted but not yet added to team
        if inv.Status != InvitationStatusAccepted && inv.Status != InvitationStatusAlreadyMember {
            updated = append(updated, inv)
            continue
        }

        // Check if already on team (avoid unnecessary API call)
        if teamMemberSet[emailLower] {
            log.V(1).Info("User already on team", "email", inv.Email)
            if inv.Status != InvitationStatusAddedToTeam {
                now := metav1.Now()
                inv.Status = InvitationStatusAddedToTeam
                inv.Message = "User is a team member"
                inv.AddedToTeamAt = &now
                alreadyMemberCount++
            }
            updated = append(updated, inv)
            continue
        }

        // Add user to team
        log.Info("Adding user to FOSSA team", "email", inv.Email, "teamID", teamID)
        err := fossaClient.AddUserToTeamByEmail(teamID, inv.Email, 0)
        if err != nil {
            // Handle idempotency error gracefully
            if errors.Is(err, fossa.ErrUserAlreadyMember) {
                log.V(1).Info("User already on team (race condition)", "email", inv.Email)
                now := metav1.Now()
                inv.Status = InvitationStatusAddedToTeam
                inv.Message = "User is a team member"
                inv.AddedToTeamAt = &now
                alreadyMemberCount++
                updated = append(updated, inv)
                continue
            }

            log.Error(err, "Failed to add user to team", "email", inv.Email)
            inv.Message = fmt.Sprintf("Failed to add to team: %v", err)
            updated = append(updated, inv)
            continue
        }

        // Success - update status
        now := metav1.Now()
        inv.Status = InvitationStatusAddedToTeam
        inv.Message = "User added to team"
        inv.AddedToTeamAt = &now
        addedCount++
        updated = append(updated, inv)

        log.Info("User added to team successfully", "email", inv.Email)
    }

    if addedCount > 0 {
        log.Info("Team membership processed", "added", addedCount, "alreadyMember", alreadyMemberCount)
    }

    return updated, nil
}
```

#### 3.2: Integrate into main `Reconcile()` function

**Location**: After `ensureUserInvitations()` call (around line 183)

```go
// 8. Process user invitations if specified
var requeueAfter time.Duration
if len(fossaCR.Spec.FossaUserEmails) > 0 {
    // 8.1: Ensure invitations are sent
    invitations, hasPending, err := r.ensureUserInvitations(ctx, fossaClient, fossaCR.Spec.FossaUserEmails, fossaCR.Status.UserInvitations)
    if err != nil {
        log.Error(err, "Failed to process user invitations")
        // ... existing error handling ...
    }

    // 8.2: Ensure accepted users are added to team (NEW)
    if team != nil && team.ID > 0 {
        invitations, err = r.ensureTeamMembership(ctx, fossaClient, team.ID, invitations)
        if err != nil {
            log.Error(err, "Failed to process team membership")
            r.Recorder.Event(fossaCR, corev1.EventTypeWarning, "TeamMembershipFailed", err.Error())
            // Continue even if team membership fails - update status with what we have
        }
    }

    fossaCR.Status.UserInvitations = invitations

    // 8.3: Update conditions based on final status
    var pending, accepted, addedToTeam, failed, expired int
    for _, inv := range invitations {
        switch inv.Status {
        case InvitationStatusPending:
            pending++
        case InvitationStatusAccepted:
            accepted++
        case InvitationStatusAddedToTeam:
            addedToTeam++
        case InvitationStatusFailed:
            failed++
        case InvitationStatusExpired:
            expired++
        }
    }

    if addedToTeam == len(invitations) {
        r.setCondition(fossaCR, ConditionTypeUserInvitations, metav1.ConditionTrue,
            ReasonTeamMembershipProcessed, fmt.Sprintf("All %d users added to team", addedToTeam))
    } else if failed > 0 && addedToTeam == 0 && accepted == 0 && pending == 0 {
        r.setCondition(fossaCR, ConditionTypeUserInvitations, metav1.ConditionFalse,
            ReasonInvitationsFailed, fmt.Sprintf("All %d invitations failed", failed))
    } else {
        r.setCondition(fossaCR, ConditionTypeUserInvitations, metav1.ConditionFalse,
            ReasonInvitationsPartial,
            fmt.Sprintf("Invitations: %d on team, %d accepted, %d pending, %d failed, %d expired",
                addedToTeam, accepted, pending, failed, expired))
    }

    // Requeue if there are pending invitations or accepted users not yet on team
    if hasPending || accepted > 0 {
        requeueAfter = time.Hour
    }
}
```

**Files to modify**:
- `code-scanners/internal/controller/codescannerfossa_controller.go`

---

### Phase 4: Add Unit Tests

**Goal**: Comprehensive test coverage for team membership functionality

**Tasks**:

#### 4.1: Extend MockFossaClient

```go
type MockFossaClient struct {
    // Existing fields
    CreateTeamFunc           func(name string) (*fossa.Team, error)
    FetchTeamFunc            func(name string) (*fossa.Team, error)
    SendUserInvitationFunc   func(email string) error
    HasPendingInvitationFunc func(email string) (bool, error)
    FetchUsersFunc           func() ([]fossa.User, error)

    // New fields
    AddUserToTeamByEmailFunc func(teamID int, email string, roleID int) error
    FetchTeamUserEmailsFunc  func(teamID int) ([]string, error)
}

func (m *MockFossaClient) AddUserToTeamByEmail(teamID int, email string, roleID int) error {
    if m.AddUserToTeamByEmailFunc != nil {
        return m.AddUserToTeamByEmailFunc(teamID, email, roleID)
    }
    return nil
}

func (m *MockFossaClient) FetchTeamUserEmails(teamID int) ([]string, error) {
    if m.FetchTeamUserEmailsFunc != nil {
        return m.FetchTeamUserEmailsFunc(teamID)
    }
    return []string{}, nil
}
```

#### 4.2: Add Test Cases

**Test Suite**: `codescannerfossa_controller_test.go`

Test cases to add:

1. **TestEnsureTeamMembership_AcceptedUserAddedToTeam**
   - Given: User with status "Accepted"
   - When: ensureTeamMembership is called
   - Then: User is added to team, status becomes "AddedToTeam", AddedToTeamAt is set

2. **TestEnsureTeamMembership_UserAlreadyOnTeam**
   - Given: User with status "Accepted", already in team members list
   - When: ensureTeamMembership is called
   - Then: No API call made, status becomes "AddedToTeam" (idempotent)

3. **TestEnsureTeamMembership_AddToTeamAPIError**
   - Given: User with status "Accepted"
   - When: AddUserToTeamByEmail returns error
   - Then: Status remains "Accepted", error message set, function returns error

4. **TestEnsureTeamMembership_UserAlreadyMemberError**
   - Given: User with status "Accepted"
   - When: AddUserToTeamByEmail returns ErrUserAlreadyMember
   - Then: Error handled gracefully, status becomes "AddedToTeam" (idempotent)

5. **TestEnsureTeamMembership_PendingUserNotProcessed**
   - Given: User with status "Pending"
   - When: ensureTeamMembership is called
   - Then: User is skipped, status remains "Pending"

6. **TestEnsureTeamMembership_FetchTeamMembersError**
   - Given: FetchTeamUserEmails returns error
   - When: ensureTeamMembership is called
   - Then: Function returns error immediately

7. **TestReconcile_FullWorkflow_InviteAndAddToTeam**
   - Given: CR with fossaUserEmails
   - When: Reconcile runs multiple times
   - Then:
     - First reconcile: Invitation sent (status: Pending)
     - Second reconcile (user accepted): Status becomes Accepted
     - Third reconcile: User added to team (status: AddedToTeam)
     - Condition shows all users on team

8. **TestReconcile_TeamMembershipRequeue**
   - Given: Users with status "Accepted" but not on team
   - When: Reconcile runs
   - Then: RequeueAfter is set to 1 hour to retry team addition

**Files to modify**:
- `code-scanners/internal/controller/codescannerfossa_controller_test.go`

---

### Phase 5: Handle Edge Cases

**Goal**: Robust handling of edge cases and failure scenarios

**Tasks**:

#### 5.1: User Removed from Team Manually

**Scenario**: Admin removes user from team outside the controller

**Handling**:
- Next reconciliation detects user not on team (via FetchTeamUserEmails)
- Status transitions back to "Accepted"
- User is re-added to team on next reconcile

**Implementation**: Already handled by comparing `teamMemberSet` in ensureTeamMembership

#### 5.2: User Not Found by Email

**Scenario**: User accepted org invitation but email doesn't match FOSSA records

**Handling**:
- `findUserIDByEmail()` in FOSSA client returns error
- `AddUserToTeamByEmail()` returns error
- Status message: "Failed to add to team: user not found by email"
- Manual intervention required (verify email in FOSSA UI)

**Implementation**: Already handled in error path of ensureTeamMembership

#### 5.3: Team Deleted Manually

**Scenario**: FOSSA team is deleted outside the controller

**Handling**:
- `FetchTeamUserEmails()` returns 404 error
- `ensureTeamMembership()` returns error
- Next reconcile recreates team via `ensureFossaTeam()`
- Team membership is reestablished

**Implementation**: Error propagates up, next reconcile recreates team

#### 5.4: Race Condition - Multiple Controllers

**Scenario**: Multiple controller replicas try to add same user simultaneously

**Handling**:
- FOSSA API returns `ErrUserAlreadyMember` (code 2001)
- Both controllers handle gracefully (idempotent)
- Both update status to "AddedToTeam"

**Implementation**: Already handled by `errors.Is(err, fossa.ErrUserAlreadyMember)` check

---

### Phase 6: Monitoring and Observability

**Goal**: Proper event emission and logging for team membership operations

**Tasks**:

1. Add Kubernetes events:
   ```go
   r.Recorder.Event(fossaCR, corev1.EventTypeNormal, "UserAddedToTeam",
       fmt.Sprintf("User %s added to FOSSA team %d", email, teamID))

   r.Recorder.Event(fossaCR, corev1.EventTypeWarning, "TeamMembershipFailed",
       fmt.Sprintf("Failed to add user %s to team: %v", email, err))
   ```

2. Add structured logging:
   ```go
   log.Info("Team membership reconciliation complete",
       "teamID", teamID,
       "addedToTeam", addedCount,
       "alreadyMember", alreadyMemberCount,
       "total", len(invitations))
   ```

3. Update condition messages to be actionable:
   - ✅ Good: "All 3 users added to team"
   - ✅ Good: "Invitations: 2 on team, 1 accepted (pending team addition), 0 pending"
   - ❌ Bad: "Partially processed"

**Files to modify**:
- `code-scanners/internal/controller/codescannerfossa_controller.go`

---

## Status Values Extended

| Status | Description | Next Action |
|--------|-------------|-------------|
| `Pending` | Invitation sent, awaiting user acceptance | Wait for user to accept (requeue 1h) |
| `Accepted` | User accepted invitation, pending team addition | Add to team on next reconcile |
| `AddedToTeam` | User is a member of the FOSSA team | No action (stable state) |
| `AlreadyMember` | User was already a FOSSA organization member | Add to team if not already |
| `Failed` | Invitation or team addition failed | Manual intervention required |
| `Expired` | Invitation expired (48h TTL) without acceptance | Resend invitation |

---

## Error Handling

| Error | Source | Handling |
|-------|--------|----------|
| `ErrUserAlreadyMember (2001)` | AddUserToTeamByEmail | Set status to `AddedToTeam`, no action needed |
| API 404 on FetchTeamUserEmails | FOSSA API | Return error, next reconcile recreates team |
| API 500/503 errors | FOSSA API | Return error, requeue with backoff |
| User not found by email | findUserIDByEmail | Set status message, require manual verification |
| FetchUsers pagination failure | FOSSA API | Return error, retry on next reconcile |

---

## Dependencies

### Required FOSSA Client Methods

All methods already exist in `/plugins/fossa/client.go`:
- ✅ `AddUserToTeamByEmail(teamID int, email string, roleID int) error` - Line 286
- ✅ `FetchTeamUserEmails(teamID int) ([]string, error)` - Line 248
- ✅ `findUserIDByEmail(email string) (int, error)` - Line 348 (internal)
- ✅ `FetchUsers() ([]fossa.User, error)` - Line 68

No changes to FOSSA plugin required.

---

## Validation Checklist

- [ ] CRD schema accepts `addedToTeamAt` field
- [ ] Controller adds accepted users to FOSSA team
- [ ] Controller is idempotent (no duplicate team additions)
- [ ] Status accurately reflects team membership
- [ ] Users removed from team are re-added on next reconcile
- [ ] Events are emitted for team membership operations
- [ ] Unit tests pass for all scenarios
- [ ] Integration tests verify end-to-end flow
- [ ] `make generate manifests` produces valid CRD

---

## Example CR and Status

### Example CR (Unchanged)

```yaml
apiVersion: maintainer-d.cncf.io/v1alpha1
kind: CodeScannerFossa
metadata:
  name: argo-fossa
  namespace: code-scanners
spec:
  projectName: argo
  fossaUserEmails:
    - "maintainer1@example.dev"
    - "maintainer2@example.dev"
```

### Example Status After Full Reconciliation

```yaml
status:
  observedGeneration: 1
  fossaTeam:
    id: 456
    name: argo
    organizationId: 162
    url: https://app.fossa.com/account/settings/organization/teams/456
  userInvitations:
    - email: "maintainer1@example.dev"
      status: AddedToTeam                        # Changed from Accepted
      invitedAt: "2026-02-01T10:00:00Z"
      acceptedAt: "2026-02-02T14:30:00Z"
      addedToTeamAt: "2026-02-02T14:35:00Z"      # NEW FIELD
      message: "User added to team"
    - email: "maintainer2@example.dev"
      status: Accepted
      invitedAt: "2026-02-04T09:00:00Z"
      acceptedAt: "2026-02-04T15:20:00Z"
      message: "User accepted invitation, pending team addition"
  conditions:
    - type: FossaTeamReady
      status: "True"
      reason: TeamCreated
      message: "FOSSA team 'argo' ready (ID: 456)"
    - type: UserInvitationsProcessed
      status: "False"                             # Changed from True
      reason: InvitationsPartial                  # Changed
      message: "Invitations: 1 on team, 1 accepted, 0 pending, 0 failed, 0 expired"
```

---

## Reconciliation Loop Behavior

### Scenario: User Accepts Invitation

**Timeline**:

| Time | Event | Status Change | Requeue |
|------|-------|---------------|---------|
| T0 | CR created with email | N/A → Pending (invitation sent) | 1 hour |
| T1 (1h later) | User still hasn't accepted | Pending → Pending | 1 hour |
| T2 (user accepts) | User accepts invitation via email | Pending → Accepted | 1 hour |
| T3 (next reconcile) | Controller detects acceptance | Accepted → AddedToTeam | None (stable) |

**Key Insight**: The controller requires **two reconciliation cycles** after acceptance:
1. First cycle: Detect acceptance (status: Accepted)
2. Second cycle: Add to team (status: AddedToTeam)

This could be optimized to happen in a single reconciliation loop, but the two-step approach is clearer and easier to debug.

---

## Performance Considerations

### API Call Overhead

**Per Reconciliation** (with 3 users):

| Operation | API Calls | Cost |
|-----------|-----------|------|
| FetchUsers (check acceptance) | 1 (paginated) | O(n) users in org |
| HasPendingInvitation × 3 | 3 | O(n) pending invites |
| SendUserInvitation × 3 | 3 | O(1) |
| FetchTeamUserEmails | 1 | O(n) team members |
| AddUserToTeamByEmail × 3 | 3 | O(1) |
| **Total** | **11** | |

**Optimization Opportunities**:
- Cache FetchUsers result for 5 minutes (reduces org-wide queries)
- Batch team member additions (if FOSSA API supports bulk operations)
- Skip HasPendingInvitation if user is already "Accepted" in status

**Current Implementation**: No caching (conservative, always fresh data)

---

## Future Enhancements (Out of Scope)

1. **Role-Based Team Membership**:
   - Add `fossaUserRoles` field to map emails → roleIDs
   - Pass roleID to `AddUserToTeamByEmail()`

2. **Team Membership Removal**:
   - Remove users from spec → remove from team
   - Add finalizer to cleanup team memberships on CR deletion

3. **Metrics**:
   - Prometheus metrics for invitation/team membership status
   - Gauge: `fossa_team_members_total{team="argo"}`
   - Counter: `fossa_team_additions_total{status="success"}`

4. **Webhooks**:
   - Validate email format in admission webhook
   - Prevent duplicate emails in spec

---

## Testing Strategy

### Unit Tests

**Target**: 80%+ coverage of new functions
- Mock FOSSA client for all API calls
- Test all status transitions
- Test idempotency thoroughly

### Integration Tests

**Goal**: Validate real API integration

1. Create CR with test emails
2. Manually accept invitation (or use test account)
3. Verify controller adds user to team
4. Verify status updates correctly

### E2E Tests (Optional)

**Goal**: Full workflow with real FOSSA environment

Requires:
- FOSSA test organization
- Disposable email accounts for testing
- Automated invitation acceptance (email polling)

**Complexity**: High - defer to Phase 6 if needed

---

## Migration Path

### Existing CRs Without Team Membership

**Scenario**: Controller is upgraded with this feature, existing CRs have users in "Accepted" status

**Behavior**:
1. Next reconciliation reads existing status
2. Users with "Accepted" status trigger team membership logic
3. Users are added to team, status becomes "AddedToTeam"
4. No manual intervention required

**Rollback**:
- Old controller version ignores `addedToTeamAt` field (backward compatible)
- Old controller version continues to track "Accepted" status
- No data loss

---

## Implementation Order

1. **Phase 1**: Extend CRD types (30 min)
2. **Phase 2**: Extend FossaClient interface (15 min)
3. **Phase 3**: Implement team membership logic (2-3 hours)
4. **Phase 4**: Add unit tests (2-3 hours)
5. **Phase 5**: Handle edge cases (1 hour)
6. **Phase 6**: Monitoring and observability (30 min)

**Total Estimated Time**: 6-8 hours

---

## Success Criteria

✅ **Functional**:
- Users who accept invitations are automatically added to project team
- Status field accurately reflects team membership state
- Idempotent operations (safe to reconcile multiple times)

✅ **Quality**:
- 80%+ test coverage
- No API rate limit issues
- Clear error messages for failures

✅ **Observability**:
- Kubernetes events for team membership changes
- Structured logs for debugging
- Condition messages are actionable

---

## References

- **FOSSA Client Implementation**: `plugins/fossa/client.go`
- **Team Membership API**: `AddUserToTeamByEmail` (line 286-346)
- **Team Members Retrieval**: `FetchTeamUserEmails` (line 248-281)
- **User Resolution**: `findUserIDByEmail` (line 348-372)
- **Invitation Plan**: `PLAN_FOSSA_EMAIL.md`
- **Controller Implementation**: `code-scanners/internal/controller/codescannerfossa_controller.go`
