"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Card } from "clo-ui/components/Card";
import ReactMarkdown from "react-markdown";
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import { atomDark } from "react-syntax-highlighter/dist/esm/styles/prism";
import rehypeRaw from "rehype-raw";
import rehypeSanitize, { defaultSchema } from "rehype-sanitize";
import remarkGfm from "remark-gfm";
import Link from "next/link";
import styles from "./ProjectReconciliationCard.module.css";
import ProjectDiffControl from "./ProjectDiffControl";
import ProjectAddMaintainerModal from "./ProjectAddMaintainerModal";
import DotProjectMaintainerFileViewer from "./DotProjectMaintainerFileViewer";

type MaintainerSummary = {
  id: number;
  name: string;
  github: string;
  inMaintainerRef: boolean;
  status?: string;
  company?: string;
};

type ServiceSummary = {
  id: number;
  name: string;
  description: string;
};

type FossaInviteSummary = {
  id: number;
  email: string;
  fossaTeamId: number;
  fossaTeamName: string;
  status: string;
  teamAssignmentStatus?: string | null;
  teamAddAttempts?: number | null;
  nextTeamAddAt?: string | null;
  lastError?: string | null;
  sentAt?: string | null;
  lastCheckedAt?: string | null;
};

type FossaTeamMemberSummary = {
  id: number;
  name: string;
  github: string;
  email: string;
};

type FossaInviteIneligibleSummary = {
  id: number;
  name: string;
  github: string;
  email: string;
  reason: string;
};

type FossaInviteCandidateSummary = {
  id: number;
  name: string;
  github: string;
  email: string;
};

type DotProjectSyncStateSummary = {
  repoExists: boolean;
  projectFileExists: boolean;
  maintainersFileExists: boolean;
  securityFileExists: boolean;
  contributingFileExists: boolean;
  governanceFileExists: boolean;
  defaultBranch?: string | null;
  maintainersFilename?: string | null;
  schemaVersion?: string | null;
  lastCheckedAt?: string | null;
  syncError?: string | null;
  parseError?: string | null;
};

type DotProjectMaintainerCacheSummary = {
  filename?: string | null;
  etag?: string | null;
  bodyHash?: string | null;
  body?: string | null;
  lastCheckedAt?: string | null;
};

type SortDirection = "asc" | "desc";

type SortState<Key extends string> = {
  key: Key;
  direction: SortDirection;
};

export type ProjectSectionId =
  | "legacy"
  | "dot-project"
  | "license-checker"
  | "mailing-maintainers"
  | "mailing-security"
  | "docs"
  | "slack"
  | "discord";

type ProjectSectionNavItem = {
  id: ProjectSectionId;
  label: string;
  href?: string;
  statusTone?: "success" | "danger";
  statusSymbol?: string;
};

export type AddMaintainerPayload = {
  name: string;
  githubHandle: string;
  email: string;
  company: string;
  companyMode: "select" | "new";
  refLine: string;
};

type ProjectReconciliationCardProps = {
  projectId: number;
  name: string;
  maturity: string;
  maintainerRef?: string | null;
  dotProjectRepoRef?: string | null;
  dotProjectProjectRef?: string | null;
  dotProjectMaintainerRef?: string | null;
  dotProjectSecurityRef?: string | null;
  dotProjectContributingRef?: string | null;
  dotProjectGovernanceRef?: string | null;
  dotProjectSchemaVersion?: string | null;
  dotProjectMaintainerCount?: number | null;
  dotProjectLastSyncedAt?: string | null;
  dotProjectAdoptionStatus?: string | null;
  dotProjectSyncState?: DotProjectSyncStateSummary | null;
  dotProjectMaintainerCache?: DotProjectMaintainerCacheSummary | null;
  maintainerRefStatus: {
    url?: string;
    status: string;
    checkedAt?: string | null;
  };
  maintainerRefBody?: string | null;
  refLines?: Record<string, string>;
  refOnlyGitHub: string[];
  companyOptions?: string[];
  onboardingIssue?: string | null;
  mailingList?: string | null;
  fossaTeamId?: number | null;
  fossaTeamName?: string | null;
  fossaTeamMembers?: FossaTeamMemberSummary[];
  fossaInviteIneligible?: FossaInviteIneligibleSummary[];
  fossaInviteCandidates?: FossaInviteCandidateSummary[];
  maintainers: MaintainerSummary[];
  services: ServiceSummary[];
  createdAt?: string | null;
  updatedAt?: string | null;
  updatedBy?: string | null;
  updatedAuditId?: number | null;
  onUpdateMaturity?: (next: string) => Promise<void>;
  onRefresh?: () => void;
  isRefreshing?: boolean;
  canEdit?: boolean;
  onAddMaintainer?: (payload: AddMaintainerPayload) => Promise<void>;
  onUpdateMaintainerRef?: (ref: string) => Promise<void>;
  onBulkStatusChange?: (ids: number[], status: string) => Promise<void>;
  activeSection?: ProjectSectionId;
  sectionNavItems?: ProjectSectionNavItem[];
  hideSectionMenu?: boolean;
};

const formatDate = (value?: string | null) => {
  if (!value) {
    return "—";
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return "—";
  }
  const weekdays = ["SUN", "MON", "TUE", "WED", "THUR", "FRI", "SAT"];
  const months = [
    "JAN",
    "FEB",
    "MAR",
    "APR",
    "MAY",
    "JUN",
    "JUL",
    "AUG",
    "SEP",
    "OCT",
    "NOV",
    "DEC",
  ];
  const weekday = weekdays[parsed.getDay()];
  const month = months[parsed.getMonth()];
  const day = String(parsed.getDate()).padStart(2, "0");
  const year = parsed.getFullYear();
  return `${weekday} ${month} ${day} ${year}`;
};

const formatDateTime = (value?: string | null) => {
  if (!value) {
    return "—";
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return "—";
  }
  const weekdays = ["SUN", "MON", "TUE", "WED", "THUR", "FRI", "SAT"];
  const months = [
    "JAN",
    "FEB",
    "MAR",
    "APR",
    "MAY",
    "JUN",
    "JUL",
    "AUG",
    "SEP",
    "OCT",
    "NOV",
    "DEC",
  ];
  const weekday = weekdays[parsed.getUTCDay()];
  const month = months[parsed.getUTCMonth()];
  const day = String(parsed.getUTCDate()).padStart(2, "0");
  const year = parsed.getUTCFullYear();
  const hours = String(parsed.getUTCHours()).padStart(2, "0");
  const minutes = String(parsed.getUTCMinutes()).padStart(2, "0");
  return `${weekday} ${month} ${day} ${year} ${hours}:${minutes} UTC`;
};

const maintainerRefSchema = {
  ...defaultSchema,
  tagNames: [
    ...(defaultSchema.tagNames || []),
    "table",
    "thead",
    "tbody",
    "tfoot",
    "tr",
    "th",
    "td",
    "img",
  ],
  attributes: {
    ...(defaultSchema.attributes || {}),
    a: [...(defaultSchema.attributes?.a || []), "target", "rel"],
    img: ["src", "alt", "title", "width", "height"],
    table: ["align"],
    th: ["align", "colspan", "rowspan"],
    td: ["align", "colspan", "rowspan"],
  },
};

