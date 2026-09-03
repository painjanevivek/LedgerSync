import { NextRequest, NextResponse } from "next/server";

import type { RateLimitStore } from "@/lib/rate-limit";
import { hasValidCSRF, hasValidHost, jsonError } from "@/lib/security";
import type { Session } from "@/lib/session";

const searchableScopes = new Set(["accounts:read", "transfers:read", "funding:read", "events:read", "reconciliation:read", "corrections:read"]);
const savedViewDomainScopes = new Set(["accounts:read", "transfers:read", "funding:read", "funding:approve", "corrections:read", "corrections:approve", "events:read", "webhooks:read"]);
const operatorRoles = new Set(["tenant:operator", "tenant:admin"]);
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/u;
const approvedReference = /^[A-Za-z0-9][A-Za-z0-9._:/-]{7,127}$/u;

export type InvestigationSearchAuthorization = Readonly<{ session: Session }>;

export async function authorizeInvestigationSearch(request: NextRequest, session: Session | null, limiter: RateLimitStore): Promise<InvestigationSearchAuthorization | NextResponse> {
	return authorizeInvestigationRead(request, session, limiter, "search");
}

export async function authorizeInvestigationRelationships(request: NextRequest, session: Session | null, limiter: RateLimitStore): Promise<InvestigationSearchAuthorization | NextResponse> {
	return authorizeInvestigationRead(request, session, limiter, "relationships");
}

export async function authorizeInvestigationSavedViews(request: NextRequest, session: Session | null, limiter: RateLimitStore, write: boolean): Promise<InvestigationSearchAuthorization | NextResponse> {
  const allowedMethods = write ? new Set(["POST", "PUT", "DELETE"]) : new Set(["GET"]);
  if (!allowedMethods.has(request.method)) {
    const response = jsonError("method_not_allowed", 405);
    response.headers.set("Allow", write ? "POST, PUT, DELETE" : "GET");
    return response;
  }
  if (!session) return jsonError("unauthorized", 401);
  if (!session.roles?.some((role) => operatorRoles.has(role)) || !session.scopes?.includes("investigation:read") || !session.scopes.some((scope) => savedViewDomainScopes.has(scope))) return jsonError("forbidden", 403);
  if (write && (!session.scopes.includes("investigation:write") || !hasValidCSRF(request, session))) return jsonError(session.scopes.includes("investigation:write") ? "csrf_failed" : "forbidden", 403);
  if (!hasValidHost(request)) return jsonError("invalid_request", 400);
  const boundary = write ? "saved-views-write" : "saved-views-read";
  const decision = await limiter.consume(`investigation:${boundary}:${session.tenantId}:${session.subjectId}`, write ? 15 : 30, 60);
  if (!decision.allowed) {
    const response = jsonError("rate_limited", 429);
    response.headers.set("Retry-After", String(decision.retryAfterSeconds));
    return response;
  }
  return { session };
}

export async function authorizeInvestigationWorkspaces(request: NextRequest, session: Session | null, limiter: RateLimitStore, write: boolean): Promise<InvestigationSearchAuthorization | NextResponse> {
  const expectedMethod = write ? "POST" : "GET";
  if (request.method !== expectedMethod) {
    const response = jsonError("method_not_allowed", 405);
    response.headers.set("Allow", expectedMethod);
    return response;
  }
  if (!session) return jsonError("unauthorized", 401);
  if (!session.roles?.some((role) => operatorRoles.has(role)) || !session.scopes?.includes("investigation:read") || !session.scopes.some((scope) => searchableScopes.has(scope))) return jsonError("forbidden", 403);
  if (write && (!session.scopes.includes("investigation:write") || !hasValidCSRF(request, session))) return jsonError(session.scopes.includes("investigation:write") ? "csrf_failed" : "forbidden", 403);
  if (!hasValidHost(request)) return jsonError("invalid_request", 400);
  const boundary = write ? "workspaces-write" : "workspaces-read";
  const decision = await limiter.consume(`investigation:${boundary}:${session.tenantId}:${session.subjectId}`, write ? 12 : 30, 60);
  if (!decision.allowed) {
    const response = jsonError("rate_limited", 429);
    response.headers.set("Retry-After", String(decision.retryAfterSeconds));
    return response;
  }
  return { session };
}

export async function authorizeInvestigationEvidenceBundle(request: NextRequest, session: Session | null, limiter: RateLimitStore): Promise<InvestigationSearchAuthorization | NextResponse> {
  if (request.method !== "POST") { const response = jsonError("method_not_allowed", 405); response.headers.set("Allow", "POST"); return response; }
  if (!session) return jsonError("unauthorized", 401);
  if (!session.roles?.some((role) => operatorRoles.has(role)) || !session.scopes?.includes("investigation:read") || !session.scopes.includes("exports:read") || !session.scopes.some((scope) => searchableScopes.has(scope))) return jsonError("forbidden", 403);
  if (!hasValidCSRF(request, session)) return jsonError("csrf_failed", 403);
  if (!hasValidHost(request)) return jsonError("invalid_request", 400);
  const decision = await limiter.consume(`investigation:evidence-bundle:${session.tenantId}:${session.subjectId}`, 10, 60);
  if (!decision.allowed) { const response = jsonError("rate_limited", 429); response.headers.set("Retry-After", String(decision.retryAfterSeconds)); return response; }
  return { session };
}

async function authorizeInvestigationRead(request: NextRequest, session: Session | null, limiter: RateLimitStore, boundary: "search" | "relationships"): Promise<InvestigationSearchAuthorization | NextResponse> {
  if (request.method !== "GET") {
    const response = jsonError("method_not_allowed", 405);
    response.headers.set("Allow", "GET");
    return response;
  }
  if (!session) return jsonError("unauthorized", 401);
  if (!session.roles?.some((role) => operatorRoles.has(role)) || !session.scopes?.includes("investigation:read") || !session.scopes.some((scope) => searchableScopes.has(scope))) return jsonError("forbidden", 403);
  if (!hasValidHost(request)) return jsonError("invalid_request", 400);
  const decision = await limiter.consume(`investigation:${boundary}:${session.tenantId}:${session.subjectId}`, 30, 60);
  if (!decision.allowed) {
    const response = jsonError("rate_limited", 429);
    response.headers.set("Retry-After", String(decision.retryAfterSeconds));
    return response;
  }
  return { session };
}

export function isInvestigationSearchDenial(value: InvestigationSearchAuthorization | NextResponse): value is NextResponse {
  return value instanceof NextResponse;
}

export function parseInvestigationSearchQuery(searchParams: URLSearchParams): Readonly<{ query: URLSearchParams; queryKind: "immutable_id" | "approved_reference" }> | null {
  const keys = [...new Set(searchParams.keys())];
  if (keys.some((key) => key !== "q" && key !== "limit") || keys.some((key) => searchParams.getAll(key).length !== 1)) return null;
  const raw = searchParams.get("q");
  if (raw === null || raw !== raw.trim() || raw.length > 128) return null;
  const normalized = raw.toLowerCase();
  const queryKind = uuid.test(normalized) ? "immutable_id" : approvedReference.test(raw) ? "approved_reference" : null;
  if (!queryKind) return null;
  const limit = searchParams.get("limit") ?? "10";
  if (!/^(?:[1-9]|1[0-9]|20)$/u.test(limit)) return null;
  const query = new URLSearchParams({ q: queryKind === "immutable_id" ? normalized : raw, limit });
  return { query, queryKind };
}
