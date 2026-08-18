export type Account = Readonly<{
  account_id: string;
  currency: string;
  status: "active" | "frozen" | "closed";
  available_minor: string;
  ledger_minor: string;
  version: string;
  as_of: string;
}>;

export type Transaction = Readonly<{
  transfer_id: string;
  direction: "debit" | "credit";
  amount: string;
  currency: string;
  status: "posted" | "rejected";
  occurred_at: string;
}>;

export type ConsoleSession = Readonly<{ subject_id: string; tenant_id: string; csrf_token: string; scopes: string[] }>;
