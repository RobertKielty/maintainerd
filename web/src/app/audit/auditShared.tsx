"use client";

import type { ReactNode } from "react";
import Link from "next/link";
import styles from "./page.module.css";

export type AuditLog = {
  id: number;
  action: string;
  message: string;
  metadata?: string;
  createdAt: string;
  projectId?: number | null;
  projectName?: string;
  maintainerId?: number | null;
  maintainerName?: string;
  serviceId?: number | null;
  staffId?: number | null;
  staffName?: string;
  staffLogin?: string;
};

type MetadataObject = Record<string, unknown>;

const metadataSummaryOrder = [
  "loaded",
  "scanned",
  "synced",
  "errored",
  "github_error_count",
  "rate_limit_error_count",
  "skipped",
  "skipped_archived",
  "skipped_excluded",
  "not_found",
  "repo_only",
  "partial",
  "adopted",
  "auto_add_candidates",
  "auto_add_dry_run_candidates",
  "auto_add_created",
  "auto_add_linked",
  "auto_add_would_create",
  "auto_add_would_link",
  "auto_add_skipped_foundation",
  "auto_add_skipped_csv_load",
  "auto_add_skipped_project",
  "auto_add_errored",
  "auto_add_audit_failures",
  "auto_add_lfx_attempted",
  "auto_add_lfx_matched",
  "auto_add_lfx_unmatched",
  "auto_add_lfx_errored",
  "lfx_enrichment_attempted",
  "lfx_enrichment_matched",
  "lfx_enrichment_ambiguous",
  "lfx_enrichment_unmatched",
  "lfx_enrichment_errored",
  "lfx_enrichment_skipped_recent",
  "lfx_enrichment_skipped_limit",
  "db_size_bytes",
  "dot_project_sync_state_bytes",
  "cached_files",
  "maintainers_body_bytes",
  "avg_maintainers_body_bytes",
  "max_maintainers_body_bytes",
  "projects_total",
  "repos_found",
  "cached_bodies",
];

export const formatMetadataLabel = (key: string) =>
  key
    .replace(/^auto_add_/, "auto add ")
    .replace(/^lfx_enrichment_/, "lfx ")
    .replace(/_/g, " ");

export const isMetadataObject = (value: unknown): value is MetadataObject =>
  typeof value === "object" && value !== null && !Array.isArray(value);

export const formatMetadataScalar = (value: unknown) => {
  if (value === null || value === undefined || value === "") {
    return "—";
  }
  if (typeof value === "boolean") {
    return value ? "true" : "false";
  }
  if (typeof value === "number") {
    return value.toLocaleString();
  }
  if (typeof value === "string") {
    return value;
  }
  return JSON.stringify(value);
};

export const getRecordArrayColumns = (rows: MetadataObject[]) => {
  const preferred = ["label", "type", "project", "github", "lfx_id", "name", "company", "email", "url", "line", "action", "status", "reason"];
  const seen = new Set<string>();
  rows.forEach((row) => {
    Object.keys(row).forEach((key) => seen.add(key));
  });
  const hidden = new Set(["project_id", "maintainer_id"]);
  const ordered = preferred.filter((key) => seen.has(key));
  const rest = [...seen].filter((key) => !preferred.includes(key) && !hidden.has(key)).sort();
  return [...ordered, ...rest];
};

export const normalizeKey = (key: string) => key.toLowerCase().replace(/[^a-z0-9]/g, "");

export const getProjectHref = (row: MetadataObject) => {
  const rawId = row.project_id ?? row.projectId;
  const projectId = typeof rawId === "number" ? rawId : Number(rawId);
  return Number.isFinite(projectId) && projectId > 0 ? `/projects/${projectId}` : null;
};

export const getGithubHref = (value: unknown) => {
  const handle = String(value || "").trim();
  return handle ? `https://github.com/${encodeURIComponent(handle)}` : null;
};

