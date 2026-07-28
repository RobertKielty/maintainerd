"use client";

import { useRef, useState } from "react";
import Link from "next/link";
import Image from "next/image";
import { Card } from "clo-ui/components/Card";
import styles from "./MaintainerCard.module.css";

const STATUS_GROUP_ORDER = ["Emeritus", "Retired", "Archived"];

export type MaintainerEditDraft = {
  name: string;
  email: string;
  github: string;
  githubEmail: string;
  location: string;
  companyId: number | null;
};

export type CompanyOption = {
  id: number;
  name: string;
};

type EditConfig = {
  draft: MaintainerEditDraft;
  companies: CompanyOption[];
  isDirty: boolean;
  saveStatus: "idle" | "saving";
  saveError: string | null;
  disableName?: boolean;
  disableGitHub?: boolean;
  disableGitHubEmail?: boolean;
  disableLocation?: boolean;
  disableCompanyAdd?: boolean;
  onEdit: () => void;
  onCancel: () => void;
  onChange: (next: MaintainerEditDraft) => void;
  onSave: () => void;
  onAddCompany: () => void;
};

export type LfxProfileSummary = {
  lfid?: string;
  matchStatus?: string;
};

export type SanitizeStatus = {
  lastRunAt?: string | null;
  nextRunAt?: string | null;
};

type MaintainerCardProps = {
  name: string;
  email: string;
  github: string;
  githubEmail: string;
  status: string;
  company?: string;
  companyId?: number | null;
  location?: string;
  country?: string;
  timezone?: string;
  projects: Array<{ id: number; name: string; status?: string; refUrl?: string } | string>;
  createdAt?: string;
  updatedAt?: string;
  updatedBy?: string;
  updatedNotice?: string | null;
  isEditing?: boolean;
  editConfig?: EditConfig;
  lfxProfile?: LfxProfileSummary | null;
  sanitizeStatus?: SanitizeStatus | null;
};

function initials(name: string) {
  const parts = name.trim().split(/\s+/);
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}

function avatarColor(name: string) {
  const colors = [
    "#0f766e", "#0369a1", "#7c3aed", "#b45309",
    "#be123c", "#15803d", "#6d28d9", "#c2410c",
  ];
  let hash = 0;
  for (let i = 0; i < name.length; i++) hash = name.charCodeAt(i) + ((hash << 5) - hash);
  return colors[Math.abs(hash) % colors.length];
}

function timeAgo(value?: string | null) {
  if (!value) return null;
  const ms = Date.now() - new Date(value).getTime();
  const mins = Math.floor(ms / 60000);
  if (mins < 2) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.floor(hrs / 24);
  if (days < 30) return `${days}d ago`;
  const months = Math.floor(days / 30);
  if (months < 12) return `${months}mo ago`;
  return `${Math.floor(months / 12)}y ago`;
}

function formatDateTime(value?: string | null) {
  if (!value) return null;
  return new Date(value).toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  });
}

function isSameDay(a: Date, b: Date) {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
}

function agoWords(value: string) {
  const ms = Date.now() - new Date(value).getTime();
  const mins = Math.floor(ms / 60000);
  if (mins < 1) return "just now";
  if (mins === 1) return "1 minute ago";
  if (mins < 60) return `${mins} minutes ago`;
  const hrs = Math.floor(mins / 60);
  return hrs === 1 ? "1 hour ago" : `${hrs} hours ago`;
}

function inWords(value: string) {
  const ms = new Date(value).getTime() - Date.now();
  if (ms <= 0) return "any moment";
  const mins = Math.ceil(ms / 60000);
  if (mins < 1) return "less than a minute";
  if (mins === 1) return "1 minute";
  if (mins < 60) return `${mins} minutes`;
  const hrs = Math.round(mins / 60);
  return hrs === 1 ? "1 hour" : `${hrs} hours`;
}

function formatCheckedAt(value?: string | null) {
  if (!value) return null;
  return isSameDay(new Date(value), new Date()) ? agoWords(value) : formatDateTime(value);
}

function formatNextCheck(value?: string | null) {
  if (!value) return null;
  return isSameDay(new Date(value), new Date())
    ? `in ${inWords(value)}`
    : formatDateTime(value);
}

