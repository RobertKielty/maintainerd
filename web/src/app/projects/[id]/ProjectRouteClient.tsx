"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useParams, useRouter, useSelectedLayoutSegment } from "next/navigation";
import AppShell from "@/components/AppShell";
import ProjectReconciliationCard, {
  AddMaintainerPayload,
  ProjectSectionId,
} from "@/components/ProjectReconciliationCard";
import { getAuthBaseUrl, redirectToAuthLogin } from "@/utils/auth";
import styles from "./page.module.css";

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

type DotProjectSyncState = {
  repoExists: boolean;
  projectFileExists: boolean;
  maintainersFileExists: boolean;
  securityFileExists: boolean;
  contributingFileExists: boolean;
  governanceFileExists: boolean;
  defaultBranch?: string;
  maintainersFilename?: string;
  schemaVersion?: string;
  lastCheckedAt?: string | null;
  syncError?: string;
  parseError?: string;
};

type DotProjectMaintainerCache = {
  filename?: string;
  etag?: string;
  bodyHash?: string;
  body?: string;
  lastCheckedAt?: string | null;
};

type DotProjectMaintainerPullRequest = {
  status: string;
  url?: string;
  number?: number;
  title?: string;
  author?: string;
  source?: string;
  createdAt?: string | null;
  updatedAt?: string | null;
  warning?: string;
};

type ProjectDetail = {
  id: number;
  name: string;
  maturity: string;
  parentProjectId?: number | null;
  legacyMaintainerRef?: string;
  dotProjectRepoRef?: string;
  dotProjectProjectRef?: string;
  dotProjectMaintainerRef?: string;
  dotProjectSecurityRef?: string;
  dotProjectContributingRef?: string;
  dotProjectGovernanceRef?: string;
  dotProjectSchemaVersion?: string;
  dotProjectMaintainerCount?: number | null;
  dotProjectLastSyncedAt?: string | null;
  dotProjectAdoptionStatus?: string;
  dotProjectSyncState?: DotProjectSyncState | null;
  dotProjectMaintainerCache?: DotProjectMaintainerCache | null;
  dotProjectMaintainerPullRequest?: DotProjectMaintainerPullRequest | null;
  dotProjectGeneratedMaintainersYaml?: string;
  maintainerRefStatus: {
    url?: string;
    status: string;
    checkedAt?: string | null;
  };
  legacyMaintainerRefBody?: string;
  refOnlyGitHub: string[];
  refLines?: Record<string, string>;
  onboardingIssue?: string;
  mailingList?: string;
  maintainers: MaintainerSummary[];
  services: ServiceSummary[];
  fossaTeamId?: number | null;
  fossaTeamName?: string | null;
  fossaTeamMembers?: {
    id: number;
    name: string;
    github: string;
    email: string;
  }[];
  fossaInviteIneligible?: {
    id: number;
    name: string;
    github: string;
    email: string;
    reason: string;
  }[];
  fossaInviteCandidates?: {
    id: number;
    name: string;
    github: string;
    email: string;
  }[];
  createdAt: string;
  updatedAt: string;
  deletedAt?: string | null;
  updatedBy?: string | null;
  updatedAuditId?: number | null;
};

type ProjectRouteClientProps = {
  children?: React.ReactNode;
};

const projectSectionRoutes: Array<{
  id: ProjectSectionId;
  label: string;
  segment: string;
}> = [
  { id: "legacy", label: "LEGACY ROLL CALL", segment: "github" },
  { id: "dot-project", label: "DOT-PROJECT ROLL CALL", segment: "dot-project" },
  { id: "license-checker", label: "LICENSE CHECKER - FOSSA", segment: "fossa" },
  {
    id: "mailing-maintainers",
    label: "MAILING LISTS / MAINTAINERS",
    segment: "mailing-maintainers",
  },
  {
    id: "mailing-security",
    label: "MAILING LISTS / SECURITY",
    segment: "mailing-security",
  },
  { id: "docs", label: "DOCUMENTATION", segment: "docs" },
  { id: "slack", label: "COLLABORATION / SLACK", segment: "slack" },
  { id: "discord", label: "COLLABORATION / DISCORD", segment: "discord" },
];

