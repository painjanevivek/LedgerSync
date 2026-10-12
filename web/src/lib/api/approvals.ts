export type ApprovalDomain = "funding" | "correction";
export type ApprovalStepUpStatus = "not_required" | "satisfied" | "required";
export type ApprovalSafeNextAction =
  | "wait_for_independent_approver"
  | "complete_evidence"
  | "reauthenticate"
  | "review_decision"
  | "open_record";

export type ApprovalItem = Readonly<{
  domain: ApprovalDomain;
  record_id: string;
  requester_subject_id: string;
  requested_at: string;
  age_seconds: string;
  status: string;
  amount_minor: string;
  currency: string;
  related_account_id?: string;
  related_transfer_id?: string;
  evidence_complete: boolean;
  self_approval_blocked: boolean;
  actionable_by_me: boolean;
  required_scope: "funding:approve" | "corrections:approve";
  step_up_status: ApprovalStepUpStatus;
  approval_expires_at?: string;
  safe_next_action: ApprovalSafeNextAction;
}>;

export type ApprovalPage = Readonly<{
  items: ApprovalItem[];
  page_count: number;
  next_cursor?: string;
}>;

export type ApprovalFilters = Readonly<{
  domain: "" | ApprovalDomain;
  status: string;
  requester: string;
  age: "" | "under_24h" | "over_24h" | "over_7d" | "over_30d";
  requestedAfter: string;
  requestedBefore: string;
  actionableByMe: boolean;
  cursor?: string;
}>;

export const emptyApprovalFilters: ApprovalFilters = {
  domain: "",
  status: "",
  requester: "",
  age: "",
  requestedAfter: "",
  requestedBefore: "",
  actionableByMe: true,
};

export function approvalDetailHref(item: ApprovalItem, returnTo: string) {
  const root = item.domain === "funding" ? "/funding" : "/corrections";
  return `${root}/${encodeURIComponent(item.record_id)}?return_to=${encodeURIComponent(returnTo)}`;
}
