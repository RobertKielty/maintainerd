"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Card } from "clo-ui/components/Card";
import styles from "./MaintainerServicesPanel.module.css";

export type MaintainerServiceAccount = {
  state: string;
  matchedBy: string;
  remoteUserId?: number;
  remoteRef?: string;
  emailUsed?: string;
  lastCheckedAt?: string;
  pendingInvitations?: number;
  acceptedInvitations?: number;
  error?: string;
};

export type MaintainerServiceTarget = {
  projectId: number;
  projectName: string;
  targetKind: string;
  targetId?: number;
  targetName: string;
  required: boolean;
  state: string;
  pendingInvite?: boolean;
  lastCheckedAt?: string;
  error?: string;
};

export type MaintainerServiceView = {
  kind: string;
  label: string;
  account: MaintainerServiceAccount;
  targets: MaintainerServiceTarget[];
};

type MaintainerServicesPanelProps = {
  apiBaseUrl: string;
  maintainerId: number;
  disabled?: boolean;
  onMaintainerUpdated: (next: unknown) => void;
  services: MaintainerServiceView[];
};

const accountStateLabel: Record<string, string> = {
  registered: "Registered",
  invited: "Invited",
  not_registered: "Not Registered",
  unknown: "Unknown",
  error: "Error",
};

const targetStateLabel: Record<string, string> = {
  member: "Member",
  missing: "Missing",
  pending: "Pending",
  error: "Error",
  not_applicable: "Not Applicable",
};

const targetKindLabel: Record<string, string> = {
  team: "Team",
  organization: "Organization",
  project: "Project",
  mailing_list: "Mailing List",
};

function buildFossaTeamHref(targetId?: number) {
  if (typeof targetId !== "number") {
    return null;
  }
  return `https://app.fossa.com/account/settings/organization/teams/${targetId}`;
}

function buildFossaUserHref(remoteUserId?: number) {
  if (typeof remoteUserId !== "number") {
    return null;
  }
  return `https://app.fossa.com/account/settings/organization/users/${remoteUserId}`;
}

function formatDateTime(value?: string | null) {
  if (!value) {
    return "—";
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return "—";
  }
  return parsed.toLocaleString("en-US", {
    dateStyle: "medium",
    timeStyle: "short",
  });
}

function AccountStateBadge({ state }: { state: string }) {
  const label = accountStateLabel[state] || state;
  return (
    <span className={`${styles.badge} ${styles[`badge_${state}`] || styles.badge_default}`}>
      {label}
    </span>
  );
}

function WorkEmailIcon() {
  return (
    <svg aria-hidden="true" className={styles.inlineIcon} viewBox="0 0 24 24">
      <path
        d="M7 4.5h10a2.5 2.5 0 0 1 2.5 2.5v10A2.5 2.5 0 0 1 17 19.5H7A2.5 2.5 0 0 1 4.5 17V7A2.5 2.5 0 0 1 7 4.5Zm0 2A.5.5 0 0 0 6.5 7v2h11V7a.5.5 0 0 0-.5-.5H7Zm-.5 4.5V17c0 .28.22.5.5.5h10a.5.5 0 0 0 .5-.5v-6H6.5Z"
        fill="currentColor"
      />
    </svg>
  );
}

function GitHubIcon() {
  return (
    <svg aria-hidden="true" className={styles.inlineIcon} viewBox="0 0 24 24">
      <path
        d="M12 2C6.48 2 2 6.58 2 12.22c0 4.5 2.87 8.3 6.84 9.64.5.1.68-.22.68-.5 0-.24-.01-1.04-.01-1.9-2.78.62-3.37-1.21-3.37-1.21-.46-1.2-1.11-1.52-1.11-1.52-.91-.64.07-.62.07-.62 1 .07 1.53 1.06 1.53 1.06.9 1.58 2.35 1.13 2.92.86.09-.67.35-1.13.63-1.39-2.22-.26-4.56-1.14-4.56-5.06 0-1.12.39-2.03 1.03-2.75-.1-.26-.45-1.3.1-2.72 0 0 .84-.28 2.75 1.05A9.3 9.3 0 0 1 12 6.85c.85 0 1.7.12 2.5.36 1.9-1.33 2.74-1.05 2.74-1.05.56 1.42.21 2.46.11 2.72.64.72 1.03 1.63 1.03 2.75 0 3.93-2.35 4.8-4.59 5.05.36.32.69.95.69 1.92 0 1.39-.01 2.5-.01 2.84 0 .28.18.61.69.5A10.24 10.24 0 0 0 22 12.22C22 6.58 17.52 2 12 2Z"
        fill="currentColor"
      />
    </svg>
  );
}

