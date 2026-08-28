import type { SanitizedOperationsResponse } from "@/lib/api/operations";

export type OrientationStepID = "confirm_health" | "understand_authority" | "inspect_accounts" | "create_account" | "fund_account" | "post_transfer" | "retry_transfer" | "inspect_postings" | "run_reconciliation" | "inspect_delivery" | "export_evidence" | "create_backup";
export type OperatorPreferenceStepID = "confirm_health" | "understand_authority" | "inspect_accounts" | "retry_transfer" | "inspect_postings" | "inspect_delivery" | "export_evidence";
export type OrientationStep = Readonly<{
  id: OrientationStepID;
  state: "completed" | "operator_confirmed" | "evidence_available" | "missing" | "unavailable";
  evidence_type: "local_health_confirmation" | "authority_acknowledgement" | "account_record" | "account_created_audit" | "funding_journal" | "posted_transfer" | "idempotency_outcome" | "journal_postings" | "reconciliation_run" | "delivery_attempt" | "evidence_export" | "recovery_backup";
  evidence_id?: string;
  occurred_at?: string;
  reason_code?: "operator_confirmation_required" | "no_authorized_account" | "no_account_creation_evidence" | "funding_workflow_unavailable" | "no_posted_transfer" | "no_reconciliation_run" | "no_delivery_attempt" | "no_exportable_evidence" | "no_validated_backup" | "recovery_evidence_unavailable";
}>;
export type LocalOrientation = Readonly<{
  generated_at: string;
  evidence_state: "complete" | "partial";
  dismissed: boolean;
  preference_version: string;
  preference_updated_at?: string;
  operator_completed_step_ids: OperatorPreferenceStepID[];
  steps: OrientationStep[];
}>;
export type OrientationPreferenceInput = Readonly<{ expected_version: string; dismissed: boolean; completed_step_ids: OperatorPreferenceStepID[] }>;

export type ExplainabilityEvidence = Readonly<{
  evidence_type: "idempotency_outcome" | "transfer" | "journal" | "posting" | "balance_version" | "outbox_event" | "delivery_attempt" | "reconciliation_run";
  evidence_id?: string;
  related_id?: string;
  status?: "in_progress" | "completed" | "failed" | "pending" | "posted" | "rejected" | "debit" | "credit" | "published" | "retrying" | "dead" | "delivered" | "matched" | "mismatch";
  account_id?: string;
  direction?: "debit" | "credit";
  amount_minor?: string;
  currency?: "INR";
  balance_version?: string;
  attempt_number?: string;
  event_type?: "account.balance.changed.v1";
  occurred_at: string;
}>;
export type ExplainabilityStage = Readonly<{
  sequence: 1 | 2 | 3 | 4 | 5 | 6 | 7;
  kind: "request" | "transfer" | "journal_postings" | "balance_versions" | "outbox" | "delivery" | "reconciliation";
  state: "available" | "missing" | "unavailable";
  reason_code?: "no_retained_idempotency_outcome" | "no_journal" | "no_postings" | "no_balance_version_evidence" | "no_outbox_events" | "no_delivery_attempts" | "coverage_not_provable" | "dependency_unavailable" | "evidence_truncated";
  truncated: boolean;
  evidence: ExplainabilityEvidence[];
}>;
export type TransferExplainability = Readonly<{ transfer_id: string; generated_at: string; evidence_state: "complete" | "partial"; stages: ExplainabilityStage[] }>;

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const safeID = /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/;
const exactMinor = /^-?(?:0|[1-9][0-9]{0,18})$/;
const exactCount = /^(?:0|[1-9][0-9]{0,18})$/;
const stepIDs = ["confirm_health", "understand_authority", "inspect_accounts", "create_account", "fund_account", "post_transfer", "retry_transfer", "inspect_postings", "run_reconciliation", "inspect_delivery", "export_evidence", "create_backup"] as const;
const stepEvidence = ["local_health_confirmation", "authority_acknowledgement", "account_record", "account_created_audit", "funding_journal", "posted_transfer", "idempotency_outcome", "journal_postings", "reconciliation_run", "delivery_attempt", "evidence_export", "recovery_backup"] as const;
const preferenceStepIDs = ["confirm_health", "understand_authority", "inspect_accounts", "retry_transfer", "inspect_postings", "inspect_delivery", "export_evidence"] as const;
const preferenceStepSet = new Set<string>(preferenceStepIDs);
const orientationStates = new Set(["completed", "operator_confirmed", "evidence_available", "missing", "unavailable"]);
const orientationReasons = new Set(["operator_confirmation_required", "no_authorized_account", "no_account_creation_evidence", "funding_workflow_unavailable", "no_posted_transfer", "no_reconciliation_run", "no_delivery_attempt", "no_exportable_evidence", "no_validated_backup", "recovery_evidence_unavailable"]);
const stageKinds = ["request", "transfer", "journal_postings", "balance_versions", "outbox", "delivery", "reconciliation"] as const;
const stageReasons = new Set(["no_retained_idempotency_outcome", "no_journal", "no_postings", "no_balance_version_evidence", "no_outbox_events", "no_delivery_attempts", "coverage_not_provable", "dependency_unavailable", "evidence_truncated"]);
const evidenceTypes = new Set(["idempotency_outcome", "transfer", "journal", "posting", "balance_version", "outbox_event", "delivery_attempt", "reconciliation_run"]);
const evidenceStatuses = new Set(["in_progress", "completed", "failed", "pending", "posted", "rejected", "debit", "credit", "published", "retrying", "dead", "delivered", "matched", "mismatch"]);
const stageBounds = [1, 1, 3, 2, 8, 25, 1] as const;

