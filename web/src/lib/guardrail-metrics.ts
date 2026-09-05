export type GuardrailKind =
  | "session"
  | "rate_limit"
  | "step_up"
  | "unavailable_action";

const allowedOutcomes: Readonly<Record<GuardrailKind, ReadonlySet<string>>> = {
  session: new Set(["created", "resolved", "rotated", "revoked", "consistency_updated", "expired", "rejected", "unavailable"]),
  rate_limit: new Set(["denied", "unavailable"]),
  step_up: new Set(["required", "completed", "rejected"]),
  unavailable_action: new Set(["busy", "offline", "prerequisite", "capability_missing", "step_up", "temporary_unavailable", "unreleased", "terminal"]),
};

export type GuardrailMetric = Readonly<{
  name: "ledgersync.guardrail.outcome";
  kind: GuardrailKind;
  outcome: string;
}>;

export function createGuardrailMetric(kind: GuardrailKind, outcome: string): GuardrailMetric {
  return Object.freeze({
    name: "ledgersync.guardrail.outcome",
    kind,
    outcome: allowedOutcomes[kind].has(outcome) ? outcome : "unknown",
  });
}

/**
 * Emits only closed, non-sensitive dimensions. In the browser this is a local
 * event for an optional RUM listener. On the server it becomes a structured
 * log only when telemetry is explicitly enabled. No identifier, amount,
 * request body, token, or error message can enter this contract.
 */
export function emitGuardrailMetric(kind: GuardrailKind, outcome: string): void {
  const metric = createGuardrailMetric(kind, outcome);
  if (typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent("ledgersync:guardrail", { detail: metric }));
    return;
  }
  if (process.env.LEDGERSYNC_TELEMETRY_ENABLED === "true") {
    console.info(JSON.stringify(metric));
  }
}
