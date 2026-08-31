"use client";

import { useEffect, useState } from "react";

import { ConsoleRouteFrame } from "@/features/console/ConsoleRouteFrame";
import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { PageHeader, StatePanel } from "@/features/console/components";
import { ReconciliationView } from "@/features/reconciliation/ReconciliationViews";
import { useReconciliationWorkspace } from "@/features/reconciliation/useReconciliationWorkspace";

export function ReconciliationController({ runId, returnTo }: Readonly<{ runId?: string; returnTo?: string }>) {
  const { session, online, hasScope } = useConsoleSession();
  const workspace = useReconciliationWorkspace(runId);
  const { loadDetail, loadList } = workspace;
  const [initialEvidenceSettled, setInitialEvidenceSettled] = useState(false);

  useEffect(() => {
    if (!session || !online) return;
    if (!hasScope("reconciliation:read")) return;
    let active = true;
    const timer = window.setTimeout(() => {
      void (runId ? loadDetail(runId) : loadList()).finally(() => {
        if (active) setInitialEvidenceSettled(true);
      });
    }, 0);
    return () => {
      active = false;
      window.clearTimeout(timer);
    };
  }, [hasScope, loadDetail, loadList, online, runId, session]);

  return (
    <ConsoleRouteFrame section="reconciliation" loadingLabel="Reconciliation" pending={hasScope("reconciliation:read") && !initialEvidenceSettled}>
      {session && !hasScope("reconciliation:read") ? (
        <><PageHeader eyebrow="Investigate / Reconciliation" title="Reconciliation" description="Compare stored ledger postings with authoritative balance evidence."/><StatePanel kind="denied" title="Reconciliation read authority required" message="Your server-issued session does not include reconciliation:read. No protected reconciliation request was made."/></>
      ) : session && (
        <ReconciliationView
          runs={workspace.runs}
          detail={workspace.detail}
          detailRequested={Boolean(runId)}
          error={workspace.error}
          loading={runId ? workspace.detailLoading : workspace.listLoading}
          verifiedAt={workspace.verifiedAt}
          nextCursor={workspace.nextCursor}
          tenantId={session.tenant_id}
          csrfToken={session.csrf_token}
          online={online}
          canWrite={hasScope("reconciliation:write")}
          canExport={hasScope("exports:read") && hasScope("reconciliation:read")}
          returnTo={returnTo}
          onObserved={workspace.observe}
          onRefresh={() => runId ? workspace.loadDetail(runId) : workspace.loadList()}
          onMore={() => {
            if (workspace.nextCursor) void workspace.loadList(workspace.nextCursor, true);
          }}
        />
      )}
    </ConsoleRouteFrame>
  );
}