function record(value: unknown): Record<string, unknown> | null { return typeof value === "object" && value !== null && !Array.isArray(value) ? value as Record<string, unknown> : null; }
function exactKeys(value: Record<string, unknown>, required: readonly string[], optional: readonly string[] = []) {
  const allowed = new Set([...required, ...optional]);
  return required.every((key) => Object.hasOwn(value, key)) && Object.keys(value).every((key) => allowed.has(key));
}
function timestamp(value: unknown): value is string { return typeof value === "string" && value.length <= 64 && /(?:Z|[+-][0-9]{2}:[0-9]{2})$/.test(value) && !Number.isNaN(Date.parse(value)); }
function identifier(value: unknown): value is string { return typeof value === "string" && safeID.test(value); }
function error(status: number, value: unknown): SanitizedOperationsResponse {
  const root = record(value); const detail = record(root?.error); const code = detail?.code;
  if (status === 401 && code === "unauthorized") return { status, body: { error: { code } } };
  if (status === 403 && code === "forbidden") return { status, body: { error: { code } } };
  if (status === 400 && code === "invalid_request") return { status, body: { error: { code } } };
  if (status === 409 && code === "preference_version_conflict") return { status, body: { error: { code } } };
  if (status === 404 && code === "not_found") return { status, body: { error: { code } } };
  if (status === 429 && code === "rate_limited") return { status, body: { error: { code } } };
  if (status === 503 && (code === "evidence_unavailable" || code === "temporary_unavailable")) return { status, body: { error: { code } } };
  if (status === 504 && code === "upstream_timeout") return { status, body: { error: { code } } };
  return { status: 503, body: { error: { code: "evidence_unavailable" } } };
}

export function sanitizeLocalOrientation(status: number, value: unknown): SanitizedOperationsResponse {
  if (status !== 200) return error(status, value);
  const root = record(value);
  if (!root || !exactKeys(root, ["generated_at", "evidence_state", "dismissed", "preference_version", "operator_completed_step_ids", "steps"], ["preference_updated_at"]) || !timestamp(root.generated_at) || !["complete", "partial"].includes(String(root.evidence_state)) || typeof root.dismissed !== "boolean" || typeof root.preference_version !== "string" || !exactCount.test(root.preference_version) || !Array.isArray(root.operator_completed_step_ids) || root.operator_completed_step_ids.length > preferenceStepIDs.length || !Array.isArray(root.steps) || root.steps.length !== stepIDs.length) return error(503, value);
  if ((root.preference_version === "0") !== (root.preference_updated_at === undefined) || (root.preference_updated_at !== undefined && !timestamp(root.preference_updated_at))) return error(503, value);
  const completedIDs = root.operator_completed_step_ids;
  if (completedIDs.some((item) => typeof item !== "string" || !preferenceStepSet.has(item)) || new Set(completedIDs).size !== completedIDs.length) return error(503, value);
  const steps: OrientationStep[] = [];
  for (let index = 0; index < stepIDs.length; index += 1) {
    const item = record(root.steps[index]);
    if (!item || !exactKeys(item, ["id", "state", "evidence_type"], ["evidence_id", "occurred_at", "reason_code"]) || item.id !== stepIDs[index] || item.evidence_type !== stepEvidence[index] || !orientationStates.has(String(item.state))) return error(503, value);
    const evidenceRequired = item.state === "completed" || item.state === "evidence_available";
    const hasEvidenceTime = timestamp(item.occurred_at);
    const hasEvidenceID = identifier(item.evidence_id);
    if ((evidenceRequired && (!hasEvidenceTime || !hasEvidenceID)) || (!evidenceRequired && item.state !== "operator_confirmed" && (item.occurred_at !== undefined || item.evidence_id !== undefined)) || (item.state === "operator_confirmed" && hasEvidenceTime !== hasEvidenceID) || (item.evidence_id !== undefined && !hasEvidenceID) || (item.occurred_at !== undefined && !hasEvidenceTime) || (item.reason_code !== undefined && !orientationReasons.has(String(item.reason_code))) || (["missing", "unavailable", "evidence_available"].includes(String(item.state)) && item.reason_code === undefined) || (["completed", "operator_confirmed"].includes(String(item.state)) && item.reason_code !== undefined)) return error(503, value);
    if (item.state === "operator_confirmed" && !preferenceStepSet.has(String(item.id))) return error(503, value);
    steps.push(item as OrientationStep);
  }
  const evidenceState = root.evidence_state as LocalOrientation["evidence_state"];
  if ((evidenceState === "complete") !== steps.every((step) => step.state === "completed" || step.state === "operator_confirmed")) return error(503, value);
  const confirmedFromSteps = steps.filter((step) => step.state === "operator_confirmed").map((step) => step.id).sort();
  if (JSON.stringify([...completedIDs].sort()) !== JSON.stringify(confirmedFromSteps)) return error(503, value);
  return { status: 200, body: { generated_at: root.generated_at, evidence_state: evidenceState, dismissed: root.dismissed, preference_version: root.preference_version, ...(root.preference_updated_at === undefined ? {} : { preference_updated_at: root.preference_updated_at }), operator_completed_step_ids: completedIDs, steps } };
}

