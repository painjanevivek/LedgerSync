"use client";

import Link from "next/link";
import { FormEvent, useEffect, useState } from "react";

import { BalanceStatus } from "@/features/accounts/BalanceStatus";
import { AccountLifecycleActions } from "@/features/accounts/AccountLifecycleActions";
import { hasPositiveMinorUnits } from "@/features/accounts/accountCommandIntent";
import { TransactionLedger } from "@/features/accounts/TransactionLedger";
import type { Account, AccountBalance, Transaction } from "@/features/accounts/types";
import { CopyControl, DataTableRegion, EvidenceFreshness, FocusedRetry, PageHeader, Pagination, RecordLink, StatePanel, StatusBadge } from "@/features/console/components";
import { accountLabel, utcDateTime } from "@/features/console/format";
import { EvidenceExportControl } from "@/features/exports/EvidenceExportControl";
import { formatMinorUnits } from "@/lib/money";

export type AccountFilters = Readonly<{ query: string; status: string; category: string; cursor?: string }>;

type Props = Readonly<{
  accounts: Account[];
  selected: Account | null;
  detailRequested: boolean;
  balance: AccountBalance | null;
  transactions: Transaction[];
  balanceLoading: boolean;
  historyLoading: boolean;
  directoryLoading: boolean;
  balanceError: string | null;
  historyError: string | null;
  balanceVerifiedAt?: string;
  historyVerifiedAt?: string;
  directoryVerifiedAt?: string;
  error: string | null;
  online: boolean;
  filters: AccountFilters;
  nextCursor?: string;
  historyNextCursor?: string;
  focusAccountId?: string;
  tenantId: string;
  csrfToken: string;
  canWrite: boolean;
  canTransfer: boolean;
  canExport: boolean;
  fundingScopeComplete: boolean;
  detailReturnTo?: string;
  onRefresh: () => void;
  onApplyFilters: (filters: Omit<AccountFilters, "cursor">) => void;
  onNext: () => void;
  onHistoryNext: () => void;
  onRefreshBalance: () => void;
  onRefreshHistory: () => void;
  onAccountChanged: () => Promise<void>;
  onRefreshLifecycleEvidence: () => Promise<{ account: Account | null; balance: AccountBalance | null }>;
}>;

function tone(status: Account["status"]) {
  return status === "active" ? "success" : status === "frozen" ? "warning" : "neutral";
}

export function accountDirectoryHref(filters: AccountFilters, focusAccountId?: string): string {
  const params = new URLSearchParams();
  if (filters.query) params.set("q", filters.query);
  if (filters.status) params.set("status", filters.status);
  if (filters.category) params.set("category", filters.category);
  if (filters.cursor) params.set("cursor", filters.cursor);
  if (focusAccountId) params.set("focus", focusAccountId);
  const query = params.toString();
  return query ? `/accounts?${query}` : "/accounts";
}

export function accountDetailHref(accountID: string, filters: AccountFilters): string {
  return `/accounts/${encodeURIComponent(accountID)}?return_to=${encodeURIComponent(accountDirectoryHref(filters, accountID))}`;
}