export const getOpenProfileHref = (value: unknown) => {
  const lfxId = String(value || "").trim();
  return lfxId ? `https://openprofile.dev/profile/${encodeURIComponent(lfxId)}` : null;
};

export const normalizeAuditHref = (rawHref: string) => {
  const href = rawHref.trim();
  if (
    href.startsWith("https://github.com/") &&
    href.includes("/blob/") &&
    href.includes(".csv#L") &&
    !href.includes("?plain=1")
  ) {
    return href.replace("#L", "?plain=1#L");
  }
  return href;
};

const urlPattern = /https?:\/\/\S+/g;

export const renderLinkedText = (value: string) => {
  const parts: ReactNode[] = [];
  let lastIndex = 0;
  for (const match of value.matchAll(urlPattern)) {
    const rawUrl = match[0];
    const url = rawUrl.replace(/[).,;:]+$/, "");
    const href = normalizeAuditHref(url);
    const trailing = rawUrl.slice(url.length);
    const index = match.index ?? 0;
    if (index > lastIndex) {
      parts.push(value.slice(lastIndex, index));
    }
    parts.push(
      <a className={styles.link} href={href} key={`${url}-${index}`} rel="noreferrer" target="_blank">
        {url}
      </a>,
    );
    if (trailing) {
      parts.push(trailing);
    }
    lastIndex = index + match[0].length;
  }
  if (lastIndex < value.length) {
    parts.push(value.slice(lastIndex));
  }
  return parts.length > 0 ? parts : value;
};

export const parseMetadata = (value?: string | null) => {
  if (!value) {
    return null;
  }
  try {
    return JSON.parse(value) as unknown;
  } catch {
    return value;
  }
};

export const metadataProjectName = (entry: AuditLog) => {
  const parsed = parseMetadata(entry.metadata);
  if (!isMetadataObject(parsed)) {
    return "";
  }
  const value = parsed.project_name ?? parsed.projectName;
  return typeof value === "string" ? value.trim() : "";
};

export const metadataMaintainerName = (entry: AuditLog) => {
  const parsed = parseMetadata(entry.metadata);
  if (!isMetadataObject(parsed)) {
    return "";
  }
  const value = parsed.name ?? parsed.maintainer_name ?? parsed.maintainerName ?? parsed.github;
  return typeof value === "string" ? value.trim() : "";
};

export const renderMetadataValue = (key: string, value: unknown, row: MetadataObject) => {
  const label = normalizeKey(key);
  const text = formatMetadataScalar(value);
  if (label === "project") {
    const href = getProjectHref(row);
    if (href) {
      return (
        <Link className={styles.link} href={href}>
          {text}
        </Link>
      );
    }
  }
  if (label === "github" || label === "githubhandle") {
    const href = getGithubHref(value);
    if (href) {
      return (
        <a className={styles.link} href={href} rel="noreferrer" target="_blank">
          {text}
        </a>
      );
    }
  }
  if (label === "lfxid" || label === "lfxuserid") {
    const href = getOpenProfileHref(value);
    if (href) {
      return (
        <a className={styles.link} href={href} rel="noreferrer" target="_blank">
          {text}
        </a>
      );
    }
  }
  if ((label.endsWith("url") || label === "href") && typeof value === "string" && value.trim() !== "") {
    const href = normalizeAuditHref(value);
    return (
      <a className={styles.link} href={href} rel="noreferrer" target="_blank">
        {text}
      </a>
    );
  }
  return text;
};

