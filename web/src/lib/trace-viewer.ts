export function traceViewerURL(traceId: unknown, configuredBase: string | undefined): URL | null {
  if (typeof traceId !== "string" || !/^[a-f0-9]{32}$/.test(traceId) || traceId === "0".repeat(32) || !configuredBase || configuredBase.length > 512) return null;
  try {
    const base = new URL(configuredBase);
    if (base.username || base.password || base.search || base.hash) return null;
    if (base.protocol !== "https:" && !(base.protocol === "http:" && ["localhost", "127.0.0.1", "[::1]"].includes(base.hostname))) return null;
    base.pathname = `${base.pathname.replace(/\/$/, "")}/trace/${traceId}`;
    return base;
  } catch { return null; }
}