function EmailIcon() {
  return (
    <svg aria-hidden="true" className={styles.icon} viewBox="0 0 24 24">
      <path d="M2 6.5A2.5 2.5 0 0 1 4.5 4h15A2.5 2.5 0 0 1 22 6.5v11a2.5 2.5 0 0 1-2.5 2.5h-15A2.5 2.5 0 0 1 2 17.5v-11Zm2.25.55V17.5c0 .14.11.25.25.25h15c.14 0 .25-.11.25-.25V7.05l-7.1 5.18a1.25 1.25 0 0 1-1.44 0L4.25 7.05ZM5.34 6.25 12 10.95l6.66-4.7H5.34Z" fill="currentColor"/>
    </svg>
  );
}

function GitHubIcon() {
  return (
    <svg aria-hidden="true" className={styles.icon} viewBox="0 0 24 24">
      <path d="M12 2C6.48 2 2 6.58 2 12.22c0 4.5 2.87 8.3 6.84 9.64.5.1.68-.22.68-.5 0-.24-.01-1.04-.01-1.9-2.78.62-3.37-1.21-3.37-1.21-.46-1.2-1.11-1.52-1.11-1.52-.91-.64.07-.62.07-.62 1 .07 1.53 1.06 1.53 1.06.9 1.58 2.35 1.13 2.92.86.09-.67.35-1.13.63-1.39-2.22-.26-4.56-1.14-4.56-5.06 0-1.12.39-2.03 1.03-2.75-.1-.26-.45-1.3.1-2.72 0 0 .84-.28 2.75 1.05A9.3 9.3 0 0 1 12 6.85c.85 0 1.7.12 2.5.36 1.9-1.33 2.74-1.05 2.74-1.05.56 1.42.21 2.46.11 2.72.64.72 1.03 1.63 1.03 2.75 0 3.93-2.35 4.8-4.59 5.05.36.32.69.95.69 1.92 0 1.39-.01 2.5-.01 2.84 0 .28.18.61.69.5A10.24 10.24 0 0 0 22 12.22C22 6.58 17.52 2 12 2Z" fill="currentColor"/>
    </svg>
  );
}

function PinIcon() {
  return (
    <svg aria-hidden="true" className={styles.icon} viewBox="0 0 24 24">
      <path d="M12 2a7 7 0 0 1 7 7c0 4.5-6.3 12.39-6.56 12.72a.58.58 0 0 1-.88 0C11.3 21.39 5 13.5 5 9a7 7 0 0 1 7-7Zm0 1.5a5.5 5.5 0 0 0-5.5 5.5c0 3.56 4.76 10.35 5.5 11.36.74-1 5.5-7.8 5.5-11.36A5.5 5.5 0 0 0 12 3.5Zm0 3a2.5 2.5 0 1 1 0 5 2.5 2.5 0 0 1 0-5Zm0 1.5a1 1 0 1 0 0 2 1 1 0 0 0 0-2Z" fill="currentColor"/>
    </svg>
  );
}

function BuildingIcon() {
  return (
    <svg aria-hidden="true" className={styles.icon} viewBox="0 0 24 24">
      <path d="M4 21V4.5A1.5 1.5 0 0 1 5.5 3h7A1.5 1.5 0 0 1 14 4.5V21h-2v-2.5H8V21H4Zm2-2h4v-2H6v2Zm0-4h4v-2H6v2Zm0-4h4V9H6v2ZM16 21v-9.5a1.5 1.5 0 0 1 1.5-1.5h3A1.5 1.5 0 0 1 22 11.5V21h-2v-2h-2v2h-2Zm2-4h2v-2h-2v2Z" fill="currentColor"/>
    </svg>
  );
}

function LfxIcon() {
  return (
    <svg aria-hidden="true" className={styles.icon} viewBox="0 0 24 24">
      <path d="M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20Zm0 1.5a8.5 8.5 0 0 1 6.36 14.15c-.83-1.6-2.86-2.9-6.36-2.9s-5.53 1.3-6.36 2.9A8.5 8.5 0 0 1 12 3.5Zm0 3a3 3 0 1 1 0 6 3 3 0 0 1 0-6Z" fill="currentColor"/>
    </svg>
  );
}

