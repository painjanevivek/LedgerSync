"use client";

import { WarningCircle } from "@phosphor-icons/react";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";

import {
  AccountsView,
  type AccountFilters,
} from "@/features/accounts/AccountViews";
import { AccountCreateFlow } from "@/features/accounts/AccountCreateFlow";
import { hasPositiveMinorUnits } from "@/features/accounts/accountCommandIntent";
import type {
  ConsoleSession,
  ReconciliationRun,
  TransferDetail,
  TransferSummary,
} from "@/features/accounts/types";
import {
  emptyAccountFilters,
  useAccountWorkspace,
} from "@/features/accounts/useAccountWorkspace";
import {
  ConsoleFooter,
  ConsoleShell,
  type ConsoleSection,
} from "@/features/console/ConsoleShell";
import { PageHeader, StatePanel } from "@/features/console/components";
import { OverviewView } from "@/features/overview/OverviewView";
import { ReconciliationView } from "@/features/reconciliation/ReconciliationViews";
import { TransfersView } from "@/features/transfers/TransferViews";
import { readJSON, unavailableMessage, writeJSON } from "@/lib/api/client";
import type {
  LocalOrientation,
  OperatorPreferenceStepID,
  TransferExplainability,
} from "@/lib/api/orientation";

type Props = Readonly<{
  initialSection?: ConsoleSection;
  initialAccountId?: string;
  initialTransferId?: string;
  initialReconciliationRunId?: string;
  initialReconciliationReturnTo?: string;
  initialAccountFilters?: AccountFilters;
  initialAccountFocusId?: string;
  initialAccountCreate?: boolean;
  initialAccountReturnTo?: string;
  initialTransferDestinationId?: string;
  initialTransferReturnTo?: string;
  initialTransferFilters?: { query: string; status: string };
  initialShowOrientation?: boolean;
}>;
type TransfersPayload = { transfers?: TransferSummary[]; next_cursor?: string };
type RunsPayload = { runs?: ReconciliationRun[]; next_cursor?: string };

