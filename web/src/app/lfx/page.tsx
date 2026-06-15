"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import AppShell from "@/components/AppShell";
import styles from "./page.module.css";

type AccessResponse = {
  canRun: boolean;
  allowedLogins: string[];
};

type LFXRun = {
  id: string;
  status: "running" | "succeeded" | "failed";
  requestedBy: string;
  createdAt: string;
  startedAt?: string;
  finishedAt?: string;
  requestDelay: string;
  requestsPerSecond: number;
  maxLookups: number;
  total: number;
  processed: number;
  current?: string;
  attempted: number;
  matched: number;
  ambiguous: number;
  unmatched: number;
  errored: number;
  skippedRecent: number;
  skippedLimit: number;
  error?: string;
};

const statusLabel = (status: LFXRun["status"]) => {
  switch (status) {
    case "running":
      return "Running";
    case "succeeded":
      return "Succeeded";
    case "failed":
      return "Failed";
  }
};

const formatDate = (value?: string) => {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
};

const progressPercent = (run: LFXRun) => {
  if (run.total <= 0) return 0;
  return Math.min(100, Math.round((run.processed / run.total) * 100));
};

export default function LFXPage() {
  const [access, setAccess] = useState<AccessResponse | null>(null);
  const [runs, setRuns] = useState<LFXRun[]>([]);
  const [activeRunID, setActiveRunID] = useState<string | null>(null);
  const [token, setToken] = useState("");
  const [acl, setACL] = useState("");
  const [requestsPerSecond, setRequestsPerSecond] = useState("4");
  const [maxLookups, setMaxLookups] = useState("50");
  const [status, setStatus] = useState<"idle" | "loading" | "ready">("idle");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const bffBaseUrl = useMemo(() => {
    const raw = process.env.NEXT_PUBLIC_BFF_BASE_URL || "/api";
    return raw.replace(/\/+$/, "");
  }, []);
  const apiBaseUrl = useMemo(() => {
    if (bffBaseUrl === "") return "/api";
    if (bffBaseUrl.endsWith("/api")) return bffBaseUrl;
    return `${bffBaseUrl}/api`;
  }, [bffBaseUrl]);

  const activeRun = useMemo(
    () => runs.find((run) => run.id === activeRunID) || runs[0] || null,
    [activeRunID, runs]
  );
  const hasRunningRun = runs.some((run) => run.status === "running");

  const loadAccess = useCallback(async () => {
    const response = await fetch(`${apiBaseUrl}/lfx/enrichment/access`, {
      credentials: "include",
    });
    if (!response.ok) {
      throw new Error(`access status ${response.status}`);
    }
    return (await response.json()) as AccessResponse;
  }, [apiBaseUrl]);

  const loadRuns = useCallback(async () => {
    const response = await fetch(`${apiBaseUrl}/lfx/enrichment/runs`, {
      credentials: "include",
    });
    if (!response.ok) {
      throw new Error(`runs status ${response.status}`);
    }
    const data = (await response.json()) as { runs: LFXRun[] };
    return data.runs || [];
  }, [apiBaseUrl]);

  useEffect(() => {
    let alive = true;
    const load = async () => {
      setStatus("loading");
      setError(null);
      try {
        const [accessData, runData] = await Promise.all([loadAccess(), loadRuns()]);
        if (!alive) return;
        setAccess(accessData);
        setRuns(runData);
        setActiveRunID((current) => current || runData[0]?.id || null);
      } catch {
        if (alive) {
          setError("Unable to load LFX enrichment state");
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
  }, [apiBaseUrl, loadAccess, loadRuns]);

  useEffect(() => {
    if (!hasRunningRun) return;
    let alive = true;
    const interval = window.setInterval(() => {
      void loadRuns()
        .then((runData) => {
          if (!alive) return;
          setRuns(runData);
          setActiveRunID((current) => current || runData[0]?.id || null);
        })
        .catch(() => {
          if (alive) {
            setError("Unable to refresh LFX enrichment progress");
          }
        });
    }, 3000);
    return () => {
      alive = false;
      window.clearInterval(interval);
    };
  }, [apiBaseUrl, hasRunningRun, loadRuns]);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      const response = await fetch(`${apiBaseUrl}/lfx/enrichment/runs`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          token,
          acl,
          requestsPerSecond: Number(requestsPerSecond),
          maxLookups: Number(maxLookups),
        }),
      });
      if (!response.ok) {
        const body = await response.text();
        throw new Error(body.trim() || `run status ${response.status}`);
      }
      const run = (await response.json()) as LFXRun;
      setToken("");
      setRuns((current) => [run, ...current.filter((entry) => entry.id !== run.id)]);
      setActiveRunID(run.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to start LFX enrichment");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <AppShell>
      <main className={styles.page}>
        <div className={styles.container}>
          <section className={styles.header}>
            <div>
              <h1 className={styles.title}>LFX Enrichment</h1>
              <p className={styles.sub}>Manual identity enrichment for selected staff operators.</p>
            </div>
            <div className={styles.operatorList}>
              <span>Allowed</span>
              <strong>{access?.allowedLogins.length ? access.allowedLogins.join(", ") : "-"}</strong>
            </div>
          </section>

          {error ? <div className={styles.banner}>{error}</div> : null}

          {status === "ready" && access && !access.canRun ? (
            <section className={styles.card}>
              <h2 className={styles.cardTitle}>Access restricted</h2>
              <p className={styles.bodyText}>Your account is not listed for LFX enrichment runs.</p>
            </section>
          ) : null}

          {access?.canRun ? (
            <section className={styles.grid}>
              <form className={styles.card} onSubmit={handleSubmit}>
                <h2 className={styles.cardTitle}>Start Run</h2>
                <label className={styles.field}>
                  <span>LFX PAT</span>
                  <input
                    autoComplete="off"
                    name="lfx-token"
                    onChange={(event) => setToken(event.target.value)}
                    required
                    type="password"
                    value={token}
                  />
                </label>
                <label className={styles.field}>
                  <span>ACL header</span>
                  <input
                    autoComplete="off"
                    name="lfx-acl"
                    onChange={(event) => setACL(event.target.value)}
                    type="text"
                    value={acl}
                  />
                </label>
                <div className={styles.fieldRow}>
                  <label className={styles.field}>
                    <span>Requests/sec</span>
                    <input
                      max="4"
                      min="0.25"
                      onChange={(event) => setRequestsPerSecond(event.target.value)}
                      step="0.25"
                      type="number"
                      value={requestsPerSecond}
                    />
                  </label>
                  <label className={styles.field}>
                    <span>Max lookups</span>
                    <input
                      min="0"
                      onChange={(event) => setMaxLookups(event.target.value)}
                      step="1"
                      type="number"
                      value={maxLookups}
                    />
                  </label>
                </div>
                <p className={styles.bodyText}>
                  Runs continue asynchronously. Keep this page open to watch progress, or return later for recent run status.
                </p>
                <button className={styles.primaryButton} disabled={submitting || !token.trim()} type="submit">
                  {submitting ? "Starting..." : "Start enrichment"}
                </button>
              </form>

              <section className={styles.card}>
                <h2 className={styles.cardTitle}>Run Status</h2>
                {activeRun ? (
                  <div className={styles.runPanel}>
                    <div className={styles.runHeader}>
                      <div>
                        <div className={styles.runID}>Run #{activeRun.id}</div>
                        <div className={styles.runMeta}>
                          {statusLabel(activeRun.status)} · {activeRun.requestedBy} · {formatDate(activeRun.startedAt)}
                        </div>
                      </div>
                      <span className={`${styles.statusPill} ${styles[activeRun.status]}`}>
                        {statusLabel(activeRun.status)}
                      </span>
                    </div>
                    <div className={styles.progressTrack}>
                      <div className={styles.progressFill} style={{ width: `${progressPercent(activeRun)}%` }} />
                    </div>
                    <div className={styles.progressText}>
                      {activeRun.processed.toLocaleString()} / {activeRun.total.toLocaleString()} candidates
                    </div>
                    {activeRun.current ? <div className={styles.current}>Current: {activeRun.current}</div> : null}
                    <dl className={styles.statsGrid}>
                      <div><dt>Attempted</dt><dd>{activeRun.attempted.toLocaleString()}</dd></div>
                      <div><dt>Matched</dt><dd>{activeRun.matched.toLocaleString()}</dd></div>
                      <div><dt>Ambiguous</dt><dd>{activeRun.ambiguous.toLocaleString()}</dd></div>
                      <div><dt>Unmatched</dt><dd>{activeRun.unmatched.toLocaleString()}</dd></div>
                      <div><dt>Errored</dt><dd>{activeRun.errored.toLocaleString()}</dd></div>
                      <div><dt>Skipped</dt><dd>{(activeRun.skippedRecent + activeRun.skippedLimit).toLocaleString()}</dd></div>
                    </dl>
                    <div className={styles.runMeta}>
                      Request delay {activeRun.requestDelay}; max lookups{" "}
                      {activeRun.maxLookups === 0 ? "unlimited" : activeRun.maxLookups.toLocaleString()}.
                    </div>
                    {activeRun.error ? <div className={styles.banner}>{activeRun.error}</div> : null}
                  </div>
                ) : (
                  <p className={styles.bodyText}>No LFX enrichment runs have been started in this process.</p>
                )}
              </section>
            </section>
          ) : null}

          {runs.length > 0 ? (
            <section className={styles.card}>
              <h2 className={styles.cardTitle}>Recent Runs</h2>
              <div className={styles.tableWrap}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th>Run</th>
                      <th>Status</th>
                      <th>Operator</th>
                      <th>Progress</th>
                      <th>Matched</th>
                      <th>Started</th>
                    </tr>
                  </thead>
                  <tbody>
                    {runs.map((run) => (
                      <tr key={run.id}>
                        <td>
                          <button className={styles.linkButton} onClick={() => setActiveRunID(run.id)} type="button">
                            #{run.id}
                          </button>
                        </td>
                        <td>{statusLabel(run.status)}</td>
                        <td>{run.requestedBy}</td>
                        <td>{run.processed.toLocaleString()} / {run.total.toLocaleString()}</td>
                        <td>{run.matched.toLocaleString()}</td>
                        <td>{formatDate(run.startedAt)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
          ) : null}
        </div>
      </main>
    </AppShell>
  );
}
