"use client";

import { useRef, useState } from "react";
import Link from "next/link";
import { Card } from "clo-ui/components/Card";
import styles from "./MaintainerCard.module.css";

type MaintainerCardProps = {
  name: string;
  email: string;
  github: string;
  githubEmail: string;
  status: string;
  company?: string;
  location?: string;
  country?: string;
  timezone?: string;
  projects: Array<{ id: number; name: string } | string>;
  createdAt?: string;
  updatedAt?: string;
  updatedBy?: string;
  updatedNotice?: string | null;
};

const formatOrdinalDay = (day: number) => {
  const mod10 = day % 10;
  const mod100 = day % 100;
  if (mod10 === 1 && mod100 !== 11) return `${day}st`;
  if (mod10 === 2 && mod100 !== 12) return `${day}nd`;
  if (mod10 === 3 && mod100 !== 13) return `${day}rd`;
  return `${day}th`;
};

const formatDateParts = (value?: string | null) => {
  if (!value) {
    return null;
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return null;
  }
  const weekday = parsed.toLocaleDateString("en-US", { weekday: "short" }).toUpperCase();
  const month = parsed.toLocaleDateString("en-US", { month: "short" }).toUpperCase();
  const day = formatOrdinalDay(parsed.getDate());
  const year = parsed.toLocaleDateString("en-US", { year: "numeric" });
  const timeLabel = parsed
    .toLocaleTimeString("en-US", { hour: "2-digit", minute: "2-digit", hour12: true })
    .replace(/^0/, "");
  const tz = new Intl.DateTimeFormat("en-US", { timeZoneName: "short" })
    .format(parsed)
    .split(" ")
    .pop();
  const tzLabel = tz ? ` ${tz}` : "";
  return { weekday, month, day, year, timeLabel, tzLabel };
};

const renderDate = (value?: string | null) => {
  const parts = formatDateParts(value);
  if (!parts) {
    return "—";
  }
  const dayMatch = parts.day.match(/^(\d+)(\D+)$/);
  if (!dayMatch) {
    return `${parts.weekday} ${parts.month} ${parts.day} ${parts.year} ${parts.timeLabel}${parts.tzLabel}`;
  }
  const [, dayNumber, suffix] = dayMatch;
  return (
    <span className={styles.dateLine}>
      {parts.weekday} {parts.month} {dayNumber}
      <sup className={styles.ordinal}>{suffix}</sup> {parts.year} {parts.timeLabel}
      {parts.tzLabel}
    </span>
  );
};

export default function MaintainerCard({
  name,
  email,
  github,
  githubEmail,
  status,
  company,
  location,
  country,
  timezone,
  projects,
  createdAt,
  updatedAt,
  updatedBy,
  updatedNotice,
}: MaintainerCardProps) {
  const displayName = name || "Unknown maintainer";
  const hasEmail = email && email !== "—";
  const githubHandle = github && github !== "—" ? github : "";
  const [copyNotice, setCopyNotice] = useState(false);
  const copyTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const handleCopyEmail = async () => {
    if (!hasEmail) {
      return;
    }
    const value = `${displayName} <${email}>`;
    try {
      await navigator.clipboard.writeText(value);
      setCopyNotice(true);
      if (copyTimer.current) {
        clearTimeout(copyTimer.current);
      }
      copyTimer.current = setTimeout(() => setCopyNotice(false), 1500);
    } catch {
      // no-op: clipboard might be unavailable
    }
  };

  return (
    <Card hoverable={false} className={styles.card}>
      <div className={styles.content}>
        <div className={styles.header}>
          <div>
            <h1 className={styles.name}>{displayName}</h1>
            {company ? <p className={styles.company}>{company}</p> : null}
          </div>
          <div className={styles.statusStack}>
            {updatedNotice ? (
              <span className={styles.updatedNotice}>{updatedNotice}</span>
            ) : null}
            {status ? <span className={styles.status}>{status}</span> : null}
          </div>
        </div>

        <div className={styles.section}>
          <h2 className={styles.sectionTitle}>Contact</h2>
          {hasEmail ? (
            <div className={styles.detailRow}>
              <span className={styles.detailLabel}>Email</span>
              <span className={styles.detailValue}>
                {email || "—"}
                <button
                  className={styles.copyButton}
                  type="button"
                  onClick={handleCopyEmail}
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
                {copyNotice ? <span className={styles.copyToast}>Copied</span> : null}
              </span>
            </div>
          ) : null}
          <div className={styles.detailRow}>
            <span className={styles.detailLabel}>GitHub</span>
            <span className={styles.detailValue}>
              {githubHandle ? (
                <a
                  className={styles.link}
                  href={`https://github.com/${githubHandle}`}
                  target="_blank"
                  rel="noreferrer"
                >
                  {githubHandle}
                </a>
              ) : (
                "—"
              )}
            </span>
          </div>
          <div className={styles.detailRow}>
            <span className={styles.detailLabel}>GitHub Email</span>
            <span className={styles.detailValue}>{githubEmail || "—"}</span>
          </div>
          {(location || country || timezone) ? (
            <div className={styles.detailRow}>
              <span className={styles.detailLabel}>Location</span>
              <span className={styles.detailValue}>
                {[location, country, timezone].filter(Boolean).join(" · ") || "—"}
              </span>
            </div>
          ) : null}
        </div>

        <div className={styles.section}>
          <h2 className={styles.sectionTitle}>Projects</h2>
          {projects.length === 0 ? (
            <div className={styles.empty}>No projects found.</div>
          ) : (
            <ul className={styles.projectList}>
              {projects.map((project, index) => {
                const item =
                  typeof project === "string"
                    ? { id: null, name: project }
                    : project;
                if (item.id) {
                  return (
                    <li key={item.id}>
                      <Link className={styles.link} href={`/projects/${item.id}`}>
                        {item.name}
                      </Link>
                    </li>
                  );
                }
                return <li key={`${item.name}-${index}`}>{item.name}</li>;
              })}
            </ul>
          )}
        </div>

        <div className={styles.section}>
          <h2 className={styles.sectionTitle}>Record</h2>
          <div className={styles.detailRow}>
            <span className={styles.detailLabel}>Created</span>
            {renderDate(createdAt)}
          </div>
          <div className={styles.detailRow}>
            <span className={styles.detailLabel}>Last updated</span>
            <span className={styles.detailValue}>
              {renderDate(updatedAt)}
              {updatedBy ? <span className={styles.updatedBy}>by {updatedBy}</span> : null}
            </span>
          </div>
        </div>
      </div>
    </Card>
  );
}
