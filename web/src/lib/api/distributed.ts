export type DistributedObservation = Readonly<{ state: "available" | "unavailable" | "historical" | "active" | "expired" | "not_observed"; sampled_at: string; count?: string; owner_id?: string; generation?: string; expires_at?: string; latest_published_at?: string }>;
export type DistributedEvidence = Readonly<{ api_replicas: DistributedObservation; worker_publications: DistributedObservation; scheduler: DistributedObservation }>;
function record(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function observation(value: unknown, kind: keyof DistributedEvidence): value is DistributedObservation {
  if (!record(value)) return false;
  const allowed = ["state", "sampled_at", ...(kind === "api_replicas" ? ["count"] : kind === "worker_publications" ? ["count", "latest_published_at"] : ["owner_id", "generation", "expires_at"])];
  const states = kind === "api_replicas" ? ["available", "unavailable"] : kind === "worker_publications" ? ["historical", "unavailable"] : ["active", "expired", "not_observed", "unavailable"];
  const date = (item: unknown) => typeof item === "string" && item.length <= 64 && /^\d{4}-\d\d-\d\dT/.test(item) && Number.isFinite(Date.parse(item));
  const count = (item: unknown) => typeof item === "string" && /^(0|[1-9][0-9]{0,18})$/.test(item) && BigInt(item) <= 9223372036854775807n;
  if (Object.keys(value).some((key) => !allowed.includes(key)) || !states.includes(String(value.state)) || !date(value.sampled_at)) return false;
  if (value.state === "unavailable" || value.state === "not_observed") return Object.keys(value).length === 2;
  if (kind === "scheduler") return typeof value.owner_id === "string" && /^[A-Za-z0-9/_.:-]{1,128}$/.test(value.owner_id) && count(value.generation) && value.generation !== "0" && date(value.expires_at);
  return count(value.count) && (kind !== "api_replicas" || BigInt(value.count as string) <= 16n) && (value.latest_published_at === undefined || date(value.latest_published_at));
}
export function sanitizeDistributedEvidence(status: number, value: unknown) {
  const keys: (keyof DistributedEvidence)[] = ["api_replicas", "worker_publications", "scheduler"];
  if (status !== 200 || !record(value) || Object.keys(value).length !== 3 || !keys.every((key) => observation(value[key], key))) return { status: 503, body: { error: { code: "evidence_unavailable" } } };
  return { status: 200, body: Object.fromEntries(keys.map((key) => [key, { ...(value[key] as Record<string, unknown>) }])) };
}
