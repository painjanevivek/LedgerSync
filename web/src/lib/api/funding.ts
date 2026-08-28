export type FundingStatus = "requested" | "approved" | "posted" | "rejected" | "compensated";

export type FundingEvent = Readonly<{
  funding_event_id: string;
  status: FundingStatus;
  destination_account_id: string;
  system_account_id?: string;
  currency: string;
  amount_minor: string;
  external_reference: string;
  evidence_reference: string;
  requester_subject_id: string;
  approver_subject_id?: string;
  decision_reason?: string;
  demo_policy: boolean;
  journal_transaction_id?: string;
  compensation_of_event_id?: string;
  compensation_event_id?: string;
  compensation_reason_code?: string;
  compensation_operator_note?: string;
  requested_at: string;
  updated_at: string;
  balance_version?: string;
}>;

export type FundingSubmission = Readonly<{ event: FundingEvent; replayed: boolean }>;
export type FundingPage = Readonly<{ events: FundingEvent[]; next_cursor?: string }>;
export type FundingReconciliation = Readonly<{
  funding_event_id: string;
  external_reference: string;
  status: "matched" | "mismatch";
  expected_minor: string;
  posted_debit_minor: string;
  posted_credit_minor: string;
  currency: string;
  checked_at: string;
}>;

export const fundingMutationMaximumBytes = 16 * 1024;

export type CreateFundingInput = Readonly<{
  destinationAccountId: string;
  amountMinor: string;
  currency: string;
  externalReference: string;
  evidenceReference: string;
}>;

export function toPrivateFundingRequest(input: CreateFundingInput) {
  const destinationAccountId = input.destinationAccountId.trim();
  const amountMinor = input.amountMinor.trim();
  const currency = input.currency.trim().toUpperCase();
  const externalReference = input.externalReference.trim();
  const evidenceReference = input.evidenceReference.trim();
  if (!destinationAccountId || destinationAccountId.length > 128 || !/^[1-9][0-9]*$/.test(amountMinor) || !/^[A-Z]{3}$/.test(currency) || !externalReference || externalReference.length > 256 || !evidenceReference || evidenceReference.length > 512) {
    throw new Error("invalid funding evidence input");
  }
  return {
    destination_account_id: destinationAccountId,
    amount_minor: amountMinor,
    currency,
    external_reference: externalReference,
    evidence_reference: evidenceReference,
  };
}

export function toPrivateFundingDecision(value: unknown) {
  const reason = typeof value === "object" && value !== null && "reason" in value && typeof value.reason === "string" ? value.reason.trim() : "";
  if (!reason || reason.length > 500) throw new Error("invalid funding decision");
  return { reason };
}

export function toPrivateFundingCompensation(value: unknown) {
  const reasonCode = typeof value === "object" && value !== null && "reasonCode" in value && typeof value.reasonCode === "string" ? value.reasonCode.trim() : "";
  const operatorNote = typeof value === "object" && value !== null && "operatorNote" in value && typeof value.operatorNote === "string" ? value.operatorNote.trim() : "";
  if (!reasonCode || reasonCode.length > 64 || !operatorNote || operatorNote.length > 500) throw new Error("invalid funding compensation");
  return { reason_code: reasonCode, operator_note: operatorNote };
}

export function isFundingEventID(value: string): boolean {
  return value.length >= 1 && value.length <= 128 && /^[A-Za-z0-9-]+$/.test(value);
}

export function isFundingIdempotencyKey(value: string | null): value is string {
  return value !== null && value.trim() === value && value.length >= 16 && value.length <= 255 && /^[\x21-\x7e]+$/.test(value);
}
