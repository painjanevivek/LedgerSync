export type Account = Readonly<{
  account_id: string;
  currency: string;
  status: "active" | "frozen" | "closed";
  available_minor: string;
  ledger_minor: string;
  account_version: string;
  version: string;
  as_of: string;
  display_name?: string;
  category?: "operating" | "customer_funds" | "payroll" | "payables" | "expenses" | "reserve";
  external_reference?: string;
  audit_context?: AccountAuditEvent[];
}>;

export type AccountAuditEvent = Readonly<{ event_id: string; event_type: string; actor_subject_id: string; outcome: string; correlation_id: string; occurred_at: string }>;

export type Transaction = Readonly<{
  transfer_id: string;
  direction: "debit" | "credit";
  amount: string;
  currency: string;
  status: "posted" | "rejected";
  occurred_at: string;
}>;

export type TransferSummary = Readonly<{
  transfer_id: string;
  source_account_id: string;
  destination_account_id: string;
  amount_minor: string;
  currency: string;
  financial_status: "posted" | "rejected" | "pending";
  delivery_status: "not_applicable" | "pending" | "retrying" | "delivered" | "dead";
  created_at: string;
  completed_at: string;
  journal_transaction_id?: string;
  rejection_code?: string;
}>;

export type Posting = Readonly<{ posting_id: string; account_id: string; direction: "debit" | "credit"; amount_minor: string; currency: string; occurred_at: string }>;
export type EvidenceEvent = Readonly<{ event_id: string; kind: string; outcome: string; reference?: string; occurred_at: string }>;
export type TransferDetail = TransferSummary & Readonly<{ actor_subject_id: string; postings: Posting[]; timeline: EvidenceEvent[] }>;

export type ReconciliationRun = Readonly<{
  run_id: string;
  status: "matched" | "mismatch" | "failed" | "running";
  correlation_id: string;
  scope: string;
  ledger_watermark: string;
  application_version: string;
  schema_version: string;
  checked_account_count: string;
  posting_count: string;
  mismatch_count: string;
  started_at: string;
  completed_at: string;
  mismatches?: ReconciliationMismatch[];
}>;

export type ReconciliationMismatch = Readonly<{ mismatch_id: string; account_id?: string; classification: string; currency?: string; expected_minor?: string; observed_minor?: string; observed_available_minor?: string; balance_version?: string; created_at: string }>;

export type ConsoleSession = Readonly<{ subject_id: string; tenant_id: string; csrf_token: string; scopes: string[]; environment?: "demo" | "production"; operator_label?: string; tenant_label?: string }>;
