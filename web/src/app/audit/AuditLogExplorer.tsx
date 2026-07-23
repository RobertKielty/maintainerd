"use client";

import { useEffect, useMemo, useState } from "react";
import { Pagination } from "clo-ui/components/Pagination";
import type { AuditLog } from "./auditShared";
import { renderMetadata, renderTargets, renderLinkedText } from "./auditShared";
import styles from "./page.module.css";

type AuditLogExplorerProps = {
  title: string;
  subtitle: string;
  /** When set, restricts results to these actions and hides the action filter. */
  lockedActions?: string[];
  /** Action choices offered in the filter dropdown (ignored when lockedActions is set). */
  actionOptions?: string[];
};

export default function AuditLogExplorer({
  title,
  subtitle,
  lockedActions,
  actionOptions,
}: AuditLogExplorerProps) {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [total, setTotal] = useState(0);
  const [status, setStatus] = useState<"idle" | "loading" | "ready">("idle");
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null);
  const limit = 25;

  const [startTime, setStartTime] = useState("");
  const [endTime, setEndTime] = useState("");
  const [action, setAction] = useState("");
  const [target, setTarget] = useState("");
  const [targetInput, setTargetInput] = useState("");

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
        const effectiveAction = lockedActions?.join(",") || action;
        if (effectiveAction) {
          params.set("action", effectiveAction);
        }
        if (startTime) {
          params.set("startTime", startTime);
        }
        if (endTime) {
          params.set("endTime", endTime);
        }
        if (target) {
          params.set("target", target);
        }
        const response = await fetch(`${apiBaseUrl}/audit?${params.toString()}`, {
          credentials: "include",
        });
        if (!response.ok) {
          if (response.status === 401) {
            if (alive) {
              setLogs([]);
              setTotal(0);
            }
            return;
          }
          throw new Error(`unexpected status ${response.status}`);
        }
        const data = (await response.json()) as { total: number; logs: AuditLog[] };
        if (alive) {
          setLogs(data.logs);
          setTotal(data.total);
        }
      } catch {
        if (alive) {
          setError("Unable to load audit logs");
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
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [apiBaseUrl, page, action, startTime, endTime, target, lockedActions?.join(",")]);

  const offset = (page - 1) * limit;
  const rangeStart = total === 0 ? 0 : offset + 1;
  const rangeEnd = Math.min(offset + limit, total);

  const applyTargetFilter = () => {
    setPage(1);
    setTarget(targetInput.trim());
  };

  const resetFilters = () => {
    setStartTime("");
    setEndTime("");
    setAction("");
    setTarget("");
    setTargetInput("");
    setPage(1);
  };

  const hasActiveFilters = Boolean(startTime || endTime || action || target);

  return (
    <div className={styles.page}>
      <div className={styles.container}>
        {error && <div className={styles.banner}>{error}</div>}
        <div className={styles.card}>
          <div className={styles.header}>
            <div>
              <h1 className={styles.title}>{title}</h1>
              <p className={styles.sub}>{subtitle}</p>
            </div>
          </div>

          <div className={styles.filterRow}>
            <label className={styles.filterField}>
              <span className={styles.filterLabel}>Start time</span>
              <input
                className={styles.filterInput}
                type="datetime-local"
                value={startTime}
                onChange={(event) => {
                  setPage(1);
                  setStartTime(event.target.value);
                }}
              />
            </label>
            <label className={styles.filterField}>
              <span className={styles.filterLabel}>End time</span>
              <input
                className={styles.filterInput}
                type="datetime-local"
                value={endTime}
                onChange={(event) => {
                  setPage(1);
                  setEndTime(event.target.value);
                }}
              />
            </label>
            {!lockedActions ? (
              <label className={styles.filterField}>
                <span className={styles.filterLabel}>Action</span>
                <select
                  className={styles.filterInput}
                  value={action}
                  onChange={(event) => {
                    setPage(1);
                    setAction(event.target.value);
                  }}
                >
                  <option value="">All actions</option>
                  {(actionOptions || []).map((option) => (
                    <option key={option} value={option}>
                      {option}
                    </option>
                  ))}
                </select>
              </label>
            ) : null}
            <label className={styles.filterField}>
              <span className={styles.filterLabel}>Target</span>
              <input
                className={styles.filterInput}
                type="text"
                placeholder="Project, maintainer, staff or ID"
                value={targetInput}
                onChange={(event) => setTargetInput(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    applyTargetFilter();
                  }
                }}
                onBlur={applyTargetFilter}
              />
            </label>
            {hasActiveFilters ? (
              <button className={styles.resetButton} type="button" onClick={resetFilters}>
                Reset filters
              </button>
            ) : null}
          </div>

          {status === "loading" ? (
            <div className={styles.empty}>Loading audit logs…</div>
          ) : logs.length === 0 ? (
            <div className={styles.empty}>No audit logs match these filters.</div>
          ) : (
            <table className={styles.table}>
              <thead>
                <tr>
                  <th>Time</th>
                  <th>Action</th>
                  <th>Target</th>
                  <th>Staff</th>
                  <th>Message</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {logs.map((entry) => (
                  <tr key={entry.id}>
                    <td className={styles.mono}>
                      {new Date(entry.createdAt).toLocaleString()}
                    </td>
                    <td className={styles.mono}>{entry.action}</td>
                    <td>{renderTargets(entry)}</td>
                    <td>
                      {entry.staffName ||
                        entry.staffLogin ||
                        (entry.staffId ? `Staff #${entry.staffId}` : "—")}
                    </td>
                    <td>{entry.message ? renderLinkedText(entry.message) : "—"}</td>
                    <td>
                      <button
                        className={styles.viewButton}
                        type="button"
                        onClick={() => setSelectedLog(entry)}
                      >
                        View
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
          {total > limit ? (
            <div className={styles.paginationRow}>
              <div className={styles.results}>
                {rangeStart}-{rangeEnd} of {total}
              </div>
              <Pagination
                limit={limit}
                total={total}
                offset={offset}
                active={page}
                onChange={(next) => setPage(next)}
              />
            </div>
          ) : null}
        </div>
      </div>
      {selectedLog ? (
        <div className={styles.modalOverlay} role="dialog" aria-modal="true">
          <div className={styles.modal}>
            <div className={styles.modalHeader}>
              <h2 className={styles.modalTitle}>Audit Event</h2>
              <button
                className={styles.closeButton}
                type="button"
                onClick={() => setSelectedLog(null)}
              >
                Close
              </button>
            </div>
            <div className={styles.modalBody}>
              <div className={styles.modalRow}>
                <span className={styles.modalLabel}>Time</span>
                <span className={styles.modalValue}>
                  {new Date(selectedLog.createdAt).toLocaleString()}
                </span>
              </div>
              <div className={styles.modalRow}>
                <span className={styles.modalLabel}>Action</span>
                <span className={`${styles.modalValue} ${styles.mono}`}>
                  {selectedLog.action}
                </span>
              </div>
              <div className={styles.modalRow}>
                <span className={styles.modalLabel}>Staff</span>
                <span className={styles.modalValue}>
                  {selectedLog.staffName ||
                    selectedLog.staffLogin ||
                    (selectedLog.staffId ? `Staff #${selectedLog.staffId}` : "—")}
                </span>
              </div>
              <div className={styles.modalRow}>
                <span className={styles.modalLabel}>Target</span>
                <span className={styles.modalValue}>{renderTargets(selectedLog)}</span>
              </div>
              <div className={styles.modalRow}>
                <span className={styles.modalLabel}>Message</span>
                <span className={styles.modalValue}>
                  {selectedLog.message ? renderLinkedText(selectedLog.message) : "—"}
                </span>
              </div>
              <div className={styles.modalRow}>
                <span className={styles.modalLabel}>Metadata</span>
                <div className={styles.modalValue}>{renderMetadata(selectedLog.metadata)}</div>
              </div>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
