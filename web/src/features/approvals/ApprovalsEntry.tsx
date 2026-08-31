"use client";

import { useRouter } from "next/navigation";
import { useEffect, useMemo } from "react";

import { ApprovalFiltersForm, ApprovalHeader, ApprovalList } from "@/features/approvals/ApprovalViews";
import { approvalQuery, approvalURL, useApprovalWorkspace } from "@/features/approvals/useApprovalWorkspace";
import { ConsoleRouteFrame } from "@/features/console/ConsoleRouteFrame";
import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { deriveConsoleCapabilities } from "@/features/console/capabilities";
import { StatePanel } from "@/features/console/components";
import type { ApprovalFilters } from "@/lib/api/approvals";
import { emptyApprovalFilters } from "@/lib/api/approvals";

export function ApprovalsEntry({ filters, invalidQuery = false }: Readonly<{ filters: ApprovalFilters; invalidQuery?: boolean }>) {
  const router = useRouter();
  const { session, online } = useConsoleSession();
  const capabilities = deriveConsoleCapabilities(session);
  const canOpen = capabilities.fundingApprove || capabilities.correctionsApprove;
  const { items, pageCount, nextCursor, loading, error, denied, verifiedAt, load } = useApprovalWorkspace();
  const filterKey = useMemo(() => approvalQuery(filters), [filters]);

  useEffect(() => {
    if (!session || !online || !canOpen || invalidQuery) return;
    void load(filters);
  }, [canOpen, filterKey, filters, invalidQuery, load, online, session]);

  const returnTo = approvalURL(filters);
  const nextHref = nextCursor
    ? approvalURL({ ...filters, cursor: nextCursor })
    : undefined;

  return (
    <ConsoleRouteFrame section="approvals" loadingLabel="Approvals" pending={loading}>
      <ApprovalHeader verifiedAt={verifiedAt} loading={loading} error={error} />
      {!canOpen ? (
        <StatePanel
          kind="denied"
          title="Approval authority required"
          message="Your server-issued session has no funding or correction approval scope. No protected approval request was made."
        />
      ) : invalidQuery ? <StatePanel kind="error" title="Invalid approval investigation URL" message="The shared URL contains an unknown, repeated, empty, oversized, or incompatible filter. No protected approval request was made." action={<button className="button secondary" type="button" onClick={() => router.replace("/approvals")}>Clear invalid filters</button>} /> : !online && items.length === 0 ? <StatePanel kind="offline" title="Approval evidence unavailable offline" message="Reconnect to request the tenant-scoped queue. No empty queue is inferred." /> : <>
        <ApprovalFiltersForm
          key={filterKey}
          filters={filters}
          capabilities={capabilities}
          busy={loading}
          onApply={(next) => router.push(approvalURL(next))}
          onClear={() => router.push(approvalURL(emptyApprovalFilters))}
        />
        {denied ? <StatePanel kind="denied" title="Approval query denied" message="The server rejected this approval scope or domain. No empty queue is inferred." /> : null}
        {error ? <StatePanel kind="error" title="Approval evidence unavailable" message={error} /> : null}
        {!denied && !error ? <ApprovalList items={items} pageCount={pageCount} nextHref={nextHref} returnTo={returnTo} loading={loading} /> : null}
      </>}
    </ConsoleRouteFrame>
  );
}
