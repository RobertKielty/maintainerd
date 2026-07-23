"use client";

import AppShell from "@/components/AppShell";
import AuditLogExplorer from "./AuditLogExplorer";

const actionOptions = [
  "PROJECT_CREATE",
  "PROJECT_MAINTAINER_REF_UPDATE",
  "PROJECT_MATURITY_UPDATE",
  "MAINTAINER_UPDATE",
  "COMPANY_CREATE",
  "DOT_PROJECT_MAINTAINER_PR_CREATE",
  "ADD_DOT_PROJECT_MAINTAINER",
  "FOSSA_ADD_MEMBER",
  "FOSSA_TEAM_MEMBER_ADDED",
  "FOSSA_TEAM_MEMBER_ADD_FAILED",
  "FOSSA_TEAM_REUSED",
];

export default function AuditPage() {
  return (
    <AppShell>
      <AuditLogExplorer
        title="Audit Log"
        subtitle="Recent staff actions across projects and maintainers."
        actionOptions={actionOptions}
      />
    </AppShell>
  );
}
