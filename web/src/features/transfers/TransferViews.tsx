"use client";

import Link from "next/link";
import { useState } from "react";

import type {
  Account,
  TransferDetail,
  TransferSummary,
} from "@/features/accounts/types";
import { CopyControl } from "@/ui/controls/CopyControl.client";
import { FocusedRetry } from "@/ui/controls/FocusedRetry.client";
import { DataTableRegion } from "@/ui/display/DataTableRegion";
import { EvidenceFreshness } from "@/ui/display/Evidence";
import { PageHeader } from "@/ui/display/PageHeader";
import { RecordLink } from "@/ui/display/RecordLink";
import { StatePanel } from "@/ui/display/StatePanel";
import { StatusBadge } from "@/ui/display/StatusBadge";
import { FormField } from "@/ui/forms/FormField.client";
import { TransferCorrectionPanel } from "@/features/corrections/TransferCorrectionPanel";
import { EvidenceExportControl } from "@/features/exports/EvidenceExportControl";
import { TransferEvidenceTimeline } from "@/features/transfers/TransferEvidenceTimeline";
import { RelatedEvidenceRail } from "@/features/investigation/RelatedEvidenceRail";
import { TransferForm } from "@/features/transfers/TransferForm";
import type { TransferExplainability } from "@/lib/api/orientation";
import { Money } from "@/ui/display/Money";
import { Timestamp } from "@/ui/display/Timestamp";
import { emptyTransferFilters, transferExportQuery, transferURL, type TransferFilters } from "@/lib/page-query/transfers";

function financialTone(status: string) {
  return status === "posted"
    ? ("success" as const)
    : status === "rejected"
      ? ("danger" as const)
      : ("warning" as const);
}

