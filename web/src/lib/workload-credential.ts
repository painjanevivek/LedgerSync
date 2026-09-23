import { readFile } from "node:fs/promises";

type TokenResponse = {
  access_token?: unknown;
  expires_in?: unknown;
  token_type?: unknown;
};

type CachedCredential = { token: string; refreshAt: number };

let cachedCredential: CachedCredential | undefined;
let refreshInFlight: Promise<CachedCredential> | undefined;

function productionRuntime(): boolean {
  return [process.env.LEDGERSYNC_ENV, process.env.LEDGERSYNC_DEPLOYMENT_ENV]
    .map((value) => value?.trim().toLowerCase())
    .some((value) => value === "production" || value === "prod");
}

function tokenExpiryMilliseconds(token: string): number | undefined {
  const parts = token.split(".");
  if (parts.length !== 3) return undefined;
  try {
    const claims = JSON.parse(Buffer.from(parts[1], "base64url").toString("utf8")) as { exp?: unknown };
    return typeof claims.exp === "number" && Number.isSafeInteger(claims.exp) ? claims.exp * 1000 : undefined;
  } catch {
    return undefined;
  }
}

function assertUsableToken(raw: string): string {
  const token = raw.trim();
  if (token.length < 16 || token.length > 16_384 || /\s/.test(token)) {
    throw new Error("private API workload credential is malformed");
  }
  const expiry = tokenExpiryMilliseconds(token);
  if (token.split(".").length === 3 && (!expiry || expiry <= Date.now() + 5_000)) {
    throw new Error("private API workload credential is expired or near expiry");
  }
  return token;
}

function requiredOAuthConfiguration() {
  const tokenURL = process.env.LEDGERSYNC_PRIVATE_API_TOKEN_URL?.trim();
  const clientID = process.env.LEDGERSYNC_PRIVATE_API_CLIENT_ID?.trim();
  const clientSecret = process.env.LEDGERSYNC_PRIVATE_API_CLIENT_SECRET?.trim();
  const audience = process.env.LEDGERSYNC_PRIVATE_API_AUDIENCE?.trim();
  const scope = process.env.LEDGERSYNC_PRIVATE_API_SCOPE?.trim();
  const configured = [tokenURL, clientID, clientSecret, audience, scope].some(Boolean);
  if (!configured) return undefined;
  if (!tokenURL || !clientID || !clientSecret || (!audience && !scope)) {
    throw new Error("private API OAuth configuration is incomplete");
  }
  let parsedURL: URL;
  try {
    parsedURL = new URL(tokenURL);
  } catch {
    throw new Error("private API token URL is invalid");
  }
  if (productionRuntime() && parsedURL.protocol !== "https:") {
    throw new Error("private API token URL must use HTTPS in production");
  }
  if (parsedURL.username || parsedURL.password) {
    throw new Error("private API token URL must not contain credentials");
  }
  return { tokenURL: parsedURL.toString(), clientID, clientSecret, audience, scope };
}

async function requestOAuthCredential(): Promise<CachedCredential> {
  const configuration = requiredOAuthConfiguration();
  if (!configuration) throw new Error("private API OAuth configuration is unavailable");
  const body = new URLSearchParams({
    grant_type: "client_credentials",
    client_id: configuration.clientID,
    client_secret: configuration.clientSecret,
  });
  if (configuration.audience) body.set("audience", configuration.audience);
  if (configuration.scope) body.set("scope", configuration.scope);

  const response = await fetch(configuration.tokenURL, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded", Accept: "application/json" },
    body,
    cache: "no-store",
    signal: AbortSignal.timeout(5_000),
  });
  if (!response.ok) throw new Error("private API workload credential request failed");
  const rawPayload = await response.text();
  if (Buffer.byteLength(rawPayload, "utf8") > 64 * 1024) {
    throw new Error("private API workload credential response is too large");
  }
  let payload: TokenResponse;
  try {
    payload = JSON.parse(rawPayload) as TokenResponse;
  } catch {
    throw new Error("private API workload credential response is malformed");
  }
  if (typeof payload.access_token !== "string" || (payload.token_type !== undefined && String(payload.token_type).toLowerCase() !== "bearer")) {
    throw new Error("private API workload credential response is malformed");
  }
  const token = assertUsableToken(payload.access_token);
  const jwtExpiry = tokenExpiryMilliseconds(token);
  const expiresIn = typeof payload.expires_in === "number" ? payload.expires_in : Number(payload.expires_in);
  const responseExpiry = Number.isFinite(expiresIn) && expiresIn > 0 ? Date.now() + expiresIn * 1_000 : undefined;
  const expiresAt = jwtExpiry ?? responseExpiry;
  if (!expiresAt || expiresAt <= Date.now() + 10_000) {
    throw new Error("private API workload credential lifetime is unavailable or too short");
  }
  return { token, refreshAt: expiresAt - Math.min(60_000, Math.max(10_000, (expiresAt - Date.now()) / 10)) };
}

async function managedOAuthCredential(): Promise<string> {
  if (cachedCredential && cachedCredential.refreshAt > Date.now()) return cachedCredential.token;
  refreshInFlight ??= requestOAuthCredential().finally(() => {
    refreshInFlight = undefined;
  });
  cachedCredential = await refreshInFlight;
  return cachedCredential.token;
}

// Containers may continue using an atomically renewed token file. Vercel uses
// OAuth client credentials to obtain and cache a short-lived workload token.
export async function getPrivateAPIWorkloadCredential(): Promise<string> {
  const tokenFile = process.env.LEDGERSYNC_PRIVATE_API_TOKEN_FILE?.trim();
  if (tokenFile) return assertUsableToken(await readFile(tokenFile, "utf8"));

  if (requiredOAuthConfiguration()) return managedOAuthCredential();

  const staticToken = process.env.LEDGERSYNC_PRIVATE_API_TOKEN?.trim();
  if (productionRuntime()) {
    throw new Error("production requires private API OAuth client credentials or managed renewal token file");
  }
  if (!staticToken) throw new Error("private API workload credential is unavailable");
  return assertUsableToken(staticToken);
}
