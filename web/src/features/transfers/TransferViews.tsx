"use client";

import Link from "next/link";
import { useRef, useCallback, useSyncExternalStore, useState } from "react";

import type {
  Account,
  ReconciliationRun,
  TransferDetail,
  TransferSummary,
} from "@/features/accounts/types";
import { CopyControl } from "@/ui/controls/CopyControl.client";
import { FocusedRetry } from "@/ui/controls/FocusedRetry.client";
import { DataTableRegion } from "@/ui/display/DataTableRegion";
import { EvidenceFreshness } from "@/ui/display/Evidence";
import { RecordLink } from "@/ui/display/RecordLink";
import { StatePanel } from "@/ui/display/StatePanel";
import { StatusBadge } from "@/ui/display/StatusBadge";
import { FormField } from "@/ui/forms/FormField.client";
import { TransferCorrectionPanel } from "@/features/corrections/TransferCorrectionPanel";
import { EvidenceExportControl } from "@/features/exports/EvidenceExportControl";
import { TransferEvidenceTimeline } from "@/features/transfers/TransferEvidenceTimeline";
import { RelatedEvidenceRail } from "@/features/investigation/RelatedEvidenceRail";
import { SavedViewCapture } from "@/features/investigation/SavedViewCapture";
import { transferIntentStorageKey } from "./transferIntent";
import { useRouter } from "next/navigation";
import { useExperienceMode } from "@/features/console/ExperienceModeBoundary";
import { SimpleTransferReceipt } from "./SimpleTransferReceipt";
import { TransferForm } from "@/features/transfers/TransferForm";
import { TransferRecovery } from "@/features/transfers/TransferRecovery";
import type { TransferExplainability } from "@/lib/api/orientation";
import { Money } from "@/ui/display/Money";
import { Timestamp } from "@/ui/display/Timestamp";
import { emptyTransferFilters, transferExportQuery, transferURL, type TransferFilters } from "@/lib/page-query/transfers";
import { ActiveFilterSummary } from "@/ui/disclosure/ActiveFilterSummary";
import { AdvancedFilterPanel } from "@/ui/disclosure/AdvancedFilterPanel";
import { DisclosureSection } from "@/ui/disclosure/DisclosureSection";
import { EvidenceDisclosure } from "@/ui/disclosure/EvidenceDisclosure";
import { PrerequisitePanel } from "@/ui/disclosure/PrerequisitePanel";
import { approvedCurrencyGroups, isAuthoritativelyReconciled } from "@/lib/financial-ui";

function financialTone(status: string) {
  return status === "posted"
    ? ("success" as const)
    : status === "rejected"
      ? ("danger" as const)
      : ("warning" as const);
}

export function TransferHistoryState({
  recordCount,
  loading,
  error,
  verifiedAt,
  online,
  onRefresh,
}: Readonly<{
  recordCount: number;
  loading: boolean;
  error: string | null;
  verifiedAt?: string;
  online: boolean;
  onRefresh: () => Promise<void>;
}>) {
  if (recordCount > 0 || error) return null;
  if (!online || (!loading && !verifiedAt)) {
    return <StatePanel
      kind="unknown"
      title="Transfer history unavailable"
      message={online ? "Refresh to load authorized transfer records." : "Connect to load transfer records. An empty history has not been verified."}
      action={online ? <FocusedRetry label="Refresh transfer history" onRetry={() => void onRefresh()} /> : undefined}
    />;
  }
  if (loading) {
    return <StatePanel title="Loading transfer history" message="Transfer records are not yet available." />;
  }
  return <StatePanel title="No transfers found" message="The verified authorized scope and active filters returned no transfer records." />;
}

