export type Account = Readonly<{
  account_id: string;
  currency: string;
  status: "active" | "frozen" | "closed";
  available_minor: string;
  ledger_minor: string;
  version: string;
  as_of: string;
  display_name?: string;
  category?: "operating" | "customer_funds" | "payroll" | "payables" | "expenses" | "reserve";
  external_reference?: string;
}>;

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
  delivery_status: "delivered" | "delayed";
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
  checked_account_count: number;
  posting_count: number;
  mismatch_count: number;
  started_at: string;
  completed_at: string;
}>;

export type ConsoleSession = Readonly<{ subject_id: string; tenant_id: string; csrf_token: string; scopes: string[]; environment?: "demo" | "production"; operator_label?: string; tenant_label?: string }>;
