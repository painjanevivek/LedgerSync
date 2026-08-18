export type RateLimitDecision = Readonly<{ allowed: boolean; retryAfterSeconds: number }>;

export interface RateLimitStore {
  consume(key: string, limit: number, windowSeconds: number): Promise<RateLimitDecision>;
}

// Development-only fallback. Production must bind this interface to an atomic,
// shared Redis implementation before public BFF mutations are enabled.
export class InMemoryRateLimitStore implements RateLimitStore {
  private readonly windows = new Map<string, { startedAt: number; count: number }>();

  async consume(key: string, limit: number, windowSeconds: number): Promise<RateLimitDecision> {
    const now = Date.now();
    const current = this.windows.get(key);
    const windowMilliseconds = windowSeconds * 1000;
    if (!current || now-current.startedAt >= windowMilliseconds) {
      this.windows.set(key, { startedAt: now, count: 1 });
      return { allowed: true, retryAfterSeconds: 0 };
    }
    if (current.count >= limit) {
      return { allowed: false, retryAfterSeconds: Math.max(1, Math.ceil((windowMilliseconds - (now-current.startedAt)) / 1000)) };
    }
    current.count += 1;
    return { allowed: true, retryAfterSeconds: 0 };
  }
}