export function TransferList({
  transfers,
  nextHref,
  exportAction,
  refreshAction,
  freshness,
  returnTo = "/transfers",
  variant = "paged",
  selectedTransferId,
  accounts = [],
}: Readonly<{
  transfers: TransferSummary[];
  nextHref?: string;
  exportAction?: React.ReactNode;
  refreshAction?: React.ReactNode;
  freshness?: React.ReactNode;
  returnTo?: string;
  variant?: "paged" | "recent";
  selectedTransferId?: string;
  accounts?: Account[];
}>) {
  if (!transfers.length)
    return (
      <StatePanel
        title="No transfers found"
        message="No transfer records match this authorized scope and filter."
      />
    );
  return (
    <section className="ledger-section">
      <div className="section-heading">
        <div>
          <h2>
            {variant === "recent"
              ? "Recent transfer records"
              : "Transfer register"}
          </h2>
          <p>
            {variant === "recent"
              ? "Latest five loaded records"
              : `${transfers.length} record${transfers.length === 1 ? "" : "s"} on this page`}
          </p>
        </div>
        <div className="transfer-register-actions">
          {variant === "recent" ? (
            <RecordLink href="/transfers" label="View all transfers" />
          ) : (
            <>
              {freshness}
              {refreshAction}
              {exportAction}
            </>
          )}
        </div>
      </div>
      <DataTableRegion label="Transfer comparison">
        <table className="data-table transfer-table">
          <caption className="sr-only">Authorized transfer records with exact amounts and separate financial and delivery outcomes</caption>
          <thead>
            <tr>
              <th scope="col" className="transfer-row-number">#</th>
              <th scope="col">Transfer ID</th>
              <th scope="col">Source <span aria-hidden="true">→</span> Destination</th>
              <th scope="col" className="number-cell">Amount</th>
              <th scope="col">Posting</th>
              <th scope="col">Delivery</th>
            </tr>
          </thead>
          <tbody>
            {transfers.map((item, index) => {
              const sourceName = accounts.find((account) => account.account_id === item.source_account_id)?.display_name;
              const destinationName = accounts.find((account) => account.account_id === item.destination_account_id)?.display_name;
              return (
              <tr
                key={item.transfer_id}
                className={item.transfer_id === selectedTransferId ? "is-selected" : undefined}
              >
                <td className="transfer-row-number">{index + 1}</td>
                <td>
                  <div className="transfer-id-cell">
                    <Link
                      className="transfer-id-link"
                      href={`/transfers/${encodeURIComponent(item.transfer_id)}?return_to=${encodeURIComponent(returnTo)}`}
                      aria-current={item.transfer_id === selectedTransferId ? "page" : undefined}
                    >
                      {item.transfer_id}
                    </Link>
                  </div>
                </td>
                <td>
                  <div className="transfer-route-cell" aria-label={`From ${item.source_account_id} to ${item.destination_account_id}`}>
                    <span>
                      <Link href={`/accounts/${item.source_account_id}?return_to=${encodeURIComponent(returnTo)}`} title={item.source_account_id} aria-label={`Source account ${sourceName ? `${sourceName} · ` : ""}${item.source_account_id}`}>
                        {sourceName ?? item.source_account_id}
                      </Link>
                    </span>
                    <span className="transfer-route-arrow" aria-hidden="true">→</span>
                    <span>
                      <Link href={`/accounts/${item.destination_account_id}?return_to=${encodeURIComponent(returnTo)}`} title={item.destination_account_id} aria-label={`Destination account ${destinationName ? `${destinationName} · ` : ""}${item.destination_account_id}`}>
                        {destinationName ?? item.destination_account_id}
                      </Link>
                    </span>
                  </div>
                </td>
                <td className="number-cell">
                  <Money currency={item.currency} minorUnits={item.amount_minor} groupThousands />
                </td>
                <td>
                  <StatusBadge tone={financialTone(item.financial_status)}>
                    {item.financial_status}
                  </StatusBadge>
                </td>
                <td>
                  <StatusBadge
                    tone={
                      item.delivery_status === "retrying" ||
                      item.delivery_status === "dead"
                        ? "warning"
                        : "neutral"
                    }
                  >
                    {item.delivery_status}
                  </StatusBadge>
                </td>
              </tr>
              );
            })}
          </tbody>
        </table>
      </DataTableRegion>
      {variant === "paged" && (
        <div className="pagination"><span>{nextHref ? "More matching transfer records are available" : "End of this filtered transfer history"}</span>{nextHref ? <Link className="button secondary" href={nextHref}>Next page</Link> : <button className="button secondary" type="button" disabled>Next page</button>}</div>
      )}
    </section>
  );
}