export const renderMetadata = (value?: string | null) => {
  const parsed = parseMetadata(value);
  if (!parsed) {
    return "—";
  }
  if (!isMetadataObject(parsed)) {
    return <pre className={styles.metadataBox}>{String(parsed)}</pre>;
  }

  const entries = Object.entries(parsed);
  const scalarEntries = entries.filter(([, entryValue]) => {
    return !Array.isArray(entryValue) && !isMetadataObject(entryValue);
  });
  const orderedScalarEntries = [
    ...metadataSummaryOrder
      .map((key) => scalarEntries.find(([entryKey]) => entryKey === key))
      .filter((entry): entry is [string, unknown] => Boolean(entry)),
    ...scalarEntries
      .filter(([key]) => !metadataSummaryOrder.includes(key))
      .sort(([left], [right]) => left.localeCompare(right)),
  ];
  const arrayEntries = entries.filter(([, entryValue]) => Array.isArray(entryValue));
  const objectEntries = entries.filter(([, entryValue]) => isMetadataObject(entryValue));

  return (
    <div className={styles.metadataPanel}>
      {orderedScalarEntries.length > 0 ? (
        <dl className={styles.metadataSummaryGrid}>
          {orderedScalarEntries.map(([key, entryValue]) => (
            <div className={styles.metadataSummaryItem} key={key}>
              <dt>{formatMetadataLabel(key)}</dt>
              <dd>{formatMetadataScalar(entryValue)}</dd>
            </div>
          ))}
        </dl>
      ) : null}

      {arrayEntries.map(([key, entryValue]) => {
        const rows = entryValue as unknown[];
        if (rows.length === 0) {
          return null;
        }
        const objectRows = rows.filter(isMetadataObject);
        if (objectRows.length !== rows.length) {
          return (
            <section className={styles.metadataSection} key={key}>
              <h3>{formatMetadataLabel(key)}</h3>
              <div className={styles.metadataList}>
                {rows.map((row, index) => (
                  <span key={`${key}-${index}`}>{formatMetadataScalar(row)}</span>
                ))}
              </div>
            </section>
          );
        }
        const columns = getRecordArrayColumns(objectRows);
        return (
          <section className={styles.metadataSection} key={key}>
            <h3>
              {formatMetadataLabel(key)} ({objectRows.length.toLocaleString()})
            </h3>
            <div className={styles.metadataTableWrap}>
              <table className={styles.metadataTable}>
                <thead>
                  <tr>
                    {columns.map((column) => (
                      <th key={column}>{formatMetadataLabel(column)}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {objectRows.map((row, rowIndex) => (
                    <tr key={`${key}-${rowIndex}`}>
                      {columns.map((column) => (
                        <td key={column}>{renderMetadataValue(column, row[column], row)}</td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>
        );
      })}

      {objectEntries.length > 0 ? (
        <section className={styles.metadataSection}>
          <h3>Nested metadata</h3>
          <pre className={styles.metadataBox}>
            {JSON.stringify(Object.fromEntries(objectEntries), null, 2)}
          </pre>
        </section>
      ) : null}

      <details className={styles.rawMetadata}>
        <summary>Raw JSON</summary>
        <pre className={styles.metadataBox}>{JSON.stringify(parsed, null, 2)}</pre>
      </details>
    </div>
  );
};

export const renderTargets = (entry: AuditLog) => {
  const targets: ReactNode[] = [];
  if (entry.projectId) {
    const projectName = entry.projectName || metadataProjectName(entry);
    targets.push(
      <Link className={styles.link} href={`/projects/${entry.projectId}`} key="project">
        {projectName || `Project #${entry.projectId}`}
      </Link>,
    );
  }
  if (entry.maintainerId) {
    const maintainerName = entry.maintainerName || metadataMaintainerName(entry);
    targets.push(
      <Link className={styles.link} href={`/maintainers/${entry.maintainerId}`} key="maintainer">
        {maintainerName || `Maintainer #${entry.maintainerId}`}
      </Link>,
    );
  } else if (entry.action === "ADD_DOT_PROJECT_MAINTAINER") {
    const maintainerName = metadataMaintainerName(entry);
    if (maintainerName) {
      targets.push(<span key="maintainer">{maintainerName}</span>);
    }
  }
  if (entry.serviceId && targets.length === 0) {
    targets.push(<span key="service">Service #{entry.serviceId}</span>);
  }
  if (targets.length === 0) {
    return <span>—</span>;
  }
  return <span className={styles.targetList}>{targets}</span>;
};