export function OperatorConsole({
  initialSection = "overview",
  initialAccountId,
  initialTransferId,
  initialReconciliationRunId,
  initialReconciliationReturnTo,
  initialAccountFilters = emptyAccountFilters,
  initialAccountFocusId,
  initialAccountCreate = false,
  initialAccountReturnTo = "/accounts",
  initialTransferDestinationId,
  initialTransferReturnTo,
  initialTransferFilters,
  initialShowOrientation = false,
}: Props) {
  const router = useRouter();
  const [session, setSession] = useState<ConsoleSession | null>(null);
  const [sessionError, setSessionError] = useState<string | null>(null);
  const accountWorkspace = useAccountWorkspace(
    initialAccountId,
    initialAccountFilters,
  );
  const {
    load: loadAccounts,
    loadDetail: loadAccountDetail,
    loadBalance: loadAccountBalance,
    loadHistory: loadAccountHistory,
  } = accountWorkspace;
  const [transfers, setTransfers] = useState<TransferSummary[]>([]);
  const [transferDetail, setTransferDetail] = useState<TransferDetail | null>(
    null,
  );
  const [runs, setRuns] = useState<ReconciliationRun[]>([]);
  const [runDetail, setRunDetail] = useState<ReconciliationRun | null>(null);
  const [loading, setLoading] = useState(true);
  const [initialEvidenceSettled, setInitialEvidenceSettled] = useState(false);
  const [transfersLoading, setTransfersLoading] = useState(false);
  const [transferDetailLoading, setTransferDetailLoading] = useState(false);
  const [runsLoading, setRunsLoading] = useState(false);
  const [runDetailLoading, setRunDetailLoading] = useState(false);
  const [transferError, setTransferError] = useState<string | null>(null);
  const [reconciliationError, setReconciliationError] = useState<string | null>(
    null,
  );
  const [transferCursor, setTransferCursor] = useState<string>();
  const [runCursor, setRunCursor] = useState<string>();
  const [transfersVerifiedAt, setTransfersVerifiedAt] = useState<string>();
  const [reconciliationVerifiedAt, setReconciliationVerifiedAt] =
    useState<string>();
  const [online, setOnline] = useState(true);
  const [orientation, setOrientation] = useState<LocalOrientation | null>(null);
  const [orientationLoading, setOrientationLoading] = useState(false);
  const [orientationError, setOrientationError] = useState<string | null>(null);
  const [orientationPreferenceSaving, setOrientationPreferenceSaving] =
    useState(false);
  const [orientationPreferenceError, setOrientationPreferenceError] = useState<
    string | null
  >(null);
  const orientationPreferenceInFlight = useRef(false);
  const [explainability, setExplainability] =
    useState<TransferExplainability | null>(null);
  const [explainabilityLoading, setExplainabilityLoading] = useState(false);
  const [explainabilityError, setExplainabilityError] = useState<string | null>(
    null,
  );
  const transferFilterQuery = initialTransferFilters?.query ?? "";
  const transferFilterStatus = initialTransferFilters?.status ?? "all";

  const loadTransfers = useCallback(
    async (cursor?: string, append = false) => {
      setTransfersLoading(true);
      const query = new URLSearchParams({ limit: "25" });
      if (cursor) query.set("cursor", cursor);
      if (transferFilterQuery) query.set("q", transferFilterQuery);
      if (transferFilterStatus !== "all")
        query.set("status", transferFilterStatus);
      const response = await readJSON<TransfersPayload>(
        `/api/transfers?${query}`,
      );
      if (!response.ok)
        setTransferError(
          unavailableMessage(
            response.status,
            "transfer records",
            response.requestReference,
          ),
        );
      else {
        const items = Array.isArray(response.data.transfers)
          ? response.data.transfers
          : [];
        setTransfers((current) => (append ? [...current, ...items] : items));
        setTransferCursor(response.data.next_cursor || undefined);
        setTransfersVerifiedAt(new Date().toISOString());
        setTransferError(null);
      }
      setTransfersLoading(false);
    },
    [transferFilterQuery, transferFilterStatus],
  );

  const loadTransferDetail = useCallback(async (id: string) => {
    setTransferDetailLoading(true);
    const response = await readJSON<TransferDetail>(
      `/api/transfers/${encodeURIComponent(id)}`,
    );
    if (response.ok && response.data.transfer_id) {
      setTransferDetail(response.data);
      setTransferError(null);
    } else
      setTransferError(
        unavailableMessage(
          response.status,
          "transfer evidence",
          response.requestReference,
        ),
      );
    setTransferDetailLoading(false);
  }, []);

  const loadOrientation = useCallback(async () => {
    setOrientationLoading(true);
    const response = await readJSON<LocalOrientation>("/api/local/orientation");
    if (response.ok && Array.isArray(response.data.steps)) {
      setOrientation(response.data);
      setOrientationError(null);
    } else
      setOrientationError(
        unavailableMessage(
          response.status,
          "local orientation evidence",
          response.requestReference,
        ),
      );
    setOrientationLoading(false);
  }, []);

  const updateOrientationPreferences = useCallback(
    async (
      change: Readonly<{
        dismissed: boolean;
        completedStepIDs: OperatorPreferenceStepID[];
      }>,
    ) => {
      if (!session || !orientation || orientationPreferenceInFlight.current)
        return false;
      orientationPreferenceInFlight.current = true;
      setOrientationPreferenceSaving(true);
      setOrientationPreferenceError(null);
      const response = await writeJSON<LocalOrientation>(
        "/api/local/orientation/preferences",
        "PUT",
        session.csrf_token,
        {
          expected_version: orientation.preference_version,
          dismissed: change.dismissed,
          completed_step_ids: change.completedStepIDs,
        },
      );
      if (response.ok && Array.isArray(response.data.steps)) {
        setOrientation(response.data);
        setOrientationError(null);
      } else {
        const responseUnknown =
          response.status === 0 ||
          response.errorCode === "upstream_timeout" ||
          response.errorCode === "temporary_unavailable";
        if (responseUnknown || response.status === 409) await loadOrientation();
        const action =
          response.status === 409
            ? "The preference changed in another session, so the latest server state was loaded."
            : responseUnknown
              ? "The response was unknown, so current server state was refreshed without assuming the change succeeded."
              : "The previous server-owned preference was preserved.";
        setOrientationPreferenceError(
          `${action} Request reference: ${response.requestReference}.`,
        );
      }
      orientationPreferenceInFlight.current = false;
      setOrientationPreferenceSaving(false);
      return response.ok;
    },
    [loadOrientation, orientation, session],
  );

  const loadExplainability = useCallback(async (id: string) => {
    setExplainabilityLoading(true);
    const response = await readJSON<TransferExplainability>(
      `/api/transfers/${encodeURIComponent(id)}/explainability`,
    );
    if (response.ok && Array.isArray(response.data.stages)) {
      setExplainability(response.data);
      setExplainabilityError(null);
    } else
      setExplainabilityError(
        response.status === 404
          ? `The linked timeline was not found in this authorized tenant scope. Request reference: ${response.requestReference}.`
          : unavailableMessage(
              response.status,
              "stored evidence timeline",
              response.requestReference,
            ),
      );
    setExplainabilityLoading(false);
  }, []);

  const loadRuns = useCallback(async (cursor?: string, append = false) => {
    setRunsLoading(true);
    const response = await readJSON<RunsPayload>(
      `/api/reconciliation/runs?limit=25${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`,
    );
    if (!response.ok)
      setReconciliationError(
        unavailableMessage(
          response.status,
          "authoritative reconciliation evidence",
          response.requestReference,
        ),
      );
    else {
      const items = Array.isArray(response.data.runs) ? response.data.runs : [];
      setRuns((current) => (append ? [...current, ...items] : items));
      setRunCursor(response.data.next_cursor || undefined);
      setReconciliationVerifiedAt(new Date().toISOString());
      setReconciliationError(null);
    }
    setRunsLoading(false);
  }, []);

  const loadRunDetail = useCallback(async (id: string) => {
    setRunDetailLoading(true);
    const response = await readJSON<ReconciliationRun>(
      `/api/reconciliation/runs/${encodeURIComponent(id)}`,
    );
    if (response.ok && response.data.run_id) {
      setRunDetail(response.data);
      setReconciliationError(null);
    } else
      setReconciliationError(
        unavailableMessage(
          response.status,
          "the selected reconciliation evidence",
          response.requestReference,
        ),
      );
    setRunDetailLoading(false);
  }, []);

  const observeRun = useCallback(
    (run: ReconciliationRun) => {
      setRuns((current) => [
        run,
        ...current.filter((candidate) => candidate.run_id !== run.run_id),
      ]);
      if (initialReconciliationRunId === run.run_id) setRunDetail(run);
      setReconciliationError(null);
    },
    [initialReconciliationRunId],
  );

  const refresh = useCallback(async () => {
    const directory =
      initialSection === "accounts" &&
      !initialAccountId &&
      !initialAccountCreate;
    if (initialSection === "accounts") {
      await Promise.all([
        loadAccounts(
          directory ? accountWorkspace.filters : emptyAccountFilters,
          directory ? 25 : 100,
        ),
        initialAccountId
          ? loadAccountDetail(initialAccountId)
          : Promise.resolve(),
      ]);
      return;
    }
    if (initialSection === "overview")
      await Promise.all([
        loadAccounts(emptyAccountFilters, 100),
        loadTransfers(),
        loadRuns(),
      ]);
  }, [
    accountWorkspace.filters,
    initialAccountCreate,
    initialAccountId,
    initialSection,
    loadAccountDetail,
    loadAccounts,
    loadRuns,
    loadTransfers,
  ]);

  useEffect(() => {
    let active = true;
    (async () => {
      const response = await readJSON<ConsoleSession>("/api/session");
      if (active && response.ok && response.data.tenant_id) {
        setSession(response.data);
        setSessionError(null);
        setLoading(false);
        const directory =
          initialSection === "accounts" &&
          !initialAccountId &&
          !initialAccountCreate;
        const canExplain = [
          "explainability:read",
          "transfers:read",
          "events:read",
          "reconciliation:read",
        ].every((scope) => response.data.scopes.includes(scope));
        if (initialSection === "overview")
          await Promise.all([
            loadAccounts(emptyAccountFilters, 100),
            loadTransfers(),
            loadRuns(),
            response.data.environment === "demo" &&
            response.data.scopes.includes("local:read")
              ? loadOrientation()
              : Promise.resolve(),
          ]);
        if (initialSection === "accounts")
          await Promise.all([
            loadAccounts(
              directory ? initialAccountFilters : emptyAccountFilters,
              directory ? 25 : 100,
            ),
            initialAccountId
              ? loadAccountDetail(initialAccountId)
              : Promise.resolve(),
          ]);
        if (initialSection === "transfers")
          await Promise.all([
            loadAccounts(emptyAccountFilters, 100),
            initialTransferId
              ? loadTransferDetail(initialTransferId)
              : loadTransfers(),
            initialTransferId && canExplain
              ? loadExplainability(initialTransferId)
              : Promise.resolve(),
          ]);
        if (initialSection === "reconciliation")
          await (initialReconciliationRunId
            ? loadRunDetail(initialReconciliationRunId)
            : loadRuns());
        if (active) setInitialEvidenceSettled(true);
      }
      if (active && (!response.ok || !response.data.tenant_id))
        setSessionError(
          response.status === 401
            ? null
            : unavailableMessage(
                response.status,
                "the authorized session",
                response.requestReference,
              ),
        );
      if (active) setLoading(false);
    })();
    return () => {
      active = false;
    };
  }, [
    initialAccountCreate,
    initialAccountFilters,
    initialAccountId,
    initialReconciliationRunId,
    initialSection,
    initialTransferId,
    loadAccountDetail,
    loadAccounts,
    loadExplainability,
    loadOrientation,
    loadRunDetail,
    loadRuns,
    loadTransferDetail,
    loadTransfers,
  ]);

  useEffect(() => {
    const update = () => setOnline(navigator.onLine);
    update();
    window.addEventListener("online", update);
    window.addEventListener("offline", update);
    return () => {
      window.removeEventListener("online", update);
      window.removeEventListener("offline", update);
    };
  }, []);

  async function signOut() {
    if (!session) return;
    await fetch("/api/auth/sign-out", {
      method: "POST",
      headers: { "X-CSRF-Token": session.csrf_token },
    });
    router.refresh();
  }

  if (loading) {
    const title =
      initialSection === "accounts"
        ? "Accounts"
        : initialSection === "transfers"
          ? "Transfers"
          : initialSection === "reconciliation"
            ? "Reconciliation"
            : "Overview";
    return (
      <ConsoleShell
        section={initialSection}
        tenantLabel="Verifying tenant"
        tenantMeta="Secure session"
        environmentLabel="Checking environment"
        operatorLabel="Verifying operator"
        operatorMeta="Authorization pending"
      >
        <div className="session-loading" aria-busy="true">
          <PageHeader
            eyebrow={`${title} · LedgerSync operator workspace`}
            title="Verifying access"
            description="Verifying the authorized tenant scope before financial evidence is displayed."
          />
          <StatePanel
            title="Loading verified evidence"
            message="Balances, transfer history, and reconciliation evidence are loading from their authoritative sources."
          />
        </div>
        <ConsoleFooter pending />
      </ConsoleShell>
    );
  }
  if (!session)
    return (
      <main className="boot-screen">
        <p className="eyebrow">Access not verified</p>
        <h1>Operator workspace unavailable</h1>
        <StatePanel
          kind={sessionError ? "error" : "denied"}
          title={
            sessionError
              ? "Session evidence unavailable"
              : "No authorized session"
          }
          message={
            sessionError ??
            "Configure the approved OIDC provider, or explicitly enable the isolated local demo environment. No financial data is displayed."
          }
        />
      </main>
    );

  return (
    <ConsoleShell
      section={initialSection}
      tenantLabel={session.tenant_label ?? "Ledger tenant"}
      tenantMeta={session.tenant_id}
      environmentLabel={
        session.environment === "demo" ? "Isolated demo" : "Verified production"
      }
      operatorLabel={session.operator_label ?? session.subject_id}
      operatorMeta={
        session.environment === "demo"
          ? "Non-production data"
          : "Authorized operator"
      }
      preview={session.environment === "demo"}
      onSignOut={() => void signOut()}
    >
      {!online && (
        <div className="offline-banner" role="status">
          <WarningCircle weight="fill" aria-hidden="true" />
          <span>
            <strong>You are offline.</strong> Writes are disabled and no
            unverified result is shown.
          </span>
        </div>
      )}
      {initialSection === "overview" && (
        <OverviewView
          accounts={accountWorkspace.accounts}
          transfers={transfers}
          reconciliation={runs[0] ?? null}
          accountsLoading={accountWorkspace.directoryLoading}
          transfersLoading={transfersLoading}
          reconciliationLoading={runsLoading}
          accountsError={
            accountWorkspace.error ??
            (!accountWorkspace.scopeComplete
              ? "The authorized account scope exceeds one bounded page. Previously verified accounts remain visible, but totals are partial and cannot be treated as tenant-wide."
              : null)
          }
          transfersError={transferError}
          reconciliationError={reconciliationError}
          accountsVerifiedAt={accountWorkspace.directoryVerifiedAt}
          transfersVerifiedAt={transfersVerifiedAt}
          reconciliationVerifiedAt={reconciliationVerifiedAt}
          online={online}
          orientation={orientation}
          orientationLoading={orientationLoading}
          orientationError={orientationError}
          orientationPreferenceError={orientationPreferenceError}
          orientationPreferenceSaving={orientationPreferenceSaving}
          canReadOrientation={
            session.environment === "demo" &&
            session.scopes.includes("local:read")
          }
          canWriteOrientation={
            session.environment === "demo" &&
            session.scopes.includes("local:write")
          }
          localDemo={session.environment === "demo"}
          forceOrientation={initialShowOrientation}
          onRefreshAccounts={() => void loadAccounts(emptyAccountFilters, 100)}
          onRefreshTransfers={() => void loadTransfers()}
          onRefreshReconciliation={() => void loadRuns()}
          onRefreshAll={() => void refresh()}
          onRefreshOrientation={() => void loadOrientation()}
          onUpdateOrientationPreferences={updateOrientationPreferences}
        />
      )}
      {initialSection === "accounts" &&
        (initialAccountCreate ? (
          <AccountCreateFlow
            tenantId={session.tenant_id}
            tenantLabel={session.tenant_label ?? "Ledger tenant"}
            environmentLabel={
              session.environment === "demo"
                ? "Isolated demo"
                : "Verified production"
            }
            csrfToken={session.csrf_token}
            online={online}
            canWrite={session.scopes.includes("accounts:write")}
            canTransfer={session.scopes.includes("transfers:write")}
            fundingScopeComplete={accountWorkspace.scopeComplete}
            fundedSourceAvailable={accountWorkspace.accounts.some(
              (candidate) =>
                candidate.status === "active" &&
                candidate.currency === "INR" &&
                hasPositiveMinorUnits(candidate.available_minor),
            )}
            returnTo={initialAccountReturnTo}
            onCreated={async () => {
              await loadAccounts(accountWorkspace.filters, 100);
            }}
          />
        ) : (
          <AccountsView
            accounts={accountWorkspace.accounts}
            selected={accountWorkspace.selected}
            detailRequested={Boolean(initialAccountId)}
            balance={accountWorkspace.balance}
            transactions={accountWorkspace.transactions}
            balanceLoading={accountWorkspace.balanceLoading}
            historyLoading={accountWorkspace.historyLoading}
            directoryLoading={accountWorkspace.directoryLoading}
            balanceError={accountWorkspace.balanceError}
            historyError={accountWorkspace.historyError}
            balanceVerifiedAt={accountWorkspace.balanceVerifiedAt}
            historyVerifiedAt={accountWorkspace.historyVerifiedAt}
            directoryVerifiedAt={accountWorkspace.directoryVerifiedAt}
            error={accountWorkspace.error}
            online={online}
            filters={accountWorkspace.filters}
            nextCursor={accountWorkspace.nextCursor}
            historyNextCursor={accountWorkspace.historyCursor}
            focusAccountId={initialAccountFocusId}
            tenantId={session.tenant_id}
            csrfToken={session.csrf_token}
            canWrite={session.scopes.includes("accounts:write")}
            canTransfer={session.scopes.includes("transfers:write")}
            canExport={
              session.scopes.includes("exports:read") &&
              session.scopes.includes("transactions:read")
            }
            fundingScopeComplete={accountWorkspace.scopeComplete}
            detailReturnTo={
              initialAccountId ? initialAccountReturnTo : undefined
            }
            onRefresh={() => void refresh()}
            onApplyFilters={accountWorkspace.applyFilters}
            onNext={accountWorkspace.loadNextPage}
            onHistoryNext={() => void accountWorkspace.loadMoreHistory()}
            onRefreshBalance={() => {
              if (initialAccountId) void loadAccountBalance(initialAccountId);
            }}
            onRefreshHistory={() => {
              if (initialAccountId) void loadAccountHistory(initialAccountId);
            }}
            onAccountChanged={async () => {
              if (initialAccountId) await loadAccountDetail(initialAccountId);
              await loadAccounts(accountWorkspace.filters, 100);
            }}
            onRefreshLifecycleEvidence={async () =>
              initialAccountId
                ? loadAccountDetail(initialAccountId)
                : { account: null, balance: null }
            }
          />
        ))}
      {initialSection === "transfers" && (
        <TransfersView
          accounts={accountWorkspace.accounts}
          accountsLoading={accountWorkspace.directoryLoading}
          accountsError={accountWorkspace.error}
          accountsVerifiedAt={accountWorkspace.directoryVerifiedAt}
          transfers={transfers}
          transfersVerifiedAt={transfersVerifiedAt}
          detail={transferDetail}
          detailRequested={Boolean(initialTransferId)}
          explainability={explainability}
          explainabilityLoading={explainabilityLoading}
          explainabilityError={explainabilityError}
          error={transferError}
          loading={initialTransferId ? transferDetailLoading : transfersLoading}
          nextCursor={transferCursor}
          online={online}
          canWrite={
            session.scopes.includes("transfers:write") &&
            accountWorkspace.scopeComplete
          }
          canExport={
            session.scopes.includes("exports:read") &&
            session.scopes.includes("transfers:read")
          }
          canReadExplainability={[
            "explainability:read",
            "transfers:read",
            "events:read",
            "reconciliation:read",
          ].every((scope) => session.scopes.includes(scope))}
          canReadCorrections={session.scopes.includes("corrections:read")}
          canWriteCorrections={session.scopes.includes("corrections:write")}
          writeUnavailableReason={
            !accountWorkspace.scopeComplete
              ? "Transfer creation is disabled because the authorized account picker is larger than one bounded page. Use the API until server-backed account selection is configured."
              : undefined
          }
          tenantId={session.tenant_id}
          csrfToken={session.csrf_token}
          preferredDestinationId={initialTransferDestinationId}
          returnTo={initialTransferReturnTo}
          initialFilters={initialTransferFilters}
          onRefreshAccounts={async () => {
            await loadAccounts(emptyAccountFilters, 100);
          }}
          onRefresh={async () => {
            if (initialTransferId) await loadTransferDetail(initialTransferId);
            else await loadTransfers();
          }}
          onRefreshExplainability={() => {
            if (initialTransferId) void loadExplainability(initialTransferId);
          }}
          onMore={() => {
            if (transferCursor) void loadTransfers(transferCursor, true);
          }}
        />
      )}
      {initialSection === "reconciliation" && (
        <ReconciliationView
          runs={runs}
          detail={runDetail}
          detailRequested={Boolean(initialReconciliationRunId)}
          error={reconciliationError}
          loading={initialReconciliationRunId ? runDetailLoading : runsLoading}
          verifiedAt={reconciliationVerifiedAt}
          nextCursor={runCursor}
          tenantId={session.tenant_id}
          csrfToken={session.csrf_token}
          online={online}
          canWrite={session.scopes.includes("reconciliation:write")}
          canExport={
            session.scopes.includes("exports:read") &&
            session.scopes.includes("reconciliation:read")
          }
          returnTo={initialReconciliationReturnTo}
          onObserved={observeRun}
          onRefresh={() =>
            initialReconciliationRunId
              ? loadRunDetail(initialReconciliationRunId)
              : loadRuns()
          }
          onMore={() => {
            if (runCursor) void loadRuns(runCursor, true);
          }}
        />
      )}
      <ConsoleFooter pending={!initialEvidenceSettled} />
    </ConsoleShell>
  );
}