function TransferProofBand({
  accounts,
  accountsLoading,
  accountsError,
  accountsVerifiedAt,
  accountsScopeComplete,
  canReadAccounts,
  reconciliation,
  reconciliationLoading,
  reconciliationError,
  reconciliationVerifiedAt,
  canReadReconciliation,
  online,
  onRefresh,
}: Readonly<{
  accounts: Account[];
  accountsLoading: boolean;
  accountsError: string | null;
  accountsVerifiedAt?: string;
  accountsScopeComplete: boolean;
  canReadAccounts: boolean;
  reconciliation: ReconciliationRun | null;
  reconciliationLoading: boolean;
  reconciliationError: string | null;
  reconciliationVerifiedAt?: string;
  canReadReconciliation: boolean;
  online: boolean;
  onRefresh: () => Promise<void>;
}>) {
  const balances = approvedCurrencyGroups(accounts);
  const asOf = accounts.map((account) => account.as_of).filter(Boolean).sort().at(0);
  const completeBalance = Boolean(
    canReadAccounts &&
      accounts.length > 0 &&
      accountsScopeComplete &&
      !accountsError &&
      !balances.mixedCurrency &&
      balances.currency,
  );
  const reconciliationPassed = reconciliation
    ? isAuthoritativelyReconciled(reconciliation)
    : false;
  const reconciliationTone = reconciliationPassed
    ? "success"
    : reconciliation?.status === "running"
      ? "warning"
      : reconciliation
        ? "danger"
        : "neutral";

  return (
    <section className="transfer-proof-band" aria-label="Current balances and internal reconciliation">
      <div className="transfer-balance-proof">
        <div className="proof-label-row">
          <h2>{completeBalance && balances.currency ? `Operating balance (${balances.currency})` : "Operating balance"}</h2>
          {accountsVerifiedAt && accounts.length > 0 && (
            <EvidenceFreshness
              state={accountsError || !online ? "historical" : accountsLoading ? "refreshing" : "current"}
              verifiedAt={accountsVerifiedAt}
              label="Account balances"
              reason={accountsError ?? (!online ? "Reconnect before treating this balance as current." : undefined)}
            />
          )}
        </div>
        {completeBalance && balances.currency ? (
          <>
            <strong className="transfer-operating-amount">
              <Money currency={balances.currency} minorUnits={balances.operatingMinor} groupThousands showCurrencyCode={false} />
            </strong>
            <p className="transfer-balance-context">
              As of <Timestamp value={asOf} />
              {balances.customerFundsMinor !== "0" && (
                <span className="separate-funds-note">
                  Customer funds are accounted for separately.
                </span>
              )}
            </p>
          </>
        ) : accountsLoading && accounts.length === 0 ? (
          <p className="proof-empty-state" aria-live="polite">Loading authorized account balances…</p>
        ) : !canReadAccounts ? (
          <p className="proof-empty-state">Account read access is not available for this session.</p>
        ) : accountsError ? (
          <div className="proof-error-copy" role="status">
            <p>Operating balance records are unavailable.</p>
            <details>
              <summary>Request details</summary>
              <p>{accountsError}</p>
            </details>
            <FocusedRetry label="Refresh proof sources" onRetry={() => void onRefresh()} disabled={!online} />
          </div>
        ) : !accountsScopeComplete || balances.mixedCurrency ? (
          <p className="proof-empty-state" role="status">
            Balance totals are unavailable until the authorized account scope is complete and consistent.
          </p>
        ) : (
          <p className="proof-empty-state">No authorized account balance is available.</p>
        )}
      </div>

      <div className="transfer-reconciliation-proof">
        <h2>Latest internal reconciliation</h2>
        {reconciliation ? (
          <>
            <div className="reconciliation-proof-result">
              <StatusBadge tone={reconciliationTone}>
                {reconciliationPassed
                  ? "Balanced"
                  : reconciliation.status === "running"
                    ? "Running"
                    : reconciliation.status === "mismatch"
                      ? "Mismatch found"
                      : "Unavailable"}
              </StatusBadge>
              <RecordLink href={`/reconciliation/${encodeURIComponent(reconciliation.run_id)}`} label="Open result" />
            </div>
            <p className="reconciliation-proof-meta">
              <span>Run <code>{reconciliation.run_id}</code></span>
              <span>{reconciliation.mismatch_count} mismatches</span>
              <span><Timestamp value={reconciliation.completed_at || reconciliation.started_at} /></span>
            </p>
            {reconciliationVerifiedAt && (
              <EvidenceFreshness
                state={reconciliationError || !online ? "historical" : reconciliationLoading ? "refreshing" : "current"}
                verifiedAt={reconciliationVerifiedAt}
                label="Reconciliation result"
                reason={reconciliationError ?? (!online ? "Reconnect before treating the run as current." : undefined)}
              />
            )}
          </>
        ) : reconciliationLoading ? (
          <p className="proof-empty-state" aria-live="polite">Loading reconciliation results…</p>
        ) : reconciliationError ? (
          <div className="proof-error-copy" role="status">
            <p>Reconciliation results are unavailable.</p>
            <details>
              <summary>Request details</summary>
              <p>{reconciliationError}</p>
            </details>
            <FocusedRetry label="Refresh proof sources" onRetry={() => void onRefresh()} disabled={!online} />
          </div>
        ) : !canReadReconciliation ? (
          <p className="proof-empty-state">Reconciliation read access is not available for this session.</p>
        ) : (
          <p className="proof-empty-state">No authoritative reconciliation result is available.</p>
        )}
      </div>
    </section>
  );
}