export function parseOrientationPreferenceInput(value: unknown): OrientationPreferenceInput {
  const root = record(value);
  if (!root || !exactKeys(root, ["expected_version", "dismissed", "completed_step_ids"]) || typeof root.expected_version !== "string" || !exactCount.test(root.expected_version) || typeof root.dismissed !== "boolean" || !Array.isArray(root.completed_step_ids) || root.completed_step_ids.length > preferenceStepIDs.length) throw new Error("invalid orientation preference");
  const completed = root.completed_step_ids;
  if (completed.some((item) => typeof item !== "string" || !preferenceStepSet.has(item)) || new Set(completed).size !== completed.length) throw new Error("invalid orientation preference");
  return { expected_version: root.expected_version, dismissed: root.dismissed, completed_step_ids: [...completed].sort() as OperatorPreferenceStepID[] };
}

function sanitizeEvidence(value: unknown): ExplainabilityEvidence | null {
  const item = record(value);
  if (!item || !exactKeys(item, ["evidence_type", "occurred_at"], ["evidence_id", "related_id", "status", "account_id", "direction", "amount_minor", "currency", "balance_version", "attempt_number", "event_type"]) || !evidenceTypes.has(String(item.evidence_type)) || !timestamp(item.occurred_at)) return null;
  for (const key of ["evidence_id", "related_id", "account_id"] as const) if (item[key] !== undefined && !identifier(item[key])) return null;
  if (item.status !== undefined && !evidenceStatuses.has(String(item.status))) return null;
  if (item.direction !== undefined && !["debit", "credit"].includes(String(item.direction))) return null;
  if (item.amount_minor !== undefined && (typeof item.amount_minor !== "string" || !exactMinor.test(item.amount_minor))) return null;
  if (item.currency !== undefined && item.currency !== "INR") return null;
  if (item.balance_version !== undefined && (typeof item.balance_version !== "string" || !exactCount.test(item.balance_version))) return null;
  if (item.attempt_number !== undefined && (typeof item.attempt_number !== "string" || !exactCount.test(item.attempt_number))) return null;
  if (item.event_type !== undefined && item.event_type !== "account.balance.changed.v1") return null;
  return item as ExplainabilityEvidence;
}

export function sanitizeTransferExplainability(status: number, value: unknown): SanitizedOperationsResponse {
  if (status !== 200) return error(status, value);
  const root = record(value);
  if (!root || !exactKeys(root, ["transfer_id", "generated_at", "evidence_state", "stages"]) || typeof root.transfer_id !== "string" || !uuid.test(root.transfer_id) || !timestamp(root.generated_at) || !["complete", "partial"].includes(String(root.evidence_state)) || !Array.isArray(root.stages) || root.stages.length !== 7) return error(503, value);
  const stages: ExplainabilityStage[] = [];
  for (let index = 0; index < 7; index += 1) {
    const stage = record(root.stages[index]);
    if (!stage || !exactKeys(stage, ["sequence", "kind", "state", "truncated", "evidence"], ["reason_code"]) || stage.sequence !== index + 1 || stage.kind !== stageKinds[index] || !["available", "missing", "unavailable"].includes(String(stage.state)) || typeof stage.truncated !== "boolean" || !Array.isArray(stage.evidence) || stage.evidence.length > stageBounds[index] || (stage.reason_code !== undefined && !stageReasons.has(String(stage.reason_code)))) return error(503, value);
    const evidence = stage.evidence.map(sanitizeEvidence);
    if (evidence.some((item) => item === null) || (stage.state === "available") !== (evidence.length > 0) || (stage.state !== "available" && stage.reason_code === undefined) || (stage.truncated && stage.reason_code !== "evidence_truncated")) return error(503, value);
    stages.push({ ...(stage as Omit<ExplainabilityStage, "evidence">), evidence: evidence as ExplainabilityEvidence[] });
  }
  const evidenceState = root.evidence_state as TransferExplainability["evidence_state"];
  if ((evidenceState === "complete") !== stages.every((stage) => stage.state === "available" && !stage.truncated)) return error(503, value);
  return { status: 200, body: { transfer_id: root.transfer_id, generated_at: root.generated_at, evidence_state: evidenceState, stages } };
}
