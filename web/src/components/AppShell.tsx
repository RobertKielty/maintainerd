"use client";

import { ReactNode, useEffect, useMemo, useRef, useState } from "react";
import { Dropdown } from "clo-ui/components/Dropdown";
import { Footer } from "clo-ui/components/Footer";
import { Navbar } from "clo-ui/components/Navbar";
import { Searchbar } from "clo-ui/components/Searchbar";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { getAuthBaseUrl, redirectToAuthLogin } from "@/utils/auth";
import { useTheme } from "./ThemeProvider";
import styles from "./AppShell.module.css";

export type AppShellViewer = {
  login: string;
  role: string;
  maintainerId?: number;
  capabilities?: string[];
} | null;

type AppShellProps = {
  children: ReactNode | ((viewer: AppShellViewer) => ReactNode);
  navCenterClassName?: string;
};

const MoonIcon = () => (
  <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
    <path
      d="M21 14.5A8.5 8.5 0 0 1 9.5 3a7 7 0 1 0 11.5 11.5Z"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    />
  </svg>
);

const SunIcon = () => (
  <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
    <circle cx="12" cy="12" r="4" fill="none" stroke="currentColor" strokeWidth="1.6" />
    <path
      d="M12 2v2M12 20v2M4.2 4.2l1.4 1.4M18.4 18.4l1.4 1.4M2 12h2M20 12h2M4.2 19.8l1.4-1.4M18.4 5.6l1.4-1.4"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
    />
  </svg>
);

const LFX_ENRICHMENT_CAPABILITY = "lfx:enrichment:run";