export function TransferEvidenceFolio({
  detail,
  detailRequested,
  selectedTransferId,
  detailError,
  detailLoading,
  detailVerifiedAt,
  accounts,
  explainability,
  explainabilityLoading,
  explainabilityError,
  online,
  canReadExplainability,
  canReadCorrections,
  canWriteCorrections,
  csrfToken,
  returnTo,
  onRefresh,
  onRefreshExplainability,
}: Readonly<{
  detail: TransferDetail | null;
  detailRequested: boolean;
  selectedTransferId?: string;
  detailError: string | null;
  detailLoading: boolean;
  detailVerifiedAt?: string;
  accounts: Account[];
  explainability: TransferExplainability | null;
  explainabilityLoading: boolean;
  explainabilityError: string | null;
  online: boolean;
  canReadExplainability: boolean;
  canReadCorrections: boolean;
  canWriteCorrections: boolean;
  csrfToken: string;
  returnTo?: string;
  onRefresh: () => Promise<void>;
  onRefreshExplainability: () => void;
}>) {
  const accountName = (accountId: string) =>
    accounts.find((account) => account.account_id === accountId)?.display_name ?? accountId;

  if (!detail) {
    return (
      <aside className="transfer-evidence-folio" aria-label="Selected transfer record">
        <header className="folio-heading">
          <div>
            <h2>Transfer details</h2>
            <p>Read-only record</p>
          </div>
        </header>
        {detailRequested ? (
          detailError ? (
            <StatePanel
              kind="error"
              title="Transfer details unavailable"
              message={detailError}
              action={<FocusedRetry label="Retry this transfer only" onRetry={() => void onRefresh()} disabled={!online} busy={detailLoading} />}
            />
          ) : !online ? (
            <StatePanel kind="unknown" title="Transfer details unavailable" message="Connect to load this transfer's posting and delivery records." />
          ) : detailLoading ? (
            <StatePanel
              title="Loading transfer details"
              message="Financial posting and downstream delivery are verified separately before they are shown."
            />
          ) : (
            <StatePanel kind="unknown" title="Transfer details unavailable" message="Refresh this transfer to load its posting and delivery records." action={<FocusedRetry label="Retry this transfer only" onRetry={() => void onRefresh()} />} />
          )
        ) : selectedTransferId ? (
          <StatePanel
            title="Transfer not found in this view"
            message="The selected transfer is outside the current authorized result or could not be loaded. Refresh the record or clear the filters."
          />
        ) : (
          <div className="folio-empty-state">
            <span className="folio-index-mark" aria-hidden="true">↗</span>
            <h3>Select a transfer</h3>
            <p>Open a transfer ID to see its posting and delivery records.</p>
          </div>
        )}
      </aside>
    );
  }

  const detailHref = `/transfers/${encodeURIComponent(detail.transfer_id)}?return_to=${encodeURIComponent(returnTo ?? "/transfers")}`;
  const statusTone = financialTone(detail.financial_status);
  const deliveryTone = detail.delivery_status === "retrying" || detail.delivery_status === "dead"
    ? "warning"
    : detail.delivery_status === "delivered"
      ? "success"
      : "neutral";

  return (
    <aside className="transfer-evidence-folio" aria-label={`Read-only record for transfer ${detail.transfer_id}`}>
      <header className="folio-heading">
        <div>
          <p className="folio-kicker">Selected record</p>
          <h2>Transfer {detail.transfer_id}</h2>
        </div>
        <div className="header-actions"><span className="folio-readonly-label">Read only</span><button className="button secondary" type="button" disabled={!online || detailLoading} onClick={() => void onRefresh()}>Refresh record</button></div>
      </header>
      {(detailError || !online) && <StatePanel
        kind="unknown"
        title="Transfer details not refreshed"
        message={`Shown posting and delivery states come from the last successful read. ${detailError ?? "Connect to refresh this transfer."}`}
        action={<FocusedRetry label="Retry this transfer only" onRetry={() => void onRefresh()} disabled={!online} busy={detailLoading} />}
        announce="polite"
      />}
      {detailVerifiedAt && <EvidenceFreshness
        state={detailError || !online ? "historical" : detailLoading ? "refreshing" : "current"}
        verifiedAt={detailVerifiedAt}
        label="Transfer record"
        reason={detailError ?? (!online ? "Connect to refresh this record." : undefined)}
      />}
      <div className="folio-id-row">
        <span className="folio-field-label">Immutable transfer ID</span>
        <CopyControl value={detail.transfer_id} />
      </div>
      <div className="folio-state-row">
        <div>
          <span className="folio-field-label">Posting state</span>
          <StatusBadge tone={statusTone}>{detail.financial_status}</StatusBadge>
        </div>
        <div>
          <span className="folio-field-label">Delivery state</span>
          <StatusBadge tone={deliveryTone}>{detail.delivery_status}</StatusBadge>
        </div>
      </div>
      <strong className="folio-amount">
        <Money currency={detail.currency} minorUnits={detail.amount_minor} groupThousands showCurrencyCode={false} />
      </strong>
      <dl className="folio-facts">
        <div>
          <dt>Source account</dt>
          <dd><Link href={`/accounts/${encodeURIComponent(detail.source_account_id)}?return_to=${encodeURIComponent(detailHref)}`}>{accountName(detail.source_account_id)}</Link></dd>
          <dd className="folio-secondary-value">{detail.source_account_id}</dd>
        </div>
        <div>
          <dt>Destination account</dt>
          <dd><Link href={`/accounts/${encodeURIComponent(detail.destination_account_id)}?return_to=${encodeURIComponent(detailHref)}`}>{accountName(detail.destination_account_id)}</Link></dd>
          <dd className="folio-secondary-value">{detail.destination_account_id}</dd>
        </div>
        <div>
          <dt>Created (UTC)</dt>
          <dd><Timestamp value={detail.created_at} /></dd>
        </div>
        <div>
          <dt>Completed (UTC)</dt>
          <dd><Timestamp value={detail.completed_at} /></dd>
        </div>
        {detail.journal_transaction_id && (
          <div>
            <dt>Journal transaction</dt>
            <dd><CopyControl value={detail.journal_transaction_id} /></dd>
          </div>
        )}
      </dl>

      {(detail.delivery_status === "retrying" || detail.delivery_status === "dead") && (
        <StatePanel
          kind="unknown"
          title={detail.financial_status === "posted" ? `Money is posted; delivery is ${detail.delivery_status}` : `Posting is ${detail.financial_status}; delivery is ${detail.delivery_status}`}
          message="Delivery status does not change the posting result."
        />
      )}

      <EvidenceDisclosure
        id="transfer-posting-evidence"
        title="Posting evidence"
        summary={`${detail.postings.length} immutable posting${detail.postings.length === 1 ? "" : "s"}`}
      >
        {detail.postings.length ? (
          <div className="folio-postings">
            {detail.postings.map((posting) => (
              <article className="posting-row" key={posting.posting_id}>
                <StatusBadge tone={posting.direction === "credit" ? "success" : "neutral"}>{posting.direction}</StatusBadge>
                <Link href={`/accounts/${encodeURIComponent(posting.account_id)}?return_to=${encodeURIComponent(detailHref)}`}>{accountName(posting.account_id)}</Link>
                <strong><Money currency={posting.currency} minorUnits={posting.amount_minor} /></strong>
                <Timestamp value={posting.occurred_at} inheritTypography={false} />
              </article>
            ))}
          </div>
        ) : (
          <StatePanel title="No postings" message="No ledger postings are recorded for this transfer." />
        )}
      </EvidenceDisclosure>

      <DisclosureSection
        id="transfer-corrections"
        title="Corrections and recovery"
        summary="Review available correction evidence and permitted next steps."
        lazy
      >
        <TransferCorrectionPanel
          transfer={detail}
          csrfToken={csrfToken}
          online={online}
          canRead={canReadCorrections}
          canWrite={canWriteCorrections}
        />
      </DisclosureSection>
      <DisclosureSection
        id="transfer-evidence-timeline"
        title="Evidence timeline and delivery"
        summary="Follow the request, journal, balance versions, delivery, and balance-check results."
        attention={Boolean(explainabilityError)}
        defaultOpen={Boolean(explainabilityError)}
        lazy
      >
        <TransferEvidenceTimeline
          evidence={explainability}
          loading={explainabilityLoading}
          error={explainabilityError}
          online={online}
          canRead={canReadExplainability}
          transferId={detail.transfer_id}
          backTo={detailHref}
          onRefresh={onRefreshExplainability}
        />
      </DisclosureSection>
      <DisclosureSection id="transfer-related-evidence" title="Related investigation evidence" summary="Open linked records and case context." lazy>
        <RelatedEvidenceRail sourceType="transfer" sourceId={detail.transfer_id} />
      </DisclosureSection>
      <Link className="text-link back-link" href={returnTo ?? "/transfers"}>{returnTo && !returnTo.startsWith("/transfers") ? "← Back to previous view" : "← Back to transfer history"}</Link>
    </aside>
  );
}