export function AccountsView({ accounts, selected, detailRequested, balance, transactions, balanceLoading, historyLoading, directoryLoading, balanceError, historyError, balanceVerifiedAt, historyVerifiedAt, directoryVerifiedAt, error, online, filters, nextCursor, historyNextCursor, focusAccountId, tenantId, csrfToken, canWrite, canTransfer, canExport, fundingScopeComplete, detailReturnTo, onRefresh, onApplyFilters, onNext, onHistoryNext, onRefreshBalance, onRefreshHistory, onAccountChanged, onRefreshLifecycleEvidence }: Props) {
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
      <div className="header-actions"><button className="button secondary" type="button" onClick={onRefresh} disabled={!online || directoryLoading}>Refresh evidence</button>{!selected && !detailRequested && canWrite && <Link className="button primary guarded-control" href={`/accounts/new?return_to=${encodeURIComponent(accountDirectoryHref(filters))}`}>Create account</Link>}</div>
    </PageHeader>
    {error && <StatePanel kind="error" title="Accounts unavailable" message={error} action={<FocusedRetry label={detailRequested ? "Retry this account only" : "Retry account directory only"} onRetry={onRefresh} disabled={!online} busy={directoryLoading} />} />}
    {detailRequested && !selected && !error && <StatePanel title="Loading account evidence" message="Balance and immutable history are verified independently before they are presented as current." />}
    {!detailRequested && !selected && <>
      <form className="filter-bar" role="search" onSubmit={submit}>
        <label>Search accounts<input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Name/reference prefix or exact ID" maxLength={128} /></label>
        <label>Status<select value={status} onChange={(event) => setStatus(event.target.value)}><option value="">All statuses</option><option value="active">Active</option><option value="frozen">Frozen</option><option value="closed">Closed</option></select></label>
        <label>Category<select value={category} onChange={(event) => setCategory(event.target.value)}><option value="">All categories</option><option value="operating">Operating</option><option value="customer_funds">Customer funds</option><option value="payroll">Payroll</option><option value="payables">Payables</option><option value="expenses">Expenses</option><option value="reserve">Reserve</option></select></label>
        <button className="button primary" type="submit" disabled={!online || directoryLoading}>Apply filters</button>
        <span aria-live="polite">{error ? "Authorized result count unavailable" : `Showing ${accounts.length} authorized result${accounts.length === 1 ? "" : "s"}`}</span>
      </form>
      {directoryVerifiedAt && accounts.length > 0 && <EvidenceFreshness state={error || !online ? "historical" : directoryLoading ? "refreshing" : "current"} verifiedAt={directoryVerifiedAt} label="Account directory" reason={error ?? (!online ? "Reconnect before relying on directory balances." : undefined)} />}
      {directoryLoading && accounts.length === 0 ? <StatePanel title="Loading authorized accounts" message="LedgerSync is requesting one bounded page from the authoritative account directory." /> : accounts.length === 0 && !error ? <StatePanel title={filters.query || filters.status || filters.category ? "No matching accounts" : "No accounts yet"} message={filters.query || filters.status || filters.category ? "Clear or change the filters. LedgerSync does not broaden the authorized account scope." : "Create your first account to begin the ledger. It opens at an exact zero balance."} action={!filters.query && !filters.status && !filters.category && canWrite ? <Link className="button primary" href="/accounts/new">Create your first account</Link> : undefined} /> : accounts.length > 0 && <section className="ledger-section account-directory" aria-labelledby="accounts-heading" aria-busy={directoryLoading}>
        <div className="section-heading"><div><p className="eyebrow">Authorized scope</p><h2 id="accounts-heading">Available balances</h2></div></div>
        <DataTableRegion label="Authorized account comparison"><table className="data-table"><thead><tr><th>Account</th><th>Category</th><th>Status</th><th>Available balance</th><th>Version / as of</th><th>Action</th></tr></thead><tbody>{accounts.map((account) => <tr key={account.account_id}><td><strong>{accountLabel(account)}</strong><code>{account.account_id}</code></td><td>{account.category?.replaceAll("_", " ") ?? "Unclassified"}</td><td><StatusBadge tone={tone(account.status)}>{account.status}</StatusBadge></td><td className="number-cell">{formatMinorUnits(account.currency, account.available_minor)}</td><td><code>v{account.version}</code><small>{utcDateTime(account.as_of)}</small></td><td><RecordLink id={`account-link-${account.account_id}`} href={accountDetailHref(account.account_id, filters)} label="Open account" /></td></tr>)}</tbody></table></DataTableRegion>
        <Pagination nextCursor={nextCursor} busy={directoryLoading} onNext={onNext} label="Next page" />
      </section>}
    </>}
    {selected && <>
      <section className="identity-strip"><div><span>Account ID</span><CopyControl value={selected.account_id} /></div><div><span>External reference</span><strong>{selected.external_reference || "Not set"}</strong></div><div><span>Category</span><strong>{selected.category?.replaceAll("_", " ") || "Unclassified"}</strong></div><div><span>Status</span><StatusBadge tone={tone(selected.status)}>{selected.status}</StatusBadge></div></section>
      {selected.status === "frozen" && <StatePanel kind="unknown" title="Account is frozen" message="The balance remains visible, but this account cannot be selected for a new debit or credit." />}
      <div className="account-detail-grid"><BalanceStatus currency={balance?.currency ?? selected.currency} availableMinor={balance?.available_minor} version={balance?.version} asOf={balance?.as_of} verifiedAt={balanceVerifiedAt} loading={balanceLoading} error={balanceError} onRetry={onRefreshBalance} retryDisabled={!online} /><TransactionLedger transactions={transactions} account={selected} loading={historyLoading} error={historyError} verifiedAt={historyVerifiedAt} nextCursor={historyNextCursor} onNext={onHistoryNext} onRetry={onRefreshHistory} retryDisabled={!online} exportAction={<EvidenceExportControl label="Export ledger evidence" subject="account ledger history" endpoint={`/api/exports/accounts/${encodeURIComponent(selected.account_id)}/transactions.csv?limit=10000`} scope={`One authorized account · ${selected.account_id}`} filters={[]} columns="Includes the full account, transfer, journal, and posting identifiers with exact quoted minor-unit strings" online={online} canExport={canExport}/>}/></div>
      <AccountLifecycleActions account={selected} balance={balance} balanceLoading={balanceLoading} balanceError={balanceError} tenantId={tenantId} csrfToken={csrfToken} online={online} canWrite={canWrite} canTransfer={canTransfer} fundingScopeComplete={fundingScopeComplete} fundedSourceAvailable={accounts.some((candidate) => candidate.account_id !== selected.account_id && candidate.status === "active" && candidate.currency === "INR" && hasPositiveMinorUnits(candidate.available_minor))} returnTo={accountDetailHref(selected.account_id, filters)} onChanged={onAccountChanged} onRefreshEvidence={onRefreshLifecycleEvidence} />
      <section className="ledger-section" aria-labelledby="account-audit-heading"><div className="section-heading"><div><p className="eyebrow">Permitted audit context</p><h2 id="account-audit-heading">Account audit events</h2><p>Only explicitly persisted account-targeted events are shown; absence is not inferred as approval.</p></div></div>{selected.audit_context?.length ? <div className="ledger-list">{selected.audit_context.map((event) => <article className="ledger-row" key={event.event_id}><div className="ledger-identity"><strong>{event.event_type.replaceAll("_", " ")}</strong><span>{event.actor_subject_id} · {utcDateTime(event.occurred_at)}</span>{event.reason && <span><strong>Audited reason:</strong> {event.reason}</span>}</div><StatusBadge tone={event.outcome === "succeeded" ? "success" : "warning"}>{event.outcome}</StatusBadge><CopyControl value={event.correlation_id} label="Copy audit correlation ID" /></article>)}</div> : <StatePanel title="No account-scoped audit events" message="No account-targeted audit event is available in the current retained evidence. LedgerSync does not invent one from balance or transfer history." />}</section>
      <Link className="text-link back-link" href={detailReturnTo ?? accountDirectoryHref(filters, focusAccountId)}>{detailReturnTo && !detailReturnTo.startsWith("/accounts") ? "← Back to previous view" : "← Back to account directory"}</Link>
    </>}
  </>;
}