const projectDataHasChanged = (current: ProjectDetail | null, next: ProjectDetail): boolean => {
  if (!current) {
    return true;
  }
  if (
    current.id !== next.id ||
    current.name !== next.name ||
    current.maturity !== next.maturity ||
    current.parentProjectId !== next.parentProjectId ||
    current.legacyMaintainerRef !== next.legacyMaintainerRef ||
    current.dotProjectRepoRef !== next.dotProjectRepoRef ||
    current.dotProjectProjectRef !== next.dotProjectProjectRef ||
    current.dotProjectMaintainerRef !== next.dotProjectMaintainerRef ||
    current.dotProjectSecurityRef !== next.dotProjectSecurityRef ||
    current.dotProjectContributingRef !== next.dotProjectContributingRef ||
    current.dotProjectGovernanceRef !== next.dotProjectGovernanceRef ||
    current.dotProjectSchemaVersion !== next.dotProjectSchemaVersion ||
    current.dotProjectMaintainerCount !== next.dotProjectMaintainerCount ||
    current.dotProjectLastSyncedAt !== next.dotProjectLastSyncedAt ||
    current.dotProjectAdoptionStatus !== next.dotProjectAdoptionStatus ||
    current.dotProjectSyncState?.repoExists !== next.dotProjectSyncState?.repoExists ||
    current.dotProjectSyncState?.projectFileExists !== next.dotProjectSyncState?.projectFileExists ||
    current.dotProjectSyncState?.maintainersFileExists !== next.dotProjectSyncState?.maintainersFileExists ||
    current.dotProjectSyncState?.securityFileExists !== next.dotProjectSyncState?.securityFileExists ||
    current.dotProjectSyncState?.contributingFileExists !== next.dotProjectSyncState?.contributingFileExists ||
    current.dotProjectSyncState?.governanceFileExists !== next.dotProjectSyncState?.governanceFileExists ||
    current.dotProjectSyncState?.defaultBranch !== next.dotProjectSyncState?.defaultBranch ||
    current.dotProjectSyncState?.maintainersFilename !== next.dotProjectSyncState?.maintainersFilename ||
    current.dotProjectSyncState?.schemaVersion !== next.dotProjectSyncState?.schemaVersion ||
    current.dotProjectSyncState?.lastCheckedAt !== next.dotProjectSyncState?.lastCheckedAt ||
    current.dotProjectSyncState?.syncError !== next.dotProjectSyncState?.syncError ||
    current.dotProjectSyncState?.parseError !== next.dotProjectSyncState?.parseError ||
    current.dotProjectMaintainerCache?.filename !== next.dotProjectMaintainerCache?.filename ||
    current.dotProjectMaintainerCache?.etag !== next.dotProjectMaintainerCache?.etag ||
    current.dotProjectMaintainerCache?.bodyHash !== next.dotProjectMaintainerCache?.bodyHash ||
    current.dotProjectMaintainerCache?.body !== next.dotProjectMaintainerCache?.body ||
    current.dotProjectMaintainerCache?.lastCheckedAt !== next.dotProjectMaintainerCache?.lastCheckedAt ||
    current.dotProjectMaintainerPullRequest?.status !== next.dotProjectMaintainerPullRequest?.status ||
    current.dotProjectMaintainerPullRequest?.url !== next.dotProjectMaintainerPullRequest?.url ||
    current.dotProjectMaintainerPullRequest?.number !== next.dotProjectMaintainerPullRequest?.number ||
    current.dotProjectMaintainerPullRequest?.updatedAt !== next.dotProjectMaintainerPullRequest?.updatedAt ||
    current.dotProjectMaintainerPullRequest?.warning !== next.dotProjectMaintainerPullRequest?.warning ||
    current.dotProjectGeneratedMaintainersYaml !== next.dotProjectGeneratedMaintainersYaml ||
    current.maintainerRefStatus.status !== next.maintainerRefStatus.status ||
    current.maintainerRefStatus.url !== next.maintainerRefStatus.url ||
    current.maintainerRefStatus.checkedAt !== next.maintainerRefStatus.checkedAt ||
    current.legacyMaintainerRefBody !== next.legacyMaintainerRefBody ||
    current.refOnlyGitHub.length !== next.refOnlyGitHub.length ||
    current.onboardingIssue !== next.onboardingIssue ||
    current.mailingList !== next.mailingList ||
    current.fossaTeamId !== next.fossaTeamId ||
    current.fossaTeamName !== next.fossaTeamName ||
    (current.fossaTeamMembers?.length || 0) !== (next.fossaTeamMembers?.length || 0) ||
    (current.fossaInviteIneligible?.length || 0) !== (next.fossaInviteIneligible?.length || 0) ||
    (current.fossaInviteCandidates?.length || 0) !== (next.fossaInviteCandidates?.length || 0) ||
    current.createdAt !== next.createdAt ||
    current.updatedAt !== next.updatedAt ||
    current.deletedAt !== next.deletedAt ||
    current.updatedBy !== next.updatedBy ||
    current.updatedAuditId !== next.updatedAuditId
  ) {
    return true;
  }
  if (current.maintainers.length !== next.maintainers.length) {
    return true;
  }
  for (let index = 0; index < current.maintainers.length; index += 1) {
    const currentMaintainer = current.maintainers[index];
    const nextMaintainer = next.maintainers[index];
    if (
      currentMaintainer.id !== nextMaintainer.id ||
      currentMaintainer.name !== nextMaintainer.name ||
      currentMaintainer.github !== nextMaintainer.github ||
      currentMaintainer.inMaintainerRef !== nextMaintainer.inMaintainerRef ||
      currentMaintainer.status !== nextMaintainer.status ||
      currentMaintainer.company !== nextMaintainer.company
    ) {
      return true;
    }
  }
  for (let index = 0; index < current.refOnlyGitHub.length; index += 1) {
    if (current.refOnlyGitHub[index] !== next.refOnlyGitHub[index]) {
      return true;
    }
  }
  const currentRefLines = current.refLines || {};
  const nextRefLines = next.refLines || {};
  const currentRefLineKeys = Object.keys(currentRefLines);
  const nextRefLineKeys = Object.keys(nextRefLines);
  if (currentRefLineKeys.length !== nextRefLineKeys.length) {
    return true;
  }
  for (const key of currentRefLineKeys) {
    if (currentRefLines[key] !== nextRefLines[key]) {
      return true;
    }
  }
  if (current.services.length !== next.services.length) {
    return true;
  }
  for (let index = 0; index < current.services.length; index += 1) {
    const currentService = current.services[index];
    const nextService = next.services[index];
    if (
      currentService.id !== nextService.id ||
      currentService.name !== nextService.name ||
      currentService.description !== nextService.description
    ) {
      return true;
    }
  }
  return false;
};

