const allowedLoopbackHosts = new Set(["127.0.0.1", "localhost", "[::1]", "::1"]);

export function parseSystemWebURL(raw: string): string {
  if (!raw || raw !== raw.trim()) throw new Error("LEDGERSYNC_SYSTEM_WEB_URL must be an exact absolute URL");
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw new Error("LEDGERSYNC_SYSTEM_WEB_URL must be an exact absolute URL");
  }
  if (url.protocol !== "http:" || !allowedLoopbackHosts.has(url.hostname.toLowerCase())) {
    throw new Error("LEDGERSYNC_SYSTEM_WEB_URL must use HTTP on an exact loopback hostname");
  }
  if (url.username || url.password || url.hash || url.search || url.pathname !== "/") {
    throw new Error("LEDGERSYNC_SYSTEM_WEB_URL cannot contain credentials, path, query, or fragment data");
  }
  if (url.port !== "3000") {
    throw new Error("LEDGERSYNC_SYSTEM_WEB_URL must use the Compose web port 3000");
  }
  return url.origin;
}

export function parseIsolatedComposeProject(raw: string): string {
  if (!/^[a-z0-9][a-z0-9_-]{0,62}$/.test(raw)) {
    throw new Error("LEDGERSYNC_SYSTEM_COMPOSE_PROJECT must be an exact lowercase Compose project name");
  }
  if (raw === "compose" || raw === "ledgersync") {
    throw new Error("the normal LedgerSync Compose project cannot be used for mutating system tests");
  }
  return raw;
}
