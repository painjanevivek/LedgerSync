"use client";

import Link from "next/link";

import type { TransferSummary } from "@/features/accounts/types";
import { transferStatusPresentation } from "@/features/console/presentation";
import { useExperienceMode } from "@/features/console/ExperienceModeBoundary";
import { CopyControl } from "@/ui/controls/CopyControl.client";
import { DataTableRegion } from "@/ui/display/DataTableRegion";
import { Money } from "@/ui/display/Money";
import { RecordLink } from "@/ui/display/RecordLink";
import { StatePanel } from "@/ui/display/StatePanel";
import { StatusBadge } from "@/ui/display/StatusBadge";
import { Timestamp } from "@/ui/display/Timestamp";
import { RecordIdentity } from "@/ui/presentation/RecordIdentity";
import { RelativeTime } from "@/ui/presentation/RelativeTime";
import { TechnicalDetails } from "@/ui/presentation/TechnicalDetails";

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
  const { mode } = useExperienceMode();
  if (!transfers.length)
    return (
      <StatePanel
        title="No transfers found"
        message="No transfer records match this authorized scope and filter."
      />
    );
  if (mode === "simple") return (
    <section className="simple-record-section" aria-labelledby="simple-transfer-history">
      <div className="simple-section-heading"><div><h2 id="simple-transfer-history">{variant === "recent" ? "Latest transfer records" : "Transfer history"}</h2><p>{variant === "recent" ? "Your most recent transfers. More history remains available." : `${transfers.length} ${transfers.length === 1 ? "transfer" : "transfers"} shown on this page.`}</p></div>{variant === "recent" ? <RecordLink href="/transfers" label="View all transfers" /> : exportAction}</div>
      <ul className="simple-record-list">
        {transfers.map((item) => {
          const presentation = transferStatusPresentation(item.financial_status);
          return <li key={item.transfer_id} data-tone={presentation.tone}>
            <article>
              <div className="simple-record-main"><div><strong>{presentation.title}</strong><span><RelativeTime value={item.completed_at || item.created_at} /></span></div><Money currency={item.currency} minorUnits={item.amount_minor} /></div>
              <p>{presentation.explanation}</p>
              <div className="simple-record-actions"><Link className="button secondary" href={`/transfers/${item.transfer_id}?return_to=${encodeURIComponent(returnTo)}`}>{presentation.attention ? "Check status" : "View transfer"}</Link><TechnicalDetails><RecordIdentity label="Transfer reference" value={item.transfer_id} /><p>Delivery: {item.delivery_status.replaceAll("_", " ")}.</p></TechnicalDetails></div>
            </article>
          </li>;
        })}
      </ul>
      {variant === "paged" && <div className="pagination"><span>{nextHref ? "More transfers are available" : "You have reached the end of this history"}</span>{nextHref ? <Link className="button secondary" href={nextHref}>Next page</Link> : null}</div>}
    </section>
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
