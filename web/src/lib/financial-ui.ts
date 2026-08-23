import type { Account, ReconciliationRun, TransferSummary } from "@/features/accounts/types";

export function isAuthoritativelyReconciled(run: ReconciliationRun | null | undefined) {
  return Boolean(run && run.status === "matched" && run.mismatch_count === "0" && run.run_id && run.completed_at && run.ledger_watermark);
}

export function approvedCurrencyGroups(accounts: readonly Account[]) {
  let operating = 0n; let customerFunds = 0n;
  const currencies = new Set(accounts.map((account) => account.currency));
  const currency = currencies.size === 1 ? [...currencies][0] : undefined;
  for (const account of accounts) {
    if (!currency || account.currency !== currency) continue;
    if (account.category === "customer_funds") customerFunds += BigInt(account.available_minor);
    else operating += BigInt(account.available_minor);
  }
  return { currency, mixedCurrency: currencies.size > 1, operatingMinor: operating.toString(), customerFundsMinor: customerFunds.toString() };
}

export function transferOutcomeLabels(transfer: Pick<TransferSummary, "financial_status" | "delivery_status">) {
  return { financial: transfer.financial_status, delivery: `Delivery ${transfer.delivery_status}` };
}