function TargetStateBadge({ state }: { state: string }) {
  const label = targetStateLabel[state] || state;
  return (
    <span className={`${styles.badge} ${styles[`badge_${state}`] || styles.badge_default}`}>
      {label}
    </span>
  );
}

function ServiceAccountStatus({
  account,
  canInvite,
  disabled = false,
  isInviting = false,
  onInvite,
  serviceKind,
}: {
  account: MaintainerServiceAccount;
  canInvite: boolean;
  disabled?: boolean;
  isInviting?: boolean;
  onInvite: () => void;
  serviceKind: string;
}) {
  const remoteUserHref =
    serviceKind === "fossa" ? buildFossaUserHref(account.remoteUserId) : null;
  const showPendingInvites = (account.pendingInvitations ?? 0) > 0;
  const showAcceptedInvites = (account.acceptedInvitations ?? 0) > 0;
  const statusSourceLabel =
    account.matchedBy === "github_email" ? "GitHub" : account.matchedBy === "maintainer_email" ? "Work" : null;

  return (
    <div className={styles.accountGrid}>
      <div className={styles.stateRow}>
        <AccountStateBadge state={account.state} />
        {account.state === "registered" || account.state === "invited" ? (
          <>
            {account.emailUsed ? (
              <span className={styles.stateText}>
                using <span className={styles.stateEmail}>{account.emailUsed}</span>
                {statusSourceLabel ? (
                  <span className={styles.stateSource}>
                    {account.matchedBy === "github_email" ? <GitHubIcon /> : <WorkEmailIcon />}
                    <span>{statusSourceLabel}</span>
                  </span>
                ) : null}
              </span>
            ) : null}
          </>
        ) : null}
        {account.state === "not_registered" && canInvite ? (
          <button
            className={styles.inlineActionButton}
            disabled={disabled || isInviting}
            onClick={onInvite}
            type="button"
          >
            {isInviting ? "Sending…" : "Send Invite"}
          </button>
        ) : null}
        {account.state === "error" && account.error ? (
          <span className={`${styles.stateText} ${styles.errorText}`}>{account.error}</span>
        ) : null}
        {remoteUserHref ? (
          <a
            className={styles.remoteLink}
            href={remoteUserHref}
            rel="noreferrer"
            target="_blank"
          >
            FOSSA User Account
          </a>
        ) : null}
      </div>
      {showPendingInvites ? (
        <div className={styles.detailRow}>
          <span className={styles.detailLabel}>Pending Invites</span>
          <span className={styles.detailValue}>{account.pendingInvitations}</span>
        </div>
      ) : null}
      {showAcceptedInvites ? (
        <div className={styles.detailRow}>
          <span className={styles.detailLabel}>Accepted Invites</span>
          <span className={styles.detailValue}>{account.acceptedInvitations}</span>
        </div>
      ) : null}
    </div>
  );
}

