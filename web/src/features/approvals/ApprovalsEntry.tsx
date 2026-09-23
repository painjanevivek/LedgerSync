"use client";

import { useRouter } from "next/navigation";
import { useEffect, useMemo } from "react";

import { ApprovalFiltersForm, ApprovalHeader, ApprovalList } from "@/features/approvals/ApprovalViews";
import { approvalQuery, approvalURL, useApprovalWorkspace } from "@/features/approvals/useApprovalWorkspace";
import { ConsoleRouteFrame } from "@/features/console/ConsoleRouteFrame";
import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { deriveConsoleCapabilities } from "@/features/console/capabilities";
import { SavedViewCapture } from "@/features/investigation/SavedViewCapture";
import { StatePanel } from "@/ui/display/StatePanel";
import { ActiveFilterSummary } from "@/ui/disclosure/ActiveFilterSummary";
import { AdvancedFilterPanel } from "@/ui/disclosure/AdvancedFilterPanel";
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
  const activeFilters = [
    ...(filters.domain ? [{ label: "Domain", value: filters.domain }] : []),
    ...(filters.status ? [{ label: "Status", value: filters.status }] : []),
    ...(filters.requester ? [{ label: "Requester", value: filters.requester }] : []),
    ...(filters.age ? [{ label: "Age", value: filters.age.replaceAll("_", " ") }] : []),
    ...(filters.requestedAfter ? [{ label: "From", value: filters.requestedAfter }] : []),
    ...(filters.requestedBefore ? [{ label: "Through", value: filters.requestedBefore }] : []),
    ...(filters.actionableByMe ? [{ label: "Queue", value: "Actionable by me" }] : []),
  ];

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
        {denied ? <StatePanel kind="denied" title="Approval query denied" message="The server rejected this approval scope or domain. No empty queue is inferred." /> : null}
        {error ? <StatePanel kind="error" title="Approval evidence unavailable" message={error} /> : null}
        {!denied && !error ? <ApprovalList items={items} pageCount={pageCount} nextHref={nextHref} returnTo={returnTo} loading={loading} /> : null}
        <ActiveFilterSummary filters={activeFilters} clearHref={approvalURL(emptyApprovalFilters)} />
        <AdvancedFilterPanel id="approval-advanced-filters" title="Advanced filters" activeCount={activeFilters.length} invalid={invalidQuery}>
          <ApprovalFiltersForm
            key={filterKey}
            filters={filters}
            capabilities={capabilities}
            busy={loading}
            onApply={(next) => router.push(approvalURL(next))}
            onClear={() => router.push(approvalURL(emptyApprovalFilters))}
          />
          <SavedViewCapture domain="approvals" filters={{ domain: filters.domain || undefined, status: filters.status || undefined, age: filters.age || undefined, requested_after: filters.requestedAfter || undefined, requested_before: filters.requestedBefore || undefined, actionable_by_me: filters.actionableByMe ? "true" : undefined }} />
        </AdvancedFilterPanel>
      </>}
    </ConsoleRouteFrame>
  );
}
