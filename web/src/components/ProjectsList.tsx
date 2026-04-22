"use client";

import { startTransition, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { Pagination } from "clo-ui/components/Pagination";
import styles from "./ProjectsList.module.css";
import ProjectCard from "./ProjectCard";

type MaintainerSummary = {
  id: number;
  name: string;
  github: string;
  country?: string;
  location?: string;
  timezone?: string;
};

type RecentProject = {
  id: number;
  name: string;
  addedBy: string;
  maturity?: string;
  onboardingIssue?: string;
  onboardingIssueStatus?: string;
  legacyMaintainerRef?: string;
  githubOrg?: string;
  dotProjectYamlRef?: string;
  maintainers?: MaintainerSummary[];
};

const countryFlag = (iso?: string): string => {
  if (!iso || iso.length !== 2) return "";
  return [...iso.toUpperCase()]
    .map((c) => String.fromCodePoint(c.charCodeAt(0) + 0x1f1a5))
    .join("");
};

const maintainerMatchesFilter = (m: MaintainerSummary, filter: string): boolean => {
  if (!filter) return false;
  const lower = filter.toLowerCase();
  return (
    m.name.toLowerCase().includes(lower) ||
    m.github.toLowerCase().includes(lower) ||
    (m.country?.toLowerCase().includes(lower) ?? false) ||
    (m.location?.toLowerCase().includes(lower) ?? false) ||
    (m.timezone?.toLowerCase().includes(lower) ?? false)
  );
};

type ProjectsListProps = {
  limit?: number;
};

export default function ProjectsList({ limit = 10 }: ProjectsListProps) {
  const [projects, setProjects] = useState<RecentProject[]>([]);
  const [total, setTotal] = useState(0);
  const [status, setStatus] = useState<"idle" | "loading" | "ready">("idle");
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [sortBy, setSortBy] = useState<"created" | "name" | "obIssue">("created");
  const [direction, setDirection] = useState<"asc" | "desc">("desc");
  const [maturityFilter, setMaturityFilter] = useState("all");
  const [projectNameFilter, setProjectNameFilter] = useState("");
  const [maintainerFilter, setMaintainerFilter] = useState("");
  const [maintainerFileFilter, setMaintainerFileFilter] = useState("");
  const [debouncedProjectNameFilter, setDebouncedProjectNameFilter] = useState("");
  const [debouncedMaintainerFilter, setDebouncedMaintainerFilter] = useState("");
  const [debouncedMaintainerFileFilter, setDebouncedMaintainerFileFilter] = useState("");
  const [mobileFiltersOpen, setMobileFiltersOpen] = useState(false);
  const [activeFilter, setActiveFilter] = useState<
    "projectName" | "maintainer" | "maintainerFile" | null
  >(null);
  const [rowMinHeight, setRowMinHeight] = useState<number | null>(null);
  const projectNameRef = useRef<HTMLInputElement>(null);
  const maintainerRef = useRef<HTMLInputElement>(null);
  const maintainerFileRef = useRef<HTMLInputElement>(null);
  const focusRestoreScheduled = useRef(false);
  const lastRowCountRef = useRef(0);

  useEffect(() => {
    const handle = window.setTimeout(() => {
      setDebouncedProjectNameFilter(projectNameFilter.trim());
    }, 350);
    return () => window.clearTimeout(handle);
  }, [projectNameFilter]);

  useEffect(() => {
    const handle = window.setTimeout(() => {
      setDebouncedMaintainerFilter(maintainerFilter.trim());
    }, 350);
    return () => window.clearTimeout(handle);
  }, [maintainerFilter]);

  useEffect(() => {
    const handle = window.setTimeout(() => {
      setDebouncedMaintainerFileFilter(maintainerFileFilter.trim());
    }, 350);
    return () => window.clearTimeout(handle);
  }, [maintainerFileFilter]);

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
    let alive = true;
    const load = async () => {
      setStatus("loading");
      setError(null);
      try {
        const params = new URLSearchParams();
        params.set("limit", String(limit));
        params.set("offset", String((page - 1) * limit));
        params.set("sort", sortBy);
        params.set("direction", direction);
        if (maturityFilter !== "all") {
          params.set("maturity", maturityFilter);
        }
        if (debouncedProjectNameFilter) {
          params.set("projectName", debouncedProjectNameFilter);
        }
        if (debouncedMaintainerFilter) {
          params.set("maintainer", debouncedMaintainerFilter);
        }
        if (debouncedMaintainerFileFilter) {
          params.set("maintainerFile", debouncedMaintainerFileFilter);
        }
        const response = await fetch(
          `${apiBaseUrl}/projects/recent?${params.toString()}`,
          { credentials: "include" }
        );
        if (!response.ok) {
          if (response.status === 401) {
            if (alive) {
              setProjects([]);
            }
            return;
          }
          throw new Error(`unexpected status ${response.status}`);
        }
        const data = (await response.json()) as { total?: number; projects?: RecentProject[] };
        if (alive) {
          setProjects(data.projects || []);
          setTotal(typeof data.total === "number" ? data.total : 0);
        }
      } catch {
        if (alive) {
          setError("Unable to load recent projects.");
        }
      } finally {
        if (alive) {
          setStatus("ready");
        }
      }
    };
    void load();
    return () => {
      alive = false;
    };
  }, [
    apiBaseUrl,
    direction,
    limit,
    page,
    sortBy,
    maturityFilter,
    debouncedProjectNameFilter,
    debouncedMaintainerFilter,
    debouncedMaintainerFileFilter,
  ]);

  useEffect(() => {
    const rowCount = projects.length === 0 ? 1 : projects.length;
    if (rowCount !== lastRowCountRef.current) {
      lastRowCountRef.current = rowCount;
      setRowMinHeight(rowCount * 54);
    }
  }, [projects]);

  useEffect(() => {
    if (status !== "ready" || !activeFilter) {
      return;
    }
    if (focusRestoreScheduled.current) {
      return;
    }
    const target =
      activeFilter === "projectName"
        ? projectNameRef.current
        : activeFilter === "maintainer"
        ? maintainerRef.current
        : maintainerFileRef.current;
    if (target && document.activeElement !== target) {
      focusRestoreScheduled.current = true;
      requestAnimationFrame(() => {
        if (document.activeElement !== target) {
          target.focus({ preventScroll: true });
          const length = target.value.length;
          target.setSelectionRange(length, length);
        }
        focusRestoreScheduled.current = false;
      });
    }
  }, [activeFilter, status, projects]);

  const toggleSort = (next: "name" | "obIssue") => {
    if (sortBy === next) {
      setDirection((current) => (current === "asc" ? "desc" : "asc"));
      setPage(1);
      return;
    }
    setSortBy(next);
    setDirection("asc");
    setPage(1);
  };

  const renderSort = (key: "name" | "obIssue") => {
    if (sortBy !== key) {
      return null;
    }
    return direction === "asc" ? " ▲" : " ▼";
  };

  const renderLink = (value?: string | null, label?: string) => {
    if (!value) {
      return "—";
    }
    return (
      <a className={styles.link} href={value} target="_blank" rel="noreferrer">
        {label || value}
      </a>
    );
  };

  const issueNumber = (value?: string | null) => {
    if (!value) {
      return "";
    }
    try {
      const parsed = new URL(value);
      const parts = parsed.pathname.replace(/^\/+/, "").split("/");
      if (parts.length >= 4 && parts[2] === "issues") {
        return `#${parts[3]}`;
      }
    } catch {
      // ignore
    }
    return "";
  };

  const fileName = (value?: string | null) => {
    if (!value) {
      return "";
    }
    try {
      const parsed = new URL(value);
      const parts = parsed.pathname.split("/");
      return parts[parts.length - 1] || value;
    } catch {
      return value;
    }
  };

  const orgLink = (org?: string | null) => {
    if (!org) {
      return "—";
    }
    const trimmed = org.trim();
    if (!trimmed) {
      return "—";
    }
    return (
      <a
        className={styles.link}
        href={`https://github.com/${encodeURIComponent(trimmed)}`}
        target="_blank"
        rel="noreferrer"
      >
        {trimmed}
      </a>
    );
  };

  const highlightMatch = (value: string, query: string) => {
    const trimmed = query.trim();
    if (!trimmed) {
      return value;
    }
    const lower = value.toLowerCase();
    const match = trimmed.toLowerCase();
    const index = lower.indexOf(match);
    if (index === -1) {
      return value;
    }
    const before = value.slice(0, index);
    const found = value.slice(index, index + match.length);
    const after = value.slice(index + match.length);
    return (
      <>
        {before}
        <span className={styles.matchHighlight}>{found}</span>
        {after}
      </>
    );
  };

  return (
    <div className={styles.card}>
      <div className={styles.header}>
        {total > limit ? (
          <div className={styles.paginationTop}>
            <Pagination
              limit={limit}
              total={total}
              offset={(page - 1) * limit}
              active={page}
              onChange={(next) => setPage(next)}
            />
          </div>
        ) : null}
        <div className={styles.headerLeft}>
          <div className={styles.filterRow}>
            <span className={styles.filterLabel}>Maturity</span>
            {["all", "Sandbox", "Incubating", "Graduated", "Archived"].map((value) => (
              <button
                key={value}
                type="button"
                className={`${styles.filterChip} ${
                  maturityFilter === value ? styles.filterChipActive : ""
                }`}
                onClick={() => {
                  setMaturityFilter(value);
                  setPage(1);
                }}
              >
                {value === "all" ? "All" : value}
              </button>
            ))}
          </div>
          <div className={styles.resultsRow}>
            <span className={styles.resultsInline}>
              {(page - 1) * limit + 1}-{Math.min(page * limit, total)} of {total}
            </span>
          </div>
          <div className={styles.mobileFiltersToggle}>
            <button
              type="button"
              className={styles.mobileFiltersButton}
              onClick={() => setMobileFiltersOpen((current) => !current)}
              aria-expanded={mobileFiltersOpen}
            >
              {mobileFiltersOpen ? "Hide filters" : "Show filters"}
            </button>
          </div>
          {mobileFiltersOpen ? (
            <div className={styles.mobileFiltersPanel}>
              <label className={styles.mobileFilterField}>
                <span>Project</span>
                <input
                  ref={projectNameRef}
                  className={styles.filterInput}
                  placeholder="Filter project"
                  value={projectNameFilter}
                  onChange={(event) => {
                    const value = event.target.value;
                    startTransition(() => {
                      setActiveFilter("projectName");
                      setProjectNameFilter(value);
                      setPage(1);
                    });
                  }}
                  onFocus={() => setActiveFilter("projectName")}
                />
              </label>
              <label className={styles.mobileFilterField}>
                <span>Maintainer</span>
                <input
                  ref={maintainerRef}
                  className={styles.filterInput}
                  placeholder="Filter using a maintainer's, name location or GitHub profile" 
                  value={maintainerFilter}
                  onChange={(event) => {
                    const value = event.target.value;
                    startTransition(() => {
                      setActiveFilter("maintainer");
                      setMaintainerFilter(value);
                      setPage(1);
                    });
                  }}
                  onFocus={() => setActiveFilter("maintainer")}
                />
              </label>
              <label className={styles.mobileFilterField}>
                <span>Maintainer file</span>
                <input
                  ref={maintainerFileRef}
                  className={styles.filterInput}
                  placeholder="Filter maintainer file"
                  value={maintainerFileFilter}
                  onChange={(event) => {
                    const value = event.target.value;
                    startTransition(() => {
                      setActiveFilter("maintainerFile");
                      setMaintainerFileFilter(value);
                      setPage(1);
                    });
                  }}
                  onFocus={() => setActiveFilter("maintainerFile")}
                />
              </label>
            </div>
          ) : null}
        </div>
      </div>
      {error ? <div className={styles.banner}>{error}</div> : null}
      {status === "loading" && projects.length === 0 ? (
        <div className={styles.empty}>Loading projects…</div>
      ) : (
        <>
          <table className={styles.table}>
            <colgroup>
              <col className={styles.projectNameCol} />
              <col className={styles.maturityCol} />
              <col className={styles.maintainersCol} />
              <col className={styles.addedByCol} />
              <col className={styles.obIssueCol} />
              <col className={styles.obStatusCol} />
              <col className={styles.legacyFileCol} />
              <col className={styles.orgCol} />
              <col className={styles.dotRepoCol} />
            </colgroup>
            <thead>
              <tr>
                <th className={styles.projectNameCol}>
                  <button
                    type="button"
                    className={styles.sortButton}
                    onClick={() => toggleSort("name")}
                  >
                    Project Name{renderSort("name")}
                  </button>
                </th>
                <th className={styles.maturityCol}>Maturity</th>
                <th className={styles.maintainersCol}>Maintainers</th>
                <th className={styles.addedByCol}>Added by</th>
                <th className={styles.obIssueCol}>
                  <button
                    type="button"
                    className={styles.sortButton}
                    onClick={() => toggleSort("obIssue")}
                  >
                    OB issue{renderSort("obIssue")}
                  </button>
                </th>
                <th className={styles.obStatusCol}>OB Issue Status</th>
                <th className={styles.legacyFileCol}>Legacy Maintainer File</th>
                <th className={styles.orgCol}>Main Org</th>
                <th className={styles.dotRepoCol}>.project repo</th>
              </tr>
              <tr className={styles.tableFilterRow}>
                <th className={styles.projectNameCol}>
                  <input
                    ref={projectNameRef}
                    className={styles.filterInput}
                    placeholder="Filter project"
                    value={projectNameFilter}
                    onChange={(event) => {
                      const value = event.target.value;
                      startTransition(() => {
                        setActiveFilter("projectName");
                        setProjectNameFilter(value);
                        setPage(1);
                      });
                    }}
                    onFocus={() => setActiveFilter("projectName")}
                  />
                </th>
                <th className={`${styles.maturityCol} ${styles.filterSpacer}`}>&nbsp;</th>
                <th className={styles.maintainersCol}>
                  <input
                    ref={maintainerRef}
                    className={styles.filterInput}
                    placeholder="Filter maintainer, location, or timezone"
                    value={maintainerFilter}
                    onChange={(event) => {
                      const value = event.target.value;
                      startTransition(() => {
                        setActiveFilter("maintainer");
                        setMaintainerFilter(value);
                        setPage(1);
                      });
                    }}
                    onFocus={() => setActiveFilter("maintainer")}
                  />
                </th>
                <th className={`${styles.addedByCol} ${styles.filterSpacer}`}>&nbsp;</th>
                <th className={`${styles.obIssueCol} ${styles.filterSpacer}`}>&nbsp;</th>
                <th className={`${styles.obStatusCol} ${styles.filterSpacer}`}>&nbsp;</th>
                <th className={styles.legacyFileCol}>
                  <input
                    ref={maintainerFileRef}
                    className={styles.filterInput}
                    placeholder="Filter maintainer file"
                    value={maintainerFileFilter}
                    onChange={(event) => {
                      const value = event.target.value;
                      startTransition(() => {
                        setActiveFilter("maintainerFile");
                        setMaintainerFileFilter(value);
                        setPage(1);
                      });
                    }}
                    onFocus={() => setActiveFilter("maintainerFile")}
                  />
                </th>
                <th className={`${styles.orgCol} ${styles.filterSpacer}`}>&nbsp;</th>
                <th className={`${styles.dotRepoCol} ${styles.filterSpacer}`}>&nbsp;</th>
              </tr>
            </thead>
            <tbody
              className={status === "loading" ? styles.tableBodyLoading : undefined}
              style={rowMinHeight ? { minHeight: `${rowMinHeight}px` } : undefined}
            >
              {projects.length === 0 ? (
                <tr>
                  <td colSpan={9} className={styles.emptyRow}>
                    No projects match these filters.
                  </td>
                </tr>
              ) : (
                projects.map((project) => {
                  const maintainers = project.maintainers ?? [];
                  return (
                    <tr key={project.id}>
                    <td className={styles.projectNameCol}>
                      <Link className={styles.link} href={`/projects/${project.id}`}>
                        {project.name}
                      </Link>
                    </td>
                    <td className={styles.maturityCol}>{project.maturity || "—"}</td>
                    <td className={styles.maintainersCol}>
                      {maintainers.length > 0
                        ? maintainers.map((maintainer, index) => {
                            const isMatch = maintainerMatchesFilter(maintainer, debouncedMaintainerFilter);
                            const flag = isMatch ? countryFlag(maintainer.country) : "";
                            return (
                              <span
                                key={maintainer.id}
                                className={isMatch ? styles.maintainerMatch : undefined}
                              >
                                <Link
                                  className={styles.link}
                                  href={`/maintainers/${maintainer.id}`}
                                >
                                  {highlightMatch(maintainer.name, maintainerFilter)}
                                </Link>
                                {flag ? <span title={maintainer.country}> {flag}</span> : null}
                                {index < maintainers.length - 1 ? ", " : ""}
                              </span>
                            );
                          })
                        : "—"}
                    </td>
                    <td className={styles.addedByCol}>{project.addedBy || "—"}</td>
                    <td className={styles.obIssueCol}>
                      {renderLink(
                        project.onboardingIssue,
                        issueNumber(project.onboardingIssue)
                      )}
                    </td>
                    <td className={styles.obStatusCol}>
                      {project.onboardingIssueStatus || "—"}
                    </td>
                    <td className={styles.legacyFileCol}>
                      {renderLink(
                        project.legacyMaintainerRef,
                        fileName(project.legacyMaintainerRef)
                      )}
                    </td>
                    <td className={styles.orgCol}>{orgLink(project.githubOrg)}</td>
                    <td className={styles.dotRepoCol}>
                      {renderLink(project.dotProjectYamlRef)}
                    </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
          <div className={styles.cardList}>
            {projects.length === 0 ? (
              <div className={styles.emptyRow}>No projects match these filters.</div>
            ) : (
              projects.map((project) => {
                return (
                  <ProjectCard
                    key={project.id}
                    id={project.id}
                    name={project.name}
                    maturity={project.maturity}
                    onboardingIssue={project.onboardingIssue}
                    onboardingIssueStatus={project.onboardingIssueStatus}
                    legacyMaintainerRef={project.legacyMaintainerRef}
                    githubOrg={project.githubOrg}
                    dotProjectYamlRef={project.dotProjectYamlRef}
                    maintainers={project.maintainers}
                    maintainerFilter={maintainerFilter}
                  />
                );
              })
            )}
          </div>
        </>
      )}
    </div>
  );
}
