"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import ProjectsList from "./ProjectsList";
import styles from "./ProjectCreateForm.module.css";

type OnboardingIssue = {
  number: number;
  title: string;
  url: string;
  projectName?: string;
};

type ProjectCreateFormProps = {
  apiBaseUrl: string;
  onCancel: () => void;
  onCreated?: (projectId: number) => void;
};

const maturityOptions = ["Sandbox", "Incubating", "Graduated", "Archived"] as const;

const onboardingIssuePattern = /^https:\/\/github\.com\/cncf\/sandbox\/issues\/\d+$/i;

const parseGitHubOrgFromURL = (raw: string): string => {
  try {
    const parsed = new URL(raw);
    if (
      parsed.host !== "github.com" &&
      parsed.host !== "www.github.com" &&
      parsed.host !== "raw.githubusercontent.com"
    ) {
      return "";
    }
    const parts = parsed.pathname.replace(/^\/+/, "").split("/");
    if (parts.length < 2) {
      return "";
    }
    return parts[0];
  } catch {
    return "";
  }
};

export default function ProjectCreateForm({
  apiBaseUrl,
  onCancel,
  onCreated,
}: ProjectCreateFormProps) {
  const [onboardingQuery, setOnboardingQuery] = useState("");
  const [onboardingIssue, setOnboardingIssue] = useState("");
  const [projectName, setProjectName] = useState("");
  const [resolveError, setResolveError] = useState<string | null>(null);
  const [isResolving, setIsResolving] = useState(false);
  const [issuesStatus, setIssuesStatus] = useState<"idle" | "loading" | "ready">("idle");
  const [issuesError, setIssuesError] = useState<string | null>(null);
  const [issuesEmpty, setIssuesEmpty] = useState(false);
  const [issueList, setIssueList] = useState<OnboardingIssue[]>([]);
  const [issueSuggestions, setIssueSuggestions] = useState<OnboardingIssue[]>([]);
  const [issueHighlightIndex, setIssueHighlightIndex] = useState(-1);
  const [issuePopupOpen, setIssuePopupOpen] = useState(false);
  const [legacyRef, setLegacyRef] = useState("");
  const [dotProjectRef, setDotProjectRef] = useState("");
  const [maturity, setMaturity] = useState<(typeof maturityOptions)[number]>("Sandbox");
  const [githubOrg, setGitHubOrg] = useState("");
  const [isSaving, setIsSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const lastResolvedIssueRef = useRef<string>("");

  const derivedOrg = useMemo(() => {
    const legacyOrg = parseGitHubOrgFromURL(legacyRef.trim());
    const dotOrg = parseGitHubOrgFromURL(dotProjectRef.trim());
    if (legacyOrg && dotOrg && legacyOrg.toLowerCase() !== dotOrg.toLowerCase()) {
      return "";
    }
    return legacyOrg || dotOrg;
  }, [legacyRef, dotProjectRef]);

  const orgMismatch =
    legacyRef.trim() !== "" &&
    dotProjectRef.trim() !== "" &&
    parseGitHubOrgFromURL(legacyRef.trim()).toLowerCase() !==
      parseGitHubOrgFromURL(dotProjectRef.trim()).toLowerCase();

  useEffect(() => {
    setGitHubOrg(derivedOrg);
  }, [derivedOrg]);

  const loadIssues = useCallback(async (signal?: AbortSignal, attempt = 0) => {
    setIssuesStatus("loading");
    setIssuesError(null);
    try {
      const response = await fetch(`${apiBaseUrl}/onboarding/issues`, {
        credentials: "include",
        signal,
      });
      if (response.status === 401 && attempt < 2) {
        await new Promise((resolve) => setTimeout(resolve, 300));
        await loadIssues(signal, attempt + 1);
        return;
      }
      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || `unexpected status ${response.status}`);
      }
      const data = (await response.json()) as { issues?: OnboardingIssue[] };
      const nextIssues = data.issues || [];
      setIssueList(nextIssues);
      setIssuesEmpty(nextIssues.length === 0);
      setIssuesError(null);
      setIssuesStatus("ready");
    } catch (err) {
      if ((err as Error).name === "AbortError") {
        return;
      }
      setIssuesStatus("ready");
      setIssuesEmpty(false);
      setIssuesError((err as Error).message || "Unable to load onboarding issues.");
    }
  }, [apiBaseUrl]);

  useEffect(() => {
    const controller = new AbortController();
    void loadIssues(controller.signal);
    return () => {
      controller.abort();
    };
  }, [loadIssues]);

  useEffect(() => {
    const query = onboardingQuery.trim();
    if (query === "") {
      if (issuePopupOpen) {
        const fallback = issueList.slice(0, 20);
        setIssueSuggestions(fallback);
        setIssueHighlightIndex(fallback.length > 0 ? 0 : -1);
      } else {
        setIssueSuggestions([]);
        setIssueHighlightIndex(-1);
      }
      return;
    }
    if (onboardingIssuePattern.test(query)) {
      setOnboardingIssue(query);
      setIssueSuggestions([]);
      setIssueHighlightIndex(-1);
      const matched = issueList.find(
        (issue) => issue.url.toLowerCase() === query.toLowerCase()
      );
      if (matched?.projectName) {
        lastResolvedIssueRef.current = query;
        setProjectName(matched.projectName);
        setResolveError(null);
      }
      return;
    }
    setOnboardingIssue("");
    const queryLower = query.toLowerCase();
    const filtered = issueList.filter((issue) => {
      const titleMatch = issue.title.toLowerCase().includes(queryLower);
      const projectMatch = (issue.projectName || "").toLowerCase().includes(queryLower);
      const numberMatch = String(issue.number).includes(queryLower);
      return titleMatch || projectMatch || numberMatch;
    });
    setIssueSuggestions(filtered.slice(0, 15));
    setIssueHighlightIndex(filtered.length > 0 ? 0 : -1);
  }, [issueList, onboardingQuery, issuePopupOpen]);

  useEffect(() => {
    const trimmed = onboardingIssue.trim();
    if (!onboardingIssuePattern.test(trimmed)) {
      if (onboardingQuery.trim() !== "") {
        setResolveError("Select a cncf/sandbox onboarding issue.");
      } else {
        setResolveError(null);
      }
      if (onboardingIssue === "") {
        setProjectName("");
      }
      return;
    }
    if (lastResolvedIssueRef.current === trimmed) {
      return;
    }
    const controller = new AbortController();
    const handle = window.setTimeout(async () => {
      setIsResolving(true);
      setResolveError(null);
      try {
        const response = await fetch(`${apiBaseUrl}/onboarding/resolve`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          credentials: "include",
          body: JSON.stringify({ issueUrl: trimmed }),
          signal: controller.signal,
        });
        if (!response.ok) {
          const text = await response.text();
          throw new Error(text || "Failed to resolve onboarding issue.");
        }
        const data = (await response.json()) as { title?: string; projectName?: string };
        const resolvedName = (data.projectName || "").trim();
        if (!resolvedName) {
          throw new Error("Unable to extract project name from issue title.");
        }
        lastResolvedIssueRef.current = trimmed;
        setProjectName(resolvedName);
      } catch (err) {
        if ((err as Error).name === "AbortError") {
          return;
        }
        setResolveError((err as Error).message || "Failed to resolve onboarding issue.");
        setProjectName("");
      } finally {
        setIsResolving(false);
      }
    }, 350);
    return () => {
      controller.abort();
      window.clearTimeout(handle);
    };
  }, [apiBaseUrl, onboardingIssue, onboardingQuery]);

  const handleIssueSelect = (issue: OnboardingIssue) => {
    setOnboardingQuery(issue.url);
    setOnboardingIssue(issue.url);
    setIssueSuggestions([]);
    setIssueHighlightIndex(-1);
    setIssuePopupOpen(false);
    if (issue.projectName) {
      lastResolvedIssueRef.current = issue.url;
      setProjectName(issue.projectName);
    } else {
      lastResolvedIssueRef.current = "";
      setProjectName("");
    }
    setResolveError(null);
  };

  const handleIssueKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "ArrowDown" && !issuePopupOpen) {
      event.preventDefault();
      setIssuePopupOpen(true);
      const fallback = issueList.slice(0, 20);
      setIssueSuggestions(fallback);
      setIssueHighlightIndex(fallback.length > 0 ? 0 : -1);
      return;
    }
    if (issueSuggestions.length === 0) {
      return;
    }
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setIssueHighlightIndex((current) => {
        const next = current + 1;
        return next >= issueSuggestions.length ? 0 : next;
      });
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setIssueHighlightIndex((current) => {
        const next = current - 1;
        return next < 0 ? issueSuggestions.length - 1 : next;
      });
    } else if (event.key === "Enter") {
      event.preventDefault();
      const selected =
        issueHighlightIndex >= 0 ? issueSuggestions[issueHighlightIndex] : null;
      if (selected) {
        handleIssueSelect(selected);
      }
    }
  };

  const handleSubmit = async () => {
    setSaveError(null);
    if (!onboardingIssuePattern.test(onboardingIssue.trim())) {
      setSaveError("Onboarding issue URL is required.");
      return;
    }
    if (projectName.trim() === "") {
      setSaveError("Project name could not be resolved from the onboarding issue.");
      return;
    }
    if (!legacyRef.trim() && !dotProjectRef.trim()) {
      setSaveError("Provide a Legacy Maintainer File or Dot-Project Maintainer File URL.");
      return;
    }
    if (orgMismatch || githubOrg.trim() === "") {
      setSaveError("GitHub Org could not be inferred from the file URLs.");
      return;
    }
    setIsSaving(true);
    try {
      const payload = {
        onboardingIssue: onboardingIssue.trim(),
        projectName: projectName.trim(),
        githubOrg: githubOrg.trim(),
        legacyMaintainerRef: legacyRef.trim() || undefined,
        dotProjectMaintainerRef: dotProjectRef.trim() || undefined,
        maturity,
      };
      const response = await fetch(`${apiBaseUrl}/projects`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify(payload),
      });
      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || "Failed to create project.");
      }
      const data = (await response.json()) as { id?: number };
      if (onCreated && typeof data.id === "number") {
        onCreated(data.id);
      }
      await loadIssues();
    } catch (err) {
      setSaveError((err as Error).message || "Failed to create project.");
    } finally {
      setIsSaving(false);
    }
  };

  if (issuesEmpty) {
    return (
      <div className={styles.emptyState}>
        <div className={styles.form}>
          <div className={styles.info}>
            All open cncf/sandbox issues labelled{" "}
            <a
              className={styles.ghLabelLink}
              href="https://github.com/cncf/sandbox/issues?q=state%3Aopen%20label%3A%22project%20onboarding%22"
              target="_blank"
              rel="noreferrer"
            >
              <span className={styles.ghLabel}>project onboarding</span>
            </a>{" "}
            are present in the database.
          </div>
        </div>
        <ProjectsList />
      </div>
    );
  }

  return (
    <div className={styles.form}>
      <label className={styles.field}>
        <span>Onboarding Issue</span>
        <div className={styles.fieldWrap}>
          <input
            value={onboardingQuery}
            onChange={(event) => {
              setOnboardingQuery(event.target.value);
              setIssuePopupOpen(true);
            }}
            onFocus={() => {
              setIssuePopupOpen(true);
              if (onboardingQuery.trim() === "") {
                const fallback = issueList.slice(0, 20);
                setIssueSuggestions(fallback);
                setIssueHighlightIndex(fallback.length > 0 ? 0 : -1);
              }
            }}
            onKeyDown={handleIssueKeyDown}
            placeholder="Search open onboarding issues or paste URL"
          />
          <a
            className={styles.issueLinkButton}
            href={onboardingIssuePattern.test(onboardingIssue.trim()) ? onboardingIssue : undefined}
            target="_blank"
            rel="noreferrer"
            aria-disabled={!onboardingIssuePattern.test(onboardingIssue.trim())}
            title="Open onboarding issue"
            onClick={(event) => {
              if (!onboardingIssuePattern.test(onboardingIssue.trim())) {
                event.preventDefault();
              }
            }}
          >
            Open this Onboarding Issue in a new tab
          </a>
          {issuesStatus === "loading" ? (
            <div className={styles.helper}>Loading onboarding issues…</div>
          ) : null}
          {issuesError ? <div className={styles.error}>{issuesError}</div> : null}
          {issuePopupOpen && !onboardingIssuePattern.test(onboardingQuery.trim()) ? (
            <div className={styles.issuePopup} role="listbox">
              <div className={styles.issueList}>
                {issueSuggestions.length > 0 ? (
                  issueSuggestions.map((issue, index) => (
                    <button
                      key={issue.url}
                      type="button"
                      className={`${styles.issueOption} ${
                        index === issueHighlightIndex ? styles.issueOptionActive : ""
                      }`}
                      onClick={() => handleIssueSelect(issue)}
                      role="option"
                      aria-selected={index === issueHighlightIndex}
                    >
                      {issue.projectName ? (
                        <>
                          <span className={styles.issuePrefix}>[PROJECT ONBOARDING]</span>{" "}
                          <span className={styles.issueName}>{issue.projectName}</span>{" "}
                          <span className={styles.issueSuffix}>#{issue.number}</span>
                        </>
                      ) : (
                        <>
                          <span className={styles.issueTitleFallback}>{issue.title}</span>{" "}
                          <span className={styles.issueSuffix}>#{issue.number}</span>
                        </>
                      )}
                    </button>
                  ))
                ) : (
                  <div className={styles.issueEmpty}>No matching onboarding issues.</div>
                )}
              </div>
            </div>
          ) : null}
        </div>
      </label>
      {isResolving ? <div className={styles.helper}>Resolving issue title…</div> : null}
      {resolveError ? <div className={styles.error}>{resolveError}</div> : null}
      <input type="hidden" value={projectName} data-testid="project-name" readOnly />
      <input type="hidden" value={githubOrg} data-testid="github-org" readOnly />
      <label className={styles.field}>
        <span>Legacy Maintainer File</span>
        <input
          value={legacyRef}
          onChange={(event) => setLegacyRef(event.target.value)}
          placeholder="https://github.com/org/repo/path/OWNERS"
        />
      </label>
      <label className={styles.field}>
        <span>Dot-Project Maintainer File</span>
        <input
          value={dotProjectRef}
          onChange={(event) => setDotProjectRef(event.target.value)}
          placeholder="https://github.com/org/.project/blob/main/MAINTAINERS.yaml"
        />
      </label>
      {orgMismatch ? (
        <div className={styles.error}>
          Maintainer file URLs must point to the same GitHub org.
        </div>
      ) : null}
      <label className={styles.field}>
        <span>Project Maturity</span>
        <select
          value={maturity}
          onChange={(event) =>
            setMaturity(event.target.value as (typeof maturityOptions)[number])
          }
        >
          {maturityOptions.map((option) => (
            <option key={option} value={option}>
              {option}
            </option>
          ))}
        </select>
      </label>
      {saveError ? <div className={styles.error}>{saveError}</div> : null}
      <div className={styles.actions}>
        <button className={styles.cancelButton} type="button" onClick={onCancel}>
          Cancel
        </button>
        <button
          className={styles.submitButton}
          type="button"
          onClick={handleSubmit}
          disabled={isSaving}
        >
          {isSaving ? "Creating…" : "Create New Project"}
        </button>
      </div>
    </div>
  );
}
