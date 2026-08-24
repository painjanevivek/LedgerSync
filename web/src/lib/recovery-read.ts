import { InMemoryRateLimitStore } from "@/lib/rate-limit";

export const recoveryReadRateLimit = new InMemoryRateLimitStore();