function ServiceTargetMembershipTable({
  serviceKind,
  targets,
}: {
  serviceKind: string;
  targets: MaintainerServiceTarget[];
}) {
  if (targets.length === 0) {
    return <div className={styles.empty}>No service targets are required for this maintainer.</div>;
  }

  return (
    <div className={styles.tableWrap}>
      <table className={styles.table}>
        <thead>
          <tr>
            <th>Project</th>
            <th>Target Type</th>
            <th>Remote Target</th>
            <th>Required</th>
            <th>State</th>
            <th>Last Checked</th>
            <th>Error</th>
          </tr>
        </thead>
        <tbody>
          {targets.map((target) => (
            <tr key={`${target.projectId}-${target.targetKind}-${target.targetId ?? target.targetName}`}>
              <td>
                <Link href={`/projects/${target.projectId}`}>{target.projectName}</Link>
              </td>
              <td>{targetKindLabel[target.targetKind] || target.targetKind}</td>
              <td>
                {serviceKind === "fossa" && buildFossaTeamHref(target.targetId) ? (
                  <a
                    className={styles.remoteLink}
                    href={buildFossaTeamHref(target.targetId) || undefined}
                    rel="noreferrer"
                    target="_blank"
                  >
                    {target.targetName}
                  </a>
                ) : (
                  <span className={styles.targetName}>{target.targetName}</span>
                )}
              </td>
              <td>{target.required ? "Yes" : "No"}</td>
              <td>
                <TargetStateBadge state={target.state} />
              </td>
              <td>{formatDateTime(target.lastCheckedAt)}</td>
              <td className={styles.errorText}>{target.error || "—"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function actionLabel(action: "refresh" | "invite" | "reconcile") {
  switch (action) {
    case "refresh":
      return "Refresh";
    case "invite":
      return "Send Invite";
    case "reconcile":
      return "Reconcile Missing";
    default:
      return action;
  }
}

function MaintainerServiceCard({
  apiBaseUrl,
  disabled = false,
  maintainerId,
  onMaintainerUpdated,
  service,
}: {
  apiBaseUrl: string;
  disabled?: boolean;
  maintainerId: number;
  onMaintainerUpdated: (next: unknown) => void;
  service: MaintainerServiceView;
}) {
  const missingCount = service.targets.filter((target) => target.state === "missing").length;
  const pendingCount = service.targets.filter((target) => target.state === "pending").length;
  const [activeAction, setActiveAction] = useState<"refresh" | "invite" | "reconcile" | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionNotice, setActionNotice] = useState<string | null>(null);

  const canInvite = service.kind === "fossa" && service.account.state === "not_registered";
  const canReconcile =
    service.kind === "fossa" &&
    service.account.state === "registered" &&
    missingCount > 0;

  useEffect(() => {
    if (!actionNotice) {
      return;
    }
    const timer = window.setTimeout(() => setActionNotice(null), 5000);
    return () => window.clearTimeout(timer);
  }, [actionNotice]);

  const handleAction = async (action: "refresh" | "invite" | "reconcile") => {
    setActiveAction(action);
    setActionError(null);
    try {
      const response = await fetch(
        `${apiBaseUrl}/maintainers/${maintainerId}/services/${service.kind}/${action}`,
        {
          method: "POST",
          credentials: "include",
        }
      );
      if (!response.ok) {
        const message = await response.text();
        throw new Error(message || `unexpected status ${response.status}`);
      }
      const data = await response.json();
      onMaintainerUpdated(data);
      setActionNotice(`${actionLabel(action)} completed`);
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "Action failed");
    } finally {
      setActiveAction(null);
    }
  };

  return (
    <Card hoverable={false} className={styles.card}>
      <div className={styles.cardContent}>
        <div className={styles.header}>
          <div>
            <h2 className={styles.title}>{service.label}</h2>
          </div>
          <div className={styles.summary}>
            <span className={styles.summaryItem}>{service.targets.length} targets</span>
            <span className={styles.summaryItem}>{missingCount} missing</span>
            <span className={styles.summaryItem}>{pendingCount} pending</span>
          </div>
        </div>

        <div className={styles.actions}>
          <button
            className={styles.actionButton}
            disabled={disabled || activeAction !== null}
            onClick={() => void handleAction("refresh")}
            type="button"
          >
            {activeAction === "refresh" ? "Refreshing…" : "Refresh"}
          </button>
          {canReconcile ? (
            <button
              className={styles.actionButton}
              disabled={disabled || activeAction !== null}
              onClick={() => void handleAction("reconcile")}
              type="button"
            >
              {activeAction === "reconcile" ? "Reconciling…" : "Reconcile Missing"}
            </button>
          ) : null}
        </div>
        {actionNotice ? <div className={styles.notice}>{actionNotice}</div> : null}
        {actionError ? <div className={styles.errorText}>{actionError}</div> : null}

        <section className={styles.section}>
          <h3 className={styles.sectionTitle}>Remote Service User Account</h3>
          <ServiceAccountStatus
            account={service.account}
            canInvite={canInvite}
            disabled={disabled || activeAction !== null}
            isInviting={activeAction === "invite"}
            onInvite={() => void handleAction("invite")}
            serviceKind={service.kind}
          />
        </section>

        <section className={styles.section}>
          <h3 className={styles.sectionTitle}>Project Target Memberships</h3>
          <ServiceTargetMembershipTable serviceKind={service.kind} targets={service.targets} />
        </section>
      </div>
    </Card>
  );
}

export default function MaintainerServicesPanel({
  apiBaseUrl,
  maintainerId,
  disabled = false,
  onMaintainerUpdated,
  services,
}: MaintainerServicesPanelProps) {
  if (services.length === 0) {
    return null;
  }

  return (
    <section className={styles.panel}>
      {services.map((service) => (
        <MaintainerServiceCard
          key={service.kind}
          apiBaseUrl={apiBaseUrl}
          disabled={disabled}
          maintainerId={maintainerId}
          onMaintainerUpdated={onMaintainerUpdated}
          service={service}
        />
      ))}
    </section>
  );
}