export function TransferList({
  transfers,
  nextHref,
  exportAction,
  returnTo = "/transfers",
  variant = "paged",
}: Readonly<{
  transfers: TransferSummary[];
  nextHref?: string;
  exportAction?: React.ReactNode;
  returnTo?: string;
  variant?: "paged" | "recent";
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
          <p className="eyebrow">
            {variant === "recent"
              ? "Recent immutable activity"
              : "Immutable history"}
          </p>
          <h2>
            {variant === "recent"
              ? "Latest transfer records"
              : "Transfer records"}
          </h2>
          <p>
            {variant === "recent"
              ? "The latest five loaded records are shown; this is not the end of transfer history."
              : `${transfers.length} record${transfers.length === 1 ? "" : "s"} on this page. A total is not calculated or implied. Financial and downstream delivery outcomes remain separate.`}
          </p>
        </div>
        {variant === "recent" ? (
          <RecordLink href="/transfers" label="View all transfers" />
        ) : (
          exportAction
        )}
      </div>
      <DataTableRegion label="Transfer comparison">
        <table className="data-table transfer-table">
          <thead>
            <tr>
              <th>Transfer</th>
              <th>Route</th>
              <th>Exact amount</th>
              <th>Financial status</th>
              <th>Delivery</th>
              <th>Completed UTC</th>
              <th>Action</th>
            </tr>
          </thead>
          <tbody>
            {transfers.map((item) => (
              <tr key={item.transfer_id}>
                <td>
                  <CopyControl value={item.transfer_id} />
                </td>
                <td>
                  <Link
                    href={`/accounts/${item.source_account_id}`}
                    aria-label={`Source account ${item.source_account_id}`}
                  >
                    {item.source_account_id.slice(0, 8)}…
                  </Link>
                  <span aria-hidden="true"> → </span>
                  <Link
                    href={`/accounts/${item.destination_account_id}`}
                    aria-label={`Destination account ${item.destination_account_id}`}
                  >
                    {item.destination_account_id.slice(0, 8)}…
                  </Link>
                </td>
                <td className="number-cell">
                  <Money currency={item.currency} minorUnits={item.amount_minor} />
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
                <td><Timestamp value={item.completed_at} /></td>
                <td>
                  <RecordLink
                    href={`/transfers/${item.transfer_id}?return_to=${encodeURIComponent(returnTo)}`}
                    label="Open record"
                  />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </DataTableRegion>
      {variant === "paged" && (
        <div className="pagination"><span>{nextHref ? "More matching transfer records are available" : "End of this filtered transfer history"}</span>{nextHref ? <Link className="button secondary" href={nextHref}>Next page</Link> : <button className="button secondary" type="button" disabled>Next page</button>}</div>
      )}
    </section>
  );
}

export function TransfersView({
  accounts,
  accountsLoading,
  accountsError,
  accountsVerifiedAt,
  transfers,
  transfersVerifiedAt,
  detail,
  detailRequested,
  explainability,
  explainabilityLoading,
  explainabilityError,
  error,
  loading,
  nextCursor,
  online,
  canWrite,
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
  onRefresh,
  onRefreshExplainability,
}: Readonly<{
  accounts: Account[];
  accountsLoading: boolean;
  accountsError: string | null;
  accountsVerifiedAt?: string;
  transfers: TransferSummary[];
  transfersVerifiedAt?: string;
  detail: TransferDetail | null;
  detailRequested: boolean;
  explainability: TransferExplainability | null;
  explainabilityLoading: boolean;
  explainabilityError: string | null;
  error: string | null;
  loading: boolean;
  nextCursor?: string;
  online: boolean;
  canWrite: boolean;
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
  onRefresh: () => Promise<void>;
  onRefreshExplainability: () => void;
}>) {
  const [query, setQuery] = useState(initialFilters.query);
  const [accountId, setAccountId] = useState(initialFilters.accountId);
  const [status, setStatus] = useState<TransferFilters["status"]>(initialFilters.status);
  const [fromDate, setFromDate] = useState(initialFilters.from.slice(0, 10));
  const [toDate, setToDate] = useState(initialFilters.to.slice(0, 10));
  if (detail) {
    const detailHref = `/transfers/${encodeURIComponent(detail.transfer_id)}?return_to=${encodeURIComponent(returnTo ?? "/transfers")}`;
    return (
      <>
        <PageHeader
          eyebrow="Money movement / Immutable record"
          title="Transfer detail"
          description="The committed financial result is permanent; delivery evidence is shown separately."
        />
        <section className="identity-strip">
          <div>
            <span>Transfer ID</span>
            <CopyControl value={detail.transfer_id} />
          </div>
          <div>
            <span>Financial status</span>
            <StatusBadge tone={financialTone(detail.financial_status)}>
              {detail.financial_status}
            </StatusBadge>
          </div>
          <div>
            <span>Delivery status</span>
            <StatusBadge
              tone={
                detail.delivery_status === "retrying" ||
                detail.delivery_status === "dead"
                  ? "warning"
                  : "neutral"
              }
            >
              {detail.delivery_status}
            </StatusBadge>
          </div>
          <div>
            <span>Completed</span>
            <strong><Timestamp value={detail.completed_at} /></strong>
          </div>
        </section>
        {(detail.delivery_status === "retrying" ||
          detail.delivery_status === "dead") && (
          <StatePanel
            kind="unknown"
            title={`Money is posted; delivery is ${detail.delivery_status}`}
            message="The double-entry ledger is complete. Downstream delivery is tracked separately from financial posting."
          />
        )}
        <section className="surface detail-document">
          <p className="eyebrow">Exact transfer facts</p>
          <strong className="detail-amount">
            <Money currency={detail.currency} minorUnits={detail.amount_minor} />
          </strong>
          <dl className="evidence-list">
            <div>
              <dt>Source account</dt>
              <dd>
                <Link
                  href={`/accounts/${detail.source_account_id}?return_to=${encodeURIComponent(detailHref)}`}
                >
                  {detail.source_account_id}
                </Link>
              </dd>
            </div>
            <div>
              <dt>Destination account</dt>
              <dd>
                <Link
                  href={`/accounts/${detail.destination_account_id}?return_to=${encodeURIComponent(detailHref)}`}
                >
                  {detail.destination_account_id}
                </Link>
              </dd>
            </div>
            <div>
              <dt>Journal transaction</dt>
              <dd>
                {detail.journal_transaction_id ? (
                  <CopyControl value={detail.journal_transaction_id} />
                ) : (
                  "No journal created"
                )}
              </dd>
            </div>
            <div>
              <dt>Created</dt>
              <dd><Timestamp value={detail.created_at} /></dd>
            </div>
          </dl>
        </section>
        <section className="ledger-section">
          <div className="section-heading">
            <div>
              <p className="eyebrow">Double-entry proof</p>
              <h2>Ledger postings</h2>
            </div>
          </div>
          {detail.postings.length ? (
            detail.postings.map((posting) => (
              <article className="posting-row" key={posting.posting_id}>
                <StatusBadge
                  tone={posting.direction === "credit" ? "success" : "neutral"}
                >
                  {posting.direction}
                </StatusBadge>
                <Link
                  href={`/accounts/${posting.account_id}?return_to=${encodeURIComponent(detailHref)}`}
                >
                  {posting.account_id}
                </Link>
                <strong>
                  <Money currency={posting.currency} minorUnits={posting.amount_minor} />
                </strong>
                <Timestamp value={posting.occurred_at} inheritTypography={false} />
              </article>
            ))
          ) : (
            <StatePanel
              title="No postings"
              message="Rejected transfers do not create ledger postings."
            />
          )}
        </section>
        <RelatedEvidenceRail sourceType="transfer" sourceId={detail.transfer_id} />
        <TransferCorrectionPanel
          transfer={detail}
          csrfToken={csrfToken}
          online={online}
          canRead={canReadCorrections}
          canWrite={canWriteCorrections}
        />
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
        <Link className="text-link back-link" href={returnTo ?? "/transfers"}>
          ← Back to previous view
        </Link>
      </>
    );
  }
  if (detailRequested)
    return (
      <>
        <PageHeader
          eyebrow="Money movement / Immutable record"
          title="Transfer detail"
          description="Loading the requested immutable transfer details."
        />
        {error ? (
          <StatePanel
            kind="error"
            title="Transfer details unavailable"
            message={error}
          />
        ) : (
          <StatePanel
            title="Loading transfer details"
            message="Ledger posting and delivery states are verified separately before display."
          />
        )}
      </>
    );
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
  return (
    <>
      <PageHeader
          eyebrow="Ledger / Transfers"
        title="Transfers"
          description="Move an exact amount between your accounts, then check the result."
      />
      <div className="two-column-layout">
        <TransferForm
          accounts={accounts}
          accountsLoading={accountsLoading}
          accountsError={accountsError}
          accountsVerifiedAt={accountsVerifiedAt}
          tenantId={tenantId}
          csrfToken={csrfToken}
          disabled={!online || !canWrite}
          disabledReason={
            !online
              ? "Transfer posting is disabled while offline. Reconnect before submitting or retrying a request."
              : (writeUnavailableReason ??
                (!canWrite
                  ? "Read-only role: transfer posting is not permitted."
                  : undefined))
          }
          preferredDestinationId={preferredDestinationId}
          returnTo={returnTo}
          onRetryAccounts={onRefreshAccounts}
          onPosted={onRefresh}
        />
        <aside className="guardrail-panel">
          <p className="eyebrow">Trust controls</p>
          <h2>One intent, one movement</h2>
          <ol>
            <li>Only authorized active accounts are selectable.</li>
            <li>Amounts use exact integer minor units.</li>
            <li>Retries retain the same idempotency key.</li>
            <li>Ledger, balance, audit, and outbox commit together.</li>
          </ol>
          {!canWrite && (
            <p className="permission-note">
              {writeUnavailableReason ??
                "Read-only role: transfer posting is not permitted."}
            </p>
          )}
        </aside>
      </div>
      <form
        className="surface list-filter-bar transfer-filters"
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
        <button
          className="button secondary"
          type="button"
          disabled={!online || loading}
          onClick={() => void onRefresh()}
        >
          Refresh history
        </button>
      </form>
      <p className="filter-scope-note">
        Filters apply server-side to the bounded authorized history and remain
        active across pagination and export.
      </p>
      {error && (
        <StatePanel
          kind="error"
          title="Transfer history unavailable"
          message={error}
          action={
            <FocusedRetry
              label="Retry transfer history only"
              onRetry={() => void onRefresh()}
              disabled={!online}
              busy={loading}
            />
          }
        />
      )}{" "}
      {transfersVerifiedAt && transfers.length > 0 && (
        <EvidenceFreshness
          state={
            error || !online ? "historical" : loading ? "refreshing" : "current"
          }
          verifiedAt={transfersVerifiedAt}
          label="Transfer history"
          reason={
            error ??
            (!online
              ? "Reconnect before treating history as current."
              : undefined)
          }
        />
      )}{" "}
      {loading && transfers.length === 0 ? (
        <StatePanel
          title="Loading transfer history"
          message="Immutable transfer records are loading from the authorized ledger scope. No empty history is inferred."
        />
      ) : !error && transfers.length === 0 ? (
        <StatePanel
          title="No transfers found"
          message="The verified authorized scope and active filters returned no transfer records."
        />
      ) : (
        transfers.length > 0 && (
          <TransferList
            transfers={transfers}
            nextHref={nextHref}
            returnTo={historyReturn}
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
        )
      )}
    </>
  );
}
