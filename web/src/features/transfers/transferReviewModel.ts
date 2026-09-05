import type { Account } from "@/features/accounts/types";
import type { PreparedTransfer } from "./transferIntent";

export function expectedTransferBalances(transfer: PreparedTransfer) {
  const { source, destination, amountMinor } = transfer;
  if (source.account_id === destination.account_id || source.currency !== destination.currency || source.status !== "active" || destination.status !== "active") throw new Error("Choose two different active accounts in the same currency.");
  if (!/^[1-9][0-9]*$/.test(amountMinor)) throw new Error("Enter an amount greater than zero.");
  const amount = BigInt(amountMinor);
  const available = BigInt(source.available_minor);
  if (amount > available) throw new Error("This amount exceeds the available money in the source account.");
  return { source: (available - amount).toString(), destination: (BigInt(destination.available_minor) + amount).toString() };
}

export function reviewAccountChanged(previous: Account, current: Account): boolean {
  return (["account_id", "currency", "status", "display_name", "available_minor", "ledger_minor", "version", "account_version"] as const).some(key => previous[key] !== current[key]);
}

/** Read-only preflight. This is not a reservation; the command API remains authoritative. */
export async function refreshTransferReview(transfer: PreparedTransfer, signal: AbortSignal): Promise<PreparedTransfer> {
  const accounts = await Promise.all([transfer.source, transfer.destination].map(async account => {
    const response = await fetch(`/api/accounts/${encodeURIComponent(account.account_id)}`, { cache: "no-store", signal });
    if (!response.ok) throw new Error("We couldn’t recheck the accounts. No transfer was submitted. Try again.");
    const current = await response.json() as Account;
    if (current.account_id !== account.account_id || !/^-?[0-9]+$/.test(current.available_minor) || !/^-?[0-9]+$/.test(current.ledger_minor) || !current.version || !current.account_version) throw new Error("The account check was incomplete. No transfer was submitted.");
    return current;
  }));
  const refreshed = { source: accounts[0], destination: accounts[1], amountMinor: transfer.amountMinor };
  expectedTransferBalances(refreshed);
  return refreshed;
}