export function TransfersView({
  accounts,
  accountsLoading,
  accountsError,
  accountsVerifiedAt,
  accountsScopeComplete,
  canReadAccounts,
  transfers,
  transfersVerifiedAt,
  detail,
  detailError,
  detailLoading,
  detailVerifiedAt,
  detailRequested,
  selectedTransferId,
  reconciliation,
  reconciliationLoading,
  reconciliationError,
  reconciliationVerifiedAt,
  canReadReconciliation,
  explainability,
  explainabilityLoading,
  explainabilityError,
  error,
  loading,
  nextCursor,
  online,
  canWrite,
  canCreateAccounts = false,
  canExport,
  canReadExplainability,
  canReadCorrections,
  canWriteCorrections,
  writeUnavailableReason,
  tenantId,
  csrfToken,
  preferredDestinationId,
  returnTo,
  initialFilters = emptyTransferFilters,
  onApplyFilters,
  onClearFilters,
  onRefreshAccounts,
  onRefreshHistory,
  onRefreshDetail,
  onRefresh,
  onRefreshExplainability,
}: Readonly<{
  accounts: Account[];
  accountsLoading: boolean;
  accountsError: string | null;
  accountsVerifiedAt?: string;
  accountsScopeComplete: boolean;
  canReadAccounts: boolean;
  transfers: TransferSummary[];
  transfersVerifiedAt?: string;
  detail: TransferDetail | null;
  detailError: string | null;
  detailLoading: boolean;
  detailVerifiedAt?: string;
  detailRequested: boolean;
  selectedTransferId?: string;
  reconciliation: ReconciliationRun | null;
  reconciliationLoading: boolean;
  reconciliationError: string | null;
  reconciliationVerifiedAt?: string;
  canReadReconciliation: boolean;
  explainability: TransferExplainability | null;
  explainabilityLoading: boolean;
  explainabilityError: string | null;
  error: string | null;
  loading: boolean;
  nextCursor?: string;
  online: boolean;
  canWrite: boolean;
  canCreateAccounts?: boolean;
  canExport: boolean;
  canReadExplainability: boolean;
  canReadCorrections: boolean;
  canWriteCorrections: boolean;
  writeUnavailableReason?: string;
  tenantId: string;
  csrfToken: string;
  preferredDestinationId?: string;
  returnTo?: string;
  initialFilters?: TransferFilters;
  onApplyFilters: (filters: TransferFilters) => void;
  onClearFilters: () => void;
  onRefreshAccounts: () => Promise<void>;
  onRefreshHistory: () => Promise<void>;
  onRefreshDetail: () => Promise<void>;
  onRefresh: () => Promise<void>;
  onRefreshExplainability: () => void;
}>) {
  const { mode } = useExperienceMode();
  const router = useRouter();
  const subscribeIntent = useCallback((notify: () => void) => { window.addEventListener("ledgersync-transfer-intent", notify); window.addEventListener("storage", notify); return () => { window.removeEventListener("ledgersync-transfer-intent", notify); window.removeEventListener("storage", notify); }; }, []);
  const intentSnapshot = useCallback(() => { try { return sessionStorage.getItem(transferIntentStorageKey(tenantId)); } catch { return "storage-unavailable"; } }, [tenantId]);
  const retainedRequest = useSyncExternalStore(subscribeIntent, intentSnapshot, () => null);
  const [createOpen, setCreateOpen] = useState(Boolean(preferredDestinationId));
  const createDisclosure = useRef<HTMLDetailsElement>(null);
  const openTransferForm = () => {
    setCreateOpen(true);
    window.requestAnimationFrame(() => createDisclosure.current?.querySelector("summary")?.focus());
  };
  const [query, setQuery] = useState(initialFilters.query);
  const [accountId, setAccountId] = useState(initialFilters.accountId);
  const [status, setStatus] = useState<TransferFilters["status"]>(initialFilters.status);
  const [fromDate, setFromDate] = useState(initialFilters.from.slice(0, 10));
  const [toDate, setToDate] = useState(initialFilters.to.slice(0, 10));
  const eligibleAccounts = accounts.filter((account) => account.status === "active" && account.currency === "INR");
  const exportQuery = transferExportQuery(initialFilters);
  const exportFilters = [
    ...(initialFilters.query ? [{ label: "Search", value: initialFilters.query }] : []),
    ...(initialFilters.accountId ? [{ label: "Account ID", value: initialFilters.accountId }] : []),
    ...(initialFilters.status
      ? [{ label: "Financial status", value: initialFilters.status }]
      : []),
    ...(initialFilters.from ? [{ label: "From UTC", value: initialFilters.from }] : []),
    ...(initialFilters.to ? [{ label: "To UTC", value: initialFilters.to }] : []),
  ];
  const historyReturn = transferURL(initialFilters);
  const nextHref = nextCursor ? transferURL({ ...initialFilters, cursor: nextCursor }) : undefined;
  const selectedEvidence = detailRequested ? (<TransferEvidenceFolio
          detail={detail}
          detailRequested={detailRequested}
          selectedTransferId={selectedTransferId}
          detailError={detailError}
          detailLoading={detailLoading}
          detailVerifiedAt={detailVerifiedAt}
          accounts={accounts}
          explainability={explainability}
          explainabilityLoading={explainabilityLoading}
          explainabilityError={explainabilityError}
          online={online}
          canReadExplainability={canReadExplainability}
          canReadCorrections={canReadCorrections}
          canWriteCorrections={canWriteCorrections}
          csrfToken={csrfToken}
          returnTo={returnTo}
          onRefresh={onRefreshDetail}
          onRefreshExplainability={onRefreshExplainability}
        />) : null;
  if (mode === "simple" && detailRequested && detail) return <SimpleTransferReceipt detail={detail} verifiedAt={detailVerifiedAt} historical={Boolean(detailError)||!online} returnTo={returnTo ?? historyReturn} evidence={selectedEvidence} />;
  return (
    <>
      <header className="transfer-page-heading">
        <div>
          <h1>Transfers</h1>
          <p>Move money between your INR accounts.</p>
        </div>
        {canWrite && !createOpen && <button className="button primary" type="button" onClick={openTransferForm} disabled={!online}>New transfer</button>}
      </header>
      {retainedRequest ? <StatePanel kind="unknown" title="An earlier transfer needs checking" message="Keep the original request. Check its result before creating another transfer." action={<Link className="button primary" href={`/transfers/new?return_to=${encodeURIComponent(historyReturn)}`}>Review original request</Link>} /> : <button className="button primary" type="button" disabled={!online || !canWrite} onClick={() => router.push(`/transfers/new?return_to=${encodeURIComponent(historyReturn)}`)}>Make a transfer</button>}
      <TransferRecovery />
      <TransferProofBand
        accounts={accounts}
        accountsLoading={accountsLoading}
        accountsError={accountsError}
        accountsVerifiedAt={accountsVerifiedAt}
        accountsScopeComplete={accountsScopeComplete}
        canReadAccounts={canReadAccounts}
        reconciliation={reconciliation}
        reconciliationLoading={reconciliationLoading}
        reconciliationError={reconciliationError}
        reconciliationVerifiedAt={reconciliationVerifiedAt}
        canReadReconciliation={canReadReconciliation}
        online={online}
        onRefresh={onRefresh}
      />
      <details
        className="transfer-create-disclosure"
        id="create-transfer"
        ref={createDisclosure}
        open={createOpen}
        onToggle={(event) => setCreateOpen(event.currentTarget.open)}
      >
        <summary>Create a transfer</summary>
        <div className="transfer-create-content">
          {!canReadAccounts ? (
            <StatePanel kind="denied" title="Account access required" message="Your session cannot load accounts for a transfer. Ask an administrator for account read access." />
          ) : !online && !accountsVerifiedAt ? (
            <StatePanel kind="unknown" title="Account picker unavailable" message="Connect to load the authorized source and destination accounts." />
          ) : eligibleAccounts.length >= 2 || accountsLoading || accountsError || preferredDestinationId || !accountsVerifiedAt ? (
            <TransferForm
              accounts={accounts}
              accountsLoading={accountsLoading}
              accountsError={accountsError}
              accountsVerifiedAt={accountsVerifiedAt}
              tenantId={tenantId}
              csrfToken={csrfToken}
              disabled={!online || !canWrite}
              disabledReason={!online
                ? "Transfer posting is disabled while offline. Reconnect before submitting or retrying a request."
                : (writeUnavailableReason ?? (!canWrite ? "Read-only role: transfer posting is not permitted." : undefined))}
              preferredDestinationId={preferredDestinationId}
              returnTo={returnTo}
              onRetryAccounts={onRefreshAccounts}
              onPosted={onRefresh}
            />
          ) : (
            <PrerequisitePanel
              title={eligibleAccounts.length === 0 ? "Create two active INR accounts" : "Create one more active INR account"}
              message="A transfer needs an eligible source and a different eligible destination. LedgerSync will not open a transfer intent until both are available."
              action={canCreateAccounts ? <Link className="button secondary" href="/accounts/new?return_to=%2Ftransfers">Create account</Link> : undefined}
            />
          )}
          <DisclosureSection id="transfer-trust-rules" title="Why this transfer is safe" summary="Review authorization, exact-money, retry, and atomic-commit protections.">
            <ol>
              <li>Only authorized active accounts are selectable.</li>
              <li>Amounts use exact integer minor units.</li>
              <li>Retries retain the same idempotency key.</li>
              <li>Ledger, balance, audit, and outbox commit together.</li>
            </ol>
            {!canWrite && <p className="permission-note">{writeUnavailableReason ?? "Read-only role: transfer posting is not permitted."}</p>}
          </DisclosureSection>
        </div>
      </details>

      <div className={`transfer-register-layout${detailRequested ? "" : " transfer-register-only"}`}>
        {selectedEvidence}
        <div className="transfer-register-column">
      <details className="transfer-filter-disclosure">
        <summary>
          Filter history
          {exportFilters.length > 0 && <span>{exportFilters.length} active filters</span>}
        </summary>
        <div className="transfer-filter-disclosure-content">
      <ActiveFilterSummary filters={exportFilters} clearHref="/transfers" />
      <form
        className="transfer-filter-form"
        aria-label="Transfer filters"
        onSubmit={(event) => {
          event.preventDefault();
          onApplyFilters({
            query: query.trim().toLowerCase(),
            accountId: accountId.trim().toLowerCase(),
            status,
            from: fromDate ? `${fromDate}T00:00:00.000Z` : "",
            to: toDate ? `${toDate}T23:59:59.999Z` : "",
          });
        }}
      >
        <FormField label="Search transfers" requirement="optional" hint="Search by transfer or account ID."><input
            type="search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Transfer or account ID"
            maxLength={128}
            pattern="[0-9A-Fa-f-]*"
            title="Use a complete or partial transfer or account identifier"
          /></FormField>
        <AdvancedFilterPanel id="transfer-advanced-filters" activeCount={exportFilters.length - (initialFilters.query ? 1 : 0)}>
          <FormField label="Exact account ID" requirement="optional" hint="Matches either side of the transfer. Select a known account or paste its full ID."><input value={accountId} onChange={(event) => setAccountId(event.target.value)} list="transfer-account-options" maxLength={36} pattern="[0-9A-Fa-f-]{36}" placeholder="00000000-0000-0000-0000-000000000000" /></FormField>
          <datalist id="transfer-account-options">{accounts.map((account) => <option key={account.account_id} value={account.account_id}>{account.display_name ?? "Authorized account"}</option>)}</datalist>
          <FormField label="Financial status" requirement="optional"><select
            value={status}
            onChange={(event) => setStatus(event.target.value as TransferFilters["status"])}
          >
            <option value="">All statuses</option>
            <option value="pending">Pending</option>
            <option value="posted">Posted</option>
            <option value="rejected">Rejected</option>
          </select></FormField>
          <FormField label="From date (UTC)" requirement="optional" hint="Inclusive start of day in UTC."><input type="date" value={fromDate} onChange={(event) => setFromDate(event.target.value)} /></FormField>
          <FormField label="To date (UTC)" requirement="optional" hint="Inclusive end of day in UTC."><input type="date" value={toDate} min={fromDate || undefined} onChange={(event) => setToDate(event.target.value)} /></FormField>
        </AdvancedFilterPanel>
        <button
          className="button primary"
          type="submit"
          disabled={!online || loading}
        >
          Apply filters
        </button>
        <button className="button secondary" type="button" disabled={loading} onClick={onClearFilters}>
          Clear filters
        </button>
      </form>
      <SavedViewCapture domain="transfers" filters={{ q: initialFilters.query || undefined, accountId: initialFilters.accountId || undefined, status: initialFilters.status || undefined, from: initialFilters.from || undefined, to: initialFilters.to || undefined }} />
      <p className="filter-scope-note">
        Filters apply server-side to the bounded authorized history and remain
        active across pagination and export.
      </p>
        </div>
      </details>

          {error && (
            <div className="transfer-history-error" role="alert">
              <div className="transfer-history-error-copy">
                <span className="error-state-mark" aria-hidden="true">!</span>
                <div>
                  <h3>Transfer history unavailable</h3>
                  <p>Authorized transfer records could not be refreshed.</p>
                  <details>
                    <summary>Request details</summary>
                    <p>{error}</p>
                  </details>
                </div>
              </div>
              <FocusedRetry label="Retry transfer history only" onRetry={() => void onRefreshHistory()} disabled={!online} busy={loading} />
            </div>
          )}
          <TransferHistoryState recordCount={transfers.length} loading={loading} error={error} verifiedAt={transfersVerifiedAt} online={online} onRefresh={onRefreshHistory} />
          {transfers.length === 0 && <div className="transfer-register-actions"><button className="button secondary" type="button" disabled={!online || loading} onClick={() => void onRefreshHistory()}>Refresh history</button></div>}
          {transfers.length > 0 ? (
            <TransferList
              transfers={transfers}
              accounts={accounts}
              freshness={transfersVerifiedAt && transfers.length > 0 ? (
                <EvidenceFreshness
                  state={error || !online ? "historical" : loading ? "refreshing" : "current"}
                  verifiedAt={transfersVerifiedAt}
                  label="Transfer history"
                  reason={error ?? (!online ? "Reconnect before treating history as current." : undefined)}
                />
              ) : undefined}
              selectedTransferId={detail?.transfer_id ?? selectedTransferId}
              nextHref={nextHref}
              returnTo={historyReturn}
              refreshAction={<button className="button secondary" type="button" disabled={!online || loading} onClick={() => void onRefreshHistory()}>Refresh history</button>}
              exportAction={
                <EvidenceExportControl
                  label="Export transfer details"
                  subject="transfer history"
                  endpoint={`/api/exports/transfers.csv?${exportQuery}`}
                  scope="Server-filtered authorized transfer history"
                  filters={exportFilters}
                  columns="Includes full transfer and account identifiers plus exact quoted minor-unit strings"
                  online={online}
                  canExport={canExport}
                />
              }
            />
          ) : null}
        </div>
      </div>


    </>
  );
}
