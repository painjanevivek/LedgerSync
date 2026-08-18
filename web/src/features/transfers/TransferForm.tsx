"use client";

import { FormEvent, useMemo, useRef, useState } from "react";

import type { Account } from "@/features/accounts/types";
import { minorUnitsFromDecimal } from "@/lib/money";

type Props = Readonly<{ accounts: Account[]; tenantId: string; csrfToken: string; disabled?: boolean; onPosted: () => Promise<void> }>;
type Outcome = { kind: "success" | "error" | "unknown"; message: string; transferId?: string } | null;

function storageKey(tenant: string) { return `ledgersync.transfer.idempotency.${tenant}`; }

export function TransferForm({ accounts, tenantId, csrfToken, disabled, onPosted }: Props) {
  const transferable = useMemo(() => accounts.filter((account) => account.status === "active"), [accounts]);
  const [source, setSource] = useState(transferable[0]?.account_id ?? "");
  const [destination, setDestination] = useState(transferable[1]?.account_id ?? "");
  const [amount, setAmount] = useState("");
  const [pending, setPending] = useState(false);
  const [outcome, setOutcome] = useState<Outcome>(null);
  const idempotencyKey = useRef<string | null>(null);
  const sourceAccount = transferable.find((account) => account.account_id === source);
  const destinations = transferable.filter((account) => account.account_id !== source && account.currency === sourceAccount?.currency);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const destinationAccountId = destinations.some((account) => account.account_id === destination) ? destination : destinations[0]?.account_id;
    if (!sourceAccount || !destinationAccountId || source === destinationAccountId) { setOutcome({ kind: "error", message: "Choose two different active accounts in the same currency." }); return; }
    let minorUnits: string;
    try { minorUnits = minorUnitsFromDecimal(sourceAccount.currency, amount); } catch (error) { setOutcome({ kind: "error", message: error instanceof Error ? error.message : "Check the amount." }); return; }
    setPending(true); setOutcome(null);
    try {
      const stored = sessionStorage.getItem(storageKey(tenantId));
      const requestKey = idempotencyKey.current ?? stored ?? crypto.randomUUID();
      idempotencyKey.current = requestKey;
      if (!stored) sessionStorage.setItem(storageKey(tenantId), requestKey);
      const response = await fetch("/api/transfers", { method: "POST", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrfToken, "Idempotency-Key": requestKey }, body: JSON.stringify({ sourceAccountId: source, destinationAccountId, amount: { currency: sourceAccount.currency, minorUnits } }) });
      const payload = await response.json().catch(() => ({})) as { transfer_id?: string; error?: { code?: string } };
      if (response.ok && payload.transfer_id) {
        sessionStorage.removeItem(storageKey(tenantId));
        idempotencyKey.current = null; setAmount("");
        setOutcome({ kind: "success", message: "Transfer posted. Current balances have been refreshed.", transferId: payload.transfer_id });
        await onPosted();
      } else if (response.status === 409 && payload.error?.code === "insufficient_funds") setOutcome({ kind: "error", message: "Transfer rejected — insufficient posted balance. No money moved." });
      else if (response.status === 409 && payload.error?.code === "idempotency_conflict") setOutcome({ kind: "error", message: "This retry key belongs to a different transfer request. Start a genuinely new transfer." });
      else setOutcome({ kind: "unknown", message: "We could not confirm the result. Retry the same request safely; it cannot create a second transfer." });
    } catch { setOutcome({ kind: "unknown", message: "We could not confirm the result. Retry the same request safely; it cannot create a second transfer." }); }
    finally { setPending(false); }
  }

  if (transferable.length < 2 || destinations.length === 0) return <section className="surface transfer-form"><h2>Internal transfer</h2><p className="muted">Two active accounts in the same currency are required before a transfer can be prepared.</p></section>;
  return <section className="surface transfer-form" aria-labelledby="transfer-heading"><div><p className="eyebrow">Guarded action</p><h2 id="transfer-heading">Internal transfer</h2><p className="muted">Exact amount only. A network retry keeps this same request key.</p></div><form onSubmit={submit}><label>From account<select value={source} onChange={(event) => setSource(event.target.value)} disabled={pending || disabled}>{transferable.map((account) => <option key={account.account_id} value={account.account_id}>{account.account_id.slice(0, 8)}… · {account.currency}</option>)}</select></label><label>To account<select value={destinations.some((account) => account.account_id === destination) ? destination : destinations[0]?.account_id} onChange={(event) => setDestination(event.target.value)} disabled={pending || disabled}>{destinations.map((account) => <option key={account.account_id} value={account.account_id}>{account.account_id.slice(0, 8)}… · {account.currency}</option>)}</select></label><label>Amount<input value={amount} onChange={(event) => setAmount(event.target.value)} inputMode="decimal" placeholder="0.00" aria-describedby="transfer-help" disabled={pending || disabled} /></label><p id="transfer-help" className="muted">Use decimal text; LedgerSync converts it to exact minor units without floating-point math.</p><button className="button primary" disabled={pending || disabled} type="submit">{pending ? "Posting transfer…" : outcome?.kind === "unknown" ? "Retry same transfer" : "Post internal transfer"}</button></form>{outcome && <div className={`inline-alert ${outcome.kind}`} role={outcome.kind === "error" ? "alert" : "status"}><strong>{outcome.kind === "success" ? "Transfer posted" : outcome.kind === "unknown" ? "Result not yet confirmed" : "Transfer not posted"}</strong><p>{outcome.message}</p>{outcome.transferId && <code>Transfer ID {outcome.transferId}</code>}</div>}</section>;
}
