"use client";

import Link from "next/link";
import { FormEvent, useEffect, useState } from "react";

import { BalanceStatus } from "@/features/accounts/BalanceStatus";
import { TransactionLedger } from "@/features/accounts/TransactionLedger";
import type { Account, Transaction } from "@/features/accounts/types";
import { CopyControl, DataTableRegion, PageHeader, Pagination, RecordLink, StatePanel, StatusBadge } from "@/features/console/components";
import { accountLabel, utcDateTime } from "@/features/console/format";
import { formatMinorUnits } from "@/lib/money";

export type AccountFilters = Readonly<{ query: string; status: string; category: string; cursor?: string }>;

type Props = Readonly<{
  accounts: Account[];
  selected: Account | null;
  detailRequested: boolean;
  balance: Account | null;
  transactions: Transaction[];
  balanceLoading: boolean;
  historyLoading: boolean;
  directoryLoading: boolean;
  balanceError: string | null;
  historyError: string | null;
  error: string | null;
  online: boolean;
  filters: AccountFilters;
  nextCursor?: string;
  historyNextCursor?: string;
  focusAccountId?: string;
  onRefresh: () => void;
  onApplyFilters: (filters: Omit<AccountFilters, "cursor">) => void;
  onNext: () => void;
  onHistoryNext: () => void;
}>;

function tone(status: Account["status"]) {
  return status === "active" ? "success" : status === "frozen" ? "warning" : "neutral";
}

function accountDirectoryHref(filters: AccountFilters, focusAccountId?: string): string {
  const params = new URLSearchParams();
  if (filters.query) params.set("q", filters.query);
  if (filters.status) params.set("status", filters.status);
  if (filters.category) params.set("category", filters.category);
  if (filters.cursor) params.set("cursor", filters.cursor);
  if (focusAccountId) params.set("focus", focusAccountId);
  const query = params.toString();
  return query ? `/accounts?${query}` : "/accounts";
}

function accountDetailHref(accountID: string, filters: AccountFilters): string {
  return `/accounts/${accountID}?return_to=${encodeURIComponent(accountDirectoryHref(filters, accountID))}`;
}

