"use client";

import { useEffect, useMemo, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import AppShell from "@/components/AppShell";
import { getAuthBaseUrl, redirectToAuthLogin } from "@/utils/auth";
import styles from "./page.module.css";

type CompanyProject = {
  id: number;
  name: string;
};

type CompanyMaintainer = {
  id: number;
  name: string;
  github: string;
  email?: string;
  projects: CompanyProject[];
};

type CompanyDetail = {
  id: number;
  name: string;
  maintainers: CompanyMaintainer[];
};

type TableRow = {
  projectId?: number;
  projectName: string;
  maintainerId: number;
  maintainerName: string;
  github: string;
  email: string;
};

export default function CompanyPage() {
  const params = useParams();
  const id = Number(params.id);
  const [company, setCompany] = useState<CompanyDetail | null>(null);
  const [status, setStatus] = useState<"idle" | "loading" | "ready">("idle");
  const [error, setError] = useState<string | null>(null);
  const [copiedEmail, setCopiedEmail] = useState<string | null>(null);
  const [sortBy, setSortBy] = useState<"project" | "maintainer" | "github" | "email">(
    "project"
  );
  const [direction, setDirection] = useState<"asc" | "desc">("asc");
  const [projectFilter, setProjectFilter] = useState("");
  const [maintainerFilter, setMaintainerFilter] = useState("");
  const [githubFilter, setGithubFilter] = useState("");
  const [emailFilter, setEmailFilter] = useState("");

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

  useEffect(() => {
    if (!Number.isFinite(id)) {
      setError("Invalid company id.");
      setStatus("ready");
      return;
    }
    let alive = true;
    const load = async () => {
      setStatus("loading");
      setError(null);
      try {
        const response = await fetch(`${apiBaseUrl}/companies/${id}`, {
          credentials: "include",
        });
        if (!response.ok) {
          if (response.status === 401) {
            redirectToAuthLogin(authBaseUrl, `/companies/${id}`);
            return;
          }
          if (response.status === 403) {
            setError("You do not have access to this company.");
            setStatus("ready");
            return;
          }
          if (response.status === 404) {
            setError("Company not found.");
            setStatus("ready");
            return;
          }
          const text = await response.text();
          throw new Error(text || `unexpected status ${response.status}`);
        }
        const data = (await response.json()) as CompanyDetail;
        if (alive) {
          setCompany(data);
        }
      } catch (err) {
        if (alive) {
          setError((err as Error).message || "Unable to load company.");
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
  }, [apiBaseUrl, authBaseUrl, id]);

  const rows = useMemo<TableRow[]>(() => {
    if (!company) return [];
    const collected: TableRow[] = [];
    for (const maintainer of company.maintainers) {
      const maintainerName = maintainer.name || maintainer.github || maintainer.email || "Unnamed";
      const github = maintainer.github || "";
      const email = maintainer.email || "";
      if (!maintainer.projects.length) {
        collected.push({
          projectName: "Unassigned",
          maintainerId: maintainer.id,
          maintainerName,
          github,
          email,
        });
        continue;
      }
      for (const project of maintainer.projects) {
        collected.push({
          projectId: project.id,
          projectName: project.name,
          maintainerId: maintainer.id,
          maintainerName,
          github,
          email,
        });
      }
    }
    return collected;
  }, [company]);

  const filteredRows = useMemo(() => {
    const projectNeedle = projectFilter.trim().toLowerCase();
    const maintainerNeedle = maintainerFilter.trim().toLowerCase();
    const githubNeedle = githubFilter.trim().toLowerCase();
    const emailNeedle = emailFilter.trim().toLowerCase();
    return rows.filter((row) => {
      if (projectNeedle && !row.projectName.toLowerCase().includes(projectNeedle)) {
        return false;
      }
      if (maintainerNeedle && !row.maintainerName.toLowerCase().includes(maintainerNeedle)) {
        return false;
      }
      if (githubNeedle && !row.github.toLowerCase().includes(githubNeedle)) {
        return false;
      }
      if (emailNeedle && !row.email.toLowerCase().includes(emailNeedle)) {
        return false;
      }
      return true;
    });
  }, [rows, projectFilter, maintainerFilter, githubFilter, emailFilter]);

  const sortedRows = useMemo(() => {
    const sorted = [...filteredRows];
    const dir = direction === "asc" ? 1 : -1;
    sorted.sort((a, b) => {
      const aVal =
        sortBy === "project"
          ? a.projectName
          : sortBy === "maintainer"
          ? a.maintainerName
          : sortBy === "github"
          ? a.github
          : a.email;
      const bVal =
        sortBy === "project"
          ? b.projectName
          : sortBy === "maintainer"
          ? b.maintainerName
          : sortBy === "github"
          ? b.github
          : b.email;
      return aVal.localeCompare(bVal) * dir;
    });
    return sorted;
  }, [filteredRows, sortBy, direction]);

  const projectCount = useMemo(() => {
    const ids = new Set<number>();
    for (const row of rows) {
      if (row.projectId) {
        ids.add(row.projectId);
      }
    }
    return ids.size;
  }, [rows]);

  const toggleSort = (next: "project" | "maintainer" | "github" | "email") => {
    if (sortBy === next) {
      setDirection((current) => (current === "asc" ? "desc" : "asc"));
      return;
    }
    setSortBy(next);
    setDirection("asc");
  };

  const renderSort = (key: "project" | "maintainer" | "github" | "email") => {
    if (sortBy !== key) return "";
    return direction === "asc" ? " ▲" : " ▼";
  };

  const copyEmail = async (value: string, name: string) => {
    if (!value) return;
    try {
      const display = name || value;
      await navigator.clipboard.writeText(`${display} <${value}>`);
      setCopiedEmail(value);
      setTimeout(() => setCopiedEmail(null), 1500);
    } catch {
      // ignore
    }
  };

  return (
    <AppShell>
      <main className={styles.main}>
        {status === "loading" ? (
          <div className={styles.empty}>Loading company…</div>
        ) : error ? (
          <div className={styles.error}>{error}</div>
        ) : company ? (
          <>
            <header className={styles.header}>
              <h1>{company.name} Maintainers&apos; by Project</h1>
              <p>
                {company.maintainers.length} maintainers across {projectCount} projects
              </p>
            </header>

            <section className={styles.section}>
              {rows.length ? (
                <div className={styles.tableWrap}>
                  <table className={styles.table}>
                    <thead>
                      <tr>
                        <th
                          className={styles.sortable}
                          onClick={() => toggleSort("project")}
                        >
                          Project{renderSort("project")}
                        </th>
                        <th
                          className={styles.sortable}
                          onClick={() => toggleSort("maintainer")}
                        >
                          Maintainer{renderSort("maintainer")}
                        </th>
                        <th
                          className={styles.sortable}
                          onClick={() => toggleSort("github")}
                        >
                          GitHub{renderSort("github")}
                        </th>
                        <th
                          className={styles.sortable}
                          onClick={() => toggleSort("email")}
                        >
                          Email address{renderSort("email")}
                        </th>
                      </tr>
                      <tr className={styles.filters}>
                        <th>
                          <input
                            type="text"
                            placeholder="Filter"
                            value={projectFilter}
                            onChange={(e) => setProjectFilter(e.target.value)}
                          />
                        </th>
                        <th>
                          <input
                            type="text"
                            placeholder="Filter"
                            value={maintainerFilter}
                            onChange={(e) => setMaintainerFilter(e.target.value)}
                          />
                        </th>
                        <th>
                          <input
                            type="text"
                            placeholder="Filter"
                            value={githubFilter}
                            onChange={(e) => setGithubFilter(e.target.value)}
                          />
                        </th>
                        <th>
                          <input
                            type="text"
                            placeholder="Filter"
                            value={emailFilter}
                            onChange={(e) => setEmailFilter(e.target.value)}
                          />
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {sortedRows.map((row) => (
                        <tr key={`${row.projectName}-${row.maintainerId}`}>
                          <td>
                            {row.projectId ? (
                              <Link href={`/projects/${row.projectId}`}>{row.projectName}</Link>
                            ) : (
                              row.projectName
                            )}
                          </td>
                          <td>
                            <Link href={`/maintainers/${row.maintainerId}`}>
                              {row.maintainerName}
                            </Link>
                          </td>
                          <td>
                            {row.github ? (
                              <a
                                href={`https://github.com/${row.github}`}
                                target="_blank"
                                rel="noreferrer"
                              >
                                {row.github}
                              </a>
                            ) : (
                              "—"
                            )}
                          </td>
                          <td>
                            {row.email ? (
                              <span className={styles.email}>
                                {row.email}
                                <button
                                  type="button"
                                  className={styles.copyButton}
                                  onClick={() => copyEmail(row.email, row.maintainerName)}
                                  aria-label="Copy email"
                                  title="Copy email"
                                >
                                  <svg
                                    className={styles.copyIcon}
                                    viewBox="0 0 24 24"
                                    aria-hidden="true"
                                  >
                                    <path
                                      d="M8 8.5A2.5 2.5 0 0 1 10.5 6h7A2.5 2.5 0 0 1 20 8.5v7a2.5 2.5 0 0 1-2.5 2.5h-7A2.5 2.5 0 0 1 8 15.5v-7ZM10.5 7.5a1 1 0 0 0-1 1v7a1 1 0 0 0 1 1h7a1 1 0 0 0 1-1v-7a1 1 0 0 0-1-1h-7Z"
                                      fill="currentColor"
                                    />
                                    <path
                                      d="M4.5 9.5A2.5 2.5 0 0 1 7 7h1.5v1.5H7a1 1 0 0 0-1 1v7A1 1 0 0 0 7 17h7a1 1 0 0 0 1-1v-1.5H16V16a2.5 2.5 0 0 1-2.5 2.5H7A2.5 2.5 0 0 1 4.5 16v-6.5Z"
                                      fill="currentColor"
                                    />
                                  </svg>
                                </button>
                                {copiedEmail === row.email ? (
                                  <span className={styles.copyToast}>Copied</span>
                                ) : null}
                              </span>
                            ) : (
                              "—"
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <div className={styles.empty}>No maintainers found.</div>
              )}
            </section>
          </>
        ) : (
          <div className={styles.empty}>No company data.</div>
        )}
      </main>
    </AppShell>
  );
}
