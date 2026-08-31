"use client";

import Link from "next/link";

import { ConsoleRouteFrame } from "@/features/console/ConsoleRouteFrame";
import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { deriveConsoleCapabilities } from "@/features/console/capabilities";
import { PageHeader, StatePanel } from "@/features/console/components";

/**
 * Safe Phase 6 route boundary. Phase 7 replaces these source-workspace links
 * with the bounded, server-owned Approval Inbox query.
 */
export function ApprovalsEntry() {
  const { session } = useConsoleSession();
  const capabilities = deriveConsoleCapabilities(session);
  const canApproveFunding = capabilities.fundingApprove;
  const canApproveCorrections = capabilities.correctionsApprove;
  const canOpen = canApproveFunding || canApproveCorrections;

  return (
    <ConsoleRouteFrame section="approvals" loadingLabel="Approvals">
      <PageHeader
        eyebrow="Work / Independent decisions"
        title="Approvals"
        description="Review work that requires an authorized decision without mixing it with posting authority."
      />
      {!canOpen ? (
        <StatePanel
          kind="denied"
          title="Approval authority required"
          message="Your server-issued session has no funding or correction approval scope. No protected approval request was made."
        />
      ) : (
        <section className="ledger-section" aria-labelledby="approval-sources-heading">
          <div className="section-heading">
            <div>
              <p className="eyebrow">Authorized sources</p>
              <h2 id="approval-sources-heading">Open the relevant decision workspace</h2>
              <p>
                Until the unified inbox contract is released, LedgerSync keeps each decision in its authoritative domain workspace.
              </p>
            </div>
          </div>
          <div className="action-row">
            {canApproveFunding ? <Link className="button primary" href="/funding">Review funding decisions</Link> : null}
            {canApproveCorrections ? <Link className="button secondary" href="/corrections">Review correction decisions</Link> : null}
          </div>
        </section>
      )}
    </ConsoleRouteFrame>
  );
}
