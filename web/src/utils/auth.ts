"use client";

export function getAuthBaseUrl(bffBaseUrl?: string): string {
  const raw = bffBaseUrl ?? process.env.NEXT_PUBLIC_BFF_BASE_URL ?? "/api";
  const normalized = raw.replace(/\/+$/, "");
  if (normalized.endsWith("/api")) {
    const stripped = normalized.slice(0, -4);
    return stripped === "" ? "" : stripped;
  }
  return normalized;
}

export function buildAuthLoginHref(authBaseUrl: string, next: string): string {
  return `${authBaseUrl}/auth/login?next=${encodeURIComponent(next)}`;
}

export function buildCurrentRelativeUrl(pathname?: string | null): string {
  const basePath = pathname && pathname.trim() !== "" ? pathname : "/";
  if (typeof window === "undefined") {
    return basePath;
  }
  return `${basePath}${window.location.search}${window.location.hash}`;
}

export function redirectToAuthLogin(authBaseUrl: string, pathname?: string | null): void {
  if (typeof window === "undefined") {
    return;
  }
  window.location.assign(buildAuthLoginHref(authBaseUrl, buildCurrentRelativeUrl(pathname)));
}