function PersonIcon() {
  return (
    <svg className={styles.chipRefIcon} viewBox="0 0 24 24" aria-hidden="true">
      <path d="M12 12a5 5 0 1 0 0-10 5 5 0 0 0 0 10Zm0 2c-4.42 0-8 2.24-8 5v1a1 1 0 0 0 1 1h14a1 1 0 0 0 1-1v-1c0-2.76-3.58-5-8-5Z" fill="currentColor"/>
    </svg>
  );
}

function CopyIcon() {
  return (
    <svg className={styles.copyIcon} viewBox="0 0 24 24" aria-hidden="true">
      <path d="M8 8.5A2.5 2.5 0 0 1 10.5 6h7A2.5 2.5 0 0 1 20 8.5v7a2.5 2.5 0 0 1-2.5 2.5h-7A2.5 2.5 0 0 1 8 15.5v-7ZM10.5 7.5a1 1 0 0 0-1 1v7a1 1 0 0 0 1 1h7a1 1 0 0 0 1-1v-7a1 1 0 0 0-1-1h-7Z" fill="currentColor"/>
      <path d="M4.5 9.5A2.5 2.5 0 0 1 7 7h1.5v1.5H7a1 1 0 0 0-1 1v7A1 1 0 0 0 7 17h7a1 1 0 0 0 1-1v-1.5H16V16a2.5 2.5 0 0 1-2.5 2.5H7A2.5 2.5 0 0 1 4.5 16v-6.5Z" fill="currentColor"/>
    </svg>
  );
}

