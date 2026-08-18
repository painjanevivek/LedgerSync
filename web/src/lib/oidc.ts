import { createHash, createHmac, randomBytes, timingSafeEqual } from "node:crypto";

import { createRemoteJWKSet, jwtVerify } from "jose";

const transactionCookieName = "ledgersync_oidc_transaction";
const transactionLifetimeMs = 10 * 60 * 1000;

type Discovery = Readonly<{ issuer: string; authorization_endpoint: string; token_endpoint: string; jwks_uri: string }>;
type Transaction = Readonly<{ state: string; codeVerifier: string; nonce: string; expiresAt: number }>;
export type VerifiedIdentity = Readonly<{ subjectId: string; tenantId: string; roles: string[]; scopes: string[]; expiresAt: number }>;

function configuration() {
  const issuer = process.env.LEDGERSYNC_OIDC_ISSUER_URL?.replace(/\/$/, "");
  const clientID = process.env.LEDGERSYNC_OIDC_CLIENT_ID;
  const redirectURI = process.env.LEDGERSYNC_OIDC_REDIRECT_URI;
  if (!issuer || !clientID || !redirectURI) throw new Error("OIDC issuer URL, client ID, and redirect URI are required");
  if (process.env.NODE_ENV === "production" && (!issuer.startsWith("https://") || !redirectURI.startsWith("https://"))) throw new Error("production OIDC URLs must use HTTPS");
  return { issuer, clientID, redirectURI, clientSecret: process.env.LEDGERSYNC_OIDC_CLIENT_SECRET };
}

function sessionSecret() {
  const value = process.env.LEDGERSYNC_WEB_SESSION_SECRET;
  if (!value || value.length < 32) throw new Error("LEDGERSYNC_WEB_SESSION_SECRET must be at least 32 characters");
  return value;
}

function signedTransaction(transaction: Transaction): string {
  const payload = Buffer.from(JSON.stringify(transaction)).toString("base64url");
  const signature = createHmac("sha256", sessionSecret()).update(payload).digest("base64url");
  return `${payload}.${signature}`;
}

export function readTransaction(raw: string | undefined): Transaction | null {
  if (!raw) return null;
  const [payload, signature] = raw.split(".");
  if (!payload || !signature) return null;
  const expected = createHmac("sha256", sessionSecret()).update(payload).digest();
  const supplied = Buffer.from(signature, "base64url");
  if (expected.length !== supplied.length || !timingSafeEqual(expected, supplied)) return null;
  try {
    const parsed = JSON.parse(Buffer.from(payload, "base64url").toString("utf8")) as Partial<Transaction>;
    if (typeof parsed.state !== "string" || typeof parsed.codeVerifier !== "string" || typeof parsed.nonce !== "string" || typeof parsed.expiresAt !== "number" || parsed.expiresAt <= Date.now()) return null;
    return parsed as Transaction;
  } catch { return null; }
}

function randomURLValue(bytes = 32) { return randomBytes(bytes).toString("base64url"); }
function challenge(verifier: string) { return createHash("sha256").update(verifier).digest("base64url"); }

async function discovery(): Promise<Discovery> {
  const { issuer } = configuration();
  const response = await fetch(`${issuer}/.well-known/openid-configuration`, { cache: "no-store", signal: AbortSignal.timeout(5_000) });
  if (!response.ok) throw new Error("OIDC discovery failed");
  const result = await response.json() as Partial<Discovery>;
  if (result.issuer !== issuer || !result.authorization_endpoint || !result.token_endpoint || !result.jwks_uri) throw new Error("OIDC discovery document is invalid");
  return result as Discovery;
}

export async function beginAuthorization(): Promise<{ authorizationURL: string; transactionCookie: string }> {
  const config = configuration();
  const metadata = await discovery();
  const transaction: Transaction = { state: randomURLValue(), codeVerifier: randomURLValue(48), nonce: randomURLValue(), expiresAt: Date.now() + transactionLifetimeMs };
  const url = new URL(metadata.authorization_endpoint);
  url.searchParams.set("response_type", "code");
  url.searchParams.set("client_id", config.clientID);
  url.searchParams.set("redirect_uri", config.redirectURI);
  url.searchParams.set("scope", "openid profile");
  url.searchParams.set("state", transaction.state);
  url.searchParams.set("nonce", transaction.nonce);
  url.searchParams.set("code_challenge", challenge(transaction.codeVerifier));
  url.searchParams.set("code_challenge_method", "S256");
  return { authorizationURL: url.toString(), transactionCookie: signedTransaction(transaction) };
}

export async function completeAuthorization(code: string, state: string, transactionCookie: string | undefined): Promise<VerifiedIdentity> {
  const transaction = readTransaction(transactionCookie);
  if (!transaction || state.length !== transaction.state.length || !timingSafeEqual(Buffer.from(state), Buffer.from(transaction.state))) throw new Error("OIDC state is invalid");
  const config = configuration();
  const metadata = await discovery();
  const parameters = new URLSearchParams({ grant_type: "authorization_code", code, redirect_uri: config.redirectURI, client_id: config.clientID, code_verifier: transaction.codeVerifier });
  const headers: HeadersInit = { "Content-Type": "application/x-www-form-urlencoded", Accept: "application/json" };
  if (config.clientSecret) headers.Authorization = `Basic ${Buffer.from(`${config.clientID}:${config.clientSecret}`).toString("base64")}`;
  const response = await fetch(metadata.token_endpoint, { method: "POST", headers, body: parameters, cache: "no-store", signal: AbortSignal.timeout(5_000) });
  if (!response.ok) throw new Error("OIDC code exchange failed");
  const token = await response.json() as { id_token?: string };
  if (!token.id_token) throw new Error("OIDC token response is missing id_token");
  const jwks = createRemoteJWKSet(new URL(metadata.jwks_uri), { timeoutDuration: 5_000 });
  const { payload, protectedHeader } = await jwtVerify(token.id_token, jwks, { issuer: config.issuer, audience: config.clientID, algorithms: ["RS256", "ES256", "PS256"] });
  if (protectedHeader.alg !== "RS256" && protectedHeader.alg !== "ES256" && protectedHeader.alg !== "PS256") throw new Error("OIDC signing algorithm is not allowed");
  if (payload.nonce !== transaction.nonce || typeof payload.sub !== "string" || typeof payload.tenant_id !== "string" || typeof payload.exp !== "number") throw new Error("OIDC identity claims are invalid");
  const roles = Array.isArray(payload.roles) ? payload.roles.filter((role): role is string => typeof role === "string" && role.length > 0 && role.length <= 64).slice(0, 16) : [];
  const scopes = typeof payload.scope === "string" ? payload.scope.split(/\s+/).filter((scope) => scope.length > 0 && scope.length <= 64).slice(0, 16) : [];
  return { subjectId: payload.sub, tenantId: payload.tenant_id, roles, scopes, expiresAt: Math.min(payload.exp * 1000, Date.now() + 30 * 60 * 1000) };
}

export { transactionCookieName };

export function transactionCookie(value: string) {
  return { name: transactionCookieName, value, httpOnly: true, sameSite: "lax" as const, secure: process.env.NODE_ENV === "production", path: "/api/auth", maxAge: transactionLifetimeMs / 1000 };
}