export function AccountsView({ accounts, selected, detailRequested, balance, transactions, balanceLoading, historyLoading, directoryLoading, balanceError, historyError, error, online, filters, nextCursor, historyNextCursor, focusAccountId, onRefresh, onApplyFilters, onNext, onHistoryNext }: Props) {
  const [query, setQuery] = useState(filters.query);
  const [status, setStatus] = useState(filters.status);
  const [category, setCategory] = useState(filters.category);

  useEffect(() => {
    if (!selected && !directoryLoading && focusAccountId) document.getElementById(`account-link-${focusAccountId}`)?.focus();
  }, [directoryLoading, focusAccountId, selected]);

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onApplyFilters({ query: query.trim(), status, category });
  }

  return <>
    <PageHeader eyebrow="Ledger / Account directory" title={selected ? accountLabel(selected) : detailRequested ? "Account detail" : "Accounts"} description={selected ? "Authoritative balance, account identity, and immutable posting history." : detailRequested ? "Loading only the requested authorized account evidence." : "Search only the accounts authorized for this operator."}>
      <button className="button secondary" type="button" onClick={onRefresh} disabled={!online || directoryLoading}>Refresh evidence</button>
    </PageHeader>
    {error && <StatePanel kind="error" title="Accounts unavailable" message={error} />}
    {detailRequested && !selected && !error && <StatePanel title="Loading account evidence" message="Balance and immutable history are verified independently before they are presented as current." />}
    {!detailRequested && !selected && !error && <>
      <form className="filter-bar" role="search" onSubmit={submit}>
        <label>Search accounts<input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Name/reference prefix or exact ID" maxLength={128} /></label>
        <label>Status<select value={status} onChange={(event) => setStatus(event.target.value)}><option value="">All statuses</option><option value="active">Active</option><option value="frozen">Frozen</option><option value="closed">Closed</option></select></label>
        <label>Category<select value={category} onChange={(event) => setCategory(event.target.value)}><option value="">All categories</option><option value="operating">Operating</option><option value="customer_funds">Customer funds</option><option value="payroll">Payroll</option><option value="payables">Payables</option><option value="expenses">Expenses</option><option value="reserve">Reserve</option></select></label>
        <button className="button primary" type="submit" disabled={!online || directoryLoading}>Apply filters</button>
        <span aria-live="polite">Showing {accounts.length} authorized result{accounts.length === 1 ? "" : "s"}</span>
      </form>
      {directoryLoading && accounts.length === 0 ? <StatePanel title="Loading authorized accounts" message="LedgerSync is requesting one bounded page from the authoritative account directory." /> : accounts.length === 0 ? <StatePanel title="No matching accounts" message="Clear or change the filters. LedgerSync does not broaden the authorized account scope." /> : <section className="ledger-section account-directory" aria-labelledby="accounts-heading" aria-busy={directoryLoading}>
        <div className="section-heading"><div><p className="eyebrow">Authorized scope</p><h2 id="accounts-heading">Available balances</h2></div></div>
        <DataTableRegion label="Authorized account comparison"><table className="data-table"><thead><tr><th>Account</th><th>Category</th><th>Status</th><th>Available balance</th><th>Version / as of</th><th>Action</th></tr></thead><tbody>{accounts.map((account) => <tr key={account.account_id}><td><strong>{accountLabel(account)}</strong><code>{account.account_id}</code></td><td>{account.category?.replaceAll("_", " ") ?? "Unclassified"}</td><td><StatusBadge tone={tone(account.status)}>{account.status}</StatusBadge></td><td className="number-cell">{formatMinorUnits(account.currency, account.available_minor)}</td><td><code>v{account.version}</code><small>{utcDateTime(account.as_of)}</small></td><td><RecordLink id={`account-link-${account.account_id}`} href={accountDetailHref(account.account_id, filters)} label="Open account" /></td></tr>)}</tbody></table></DataTableRegion>
        <Pagination nextCursor={nextCursor} busy={directoryLoading} onNext={onNext} label="Next page" />
      </section>}
    </>}
    {selected && <>
      <section className="identity-strip"><div><span>Account ID</span><CopyControl value={selected.account_id} /></div><div><span>External reference</span><strong>{selected.external_reference || "Not set"}</strong></div><div><span>Category</span><strong>{selected.category?.replaceAll("_", " ") || "Unclassified"}</strong></div><div><span>Status</span><StatusBadge tone={tone(selected.status)}>{selected.status}</StatusBadge></div></section>
      {selected.status === "frozen" && <StatePanel kind="unknown" title="Account is frozen" message="The balance remains visible, but this account cannot be selected for a new debit or credit." />}
      <div className="account-detail-grid"><BalanceStatus currency={balance?.currency ?? selected.currency} availableMinor={balance?.available_minor} version={balance?.version} asOf={balance?.as_of} loading={balanceLoading} error={balanceError} /><TransactionLedger transactions={transactions} account={selected} loading={historyLoading} error={historyError} nextCursor={historyNextCursor} onNext={onHistoryNext} /></div>
      <section className="ledger-section" aria-labelledby="account-audit-heading"><div className="section-heading"><div><p className="eyebrow">Permitted audit context</p><h2 id="account-audit-heading">Account audit events</h2><p>Only explicitly persisted account-targeted events are shown; absence is not inferred as approval.</p></div></div>{selected.audit_context?.length ? <div className="ledger-list">{selected.audit_context.map((event) => <article className="ledger-row" key={event.event_id}><div className="ledger-identity"><strong>{event.event_type.replaceAll("_", " ")}</strong><span>{event.actor_subject_id} · {utcDateTime(event.occurred_at)}</span></div><StatusBadge tone={event.outcome === "succeeded" ? "success" : "warning"}>{event.outcome}</StatusBadge><CopyControl value={event.correlation_id} label="Copy audit correlation ID" /></article>)}</div> : <StatePanel title="No account-scoped audit events" message="No account-targeted audit event is available in the current retained evidence. LedgerSync does not invent one from balance or transfer history." />}</section>
      <Link className="text-link back-link" href={accountDirectoryHref(filters, focusAccountId)}>← Back to account directory</Link>
    </>}
  </>;
}