const isYamlRef = (value: string) => /\.(ya?ml)(\?|#|$)/i.test(value);

const buildStatusBadgeClassName = (stylesMap: Record<string, string>, found: boolean) =>
  `${stylesMap.statusBadge} ${found ? stylesMap.statusOk : stylesMap.statusWarn}`;

export default function ProjectReconciliationCard({
  projectId,
  name,
  maturity,
  maintainerRef,
  dotProjectRepoRef,
  dotProjectProjectRef,
  dotProjectMaintainerRef,
  dotProjectSecurityRef,
  dotProjectContributingRef,
  dotProjectGovernanceRef,
  dotProjectSchemaVersion,
  dotProjectMaintainerCount,
  dotProjectLastSyncedAt,
  dotProjectAdoptionStatus,
  dotProjectSyncState,
  dotProjectMaintainerCache,
  maintainerRefStatus,
  maintainerRefBody,
  refLines,
  refOnlyGitHub,
  companyOptions = [],
  fossaTeamId,
  fossaTeamName,
  fossaTeamMembers = [],
  fossaInviteIneligible = [],
  fossaInviteCandidates = [],
  services,
  maintainers,
  createdAt,
  updatedAt,
  updatedBy,
  updatedAuditId,
  onUpdateMaturity,
  onRefresh,
  isRefreshing,
  canEdit = false,
  onAddMaintainer,
  onUpdateMaintainerRef,
  onBulkStatusChange,
  activeSection: controlledSection,
  sectionNavItems,
  hideSectionMenu = false,
}: ProjectReconciliationCardProps) {
  const refStatus = maintainerRefStatus?.status || "missing";
  const refCheckedAt = maintainerRefStatus?.checkedAt || null;
  const refUrl = maintainerRefStatus?.url || maintainerRef || "";
  const refBody = maintainerRefBody?.trim() ?? "";
  const hasDotProjectMaintainerFile = Boolean(dotProjectMaintainerRef);
  const hasDotProjectRepo = Boolean(dotProjectRepoRef);
  const dotProjectMissing = dotProjectAdoptionStatus === "not_found";
  const dotProjectPresent = !dotProjectMissing && hasDotProjectRepo;
  const dotProjectRepoExists = dotProjectSyncState?.repoExists ?? dotProjectPresent;
  const dotProjectLastCheckedAt = dotProjectSyncState?.lastCheckedAt || dotProjectLastSyncedAt || null;
  const dotProjectSchema = dotProjectSyncState?.schemaVersion || dotProjectSchemaVersion || "";
  const dotProjectMaintainersFilename = dotProjectSyncState?.maintainersFilename || "MAINTAINERS.yaml";
  const dotProjectMaintainerCacheBody = dotProjectMaintainerCache?.body ?? "";
  const dotProjectMaintainerCacheFilename =
    dotProjectMaintainerCache?.filename || dotProjectMaintainersFilename || "maintainers.yaml";
  const dotProjectFiles = [
    {
      label: ".project repo",
      present: dotProjectRepoExists,
      href: dotProjectRepoRef || "",
      detail: dotProjectSyncState?.defaultBranch ? `Default branch: ${dotProjectSyncState.defaultBranch}` : "Repository root",
    },
    {
      label: "project.yaml",
      present: dotProjectSyncState?.projectFileExists ?? Boolean(dotProjectProjectRef),
      href: dotProjectProjectRef || "",
      detail: dotProjectSchema ? `Schema ${dotProjectSchema}` : "Core project metadata",
    },
    {
      label: dotProjectMaintainersFilename,
      present: dotProjectSyncState?.maintainersFileExists ?? hasDotProjectMaintainerFile,
      href: dotProjectMaintainerRef || "",
      detail: dotProjectMaintainerCount != null ? `${dotProjectMaintainerCount} maintainers` : "Maintainer roster",
    },
    {
      label: "SECURITY.md",
      present: dotProjectSyncState?.securityFileExists ?? Boolean(dotProjectSecurityRef),
      href: dotProjectSecurityRef || "",
      detail: "Security policy",
    },
    {
      label: "CONTRIBUTING.md",
      present: dotProjectSyncState?.contributingFileExists ?? Boolean(dotProjectContributingRef),
      href: dotProjectContributingRef || "",
      detail: "Contribution guidelines",
    },
    {
      label: "GOVERNANCE.md",
      present: dotProjectSyncState?.governanceFileExists ?? Boolean(dotProjectGovernanceRef),
      href: dotProjectGovernanceRef || "",
      detail: "Governance document",
    },
  ];
  const refMatchCount = maintainers.filter((maintainer) => maintainer.inMaintainerRef).length;
  const refMissingCount = maintainers.length - refMatchCount;
  const refOnlyCount = refOnlyGitHub.length;
  const normalizedRefLines = useMemo(() => {
    const entries = Object.entries(refLines ?? {}).map(([key, value]) => [key.toLowerCase(), value]);
    return Object.fromEntries(entries) as Record<string, string>;
  }, [refLines]);

  const [selectedMaintainers, setSelectedMaintainers] = useState<Set<number>>(new Set());
  const toggleSelected = (id: number) => {
    setSelectedMaintainers((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  const clearSelection = () => setSelectedMaintainers(new Set());

  const groupedMaintainers = useMemo(() => {
    const order = ["active", "archived", "emeritus", "retired"];
    const labels: Record<string, string> = {
      active: "Active",
      archived: "Archived",
      emeritus: "Emeritus",
      retired: "Retired",
    };
    const groups: { key: string; label: string; items: MaintainerSummary[] }[] = order.map((k) => ({
      key: k,
      label: labels[k],
      items: [],
    }));
    const bucket: Record<string, MaintainerSummary[]> = {};
    for (const m of maintainers) {
      const key = (m.status || "").toLowerCase();
      bucket[key] = bucket[key] || [];
      bucket[key].push(m);
    }
    return groups
      .map((g) => ({
        ...g,
        items: bucket[g.key] || [],
      }))
      .filter((g) => g.items.length > 0);
  }, [maintainers]);

  const renderMaintainerGroups = () => (
    <div className={styles.groupStack}>
      {groupedMaintainers.map((group) => (
        <div key={group.key} className={styles.group}>
          <div className={styles.groupHeader}>{group.label}</div>
          {group.items.length > 1 ? (
            <label className={styles.selectAll}>
              <input
                type="checkbox"
                checked={group.items.every((m) => selectedMaintainers.has(m.id))}
                onChange={(e) => {
                  const allSelected = e.target.checked;
                  setSelectedMaintainers((prev) => {
                    const next = new Set(prev);
                    if (allSelected) {
                      group.items.forEach((m) => next.add(m.id));
                    } else {
                      group.items.forEach((m) => next.delete(m.id));
                    }
                    return next;
                  });
                }}
              />
              Select all
            </label>
          ) : null}
          <ul className={styles.list}>
            {group.items.map((maintainer) => {
              const checked = selectedMaintainers.has(maintainer.id);

              return (
                <li key={maintainer.id} className={styles.listItem}>
                  <div className={styles.listRow}>
                    <input
                      type="checkbox"
                      className={styles.checkbox}
                      checked={checked}
                      onChange={() => toggleSelected(maintainer.id)}
                    />
                    <Link className={styles.link} href={`/maintainers/${maintainer.id}`}>
                      {maintainer.name || maintainer.github || "Unknown maintainer"}
                    </Link>
                    {maintainer.github ? <span className={styles.secondary}>@{maintainer.github}</span> : null}
                    {maintainer.company ? <span className={styles.secondary}>{maintainer.company}</span> : null}
                    {refStatus === "fetched" ? (
                      maintainer.inMaintainerRef ? null : (
                        <span className={`${styles.statusBadge} ${styles.statusWarn}`}>NOT PRESENT</span>
                      )
                    ) : (
                      <span className={`${styles.statusBadge} ${styles.statusMuted}`}>Not checked</span>
                    )}
                  </div>
                </li>
              );
            })}
          </ul>
        </div>
      ))}
      <div className={styles.bulkActions}>
        <span className={styles.secondary}>
          {selectedMaintainers.size > 0 ? `${selectedMaintainers.size} selected` : "No maintainers selected"}
        </span>
        <div className={styles.bulkButtons}>
          <button
            type="button"
            className={styles.bulkButton}
            disabled={selectedMaintainers.size === 0}
            onClick={() => updateMaintainerStatuses("Active")}
          >
            Activate
          </button>
          <button
            type="button"
            className={styles.bulkButton}
            disabled={selectedMaintainers.size === 0}
            onClick={() => updateMaintainerStatuses("Emeritus")}
          >
            Emeritus
          </button>
          <button
            type="button"
            className={styles.bulkButton}
            disabled={selectedMaintainers.size === 0}
            onClick={() => updateMaintainerStatuses("Retired")}
          >
            Retire
          </button>
          <button
            type="button"
            className={`${styles.bulkButton} ${styles.bulkDanger}`}
            disabled={selectedMaintainers.size === 0}
            onClick={() => updateMaintainerStatuses("Archived")}
          >
            Archive
          </button>
          <button type="button" className={styles.bulkClear} onClick={clearSelection}>
            Clear
          </button>
        </div>
      </div>
    </div>
  );

  const updateMaintainerStatuses = async (status: string) => {
    if (!onBulkStatusChange) return;
    const ids = Array.from(selectedMaintainers);
    if (ids.length === 0) return;
    await onBulkStatusChange(ids, status);
    clearSelection();
  };
  const isRefBroken = Boolean(refUrl) && refStatus !== "fetched";
  const [modalOpen, setModalOpen] = useState(false);
  const [draft, setDraft] = useState<AddMaintainerPayload | null>(null);
  const [refInput, setRefInput] = useState("");
  const [refSaving, setRefSaving] = useState(false);
  const [refError, setRefError] = useState<string | null>(null);
  const [activeSection, setActiveSection] = useState<ProjectSectionId>("legacy");
  const [refEditing, setRefEditing] = useState(false);
  const [maturityModalOpen, setMaturityModalOpen] = useState(false);
  const [maturitySaving, setMaturitySaving] = useState(false);
  const [maturityError, setMaturityError] = useState<string | null>(null);
  const [fossaInvites, setFossaInvites] = useState<FossaInviteSummary[]>([]);
  const [fossaInviteStatus, setFossaInviteStatus] = useState<"idle" | "loading" | "ready">("idle");
  const [fossaInviteError, setFossaInviteError] = useState<string | null>(null);
  const [fossaInviteSending, setFossaInviteSending] = useState(false);
  const [fossaInviteRefreshing, setFossaInviteRefreshing] = useState(false);
  const [fossaTeamSyncing, setFossaTeamSyncing] = useState(false);
  const [fossaTeamSyncError, setFossaTeamSyncError] = useState<string | null>(null);
  const [fossaInviteSummary, setFossaInviteSummary] = useState<{
    invited: number;
    skipped: number;
    errors: number;
  } | null>(null);
  const [fossaTeamSort, setFossaTeamSort] = useState<SortState<"name" | "email" | "role">>({
    key: "name",
    direction: "asc",
  });
  const [inviteSort, setInviteSort] = useState<
    SortState<"name" | "email" | "pending" | "sent" | "checked" | "error">
  >({
    key: "name",
    direction: "asc",
  });
  const [fossaChooseStatus, setFossaChooseStatus] = useState<"idle" | "loading" | "done" | "error">("idle");
  const [fossaChooseError, setFossaChooseError] = useState<string | null>(null);
  const currentSection = controlledSection ?? activeSection;

  const toggleSort = useCallback(
    <Key extends string>(current: SortState<Key>, setter: (next: SortState<Key>) => void, key: Key) => {
      if (current.key === key) {
        setter({ key, direction: current.direction === "asc" ? "desc" : "asc" });
        return;
      }
      setter({ key, direction: "asc" });
    },
    []
  );
  const sortLabel = useCallback(<Key extends string>(label: string, state: SortState<Key>, key: Key) => {
    if (state.key !== key) {
      return label;
    }
    return `${label} ${state.direction === "asc" ? "↑" : "↓"}`;
  }, []);

  const hasSnykService = useMemo(() => {
    return services.some((service) => service.name.toLowerCase() === "snyk");
  }, [services]);
  const fossaInvitesByEmail = useMemo(() => {
    const map = new Map<string, FossaInviteSummary>();
    fossaInvites.forEach((invite) => {
      const normalized = invite.email.trim().toLowerCase();
      if (normalized) {
        map.set(normalized, invite);
      }
    });
    return map;
  }, [fossaInvites]);
  const inviteStatusLabel = useCallback((invite: FossaInviteSummary) => {
    if (invite.status === "accepted") {
      if (invite.teamAssignmentStatus === "pending") {
        return "accepted (team assignment pending)";
      }
      if (invite.teamAssignmentStatus === "error") {
        return "accepted (team assignment error)";
      }
    }
    return invite.status;
  }, []);
  const pendingInvites = useMemo(() => {
    return fossaInvites.filter((invite) => {
      if (invite.status === "pending") {
        return true;
      }
      if (invite.status === "accepted" && invite.teamAssignmentStatus && invite.teamAssignmentStatus !== "done") {
        return true;
      }
      return false;
    });
  }, [fossaInvites]);

  const onboardedMaintainerIds = useMemo(() => {
    return new Set(fossaTeamMembers.filter((member) => member.id && member.id > 0).map((member) => member.id));
  }, [fossaTeamMembers]);
  const onboardedMaintainerEmails = useMemo(() => {
    return new Set(
      fossaTeamMembers
        .map((member) => member.email.trim().toLowerCase())
        .filter((email) => email !== "")
    );
  }, [fossaTeamMembers]);

  const fossaTeamMembersSorted = useMemo(() => {
    const members = [...fossaTeamMembers];
    const direction = fossaTeamSort.direction === "asc" ? 1 : -1;
    members.sort((a, b) => {
      if (fossaTeamSort.key === "email") {
        return direction * (a.email || "").localeCompare(b.email || "");
      }
      if (fossaTeamSort.key === "role") {
        return direction * "Team Admin".localeCompare("Team Admin");
      }
      return direction * (a.name || a.github || "").localeCompare(b.name || b.github || "");
    });
    return members;
  }, [fossaTeamMembers, fossaTeamSort]);

  const eligibleInviteRows = useMemo(() => {
    const eligibleCandidates = fossaInviteCandidates.filter((maintainer) => {
      const email = maintainer.email.trim().toLowerCase();
      return !onboardedMaintainerIds.has(maintainer.id) && !onboardedMaintainerEmails.has(email);
    });
    const rows = eligibleCandidates.map((maintainer) => {
      const invite = fossaInvitesByEmail.get(maintainer.email.trim().toLowerCase());
      return {
        maintainer,
        invite,
        pending: invite?.status === "pending",
        sentAt: invite?.sentAt ?? null,
        checkedAt: invite?.lastCheckedAt ?? null,
        error: invite?.lastError ?? null,
      };
    });
    const direction = inviteSort.direction === "asc" ? 1 : -1;
    rows.sort((a, b) => {
      switch (inviteSort.key) {
        case "email":
          return direction * (a.maintainer.email || "").localeCompare(b.maintainer.email || "");
        case "pending":
          return direction * Number(a.pending) - direction * Number(b.pending);
        case "sent": {
          const aVal = a.sentAt ? new Date(a.sentAt).getTime() : 0;
          const bVal = b.sentAt ? new Date(b.sentAt).getTime() : 0;
          return direction * (aVal - bVal);
        }
        case "checked": {
          const aVal = a.checkedAt ? new Date(a.checkedAt).getTime() : 0;
          const bVal = b.checkedAt ? new Date(b.checkedAt).getTime() : 0;
          return direction * (aVal - bVal);
        }
        case "error":
          return direction * (a.error || "").localeCompare(b.error || "");
        default:
          return direction * (a.maintainer.name || a.maintainer.github || "").localeCompare(
            b.maintainer.name || b.maintainer.github || ""
          );
      }
    });
    return rows;
  }, [fossaInviteCandidates, fossaInvitesByEmail, inviteSort, onboardedMaintainerEmails, onboardedMaintainerIds]);

  const bffBaseUrl = useMemo(() => {
    const raw = process.env.NEXT_PUBLIC_BFF_BASE_URL || "/api";
    return raw.replace(/\/+$/, "");
  }, []);
  const apiBaseUrl = useMemo(() => {
    if (bffBaseUrl === "") {
      return "/api";
    }
    if (bffBaseUrl.endsWith("/api")) {
      return bffBaseUrl;
    }
    return `${bffBaseUrl}/api`;
  }, [bffBaseUrl]);
  useEffect(() => {
    if ((isRefBroken || refEditing) && refInput.trim() === "" && refUrl) {
      setRefInput(refUrl);
    }
  }, [isRefBroken, refEditing, refInput, refUrl]);

  const loadFossaInvites = useCallback(async () => {
    if (!projectId || !fossaTeamId) {
      return;
    }
    setFossaInviteStatus("loading");
    setFossaInviteError(null);
    try {
      const response = await fetch(
        `${apiBaseUrl}/services/fossa/invites?projectId=${projectId}`,
        { credentials: "include" }
      );
      if (!response.ok) {
        throw new Error("failed to load invites");
      }
      const data = (await response.json()) as FossaInviteSummary[];
      setFossaInvites(data);
    } catch {
      setFossaInviteError("Unable to load FOSSA invites.");
    } finally {
      setFossaInviteStatus("ready");
    }
  }, [apiBaseUrl, fossaTeamId, projectId]);

  useEffect(() => {
    if (currentSection !== "license-checker" || !projectId || !fossaTeamId) {
      return;
    }
    void loadFossaInvites();
  }, [currentSection, fossaTeamId, loadFossaInvites, projectId]);
  const sendFossaInvites = async () => {
    if (!projectId || !fossaTeamId || selectedMaintainers.size === 0) {
      return;
    }
    setFossaInviteSending(true);
    setFossaInviteError(null);
    try {
      const response = await fetch(`${apiBaseUrl}/services/fossa/invite`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ projectId, maintainerIds: Array.from(selectedMaintainers) }),
      });
      if (!response.ok) {
        throw new Error("failed to send invites");
      }
      const data = (await response.json()) as {
        invited: string[];
        skipped: string[];
        errors: Record<string, string>;
      };
      setFossaInviteSummary({
        invited: data.invited?.length || 0,
        skipped: data.skipped?.length || 0,
        errors: Object.keys(data.errors || {}).length,
      });
      await loadFossaInvites();
      onRefresh?.();
      clearSelection();
    } catch {
      setFossaInviteError("Unable to send FOSSA invites.");
    } finally {
      setFossaInviteSending(false);
    }
  };

  const deleteFossaInvite = async (inviteId: number) => {
    if (!fossaTeamId) {
      return;
    }
    setFossaInviteError(null);
    try {
      const response = await fetch(`${apiBaseUrl}/services/fossa/invites/${inviteId}/delete`, {
        method: "POST",
        credentials: "include",
      });
      if (!response.ok) {
        throw new Error("failed to delete invite");
      }
      await loadFossaInvites();
      onRefresh?.();
    } catch {
      setFossaInviteError("Unable to delete invite.");
    }
  };

  const reissueFossaInvite = async (inviteId: number) => {
    if (!fossaTeamId) {
      return;
    }
    setFossaInviteError(null);
    try {
      const response = await fetch(`${apiBaseUrl}/services/fossa/invites/${inviteId}/reissue`, {
        method: "POST",
        credentials: "include",
      });
      if (!response.ok) {
        throw new Error("failed to reissue invite");
      }
      await loadFossaInvites();
    } catch {
      setFossaInviteError("Unable to reissue invite.");
    }
  };

  const refreshFossaInvites = async () => {
    if (!projectId || !fossaTeamId) {
      return;
    }
    setFossaInviteRefreshing(true);
    setFossaInviteError(null);
    try {
      const response = await fetch(`${apiBaseUrl}/services/fossa/invites/refresh?projectId=${projectId}`, {
        method: "POST",
        credentials: "include",
      });
      if (!response.ok) {
        throw new Error("failed to refresh invites");
      }
      await loadFossaInvites();
      onRefresh?.();
    } catch {
      setFossaInviteError("Unable to refresh FOSSA invites.");
    } finally {
      setFossaInviteRefreshing(false);
    }
  };

  const syncFossaTeam = async () => {
    if (!projectId || !fossaTeamId) {
      return;
    }
    setFossaTeamSyncing(true);
    setFossaTeamSyncError(null);
    try {
      const response = await fetch(`${apiBaseUrl}/services/fossa/team/sync?projectId=${projectId}`, {
        method: "POST",
        credentials: "include",
      });
      if (!response.ok) {
        throw new Error("failed to sync team");
      }
      await loadFossaInvites();
      onRefresh?.();
    } catch {
      setFossaTeamSyncError("Unable to sync FOSSA team members.");
    } finally {
      setFossaTeamSyncing(false);
    }
  };

  const chooseFossa = async () => {
    if (!projectId) {
      return;
    }
    setFossaChooseStatus("loading");
    setFossaChooseError(null);
    try {
      const response = await fetch(`${apiBaseUrl}/services/fossa/choose`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ projectId }),
      });
      if (!response.ok) {
        const payload = await response.json().catch(() => null);
        if (payload && typeof payload.error === "string") {
          const errorAt = typeof payload.errorAt === "string" ? payload.errorAt : null;
          const errorTime = errorAt ? formatDateTime(errorAt) : "—";
          throw new Error(`${payload.error} (at ${errorTime})`);
        }
        throw new Error("failed to choose fossa");
      }
      setFossaChooseStatus("done");
      onRefresh?.();
    } catch (err) {
      setFossaChooseStatus("error");
      const message = err instanceof Error ? err.message : "Unable to start FOSSA onboarding.";
      setFossaChooseError(message);
    }
  };

  const dotProjectSection = (
    <div className={styles.section}>
      {dotProjectMaintainerCacheBody.trim() ? (
        <div className={styles.tableSection}>
          <div className={styles.tableHeader}>
            <h3 className={styles.tableTitle}>{dotProjectMaintainerCacheFilename}</h3>
            {dotProjectMaintainerRef ? (
              <a className={styles.link} href={dotProjectMaintainerRef} target="_blank" rel="noreferrer">
                Open source file
              </a>
            ) : null}
          </div>
          <DotProjectMaintainerFileViewer
            apiBaseUrl={apiBaseUrl}
            filename={dotProjectMaintainerCacheFilename}
            maintainers={maintainers}
            projectId={projectId}
            canEdit={canEdit}
            onAddMissingMaintainer={(handle, refLine) => {
              setDraft({
                githubHandle: handle,
                name: "",
                email: "",
                company: "",
                companyMode: "select",
                refLine,
              });
              setModalOpen(true);
            }}
            source={dotProjectMaintainerCacheBody}
          />
        </div>
      ) : null}
      <div className={styles.tableSection}>
        <div className={styles.tableHeader}>
          <h3 className={styles.tableTitle}>Discovery details</h3>
        </div>
        <div className={styles.dotProjectSummaryGrid}>
          <div className={styles.dotProjectSummaryCard}>
            <span className={styles.detailLabel}>Repo</span>
            <div className={styles.statusRow}>
              <span className={buildStatusBadgeClassName(styles, dotProjectRepoExists)}>
                {dotProjectRepoExists ? "FOUND" : "MISSING"}
              </span>
              <span className={styles.secondary}>{dotProjectSyncState?.defaultBranch || "Branch unknown"}</span>
            </div>
          </div>
          <div className={styles.dotProjectSummaryCard}>
            <span className={styles.detailLabel}>Schema version</span>
            <span className={styles.secondary}>{dotProjectSchema || "Unknown"}</span>
          </div>
          <div className={styles.dotProjectSummaryCard}>
            <span className={styles.detailLabel}>Maintainer count</span>
            <span className={styles.secondary}>
              {dotProjectMaintainerCount != null ? String(dotProjectMaintainerCount) : "Not parsed"}
            </span>
          </div>
          <div className={styles.dotProjectSummaryCard}>
            <span className={styles.detailLabel}>Last checked</span>
            <span className={styles.secondary}>{formatDateTime(dotProjectLastCheckedAt)}</span>
          </div>
        </div>
      </div>
      <div className={styles.tableSection}>
        <div className={styles.tableHeader}>
          <h3 className={styles.tableTitle}>Tracked files</h3>
        </div>
        <div className={styles.tableWrap}>
          <table className={styles.dataTable}>
            <thead>
              <tr>
                <th>Artifact</th>
                <th>Notes</th>
              </tr>
            </thead>
            <tbody>
              {dotProjectFiles.map((file) => (
                <tr key={file.label}>
                  <td>
                    {file.present && file.href ? (
                      <a className={styles.dotProjectArtifactLink} href={file.href} target="_blank" rel="noreferrer">
                        <span className={styles.dotProjectArtifactName}>{file.label}</span>
                        <span className={styles.dotProjectArtifactUrl}>{file.href}</span>
                        <span className={styles.externalLinkIcon} aria-hidden="true">
                          ↗
                        </span>
                        <span className={styles.srOnly}>Opens on GitHub</span>
                      </a>
                    ) : (
                      <span className={styles.dotProjectMissingArtifact}>
                        <span className={styles.dotProjectArtifactName}>{file.label}</span>
                        <span className={`${styles.statusBadge} ${styles.statusWarn}`}>NOT FOUND</span>
                      </span>
                    )}
                  </td>
                  <td>{file.detail}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
      {dotProjectSyncState?.syncError || dotProjectSyncState?.parseError ? (
        <div className={styles.statusCallout}>
          <div className={styles.statusCalloutTitle}>Recorded sync issues</div>
          {dotProjectSyncState?.syncError ? (
            <div className={styles.statusCalloutBody}>
              Sync error: <strong>{dotProjectSyncState.syncError}</strong>
            </div>
          ) : null}
          {dotProjectSyncState?.parseError ? (
            <div className={styles.statusCalloutBody}>
              Parse error: <strong>{dotProjectSyncState.parseError}</strong>
            </div>
          ) : null}
        </div>
      ) : null}
      {!dotProjectSyncState && !dotProjectAdoptionStatus ? (
        <div className={styles.statusCallout}>
          <div className={styles.statusCalloutTitle}>No persisted sync state yet</div>
          <div className={styles.statusCalloutBody}>
            Run the dot-project background sync job to populate this roll call from the CNCF <code>.project</code> repo.
          </div>
        </div>
        ) : null}
    </div>
  );

  const maturityOptions = ["Sandbox", "Incubating", "Graduated", "Archived"];
  const allowedTransitions = maturityOptions.filter((option) => option !== maturity);

  const legacyContent = (
    <div className={styles.legacyStack}>
      {hasDotProjectMaintainerFile ? (
        <div className={styles.statusCallout}>
          <div className={styles.statusCalloutTitle}>Dot-project maintainer file detected</div>
          <div className={styles.statusCalloutBody}>
            This project has a maintainer file in its <code>.project</code> repo. Use DOT-PROJECT ROLL CALL to track the
            migration from the legacy maintainer file.
          </div>
          <div className={styles.statusCalloutLinks}>
            <a className={styles.link} href={dotProjectMaintainerRef!} target="_blank" rel="noreferrer">
              Open maintainer file
            </a>
            {hasDotProjectRepo ? (
              <a className={styles.link} href={dotProjectRepoRef!} target="_blank" rel="noreferrer">
                Open .project repo
              </a>
            ) : null}
          </div>
        </div>
      ) : null}
      <div className={styles.legacyGrid}>
        <div className={styles.column}>
          <div className={styles.sectionHeader}>
            <h2 className={styles.sectionTitle}>PRESENT IN CNCF DATABASE</h2>
          </div>

          <div className={styles.section}>
            {maintainers.length === 0 ? (
              <div className={styles.empty}>No maintainers found.</div>
            ) : (
              renderMaintainerGroups()
            )}
          </div>
        </div>

        <div className={styles.column}>
          <div className={styles.sectionHeader}>
            <h2 className={styles.sectionTitle}>Legacy Maintainer File</h2>
            {canEdit && onUpdateMaintainerRef ? (
              <button
                className={styles.refEditButton}
                type="button"
                onClick={() => {
                  setRefEditing((value) => !value);
                  setRefError(null);
                  if (refUrl) {
                    setRefInput(refUrl);
                  }
                }}
              >
                {refEditing ? "Cancel" : "Edit Link"}
              </button>
            ) : null}
            {refUrl ? (
              <a className={styles.refLink} href={refUrl} target="_blank" rel="noreferrer">
                {refUrl}
              </a>
            ) : null}
          </div>

          <div className={`${styles.section} ${styles.ownersSection}`}>
            {canEdit && onUpdateMaintainerRef && (refEditing || !refUrl || isRefBroken) ? (
              <div className={styles.refMissing}>
                <div className={styles.refMissingText}>
                  {!refUrl
                    ? "No project admin file is registered for this project."
                    : isRefBroken
                    ? "The project admin file could not be loaded. Update the URL below."
                    : "Update Legacy Maintainer File."}
                </div>
                <div className={styles.refInputRow}>
                  <input
                    className={styles.refInput}
                    type="url"
                    placeholder="https://github.com/org/repo/blob/main/MAINTAINERS.md"
                    value={refInput}
                    onChange={(event) => {
                      setRefInput(event.target.value);
                      setRefError(null);
                    }}
                  />
                  <button
                    className={styles.refSaveButton}
                    type="button"
                    disabled={refSaving || refInput.trim() === ""}
                    onClick={async () => {
                      if (!onUpdateMaintainerRef) {
                        return;
                      }
                      const next = refInput.trim();
                      if (!next) {
                        setRefError("Enter a URL for the project admin file.");
                        return;
                      }
                      setRefSaving(true);
                      setRefError(null);
                      try {
                        await onUpdateMaintainerRef(next);
                        setRefEditing(false);
                        setRefInput("");
                        if (onRefresh) {
                          onRefresh();
                        }
                      } catch {
                        setRefError("Unable to update project admin file.");
                      } finally {
                        setRefSaving(false);
                      }
                    }}
                  >
                    {refSaving ? "Saving..." : "Save"}
                  </button>
                </div>
                {refError ? <div className={styles.refError}>{refError}</div> : null}
              </div>
            ) : null}
            {refBody ? (
              isYamlRef(refUrl) ? (
                <div className={styles.refYaml}>
                  <SyntaxHighlighter
                    language="yaml"
                    style={atomDark}
                    customStyle={{ margin: 0, background: "transparent" }}
                    codeTagProps={{ style: { fontFamily: "var(--font-geist-mono)" } }}
                  >
                    {refBody}
                  </SyntaxHighlighter>
                </div>
              ) : (
                <div className={styles.refMarkdown}>
                  <ReactMarkdown
                    remarkPlugins={[remarkGfm]}
                    rehypePlugins={[rehypeRaw, [rehypeSanitize, maintainerRefSchema]]}
                  >
                    {refBody}
                  </ReactMarkdown>
                </div>
              )
            ) : (
              <div className={styles.empty}>
                {refStatus === "fetched" ? "No maintainer ref contents available." : "Maintainer ref not available."}
              </div>
            )}
          </div>
        </div>

        <div className={styles.column}>
          <div className={styles.sectionHeader}>
            <h2 className={styles.sectionTitle}>NOT PRESENT ON CNCF DATABASE</h2>
          </div>
          <div className={styles.section}>
            {refOnlyGitHub.length === 0 ? (
              <div className={styles.empty}>None detected.</div>
            ) : (
              <ul className={styles.list}>
                {refOnlyGitHub.map((handle) => (
                  <li key={handle} className={styles.listItem}>
                    <div className={styles.listRow}>
                    {canEdit ? (
                      <button
                        className={styles.addButton}
                        type="button"
                          onClick={() => {
                            setDraft({
                              githubHandle: handle,
                              name: "",
                              email: "",
                              company: "",
                              companyMode: "select",
                              refLine: normalizedRefLines[handle.toLowerCase()] || "",
                            });
                            setModalOpen(true);
                        }}
                      >
                        ADD MAINTAINER
                      </button>
                    ) : null}
                    <a className={styles.link} href={`https://github.com/${handle}`} target="_blank" rel="noreferrer">
                      @{handle}
                    </a>
                  </div>
                </li>
              ))}
              </ul>
            )}
          </div>
        </div>
      </div>
    </div>
  );

  const defaultMenuItems: ProjectSectionNavItem[] = [
    {
      id: "legacy",
      label: "LEGACY ROLL CALL",
      statusTone: dotProjectMissing ? "success" : dotProjectPresent ? "danger" : undefined,
      statusSymbol: dotProjectMissing ? "✓" : dotProjectPresent ? "✕" : undefined,
    },
    {
      id: "dot-project",
      label: "DOT-PROJECT ROLL CALL",
      statusTone: dotProjectPresent ? "success" : dotProjectMissing ? "danger" : undefined,
      statusSymbol: dotProjectPresent ? "✓" : dotProjectMissing ? "✕" : undefined,
    },
    { id: "license-checker", label: "SERVICES / LICENSE CHECKER" },
    { id: "mailing-maintainers", label: "SERVICES / MAILING LISTS / MAINTAINERS" },
    { id: "mailing-security", label: "SERVICES / MAILING LISTS / SECURITY" },
    { id: "docs", label: "SERVICES / DOCUMENTATION" },
    { id: "slack", label: "SERVICES / COLLABORATION / SLACK" },
    { id: "discord", label: "SERVICES / COLLABORATION / DISCORD" },
  ];
  const menuItems = sectionNavItems ?? defaultMenuItems;

  const renderContent = () => {
    switch (currentSection) {
      case "legacy":
        return (
          <>
            {legacyContent}
            <ProjectDiffControl
              status={refStatus}
              checkedAt={refCheckedAt}
              matchCount={refMatchCount}
              missingCount={refMissingCount}
              refOnlyCount={refOnlyCount}
              onRefresh={onRefresh}
              isRefreshing={isRefreshing}
            />
          </>
        );
      case "dot-project":
        return dotProjectSection;
      case "license-checker":
        return (
          <div className={styles.section}>
            {fossaTeamId ? (
              <>
                <div className={styles.tableSection}>
                  <div className={styles.tableHeader}>
                    <h4 className={styles.tableTitle}>CNCF FOSSA TEAM</h4>
                    <a
                      className={styles.link}
                      href={`https://app.fossa.com/account/settings/organization/teams/${fossaTeamId}`}
                      target="_blank"
                      rel="noreferrer"
                    >
                      {fossaTeamName || `Team ${fossaTeamId}`}
                    </a>
                  </div>
                  {fossaTeamMembersSorted.length === 0 ? (
                    <div className={styles.empty}>
                      <div>No FOSSA team members recorded.</div>
                      {canEdit ? (
                        <button
                          type="button"
                          className={styles.refreshButton}
                          onClick={syncFossaTeam}
                          disabled={fossaTeamSyncing}
                        >
                          {fossaTeamSyncing ? "Syncing..." : "Sync from FOSSA"}
                        </button>
                      ) : null}
                      {fossaTeamSyncError ? <div className={styles.inviteError}>{fossaTeamSyncError}</div> : null}
                    </div>
                  ) : (
                    <div className={styles.tableWrap}>
                      <table className={styles.dataTable}>
                        <thead>
                          <tr>
                            <th>
                              <button
                                type="button"
                                className={styles.sortButton}
                                onClick={() => toggleSort(fossaTeamSort, setFossaTeamSort, "name")}
                              >
                                {sortLabel("Maintainer Name", fossaTeamSort, "name")}
                              </button>
                            </th>
                            <th>GitHub</th>
                            <th>
                              <button
                                type="button"
                                className={styles.sortButton}
                                onClick={() => toggleSort(fossaTeamSort, setFossaTeamSort, "email")}
                              >
                                {sortLabel("FOSSA Email", fossaTeamSort, "email")}
                              </button>
                            </th>
                            <th>
                              <button
                                type="button"
                                className={styles.sortButton}
                                onClick={() => toggleSort(fossaTeamSort, setFossaTeamSort, "role")}
                              >
                                {sortLabel("Role", fossaTeamSort, "role")}
                              </button>
                            </th>
                            <th>Maintainerd Match</th>
                          </tr>
                        </thead>
                        <tbody>
                          {fossaTeamMembersSorted.map((member, index) => {
                            const emailMissing = member.email === "EMAIL_MISSING" || member.email === "";
                            const rowKey =
                              member.id && member.id > 0
                                ? `maintainer-${member.id}`
                                : member.email || member.github || member.name || `member-${index}`;
                            const matched = member.id && member.id > 0;
                            return (
                              <tr key={rowKey}>
                                <td>
                                  {matched ? (
                                    <Link href={`/maintainers/${member.id}`}>
                                      {member.name || member.github || "Unknown maintainer"}
                                    </Link>
                                  ) : (
                                    member.name || member.github || "Unknown maintainer"
                                  )}
                                </td>
                                <td>
                                  {member.github ? (
                                    <a
                                      className={styles.link}
                                      href={`https://github.com/${member.github}`}
                                      target="_blank"
                                      rel="noreferrer"
                                    >
                                      {member.github}
                                    </a>
                                  ) : matched ? (
                                    "GitHub missing"
                                  ) : (
                                    "—"
                                  )}
                                </td>
                                <td>{emailMissing ? "Email missing" : member.email}</td>
                                <td>Team Admin</td>
                                <td>{matched ? "Matched" : "Unmatched"}</td>
                              </tr>
                            );
                          })}
                        </tbody>
                      </table>
                    </div>
                  )}
                </div>
                {eligibleInviteRows.length > 0 ? (
                  <div className={styles.tableSection}>
                    <div className={styles.tableHeader}>
                      <h4 className={styles.tableTitle}>ACTIVE MAINTAINERS ELIGABLE FOR INVITATION</h4>
                    </div>
                    <div className={styles.tableWrap}>
                      <table className={styles.dataTable}>
                        <thead>
                          <tr>
                            <th>
                              <button
                                type="button"
                                className={styles.sortButton}
                                onClick={() => toggleSort(inviteSort, setInviteSort, "name")}
                              >
                                {sortLabel("Maintainer Name", inviteSort, "name")}
                              </button>
                            </th>
                            <th>
                              <button
                                type="button"
                                className={styles.sortButton}
                                onClick={() => toggleSort(inviteSort, setInviteSort, "email")}
                              >
                                {sortLabel("CNCF Registered Email", inviteSort, "email")}
                              </button>
                            </th>
                            <th>
                              <button
                                type="button"
                                className={styles.sortButton}
                                onClick={() => toggleSort(inviteSort, setInviteSort, "pending")}
                              >
                                {sortLabel("Invitation Pending", inviteSort, "pending")}
                              </button>
                            </th>
                            <th>
                              <button
                                type="button"
                                className={styles.sortButton}
                                onClick={() => toggleSort(inviteSort, setInviteSort, "sent")}
                              >
                                {sortLabel("Invite sent on", inviteSort, "sent")}
                              </button>
                            </th>
                            <th>
                              <button
                                type="button"
                                className={styles.sortButton}
                                onClick={() => toggleSort(inviteSort, setInviteSort, "checked")}
                              >
                                {sortLabel("Last Checked on", inviteSort, "checked")}
                              </button>
                            </th>
                            <th>
                              <button
                                type="button"
                                className={styles.sortButton}
                                onClick={() => toggleSort(inviteSort, setInviteSort, "error")}
                              >
                                {sortLabel("Invite Error", inviteSort, "error")}
                              </button>
                            </th>
                          </tr>
                        </thead>
                        <tbody>
                          {eligibleInviteRows.map(({ maintainer, invite, pending, sentAt, checkedAt, error }) => (
                            <tr key={maintainer.id}>
                              <td>
                                <label className={styles.tableSelect}>
                                  <input
                                    type="checkbox"
                                    checked={selectedMaintainers.has(maintainer.id)}
                                    onChange={() => toggleSelected(maintainer.id)}
                                  />
                                  <span>
                                    <Link href={`/maintainers/${maintainer.id}`}>
                                      {maintainer.name || maintainer.github || "Unknown maintainer"}
                                    </Link>
                                  </span>
                                </label>
                              </td>
                              <td>{maintainer.email || "Email missing"}</td>
                              <td>{pending ? "Yes" : "No"}</td>
                              <td>{formatDateTime(sentAt)}</td>
                              <td>{formatDateTime(checkedAt)}</td>
                              <td>
                                {error || (invite?.status === "error" ? "Unknown error" : "—")}
                                {invite && (invite.status === "expired" || invite.status === "error") ? (
                                  <button
                                    type="button"
                                    className={styles.inviteAction}
                                    onClick={() => reissueFossaInvite(invite.id)}
                                  >
                                    Re-issue
                                  </button>
                                ) : null}
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                    {canEdit ? (
                      <div className={styles.inviteActions}>
                        <button
                          className={styles.addButton}
                          type="button"
                          onClick={sendFossaInvites}
                          disabled={fossaInviteSending || selectedMaintainers.size === 0}
                        >
                          {fossaInviteSending
                            ? "Sending invites..."
                            : `Send CNCF FOSSA Invites to ${selectedMaintainers.size} Selected Maintainers`}
                        </button>
                        {fossaInviteSummary ? (
                          <div className={styles.inviteSummary}>
                            Invited {fossaInviteSummary.invited}, skipped {fossaInviteSummary.skipped}, errors{" "}
                            {fossaInviteSummary.errors}
                          </div>
                        ) : null}
                      </div>
                    ) : null}
                    {fossaInviteError ? <div className={styles.inviteError}>{fossaInviteError}</div> : null}
                  </div>
                ) : null}
                <div className={styles.tableSection}>
                  <div className={styles.tableHeader}>
                    <h4 className={styles.tableTitle}>PENDING INVITATIONS</h4>
                    {canEdit ? (
                      <button
                        type="button"
                        className={styles.refreshButton}
                        onClick={refreshFossaInvites}
                        disabled={fossaInviteRefreshing}
                        aria-label="Refresh pending FOSSA invites"
                        title="Refresh from FOSSA"
                      >
                        <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
                          <path
                            d="M4 12a8 8 0 0 1 13.66-5.66l1.41-1.41v5.66h-5.66l1.93-1.93A6 6 0 1 0 18 12h2a8 8 0 0 1-16 0z"
                            fill="currentColor"
                          />
                        </svg>
                        {fossaInviteRefreshing ? "Refreshing..." : "Refresh"}
                      </button>
                    ) : null}
                  </div>
                  <div className={styles.tableWrap}>
                    <table className={styles.dataTable}>
                      <thead>
                        <tr>
                          <th>Maintainer Email</th>
                          <th>Team</th>
                          <th>Status</th>
                          <th>Invite sent on</th>
                          <th>Estimated time of expiry</th>
                          <th>Last Checked</th>
                          <th>Error</th>
                        </tr>
                      </thead>
                      <tbody>
                        {fossaInviteStatus === "loading" ? (
                          <tr>
                            <td colSpan={7} className={styles.tableEmpty}>
                              Loading invites…
                            </td>
                          </tr>
                        ) : pendingInvites.length === 0 ? (
                          <tr>
                            <td colSpan={7} className={styles.tableEmpty}>
                              No pending FOSSA invites.
                            </td>
                          </tr>
                        ) : (
                          pendingInvites.map((invite) => {
                            const sentAt = invite.sentAt ?? null;
                            const expiry =
                              sentAt && !Number.isNaN(new Date(sentAt).getTime())
                                ? new Date(new Date(sentAt).getTime() + 72 * 60 * 60 * 1000).toISOString()
                                : null;
                            return (
                              <tr key={invite.id}>
                                <td>{invite.email}</td>
                                <td>{invite.fossaTeamName || "FOSSA"}</td>
                                <td>{inviteStatusLabel(invite)}</td>
                                <td>{formatDateTime(sentAt)}</td>
                                <td>{formatDateTime(expiry)}</td>
                                <td>{formatDateTime(invite.lastCheckedAt || null)}</td>
                                <td>
                                  {invite.lastError || "—"}
                                  <button
                                    type="button"
                                    className={styles.inviteAction}
                                    onClick={() => deleteFossaInvite(invite.id)}
                                  >
                                    Remove
                                  </button>
                                </td>
                              </tr>
                            );
                          })
                        )}
                      </tbody>
                    </table>
                  </div>
                </div>
                <div className={styles.tableSection}>
                  <div className={styles.tableHeader}>
                    <h4 className={styles.tableTitle}>REPOS IMPORTED</h4>
                  </div>
                  <div className={styles.tableWrap}>
                    <table className={styles.dataTable}>
                      <thead>
                        <tr>
                          <th>Repo URL</th>
                          <th>Time last scanned</th>
                          <th>Number of reported issues</th>
                        </tr>
                      </thead>
                      <tbody>
                        {/* TODO: wire up imported repos from the backend */}
                        <tr>
                          <td colSpan={3} className={styles.tableEmpty}>
                            No repository data available yet.
                          </td>
                        </tr>
                      </tbody>
                    </table>
                  </div>
                </div>
                {fossaInviteIneligible.length > 0 ? (
                  <div className={styles.tableSection}>
                    <div className={styles.tableHeader}>
                      <h4 className={styles.tableTitle}>NOT ELIGIBLE FOR INVITES</h4>
                    </div>
                    <div className={styles.tableWrap}>
                      <table className={styles.dataTable}>
                        <thead>
                          <tr>
                            <th>Maintainer Name</th>
                            <th>GitHub</th>
                            <th>CNCF Registered Email</th>
                            <th>Reason</th>
                          </tr>
                        </thead>
                        <tbody>
                          {fossaInviteIneligible.map((maintainer) => (
                            <tr key={maintainer.id}>
                              <td>
                                <Link href={`/maintainers/${maintainer.id}`}>
                                  {maintainer.name || maintainer.github || "Unknown maintainer"}
                                </Link>
                              </td>
                              <td>{maintainer.github || "GitHub missing"}</td>
                              <td>{maintainer.email || "Email missing"}</td>
                              <td>{maintainer.reason}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </div>
                ) : null}
              </>
            ) : (
              <div className={styles.stub}>
                {hasSnykService ? (
                  <>This project has not selected FOSSA. It may be using Snyk for license checks.</>
                ) : (
                  <>This project has not selected a license checker.</>
                )}
              </div>
            )}
            {!fossaTeamId && canEdit && !hasSnykService ? (
              <div className={styles.inviteActions}>
                <button
                  className={styles.addButton}
                  type="button"
                  onClick={chooseFossa}
                  disabled={fossaChooseStatus === "loading"}
                >
                  {fossaChooseStatus === "loading" ? "Starting FOSSA..." : "Choose FOSSA"}
                </button>
                {fossaChooseStatus === "done" ? (
                  <div className={styles.inviteSummary}>FOSSA onboarding started.</div>
                ) : null}
                {fossaChooseError ? <div className={styles.inviteError}>{fossaChooseError}</div> : null}
              </div>
            ) : null}
          </div>
        );
      case "mailing-maintainers":
        return (
          <div className={styles.section}>
            <h3 className={styles.subSectionTitle}>Mailing Lists · Maintainers</h3>
            <p className={styles.stub}>Placeholder for maintainer mailing list references (Groups.io / Google Groups).</p>
          </div>
        );
      case "mailing-security":
        return (
          <div className={styles.section}>
            <h3 className={styles.subSectionTitle}>Mailing Lists · Security</h3>
            <p className={styles.stub}>Placeholder for security mailing list references.</p>
          </div>
        );
      case "docs":
        return (
          <div className={styles.section}>
            <h3 className={styles.subSectionTitle}>Documentation</h3>
            <p className={styles.stub}>Placeholder for documentation hosting details.</p>
          </div>
        );
      case "slack":
        return (
          <div className={styles.section}>
            <h3 className={styles.subSectionTitle}>Collaboration · Slack</h3>
            <p className={styles.stub}>Placeholder for Slack workspace/channel references.</p>
          </div>
        );
      case "discord":
        return (
          <div className={styles.section}>
            <h3 className={styles.subSectionTitle}>Collaboration · Discord</h3>
            <p className={styles.stub}>Placeholder for Discord server/channel references.</p>
          </div>
        );
      default:
        return null;
    }
  };

  return (
    <Card hoverable={false} className={styles.card}>
      <div className={styles.content}>
        <div className={styles.topRow}>
          <div className={styles.header}>
            <div>
              <h1 className={styles.name}>{name || "Unknown project"}</h1>
              <p className={styles.subTitle}>{maturity || "—"}</p>
            </div>
            <div className={styles.meta}>
              <span className={styles.metaItem}>Imported from google worksheet on {formatDate(createdAt)}</span>
              <span className={styles.metaItem}>Last edited {formatDate(updatedAt)}</span>
              {updatedBy ? (
                <span className={styles.metaItem}>
                  Updated by{" "}
                  {updatedAuditId ? (
                    <Link className={styles.metaLink} href={`/audit?entry=${updatedAuditId}`}>
                      {updatedBy}
                    </Link>
                  ) : (
                    updatedBy
                  )}
                </span>
              ) : null}
            </div>
            {canEdit && onUpdateMaturity && allowedTransitions.length > 0 ? (
              <button
                className={styles.transitionButton}
                type="button"
                onClick={() => {
                  setMaturityModalOpen(true);
                  setMaturityError(null);
                }}
              >
                MOVE LEVEL
              </button>
            ) : null}
          </div>

        </div>

        <div className={styles.bottomRow}>
          {!hideSectionMenu ? (
            <div className={styles.menuColumn}>
              <div className={styles.projectMenu}>
                {menuItems.map((item) =>
                  item.href ? (
                    <Link
                      key={item.id}
                      href={item.href}
                      className={`${styles.menuItem} ${currentSection === item.id ? styles.menuItemActive : ""}`}
                    >
                      <span className={styles.menuItemInner}>
                        <span>{item.label}</span>
                        {item.statusSymbol ? (
                          <span
                            aria-hidden="true"
                            className={`${styles.menuStatus} ${
                              item.statusTone === "success" ? styles.menuStatusSuccess : styles.menuStatusDanger
                            }`}
                          >
                            {item.statusSymbol}
                          </span>
                        ) : null}
                      </span>
                    </Link>
                  ) : (
                    <button
                      key={item.id}
                      type="button"
                      className={`${styles.menuItem} ${currentSection === item.id ? styles.menuItemActive : ""}`}
                      onClick={() => setActiveSection(item.id)}
                    >
                      <span className={styles.menuItemInner}>
                        <span>{item.label}</span>
                        {item.statusSymbol ? (
                          <span
                            aria-hidden="true"
                            className={`${styles.menuStatus} ${
                              item.statusTone === "success" ? styles.menuStatusSuccess : styles.menuStatusDanger
                            }`}
                          >
                            {item.statusSymbol}
                          </span>
                        ) : null}
                      </span>
                    </button>
                  )
                )}
              </div>
            </div>
          ) : null}
          <div className={styles.contentColumn}>
            <div className={styles.nestedCard}>
              <div className={styles.collapsibleHeader}>
                <h2 className={styles.sectionTitle}>
                  <span className={styles.menuItemInner}>
                    <span>{menuItems.find((m) => m.id === currentSection)?.label}</span>
                    {menuItems.find((m) => m.id === currentSection)?.statusSymbol ? (
                      <span
                        aria-hidden="true"
                        className={`${styles.menuStatus} ${
                          menuItems.find((m) => m.id === currentSection)?.statusTone === "success"
                            ? styles.menuStatusSuccess
                            : styles.menuStatusDanger
                        }`}
                      >
                        {menuItems.find((m) => m.id === currentSection)?.statusSymbol}
                      </span>
                    ) : null}
                  </span>
                </h2>
              </div>
              {renderContent()}
            </div>
          </div>
        </div>

        {modalOpen && draft ? (
          <ProjectAddMaintainerModal
            draft={draft}
            onClose={() => setModalOpen(false)}
            onChange={(next) => setDraft(next)}
            companyOptions={companyOptions}
            onSubmit={async () => {
              if (!onAddMaintainer || !draft) {
                return;
              }
              await onAddMaintainer(draft);
              setModalOpen(false);
            }}
          />
        ) : null}
        {maturityModalOpen ? (
          <div className={styles.modalOverlay} role="dialog" aria-modal="true">
            <div className={styles.modal}>
              <div className={styles.modalHeader}>
                <h2 className={styles.modalTitle}>Move Project to new Level</h2>
                <button
                  className={styles.modalClose}
                  type="button"
                  onClick={() => setMaturityModalOpen(false)}
                >
                  Close
                </button>
              </div>
              <div className={styles.modalBody}>
                <div className={styles.modalRow}>
                  <span className={styles.modalLabel}>Current</span>
                  <span className={styles.modalValue}>{maturity || "—"}</span>
                </div>
                <div className={styles.modalRow}>
                  <span className={styles.modalLabel}>Next state</span>
                  <div className={styles.transitionOptions}>
                    {allowedTransitions.map((next) => (
                      <button
                        key={next}
                        className={styles.transitionOption}
                        type="button"
                        disabled={maturitySaving}
                        onClick={async () => {
                          if (!onUpdateMaturity) {
                            return;
                          }
                          setMaturitySaving(true);
                          setMaturityError(null);
                          try {
                            await onUpdateMaturity(next);
                            setMaturityModalOpen(false);
                          } catch {
                            setMaturityError("Unable to update project status.");
                          } finally {
                            setMaturitySaving(false);
                          }
                        }}
                      >
                        {next}
                      </button>
                    ))}
                  </div>
                </div>
                {maturityError ? <div className={styles.modalError}>{maturityError}</div> : null}
              </div>
            </div>
          </div>
        ) : null}
      </div>
    </Card>
  );
}