export default function MaintainerCard({
  name,
  email,
  github,
  githubEmail,
  status,
  company,
  companyId,
  location,
  country,
  timezone,
  projects,
  updatedAt,
  updatedBy,
  updatedNotice,
  isEditing = false,
  editConfig,
  lfxProfile,
  sanitizeStatus,
}: MaintainerCardProps) {
  const displayName = name || "Unknown maintainer";
  const hasEmail = email && email !== "—" && email !== "EMAIL_MISSING";
  const githubHandle = github && github !== "—" && github !== "GITHUB_MISSING" ? github : "";
  const hasGithubEmail = githubEmail && githubEmail !== "—" && githubEmail !== "GITHUB_MISSING" && githubEmail !== email;
  const locationLine = [location, country, timezone].filter(Boolean).join(" · ");
  const openProfileHref = lfxProfile?.lfid
    ? `https://openprofile.dev/profile/${encodeURIComponent(lfxProfile.lfid)}`
    : null;
  const color = avatarColor(displayName);
  const ago = timeAgo(updatedAt);

  const normalizedProjects = projects.map((project, i) => {
    const item =
      typeof project === "string"
        ? { id: null as number | null, name: project, status: undefined as string | undefined, refUrl: undefined as string | undefined }
        : project;
    return { ...item, key: item.id ?? `${item.name}-${i}` };
  });
  const activeProjects = normalizedProjects.filter((p) => !p.status || p.status === "Active");
  const inactiveProjects = normalizedProjects.filter((p) => p.status && p.status !== "Active");
  // Group non-active projects by their actual status, rather than lumping every
  // non-Active status under a single "Emeritus Maintainer" heading — a maintainer
  // can be Retired or Archived on a project without being Emeritus there.
  const inactiveStatuses = Array.from(new Set(inactiveProjects.map((p) => p.status as string))).sort(
    (a, b) => STATUS_GROUP_ORDER.indexOf(a) - STATUS_GROUP_ORDER.indexOf(b),
  );
  const inactiveGroups = inactiveStatuses.map((status) => ({
    status,
    heading: status === "Emeritus" ? "Emeritus Maintainer" : `${status} Maintainer`,
    items: inactiveProjects.filter((p) => p.status === status),
  }));
  const renderProjectChip = (item: (typeof normalizedProjects)[number]) => {
    const pill = item.id ? (
      <Link className={styles.chip} href={`/projects/${item.id}`}>
        {item.name}
      </Link>
    ) : (
      <span className={styles.chipPlain}>{item.name}</span>
    );
    return (
      <span key={item.key} className={styles.chipGroup}>
        {pill}
        {item.refUrl ? (
          <a
            className={styles.chipRefLink}
            href={item.refUrl}
            target="_blank"
            rel="noreferrer"
            aria-label={`View ${item.name}'s entry in the maintainers file`}
            title="View this maintainer's entry in the project's maintainers file"
          >
            <PersonIcon />
          </a>
        ) : null}
      </span>
    );
  };

  const [copyNotice, setCopyNotice] = useState(false);
  const copyTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const handleCopyEmail = async () => {
    if (!hasEmail) return;
    try {
      await navigator.clipboard.writeText(email);
      setCopyNotice(true);
      if (copyTimer.current) clearTimeout(copyTimer.current);
      copyTimer.current = setTimeout(() => setCopyNotice(false), 1500);
    } catch {
      // clipboard unavailable
    }
  };

  const [githubEmailCopyNotice, setGithubEmailCopyNotice] = useState(false);
  const githubEmailCopyTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const handleCopyGithubEmail = async () => {
    if (!hasGithubEmail) return;
    try {
      await navigator.clipboard.writeText(githubEmail);
      setGithubEmailCopyNotice(true);
      if (githubEmailCopyTimer.current) clearTimeout(githubEmailCopyTimer.current);
      githubEmailCopyTimer.current = setTimeout(() => setGithubEmailCopyNotice(false), 1500);
    } catch {
      // clipboard unavailable
    }
  };

  const [lfidCopyNotice, setLfidCopyNotice] = useState(false);
  const lfidCopyTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const handleCopyLfid = async () => {
    if (!lfxProfile?.lfid) return;
    try {
      await navigator.clipboard.writeText(lfxProfile.lfid);
      setLfidCopyNotice(true);
      if (lfidCopyTimer.current) clearTimeout(lfidCopyTimer.current);
      lfidCopyTimer.current = setTimeout(() => setLfidCopyNotice(false), 1500);
    } catch {
      // clipboard unavailable
    }
  };

  const draft = editConfig?.draft;
  const canEdit = !!editConfig;

  const [erroredAvatarHandle, setErroredAvatarHandle] = useState<string | null>(null);
  const showAvatarPhoto = !isEditing && !!githubHandle && erroredAvatarHandle !== githubHandle;

  return (
    <Card hoverable={false} className={styles.card}>
      <div className={styles.content}>

        {/* ── Hero ── */}
        <div className={styles.hero}>
          {showAvatarPhoto ? (
            <Image
              className={styles.avatarImage}
              src={`https://github.com/${githubHandle}.png?size=112`}
              alt={`${displayName} on GitHub`}
              width={56}
              height={56}
              onError={() => setErroredAvatarHandle(githubHandle)}
            />
          ) : (
            <div className={styles.avatar} style={{ background: color }} aria-hidden="true">
              {initials(isEditing && draft?.name ? draft.name : displayName)}
            </div>
          )}
          <div className={styles.heroInfo}>
            {isEditing && draft && !editConfig?.disableName ? (
              <input
                className={styles.nameInput}
                type="text"
                value={draft.name}
                aria-label="Name"
                onChange={(e) => editConfig?.onChange({ ...draft, name: e.target.value })}
              />
            ) : (
              <h1 className={styles.name}>{displayName}</h1>
            )}
            {isEditing && draft ? (
              <select
                className={styles.companySelect}
                value={draft.companyId ?? ""}
                aria-label="Company"
                onChange={(e) =>
                  editConfig?.onChange({
                    ...draft,
                    companyId: e.target.value ? Number(e.target.value) : null,
                  })
                }
              >
                <option value="">No company</option>
                {editConfig?.companies.map((c) => (
                  <option key={c.id} value={c.id}>{c.name}</option>
                ))}
              </select>
            ) : company ? (
              companyId ? (
                <Link href={`/companies/${companyId}`} className={styles.companyPill}>
                  <BuildingIcon />
                  {company}
                </Link>
              ) : (
                <span className={styles.companyPill}>
                  <BuildingIcon />
                  {company}
                </span>
              )
            ) : null}
          </div>
          <div className={styles.heroMeta}>
            {updatedNotice ? (
              <span className={styles.savedBadge}>{updatedNotice}</span>
            ) : null}
            {status && status !== "Active" ? (
              <span className={styles.statusBadge}>{status}</span>
            ) : null}
            {canEdit && !isEditing ? (
              <button className={styles.editButton} type="button" onClick={editConfig?.onEdit}>
                Edit
              </button>
            ) : null}
          </div>
        </div>

        {/* ── Body: two columns ── */}
        <div className={styles.body}>

          {/* Contact */}
          <div className={styles.contactCol}>
            {/* Email */}
            {isEditing && draft ? (
              <div className={styles.contactRow}>
                <EmailIcon />
                <input
                  className={styles.contactInput}
                  type="email"
                  value={draft.email}
                  placeholder="Email"
                  aria-label="Email"
                  onChange={(e) => editConfig?.onChange({ ...draft, email: e.target.value })}
                />
              </div>
            ) : hasEmail ? (
              <div className={styles.contactRow}>
                <EmailIcon />
                <span className={styles.contactValue} aria-label="Email">
                  {email}
                  <button
                    className={styles.copyButton}
                    type="button"
                    onClick={handleCopyEmail}
                    aria-label="Copy email address"
                    title="Copy"
                  >
                    <CopyIcon />
                  </button>
                  {copyNotice ? <span className={styles.copyToast}>Copied</span> : null}
                </span>
              </div>
            ) : null}

            {/* GitHub */}
            {isEditing && draft && !editConfig?.disableGitHub ? (
              <div className={styles.contactRow}>
                <GitHubIcon />
                <input
                  className={styles.contactInput}
                  type="text"
                  value={draft.github}
                  placeholder="GitHub handle"
                  aria-label="GitHub account"
                  onChange={(e) => editConfig?.onChange({ ...draft, github: e.target.value })}
                />
              </div>
            ) : githubHandle ? (
              <div className={styles.contactRow}>
                <GitHubIcon />
                <a
                  className={styles.contactLink}
                  href={`https://github.com/${githubHandle}`}
                  target="_blank"
                  rel="noreferrer"
                  aria-label={`GitHub: ${githubHandle}`}
                >
                  <Image
                    className={styles.githubAvatar}
                    src={`https://github.com/${githubHandle}.png?size=40`}
                    alt=""
                    aria-hidden="true"
                    width={18}
                    height={18}
                    onError={(e) => { e.currentTarget.style.display = "none"; }}
                  />
                  {githubHandle}
                </a>
              </div>
            ) : null}

            {/* GitHub email */}
            {isEditing && draft && !editConfig?.disableGitHubEmail ? (
              <div className={styles.contactRow}>
                <EmailIcon />
                <input
                  className={styles.contactInput}
                  type="email"
                  value={draft.githubEmail}
                  placeholder="GitHub email"
                  aria-label="GitHub Email"
                  onChange={(e) => editConfig?.onChange({ ...draft, githubEmail: e.target.value })}
                />
              </div>
            ) : hasGithubEmail ? (
              <div className={styles.contactRow}>
                <EmailIcon />
                <span className={styles.contactSecondary} aria-label="GitHub Email">
                  {githubEmail}
                  <span className={styles.contactTag}>GitHub Email</span>
                  <button
                    className={styles.copyButton}
                    type="button"
                    onClick={handleCopyGithubEmail}
                    aria-label="Copy GitHub email address"
                    title="Copy"
                  >
                    <CopyIcon />
                  </button>
                  {githubEmailCopyNotice ? <span className={styles.copyToast}>Copied</span> : null}
                </span>
              </div>
            ) : null}

            {/* LFX / OpenProfile.dev */}
            {!isEditing && lfxProfile ? (
              <div className={styles.contactRow}>
                <LfxIcon />
                {openProfileHref ? (
                  <span className={styles.contactValue} aria-label="OpenProfile.dev">
                    <a
                      className={styles.contactLink}
                      href={openProfileHref}
                      target="_blank"
                      rel="noreferrer"
                      aria-label="OpenProfile.dev profile"
                      onClick={handleCopyLfid}
                    >
                      OpenProfile.dev
                    </a>
                    <span
                      className={styles.contactTag}
                      title="openprofile.dev has a known issue where it may report this profile as private regardless of the maintainer's settings. Clicking the search link copies the LFX ID to your clipboard so you can paste it into openprofile.dev's own search if the link doesn't work."
                    >
                      known issue · ID copied on click
                    </span>
                    <button
                      className={styles.copyButton}
                      type="button"
                      onClick={handleCopyLfid}
                      aria-label="Copy LFX ID"
                      title="Copy LFX ID to search directly on openprofile.dev"
                    >
                      <CopyIcon />
                    </button>
                    {lfidCopyNotice ? <span className={styles.copyToast}>Copied</span> : null}
                  </span>
                ) : (
                  <span className={styles.contactSecondary} aria-label="OpenProfile.dev status">
                    Not on OpenProfile.dev
                  </span>
                )}
              </div>
            ) : null}

            {/* Location */}
            {isEditing && draft && !editConfig?.disableLocation ? (
              <div className={styles.contactRow}>
                <PinIcon />
                <input
                  className={styles.contactInput}
                  type="text"
                  value={draft.location}
                  placeholder="Location"
                  aria-label="Location"
                  onChange={(e) => editConfig?.onChange({ ...draft, location: e.target.value })}
                />
              </div>
            ) : locationLine ? (
              <div className={styles.contactRow}>
                <PinIcon />
                <span className={styles.contactValue} aria-label="Location">{locationLine}</span>
              </div>
            ) : null}

            {/* Add company button (edit mode only) */}
            {isEditing && !editConfig?.disableCompanyAdd ? (
              <button
                className={styles.addCompanyButton}
                type="button"
                onClick={editConfig?.onAddCompany}
              >
                Change company affiliation
              </button>
            ) : null}
          </div>

          {/* Projects */}
          <div className={styles.projectsCol}>
            {projects.length === 0 ? (
              <p className={styles.noProjects}>No projects</p>
            ) : (
              <>
                {activeProjects.length > 0 ? (
                  <div className={styles.projectGroup}>
                    <h2 className={styles.projectGroupHeading}>Actively Maintaining</h2>
                    <div className={styles.chips}>{activeProjects.map(renderProjectChip)}</div>
                  </div>
                ) : null}
                {inactiveGroups.map((group) => (
                  <div key={group.status} className={styles.projectGroup}>
                    <h2 className={styles.projectGroupHeading}>{group.heading}</h2>
                    <div className={styles.chips}>{group.items.map(renderProjectChip)}</div>
                  </div>
                ))}
              </>
            )}
            {sanitizeStatus?.lastRunAt ? (
              <p className={styles.syncNote}>
                Maintainer activity status last checked {formatCheckedAt(sanitizeStatus.lastRunAt)}.
                {sanitizeStatus.nextRunAt ? (
                  <> Next check will happen {formatNextCheck(sanitizeStatus.nextRunAt)}.</>
                ) : null}
              </p>
            ) : null}
          </div>
        </div>

        {/* ── Edit actions ── */}
        {isEditing && editConfig ? (
          <div className={styles.editActions}>
            {editConfig.saveError ? (
              <span className={styles.saveError}>{editConfig.saveError}</span>
            ) : null}
            <div className={styles.editButtons}>
              <button
                className={styles.cancelButton}
                type="button"
                onClick={editConfig.onCancel}
                disabled={editConfig.saveStatus === "saving"}
              >
                Cancel
              </button>
              <button
                className={styles.saveButton}
                type="button"
                onClick={editConfig.onSave}
                disabled={!editConfig.isDirty || editConfig.saveStatus === "saving"}
              >
                {editConfig.saveStatus === "saving" ? "Saving…" : "Save changes"}
              </button>
            </div>
          </div>
        ) : (
          /* ── Footer (view mode) ── */
          ago ? (
            <p className={styles.footer}>
              Updated {ago}{updatedBy ? <> · <span className={styles.footerBy}>{updatedBy}</span></> : null}
            </p>
          ) : null
        )}
      </div>
    </Card>
  );
}
