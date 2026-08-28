export type CorrectionStatus =
  "requested" | "approved" | "rejected" | "cancelled" | "expired" | "posted";
export type CorrectionReasonCode =
  | "duplicate"
  | "wrong_destination"
  | "wrong_amount"
  | "customer_request"
  | "operational_error"
  | "compliance_reversal";

export type TransferCorrection = Readonly<{
  correction_id: string;
  original_transfer_id: string;
  compensation_transfer_id?: string;
  original_journal_id: string;
  compensation_journal_id?: string;
  requester_subject_id: string;
  approver_subject_id?: string;
  debit_account_id: string;
  credit_account_id: string;
  amount_minor: string;
  currency: string;
  reason_code: CorrectionReasonCode;
  operator_note: string;
  decision_reason?: string;
  status: CorrectionStatus;
  policy_version: string;
  control_mode: string;
  step_up_required: boolean;
  approval_expires_at: string;
  requested_at: string;
  updated_at: string;
}>;

export type CorrectionSubmission = Readonly<{
  event: TransferCorrection;
  replayed: boolean;
}>;
export type CorrectionPage = Readonly<{
  events: TransferCorrection[];
  next_cursor?: string;
}>;
export const correctionMutationMaximumBytes = 16 * 1024;

const correctionReasons = new Set<CorrectionReasonCode>([
  "duplicate",
  "wrong_destination",
  "wrong_amount",
  "customer_request",
  "operational_error",
  "compliance_reversal",
]);

function exactRecord(
  value: unknown,
  keys: readonly string[],
): Record<string, unknown> {
  if (!value || Array.isArray(value) || typeof value !== "object")
    throw new Error("invalid correction input");
  const record = value as Record<string, unknown>;
  if (Object.keys(record).some((key) => !keys.includes(key)))
    throw new Error("unknown correction input");
  return record;
}

export function toPrivateCorrectionRequest(value: unknown) {
  const input = exactRecord(value, ["reasonCode", "operatorNote"]);
  const reasonCode =
    typeof input.reasonCode === "string"
      ? (input.reasonCode.trim() as CorrectionReasonCode)
      : "";
  const operatorNote =
    typeof input.operatorNote === "string" ? input.operatorNote.trim() : "";
  if (
    !correctionReasons.has(reasonCode as CorrectionReasonCode) ||
    !operatorNote ||
    operatorNote.length > 500
  )
    throw new Error("invalid correction request");
  return { reason_code: reasonCode, operator_note: operatorNote };
}

export function toPrivateCorrectionDecision(value: unknown) {
  const input = exactRecord(value, ["reason"]);
  const reason = typeof input.reason === "string" ? input.reason.trim() : "";
  if (!reason || reason.length > 500)
    throw new Error("invalid correction decision");
  return { reason };
}

export function isCorrectionID(value: string): boolean {
  return (
    value.length >= 1 && value.length <= 128 && /^[A-Za-z0-9-]+$/.test(value)
  );
}

export function isCorrectionIdempotencyKey(
  value: string | null,
): value is string {
  return (
    value !== null &&
    value.trim() === value &&
    value.length >= 16 &&
    value.length <= 255 &&
    /^[\x21-\x7e]+$/.test(value)
  );
}
