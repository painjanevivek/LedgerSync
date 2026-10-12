"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { FormEvent, useEffect, useState } from "react";

import { BalanceStatus } from "@/features/accounts/BalanceStatus";
import { AccountLifecycleActions } from "@/features/accounts/AccountLifecycleActions";
import { hasPositiveMinorUnits } from "@/features/accounts/accountCommandIntent";
import { TransactionLedger } from "@/features/accounts/TransactionLedger";
import type { Account, AccountBalance, Transaction } from "@/features/accounts/types";
import { CopyControl } from "@/ui/controls/CopyControl.client";
import { FocusedRetry } from "@/ui/controls/FocusedRetry.client";
import { DataTableRegion } from "@/ui/display/DataTableRegion";
import { EvidenceFreshness } from "@/ui/display/Evidence";
import { PageHeader } from "@/ui/display/PageHeader";
import { RecordLink } from "@/ui/display/RecordLink";
import { StatePanel } from "@/ui/display/StatePanel";
import { StatusBadge } from "@/ui/display/StatusBadge";
import { FormField } from "@/ui/forms/FormField.client";
import { accountLabel } from "@/features/console/format";
import { EvidenceExportControl } from "@/features/exports/EvidenceExportControl";
import { RelatedEvidenceRail } from "@/features/investigation/RelatedEvidenceRail";
import { SavedViewCapture } from "@/features/investigation/SavedViewCapture";
import { Money } from "@/ui/display/Money";
import { Timestamp } from "@/ui/display/Timestamp";
import { accountDetailURL, accountDirectoryURL, type AccountFilters } from "@/lib/page-query/accounts";
import { ActiveFilterSummary } from "@/ui/disclosure/ActiveFilterSummary";
import { AdvancedFilterPanel } from "@/ui/disclosure/AdvancedFilterPanel";
import { DisclosureSection } from "@/ui/disclosure/DisclosureSection";

export type { AccountFilters } from "@/lib/page-query/accounts";

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
  return accountDirectoryURL(filters, focusAccountId);
}

export function accountDetailHref(accountID: string, filters: AccountFilters): string {
  return accountDetailURL(accountID, filters);
}