export default function ProjectRouteClient({ children }: ProjectRouteClientProps) {
  const segment = useSelectedLayoutSegment();
  const [project, setProject] = useState<ProjectDetail | null>(null);
  const [status, setStatus] = useState<"idle" | "loading" | "ready">("idle");
  const [error, setError] = useState<string | null>(null);
  const [role, setRole] = useState<string | null>(null);
  const [companies, setCompanies] = useState<string[]>([]);
  const projectRef = useRef<ProjectDetail | null>(null);
  const router = useRouter();
  const params = useParams<{ id: string }>();
  const projectId = params?.id;

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
  const authBaseUrl = useMemo(() => getAuthBaseUrl(bffBaseUrl), [bffBaseUrl]);
  const section = useMemo<ProjectSectionId>(() => {
    const matchedRoute = projectSectionRoutes.find((item) => item.segment === segment);
    return matchedRoute?.id ?? "legacy";
  }, [segment]);
  const currentProjectPath = useMemo(() => {
    if (!projectId) {
      return "/projects";
    }
    return segment ? `/projects/${projectId}/${segment}` : `/projects/${projectId}`;
  }, [projectId, segment]);

  useEffect(() => {
    let alive = true;
    if (!projectId) {
      return () => {
        alive = false;
      };
    }

    const loadProject = async () => {
      if (projectRef.current === null) {
        setStatus("loading");
        setError(null);
      }
      try {
        const response = await fetch(`${apiBaseUrl}/projects/${projectId}`, {
          credentials: "include",
        });
        if (!response.ok) {
          if (response.status === 401) {
            redirectToAuthLogin(authBaseUrl, currentProjectPath);
            return;
          }
          throw new Error(`unexpected status ${response.status}`);
        }
        const data = (await response.json()) as ProjectDetail;
        if (alive && projectDataHasChanged(projectRef.current, data)) {
          projectRef.current = data;
          setProject(data);
        }
      } catch {
        if (alive) {
          setError((prev) => (prev === "Unable to load project" ? prev : "Unable to load project"));
        }
      } finally {
        if (alive) {
          setStatus("ready");
        }
      }
    };

    void loadProject();

    return () => {
      alive = false;
    };
  }, [apiBaseUrl, authBaseUrl, currentProjectPath, projectId, router]);

  useEffect(() => {
    let alive = true;
    const loadRole = async () => {
      try {
        const response = await fetch(`${apiBaseUrl}/me`, { credentials: "include" });
        if (!response.ok) {
          return;
        }
        const data = (await response.json()) as { role?: string };
        if (alive) {
          setRole(data.role || null);
        }
      } catch {
        // Ignore.
      }
    };
    void loadRole();
    return () => {
      alive = false;
    };
  }, [apiBaseUrl]);

  useEffect(() => {
    let alive = true;
    const loadCompanies = async () => {
      if (role !== "staff") {
        return;
      }
      try {
        const response = await fetch(`${apiBaseUrl}/companies`, {
          credentials: "include",
        });
        if (!response.ok) {
          return;
        }
        const data = (await response.json()) as { name: string }[];
        if (alive) {
          setCompanies(
            data
              .map((item) => item.name)
              .filter((name) => name && name.trim() !== "")
              .sort((a, b) => a.localeCompare(b))
          );
        }
      } catch {
        // Ignore.
      }
    };
    void loadCompanies();
    return () => {
      alive = false;
    };
  }, [apiBaseUrl, role]);

  const handleRefresh = async () => {
    if (!projectId) {
      return;
    }
    setStatus("loading");
    setError(null);
    try {
      const response = await fetch(`${apiBaseUrl}/projects/${projectId}`, {
        credentials: "include",
      });
      if (!response.ok) {
        if (response.status === 401) {
          redirectToAuthLogin(authBaseUrl, currentProjectPath);
          return;
        }
        throw new Error(`unexpected status ${response.status}`);
      }
      const data = (await response.json()) as ProjectDetail;
      if (projectDataHasChanged(projectRef.current, data)) {
        projectRef.current = data;
        setProject(data);
      }
    } catch {
      setError((prev) => (prev === "Unable to load project" ? prev : "Unable to load project"));
    } finally {
      setStatus("ready");
    }
  };

  const sectionNavItems: Array<{
    id: ProjectSectionId;
    label: string;
    href: string;
    statusTone?: "success" | "danger";
    statusSymbol?: string;
  }> = projectId
    ? projectSectionRoutes.map((item) => {
        const adoptionStatus = project?.dotProjectAdoptionStatus || "";
        const dotProjectMissing = adoptionStatus === "not_found";
        const dotProjectPresent = !dotProjectMissing && Boolean(project?.dotProjectRepoRef);
        const isLegacySuccess = item.id === "legacy" && dotProjectMissing;
        const isLegacyDanger = item.id === "legacy" && dotProjectPresent;
        const isDotProjectSuccess = item.id === "dot-project" && dotProjectPresent;
        const isDotProjectDanger = item.id === "dot-project" && dotProjectMissing;
        return {
          id: item.id,
          label: item.label,
          href: `/projects/${projectId}/${item.segment}`,
          statusTone: isLegacySuccess || isDotProjectSuccess ? "success" : isLegacyDanger || isDotProjectDanger ? "danger" : undefined,
          statusSymbol: isLegacySuccess || isDotProjectSuccess ? "✓" : isLegacyDanger || isDotProjectDanger ? "✕" : undefined,
        };
      })
    : [];

  return (
    <AppShell>
      <div className={styles.page}>
        <div className={styles.container}>
          {status === "loading" && <div className={styles.banner}>Loading…</div>}
          {error && <div className={styles.banner}>{error}</div>}
          {project ? (
            <>
              <ProjectReconciliationCard
                projectId={project.id}
                name={project.name}
                maturity={project.maturity}
                maintainerRef={project.legacyMaintainerRef}
                dotProjectRepoRef={project.dotProjectRepoRef}
                dotProjectProjectRef={project.dotProjectProjectRef}
                dotProjectMaintainerRef={project.dotProjectMaintainerRef}
                dotProjectSecurityRef={project.dotProjectSecurityRef}
                dotProjectContributingRef={project.dotProjectContributingRef}
                dotProjectGovernanceRef={project.dotProjectGovernanceRef}
                dotProjectSchemaVersion={project.dotProjectSchemaVersion}
                dotProjectMaintainerCount={project.dotProjectMaintainerCount}
                dotProjectLastSyncedAt={project.dotProjectLastSyncedAt}
                dotProjectAdoptionStatus={project.dotProjectAdoptionStatus}
                dotProjectSyncState={project.dotProjectSyncState}
                dotProjectMaintainerCache={project.dotProjectMaintainerCache}
                dotProjectMaintainerPullRequest={project.dotProjectMaintainerPullRequest}
                dotProjectGeneratedMaintainersYaml={project.dotProjectGeneratedMaintainersYaml}
                maintainerRefStatus={project.maintainerRefStatus}
                maintainerRefBody={project.legacyMaintainerRefBody}
                refOnlyGitHub={project.refOnlyGitHub}
                refLines={project.refLines}
                onboardingIssue={project.onboardingIssue}
                mailingList={project.mailingList}
                maintainers={project.maintainers}
                services={project.services}
                fossaTeamId={project.fossaTeamId}
                fossaTeamName={project.fossaTeamName}
                fossaTeamMembers={project.fossaTeamMembers}
                fossaInviteIneligible={project.fossaInviteIneligible}
                fossaInviteCandidates={project.fossaInviteCandidates}
                createdAt={project.createdAt}
                updatedAt={project.updatedAt}
                updatedBy={project.updatedBy}
                updatedAuditId={project.updatedAuditId}
                onRefresh={handleRefresh}
                isRefreshing={status === "loading"}
                canEdit={role === "staff"}
                companyOptions={companies}
                activeSection={section}
                sectionNavItems={sectionNavItems}
                hideSectionMenu={false}
                onUpdateMaturity={async (next) => {
                  if (!projectId) {
                    return;
                  }
                  const response = await fetch(`${apiBaseUrl}/projects/${projectId}/maturity`, {
                    method: "PATCH",
                    headers: { "Content-Type": "application/json" },
                    credentials: "include",
                    body: JSON.stringify({ maturity: next }),
                  });
                  if (!response.ok) {
                    setError("Unable to update project status");
                    throw new Error("update failed");
                  }
                  await handleRefresh();
                }}
                onUpdateMaintainerRef={async (nextRef) => {
                  if (!projectId) {
                    return;
                  }
                  const response = await fetch(`${apiBaseUrl}/projects/${projectId}`, {
                    method: "PATCH",
                    headers: { "Content-Type": "application/json" },
                    credentials: "include",
                    body: JSON.stringify({ legacyMaintainerRef: nextRef }),
                  });
                  if (!response.ok) {
                    setError("Unable to update project admin file");
                    throw new Error("update failed");
                  }
                  await handleRefresh();
                }}
                onAddMaintainer={async (payload: AddMaintainerPayload) => {
                  if (!projectId) {
                    return;
                  }
                  if (payload.companyMode === "new" && payload.company.trim() !== "") {
                    const companyResponse = await fetch(`${apiBaseUrl}/companies`, {
                      method: "POST",
                      headers: { "Content-Type": "application/json" },
                      credentials: "include",
                      body: JSON.stringify({ name: payload.company }),
                    });
                    if (companyResponse.status === 409) {
                      setError("Company already exists. Select it from the list instead.");
                      return;
                    }
                  }
                  const response = await fetch(`${apiBaseUrl}/maintainers/from-ref`, {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    credentials: "include",
                    body: JSON.stringify({
                      projectId: Number(projectId),
                      name: payload.name,
                      githubHandle: payload.githubHandle,
                      email: payload.email,
                      company: payload.company,
                    }),
                  });
                  if (!response.ok) {
                    setError("Unable to add maintainer");
                    return;
                  }
                  await handleRefresh();
                }}
                onBulkStatusChange={async (ids, statusValue) => {
                  await fetch(`${apiBaseUrl}/maintainers/status`, {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    credentials: "include",
                    body: JSON.stringify({ ids, status: statusValue, projectId: Number(projectId) }),
                  });
                  await handleRefresh();
                }}
              />
              {children}
            </>
          ) : null}
        </div>
      </div>
    </AppShell>
  );
}