export default function AppShell({
  children,
  navCenterClassName,
}: AppShellProps) {
  const [me, setMe] = useState<AppShellViewer>(null);
  const { theme, toggleTheme } = useTheme();
  const devLoginAttemptedRef = useRef(false);
  const authRedirectAttemptedRef = useRef(false);
  const [mounted, setMounted] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const pathname = usePathname();
  const router = useRouter();

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

  const authBaseUrl = useMemo(() => {
    return getAuthBaseUrl(bffBaseUrl);
  }, [bffBaseUrl]);

  useEffect(() => {
    let alive = true;
    const devLogin = (process.env.NEXT_PUBLIC_DEV_AUTH_LOGIN || "").trim();
    const loadMe = async () => {
      try {
        const response = await fetch(`${apiBaseUrl}/me`, {
          credentials: "include",
        });
        if (!response.ok) {
          if (response.status === 401) {
            if (
              devLogin &&
              !devLoginAttemptedRef.current &&
              typeof window !== "undefined"
            ) {
              devLoginAttemptedRef.current = true;
              try {
                const testLogin = await fetch(
                  `${authBaseUrl}/auth/test-login?login=${encodeURIComponent(
                    devLogin
                  )}`,
                  { credentials: "include" }
                );
                if (testLogin.ok) {
                  await loadMe();
                  return;
                }
              } catch {
                // Ignore dev login failures; fall back to unauthenticated state.
              }
            }
            if (alive) {
              setMe(null);
            }
            if (
              pathname &&
              pathname !== "/" &&
              typeof window !== "undefined" &&
              !authRedirectAttemptedRef.current
            ) {
              authRedirectAttemptedRef.current = true;
              redirectToAuthLogin(authBaseUrl, pathname);
            }
            return;
          }
          throw new Error(`unexpected status ${response.status}`);
        }
        const data = (await response.json()) as NonNullable<AppShellViewer>;
        if (alive) {
          setMe(data);
        }
      } catch {
        if (alive) {
          setMe(null);
        }
      } finally {
      }
    };
    void loadMe();
    return () => {
      alive = false;
    };
  }, [apiBaseUrl, authBaseUrl, pathname]);

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    const nextQuery = new URLSearchParams(window.location.search).get("query") || "";
    setSearchQuery((current) => (current === nextQuery ? current : nextQuery));
  }, [pathname]);

  const handleLogout = async () => {
    try {
      await fetch(`${authBaseUrl}/auth/logout`, {
        method: "POST",
        credentials: "include",
      });
    } finally {
      setMe(null);
    }
  };

  const handleSearch = () => {
    const nextQuery = searchQuery.trim();
    setSearchQuery(nextQuery);
    const params = new URLSearchParams();
    if (nextQuery) {
      params.set("query", nextQuery);
    }
    const qs = params.toString();
    router.push(qs ? `/search?${qs}` : "/search");
  };

  const userLabel = me ? `${me.login} · ${me.role}` : "";
  const canRunLFXEnrichment = Boolean(me?.capabilities?.includes(LFX_ENRICHMENT_CAPABILITY));
  const content = typeof children === "function" ? children(me) : children;

  return (
    <div className={styles.page}>
      <Navbar navbarClassname={styles.navbar}>
        <div className={styles.navContent}>
          <div className={styles.brandWrap}>
            <Link className={styles.brand} href="/">
              maintainer-d
            </Link>
            <div className={styles.alpha}>Alpha</div>
          </div>
          <div className={`${styles.navCenter} ${navCenterClassName ?? ""}`}>
          </div>
          <div className={styles.userArea}>
            <button
              className={styles.themeToggle}
              type="button"
              onClick={toggleTheme}
              aria-label={
                mounted
                  ? theme === "light"
                    ? "Enable dark mode"
                    : "Enable light mode"
                  : "Toggle theme"
              }
              suppressHydrationWarning
            >
              {mounted ? (theme === "light" ? <MoonIcon /> : <SunIcon />) : <MoonIcon />}
            </button>
            {me?.role === "staff" ? (
              <Link className={styles.createProjectButton} href="/create/project">
                New Project…
              </Link>
            ) : null}
            {me?.role === "staff" ? (
              <Link className={styles.auditButton} href="/audit">
                AUDIT LOGS
              </Link>
            ) : null}
            {canRunLFXEnrichment ? (
              <Link className={styles.auditButton} href="/lfx">
                LFX
              </Link>
            ) : null}
            {me ? (
              <Dropdown
                label="User menu"
                btnContent={<div className={styles.userPill}>{userLabel}</div>}
                btnClassName={styles.dropdownButton}
                dropdownClassName={styles.dropdownMenu}
              >
                <UserMenu
                  onLogout={handleLogout}
                  role={me.role}
                  maintainerId={me.maintainerId}
                />
              </Dropdown>
            ) : null}
          </div>
        </div>
      </Navbar>
      {me ? (
        <div className={styles.searchBand}>
          <div className={styles.searchBandInner}>
            <Searchbar
              placeholder="Search projects, maintainers, companies, or roster URLs"
              value={searchQuery}
              onValueChange={setSearchQuery}
              onSearch={handleSearch}
              cleanSearchValue={() => setSearchQuery("")}
              bigSize={false}
              classNameWrapper={styles.searchbar}
            />
          </div>
        </div>
      ) : null}
      <div className={`${styles.content} ${me ? styles.contentWithSearch : ""}`}>{content}</div>
      <Footer className={styles.footer} logo={<span className={styles.footerLogo}>maintainer-d</span>}>
        <div className={styles.footerCol}>
          <div className="h6 fw-bold text-uppercase">Project</div>
          <div className="d-flex flex-column text-start">
            <a
              className="mb-1 opacity-75"
              href="https://github.com/cncf/maintainer-d#readme"
              target="_blank"
              rel="noreferrer"
            >
              Documentation
            </a>
          </div>
        </div>

        <div className={styles.footerCol}>
          <div className="h6 fw-bold text-uppercase">Community</div>
          <div className="d-flex flex-column text-start">
            <a
              className="mb-1 opacity-75"
              href="https://github.com/cncf/maintainer-d"
              target="_blank"
              rel="noreferrer"
            >
              GitHub
            </a>
          </div>
        </div>

        <div className={styles.footerCol}>
          <div className="h6 fw-bold text-uppercase">About</div>
          <div className="opacity-75 d-flex flex-column">
            maintainer-d is an{" "}
            <b className="d-inline-block">Open Source</b> project licensed under
            the{" "}
            <a
              className="d-inline-block mb-1"
              href="https://www.apache.org/licenses/LICENSE-2.0"
              target="_blank"
              rel="noreferrer"
            >
              Apache License 2.0
            </a>
          </div>
        </div>
      </Footer>
    </div>
  );
}

type UserMenuProps = {
  onLogout: () => void;
  role: string;
  maintainerId?: number;
  closeDropdown?: () => void;
  isVisibleDropdown?: boolean;
};

function UserMenu({ onLogout, closeDropdown, role, maintainerId }: UserMenuProps) {
  const handleClick = () => {
    onLogout();
    closeDropdown?.();
  };

  return (
    <div className={styles.dropdownContent}>
      {role === "maintainer" && maintainerId ? (
        <Link
          className={styles.dropdownItem}
          href={`/maintainers/${maintainerId}`}
          onClick={() => closeDropdown?.()}
        >
          Manage my details
        </Link>
      ) : null}
      <button className={styles.dropdownItem} onClick={handleClick} type="button">
        Sign out
      </button>
    </div>
  );
}