export function AccountsView({ accounts, selected, detailRequested, balance, transactions, balanceLoading, historyLoading, directoryLoading, balanceError, historyError, balanceVerifiedAt, historyVerifiedAt, directoryVerifiedAt, error, online, filters, nextCursor, historyNextCursor, focusAccountId, tenantId, csrfToken, canWrite, canTransfer, canExport, fundingScopeComplete, detailReturnTo, onRefresh, onHistoryNext, onRefreshBalance, onRefreshHistory, onAccountChanged, onRefreshLifecycleEvidence }: Props) {
  const router = useRouter();
  const [query, setQuery] = useState(filters.query);
  const [status, setStatus] = useState(filters.status);
  const [category, setCategory] = useState(filters.category);

  useEffect(() => {
    if (!selected && !directoryLoading && focusAccountId) document.getElementById(`account-link-${focusAccountId}`)?.focus();
  }, [directoryLoading, focusAccountId, selected]);

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    router.push(accountDirectoryURL({ query: query.trim(), status, category }));
  }

  return <>
    <PageHeader eyebrow="Ledger / Accounts" title={selected ? accountLabel(selected) : detailRequested ? "Account details" : "Accounts"} description={selected ? "See this account’s balance, details, and transaction history." : detailRequested ? "Loading this account’s details." : "Find and manage the accounts you are allowed to use."}>
      <div className="header-actions"><button className="button secondary" type="button" onClick={onRefresh} disabled={!online || directoryLoading}>Refresh accounts</button>{!selected && !detailRequested && canWrite && <Link className="button primary guarded-control" href={`/accounts/new?return_to=${encodeURIComponent(accountDirectoryHref(filters))}`}>Create account</Link>}</div>
    </PageHeader>
    {error && <StatePanel kind="error" title="Accounts unavailable" message={error} action={<FocusedRetry label={detailRequested ? "Retry this account only" : "Retry account directory only"} onRetry={onRefresh} disabled={!online} busy={directoryLoading} />} />}
    {detailRequested && !selected && !error && <StatePanel title="Loading account details" message="Balance and immutable history are verified independently before they are presented as current." />}
    {!detailRequested && !selected && <>
      {directoryVerifiedAt && accounts.length > 0 && <EvidenceFreshness state={error || !online ? "historical" : directoryLoading ? "refreshing" : "current"} verifiedAt={directoryVerifiedAt} label="Account directory" reason={error ?? (!online ? "Reconnect before relying on directory balances." : undefined)} />}
      {directoryLoading && accounts.length === 0 ? <StatePanel title="Loading authorized accounts" message="LedgerSync is requesting one bounded page from the authoritative account directory." /> : accounts.length === 0 && !error ? <StatePanel title={filters.query || filters.status || filters.category ? "No matching accounts" : "No accounts yet"} message={filters.query || filters.status || filters.category ? "Clear or change the filters. LedgerSync does not broaden the authorized account scope." : "Create your first account to begin the ledger. It opens at an exact zero balance."} action={!filters.query && !filters.status && !filters.category && canWrite ? <Link className="button primary" href="/accounts/new">Create your first account</Link> : undefined} /> : accounts.length > 0 && <section className="ledger-section account-directory" aria-labelledby="accounts-heading" aria-busy={directoryLoading}>
        <div className="section-heading"><div><p className="eyebrow">Oldest created first</p><h2 id="accounts-heading">Available balances</h2><p>{accounts.length} account{accounts.length === 1 ? "" : "s"} on this page. A total is not calculated or implied.</p></div></div>
        <DataTableRegion label="Authorized account comparison"><table className="data-table"><thead><tr><th>Account</th><th>Category</th><th>Status</th><th>Available balance</th><th>Version / as of</th><th>Action</th></tr></thead><tbody>{accounts.map((account) => <tr key={account.account_id}><td><strong>{accountLabel(account)}</strong><code>{account.account_id}</code></td><td>{account.category?.replaceAll("_", " ") ?? "Unclassified"}</td><td><StatusBadge tone={tone(account.status)}>{account.status}</StatusBadge></td><td className="number-cell"><Money currency={account.currency} minorUnits={account.available_minor} /></td><td><code>v{account.version}</code><small><Timestamp value={account.as_of} /></small></td><td><RecordLink id={`account-link-${account.account_id}`} href={accountDetailHref(account.account_id, filters)} label="Open account" /></td></tr>)}</tbody></table></DataTableRegion>
        <div className="pagination"><span>{nextCursor ? "More account records are available" : "End of available accounts"}</span>{nextCursor && !directoryLoading ? <Link className="button secondary" href={accountDirectoryURL({ ...filters, cursor: nextCursor })}>Next page</Link> : <button className="button secondary" type="button" disabled>Next page</button>}</div>
      </section>}
      {(accounts.length > 0 || Boolean(filters.query || filters.status || filters.category)) && <>
        <ActiveFilterSummary filters={[...(filters.query ? [{ label: "Search", value: filters.query }] : []), ...(filters.status ? [{ label: "Status", value: filters.status }] : []), ...(filters.category ? [{ label: "Category", value: filters.category.replaceAll("_", " ") }] : [])]} clearHref="/accounts" />
        <form className="filter-bar" role="search" onSubmit={submit}>
          <FormField label="Search accounts" requirement="optional" hint="Search by name, reference, or account ID."><input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Example: ACME-OPERATING-01" maxLength={128} /></FormField>
          <AdvancedFilterPanel id="account-advanced-filters" activeCount={Number(Boolean(filters.status)) + Number(Boolean(filters.category))}>
            <FormField label="Status" requirement="optional"><select value={status} onChange={(event) => setStatus(event.target.value as AccountFilters["status"])}><option value="">All statuses</option><option value="active">Active</option><option value="frozen">Frozen</option><option value="closed">Closed</option></select></FormField>
            <FormField label="Category" requirement="optional"><select value={category} onChange={(event) => setCategory(event.target.value as AccountFilters["category"])}><option value="">All categories</option><option value="operating">Operating</option><option value="customer_funds">Customer funds</option><option value="payroll">Payroll</option><option value="payables">Payables</option><option value="expenses">Expenses</option><option value="reserve">Reserve</option></select></FormField>
          </AdvancedFilterPanel>
          <button className="button primary" type="submit" disabled={!online || directoryLoading}>Apply filters</button>
          <Link className="button secondary" href="/accounts">Clear filters</Link>
          <span aria-live="polite">{error ? "Authorized page count unavailable" : `${accounts.length} authorized account${accounts.length === 1 ? "" : "s"} on this page; no total is implied.`}</span>
        </form>
        <DisclosureSection id="account-saved-views" title="Saved account views" summary="Store this authorized filter combination for investigation work." lazy><SavedViewCapture domain="accounts" filters={{ status: filters.status || undefined, category: filters.category || undefined }} /></DisclosureSection>
      </>}
    </>}
    {selected && <>
      <section className="identity-strip"><div><span>Account ID</span><CopyControl value={selected.account_id} /></div><div><span>External reference</span><strong>{selected.external_reference || "Not set"}</strong></div><div><span>Category</span><strong>{selected.category?.replaceAll("_", " ") || "Unclassified"}</strong></div><div><span>Status</span><StatusBadge tone={tone(selected.status)}>{selected.status}</StatusBadge></div></section>
      {selected.status === "frozen" && <StatePanel kind="unknown" title="Account is frozen" message="The balance remains visible, but this account cannot be selected for a new debit or credit." />}
      <BalanceStatus currency={balance?.currency ?? selected.currency} availableMinor={balance?.available_minor} version={balance?.version} asOf={balance?.as_of} verifiedAt={balanceVerifiedAt} loading={balanceLoading} error={balanceError} onRetry={onRefreshBalance} retryDisabled={!online} />
      <AccountLifecycleActions account={selected} balance={balance} balanceLoading={balanceLoading} balanceError={balanceError} tenantId={tenantId} csrfToken={csrfToken} online={online} canWrite={canWrite} canTransfer={canTransfer} fundingScopeComplete={fundingScopeComplete} fundedSourceAvailable={accounts.some((candidate) => candidate.account_id !== selected.account_id && candidate.status === "active" && candidate.currency === "INR" && hasPositiveMinorUnits(candidate.available_minor))} returnTo={accountDetailHref(selected.account_id, filters)} onChanged={onAccountChanged} onRefreshEvidence={onRefreshLifecycleEvidence} />
      <DisclosureSection id="account-transactions" title="Transactions and export" summary="Open the immutable ledger history for this account." defaultOpen={Boolean(historyError)} attention={Boolean(historyError)} lazy><TransactionLedger transactions={transactions} account={selected} loading={historyLoading} error={historyError} verifiedAt={historyVerifiedAt} nextCursor={historyNextCursor} onNext={onHistoryNext} onRetry={onRefreshHistory} retryDisabled={!online} exportAction={<EvidenceExportControl label="Export ledger history" subject="account ledger history" endpoint={`/api/exports/accounts/${encodeURIComponent(selected.account_id)}/transactions.csv?limit=10000`} scope={`One authorized account · ${selected.account_id}`} filters={[]} columns="Includes the full account, transfer, journal, and posting identifiers with exact quoted minor-unit strings" online={online} canExport={canExport}/>}/></DisclosureSection>
      <DisclosureSection id="account-related-evidence" title="Related evidence" summary="Investigate records linked to this account." lazy><RelatedEvidenceRail sourceType="account" sourceId={selected.account_id} /></DisclosureSection>
      <DisclosureSection id="account-audit-history" title="Configuration and audit history" summary="Review retained account-targeted events." lazy><section className="ledger-section" aria-labelledby="account-audit-heading"><div className="section-heading"><div><p className="eyebrow">Permitted audit context</p><h2 id="account-audit-heading">Account audit events</h2><p>Only explicitly persisted account-targeted events are shown; absence is not inferred as approval.</p></div></div>{selected.audit_context?.length ? <div className="ledger-list">{selected.audit_context.map((event) => <article className="ledger-row" key={event.event_id}><div className="ledger-identity"><strong>{event.event_type.replaceAll("_", " ")}</strong><span>{event.actor_subject_id} · <Timestamp value={event.occurred_at} /></span>{event.reason && <span><strong>Audited reason:</strong> {event.reason}</span>}</div><StatusBadge tone={event.outcome === "succeeded" ? "success" : "warning"}>{event.outcome}</StatusBadge><CopyControl value={event.correlation_id} label="Copy audit correlation ID" /></article>)}</div> : <StatePanel title="No account-scoped audit events" message="No account-targeted audit event is available in the current retained evidence. LedgerSync does not invent one from balance or transfer history." />}</section></DisclosureSection>
      <Link className="text-link back-link" href={detailReturnTo ?? accountDirectoryHref(filters, focusAccountId)}>{detailReturnTo && !detailReturnTo.startsWith("/accounts") ? "← Back to previous view" : "← Back to account directory"}</Link>
    </>}
  </>;
}
