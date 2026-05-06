"use client";

import { useEffect, useMemo, useState } from "react";
import { usePathname, useSearchParams } from "next/navigation";
import Link from "next/link";
import { Pagination } from "clo-ui/components/Pagination";
import AppShell from "@/components/AppShell";
import { getAuthBaseUrl, redirectToAuthLogin } from "@/utils/auth";
import styles from "./page.module.css";

type SearchProject = {
  id: number;
  name: string;
  githubOrg?: string;
  onboardingIssue?: string | null;
  legacyMaintainerRef?: string;
  dotProjectMaintainerRef?: string;
};

type SearchMaintainer = {
  id: number;
  name: string;
  github: string;
  email?: string;
  company?: string;
  projects?: { id: number; name: string }[];
};

type SearchCompany = {
  id: number;
  name: string;
};

type SearchResponse = {
  query: string;
  projects: SearchProject[];
  maintainers: SearchMaintainer[];
  companies: SearchCompany[];
  projectsTotal: number;
  maintainersTotal: number;
  companiesTotal: number;
};

export default function SearchClient() {
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const query = (searchParams.get("query") || "").trim();
  const [results, setResults] = useState<SearchResponse | null>(null);
  const [status, setStatus] = useState<"idle" | "loading" | "ready">("idle");
  const [error, setError] = useState<string | null>(null);
  const [projectsPage, setProjectsPage] = useState(1);
  const [maintainersPage, setMaintainersPage] = useState(1);
  const [companiesPage, setCompaniesPage] = useState(1);
  const limit = 20;

  const bffBaseUrl = useMemo(() => {
    const raw = process.env.NEXT_PUBLIC_BFF_BASE_URL || "/api";
    return raw.replace(/\/+$/, "");
  }, []);
  const authBaseUrl = useMemo(() => getAuthBaseUrl(bffBaseUrl), [bffBaseUrl]);
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
    setProjectsPage(1);
    setMaintainersPage(1);
    setCompaniesPage(1);
  }, [query]);

  useEffect(() => {
    let alive = true;
    if (!query) {
      setResults(null);
      setStatus("idle");
      setError(null);
      return;
    }
    const load = async () => {
      setStatus("loading");
      setError(null);
      try {
        const response = await fetch(
          `${apiBaseUrl}/search?query=${encodeURIComponent(query)}&projectsPage=${projectsPage}&maintainersPage=${maintainersPage}&companiesPage=${companiesPage}`,
          { credentials: "include" }
        );
        if (!response.ok) {
          if (response.status === 401) {
            redirectToAuthLogin(authBaseUrl, pathname);
            return;
          }
          if (response.status === 403) {
            setError("You do not have access to global search.");
            setStatus("ready");
            return;
          }
          const text = await response.text();
          throw new Error(text || `unexpected status ${response.status}`);
        }
        const data = (await response.json()) as SearchResponse;
        if (alive) {
          setResults(data);
        }
      } catch (err) {
        if (alive) {
          setError((err as Error).message || "Unable to load search results.");
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
  }, [apiBaseUrl, authBaseUrl, companiesPage, maintainersPage, pathname, projectsPage, query]);

  const hasQuery = query.length > 0;
  const hasResults =
    results &&
    (results.projects.length > 0 ||
      results.maintainers.length > 0 ||
      results.companies.length > 0);
  const showRefineNotice =
    results &&
    (results.projectsTotal > 200 ||
      results.maintainersTotal > 200 ||
      results.companiesTotal > 200);

  return (
    <AppShell>
      <main className={styles.main}>
        <header className={styles.header}>
          <h1>Global Search</h1>
          <p>Search projects, maintainers, and companies.</p>
          <p className={styles.note}>
            Results are a quick finder, not an exhaustive database scan. Refine your query for
            deeper coverage.
          </p>
          {hasQuery ? <div className={styles.query}>Query: “{query}”</div> : null}
          {showRefineNotice ? (
            <div className={styles.refine}>Large result set — refine your query.</div>
          ) : null}
        </header>

        {!hasQuery ? (
          <div className={styles.empty}>Enter a search term in the shared search bar to search the directory.</div>
        ) : status === "loading" ? (
          <div className={styles.empty}>Searching…</div>
        ) : error ? (
          <div className={styles.error}>{error}</div>
        ) : hasResults ? (
          <div className={styles.results}>
            {results?.projects.length ? (
              <section className={styles.section}>
                <div className={styles.sectionHeader}>
                  <h2>Projects</h2>
                  <span className={styles.sectionCount}>
                    {(projectsPage - 1) * limit + 1}-
                    {Math.min(projectsPage * limit, results.projectsTotal)} of{" "}
                    {results.projectsTotal}
                  </span>
                </div>
                <ul>
                  {results.projects.map((project) => (
                    <li key={project.id}>
                      <Link href={`/projects/${project.id}`}>{project.name}</Link>
                      {project.githubOrg ? (
                        <span className={styles.meta}> · {project.githubOrg}</span>
                      ) : null}
                    </li>
                  ))}
                </ul>
                {results.projectsTotal > limit ? (
                  <div className={styles.paginationRow}>
                    <Pagination
                      limit={limit}
                      total={results.projectsTotal}
                      offset={(projectsPage - 1) * limit}
                      active={projectsPage}
                      onChange={(next) => setProjectsPage(next)}
                    />
                  </div>
                ) : null}
              </section>
            ) : null}

            {results?.maintainers.length ? (
              <section className={styles.section}>
                <div className={styles.sectionHeader}>
                  <h2>Maintainers</h2>
                  <span className={styles.sectionCount}>
                    {(maintainersPage - 1) * limit + 1}-
                    {Math.min(maintainersPage * limit, results.maintainersTotal)} of{" "}
                    {results.maintainersTotal}
                  </span>
                </div>
                <ul>
                  {results.maintainers.map((maintainer) => (
                    <li key={maintainer.id}>
                      <Link href={`/maintainers/${maintainer.id}`}>{maintainer.name}</Link>
                      {maintainer.github ? (
                        <span className={styles.meta}> · {maintainer.github}</span>
                      ) : null}
                      {maintainer.email ? (
                        <span className={styles.meta}> · {maintainer.email}</span>
                      ) : null}
                      {maintainer.company ? (
                        <span className={styles.meta}> · {maintainer.company}</span>
                      ) : null}
                      {maintainer.projects && maintainer.projects.length ? (
                        <span className={styles.meta}>
                          {" "}
                          ·{" "}
                          {maintainer.projects.map((project, idx) => (
                            <span key={project.id}>
                              <Link href={`/projects/${project.id}`}>{project.name}</Link>
                              {idx < maintainer.projects!.length - 1 ? ", " : ""}
                            </span>
                          ))}
                        </span>
                      ) : null}
                    </li>
                  ))}
                </ul>
                {results.maintainersTotal > limit ? (
                  <div className={styles.paginationRow}>
                    <Pagination
                      limit={limit}
                      total={results.maintainersTotal}
                      offset={(maintainersPage - 1) * limit}
                      active={maintainersPage}
                      onChange={(next) => setMaintainersPage(next)}
                    />
                  </div>
                ) : null}
              </section>
            ) : null}

            {results?.companies.length ? (
              <section className={styles.section}>
                <div className={styles.sectionHeader}>
                  <h2>Companies</h2>
                  <span className={styles.sectionCount}>
                    {(companiesPage - 1) * limit + 1}-
                    {Math.min(companiesPage * limit, results.companiesTotal)} of{" "}
                    {results.companiesTotal}
                  </span>
                </div>
                <ul>
                  {results.companies.map((company) => (
                    <li key={company.id}>
                      <Link href={`/companies/${company.id}`}>{company.name}</Link>
                    </li>
                  ))}
                </ul>
                {results.companiesTotal > limit ? (
                  <div className={styles.paginationRow}>
                    <Pagination
                      limit={limit}
                      total={results.companiesTotal}
                      offset={(companiesPage - 1) * limit}
                      active={companiesPage}
                      onChange={(next) => setCompaniesPage(next)}
                    />
                  </div>
                ) : null}
              </section>
            ) : null}
          </div>
        ) : (
          <div className={styles.empty}>No results found.</div>
        )}
      </main>
    </AppShell>
  );
}
