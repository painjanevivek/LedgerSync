import { createHmac } from "node:crypto";

import { Redis } from "@upstash/redis";
import { NextResponse } from "next/server";

import { jsonError } from "@/lib/security";
import { emitGuardrailMetric } from "@/lib/guardrail-metrics";

export type RateLimitDecision = Readonly<{
  allowed: boolean;
  retryAfterSeconds: number;
  available?: boolean;
}>;

export interface RateLimitStore {
  consume(key: string, limit: number, windowSeconds: number): Promise<RateLimitDecision>;
}

export type RateLimitDenial = Readonly<{
  code: "rate_limited" | "temporary_unavailable";
  status: 429 | 503;
  retryAfterSeconds?: number;
}>;

export function rateLimitDenial(decision: RateLimitDecision): RateLimitDenial | null {
  if (decision.allowed) return null;
  if (decision.available === false) return { code: "temporary_unavailable", status: 503 };
  return { code: "rate_limited", status: 429, retryAfterSeconds: decision.retryAfterSeconds };
}

export function rateLimitResponse(decision: RateLimitDecision): NextResponse | null {
  const denial = rateLimitDenial(decision);
  if (!denial) return null;
  emitGuardrailMetric("rate_limit", denial.code === "rate_limited" ? "denied" : "unavailable");
  const response = jsonError(denial.code, denial.status);
  if (denial.retryAfterSeconds !== undefined) {
    response.headers.set("Retry-After", String(denial.retryAfterSeconds));
  }
  return response;
}

// Local development and tests deliberately use a process-local fixed window.
// Multi-instance deployments must use createRateLimitStore(), which selects the
// shared atomic implementation on Vercel and in production.
export class InMemoryRateLimitStore implements RateLimitStore {
  private readonly windows = new Map<string, { startedAt: number; count: number }>();

  async consume(key: string, limit: number, windowSeconds: number): Promise<RateLimitDecision> {
    const now = Date.now();
    const current = this.windows.get(key);
    const windowMilliseconds = windowSeconds * 1000;
    if (!current || now - current.startedAt >= windowMilliseconds) {
      this.windows.set(key, { startedAt: now, count: 1 });
      return { allowed: true, retryAfterSeconds: 0 };
    }
    if (current.count >= limit) {
      return { allowed: false, retryAfterSeconds: Math.max(1, Math.ceil((windowMilliseconds - (now - current.startedAt)) / 1000)) };
    }
    current.count += 1;
    return { allowed: true, retryAfterSeconds: 0 };
  }
}

type AtomicRateLimitCounter = (key: string, windowSeconds: number) => Promise<readonly [number, number]>;

const fixedWindowScript = `
local count = redis.call("INCR", KEYS[1])
local ttl = redis.call("TTL", KEYS[1])
if count == 1 or ttl < 0 then
  redis.call("EXPIRE", KEYS[1], tonumber(ARGV[1]))
  ttl = tonumber(ARGV[1])
end
return {count, ttl}
`;

function sharedCounter(): AtomicRateLimitCounter {
  return async (key, windowSeconds) => {
    // Construct per request so the abort signal has a fresh deadline. Redis is
    // REST-backed, so this does not create a persistent network connection.
    const redis = Redis.fromEnv({
      signal: AbortSignal.timeout(2_000),
      retry: { retries: 0 },
    });
    return redis.eval<[string], [number, number]>(
      fixedWindowScript,
      [key],
      [String(windowSeconds)],
    );
  };
}

function safeRateLimitKey(raw: string, namespace: string, secret: string): string {
  if (secret.length < 32) throw new Error("rate-limit key secret is unavailable");
  const digest = createHmac("sha256", secret).update(raw).digest("hex");
  return `ledgersync:${namespace}:bff-rate:${digest}`;
}

export class SharedRateLimitStore implements RateLimitStore {
  constructor(
    private readonly counter: AtomicRateLimitCounter = sharedCounter(),
    private readonly namespace = process.env.LEDGERSYNC_RATE_LIMIT_NAMESPACE?.trim() || "default",
    private readonly keySecret = process.env.LEDGERSYNC_WEB_SESSION_SECRET ?? "",
  ) {}

  async consume(key: string, limit: number, windowSeconds: number): Promise<RateLimitDecision> {
    if (!Number.isSafeInteger(limit) || limit < 1 || !Number.isSafeInteger(windowSeconds) || windowSeconds < 1) {
      return { allowed: false, retryAfterSeconds: 0, available: false };
    }
    try {
      const [count, ttl] = await this.counter(safeRateLimitKey(key, this.namespace, this.keySecret), windowSeconds);
      if (!Number.isSafeInteger(count) || count < 1 || !Number.isSafeInteger(ttl) || ttl < 0) {
        return { allowed: false, retryAfterSeconds: 0, available: false };
      }
      return count <= limit
        ? { allowed: true, retryAfterSeconds: 0 }
        : { allowed: false, retryAfterSeconds: Math.max(1, ttl) };
    } catch {
      // Missing or unavailable shared infrastructure fails closed. Callers map
      // this to 503 rather than misrepresenting infrastructure failure as 429.
      return { allowed: false, retryAfterSeconds: 0, available: false };
    }
  }
}

export function requiresSharedRateLimit(environment: Readonly<Record<string, string | undefined>> = process.env): boolean {
  const deployment = (environment.LEDGERSYNC_DEPLOYMENT_ENV ?? environment.NODE_ENV ?? "development").trim().toLowerCase();
  return deployment === "production" || deployment === "prod" || Boolean(environment.VERCEL);
}

let productionStore: SharedRateLimitStore | undefined;

export function createRateLimitStore(environment: Readonly<Record<string, string | undefined>> = process.env): RateLimitStore {
  if (!requiresSharedRateLimit(environment)) return new InMemoryRateLimitStore();
  productionStore ??= new SharedRateLimitStore();
  return productionStore;
}
