"use client";

import AppShell from "@/components/AppShell";
import AuditLogExplorer from "../AuditLogExplorer";

const syncRunActions = ["DOT_PROJECT_SYNC_RUN_STARTED", "DOT_PROJECT_SYNC_RUN_FINISHED"];

export default function AuditSyncRunsPage() {
  return (
    <AppShell>
      <AuditLogExplorer
        title="Sync Runs"
        subtitle="DOT-PROJECT sync run history, separate from the general audit log."
        lockedActions={syncRunActions}
      />
    </AppShell>
  );
}
