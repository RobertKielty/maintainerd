"use client";

import AppShell from "@/components/AppShell";
import type { AppShellViewer } from "@/components/AppShell";
import ProjectsList from "@/components/ProjectsList";
import { buildAuthLoginHref, getAuthBaseUrl } from "@/utils/auth";
import { usePathname } from "next/navigation";
import { useMemo } from "react";
import styles from "./page.module.css";

export default function Home() {
  const authBaseUrl = useMemo(() => {
    return getAuthBaseUrl();
  }, []);

  return (
    <AppShell>
      {(viewer) => <HomeContent viewer={viewer} authBaseUrl={authBaseUrl} />}
    </AppShell>
  );
}

type HomeContentProps = {
  viewer: AppShellViewer;
  authBaseUrl: string;
};

function HomeContent({ viewer, authBaseUrl }: HomeContentProps) {
  const pathname = usePathname();

  return (
    <main className={styles.main}>
      {viewer ? (
        <ProjectsList />
      ) : (
        <section className={styles.hero}>
          <div className={styles.heroPanel}>
            <p className={styles.eyebrow}>Maintainer Intelligence</p>
            <h1 className={styles.heroTitle}>CNCF Staff sign-in using GitHub</h1>
            <p className={styles.heroText}>
              maintainer-d helps authorized staff and maintainers review project ownership,
              roster files, and onboarding status. Sign in to search the directory and view
              the maintainer table.
            </p>
            <a
              className={styles.loginButton}
              href={buildAuthLoginHref(authBaseUrl, pathname || "/")}
            >
              Sign in with GitHub
            </a>
          </div>
        </section>
      )}
    </main>
  );
}
