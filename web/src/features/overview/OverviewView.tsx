"use client";

import { ArrowRight, Bank, ShieldCheck } from "@phosphor-icons/react";
import Link from "next/link";

import type { Account, ReconciliationRun, TransferSummary } from "@/features/accounts/types";
import { FocusedRetry } from "@/ui/controls/FocusedRetry.client";
import { EvidenceFreshness } from "@/ui/display/Evidence";
import { PageHeader } from "@/ui/display/PageHeader";
import { RecordLink } from "@/ui/display/RecordLink";
import { StatePanel } from "@/ui/display/StatePanel";
import type { ConsoleCapabilities } from "@/features/console/capabilities";
import { deriveWorkspaceStage } from "@/features/console/workspaceStage";
import { LocalOrientationPanel } from "@/features/orientation/LocalOrientationPanel";
import { TransferList } from "@/features/transfers/TransferViews";
import { Money } from "@/ui/display/Money";
import { Timestamp } from "@/ui/display/Timestamp";
import { NextBestAction } from "@/ui/disclosure/NextBestAction";
import { approvedCurrencyGroups, isAuthoritativelyReconciled } from "@/lib/financial-ui";
import type { LocalOrientation, OperatorPreferenceStepID } from "@/lib/api/orientation";
import { uiDataState } from "@/lib/api/client";
import { useExperienceMode } from "@/features/console/ExperienceModeBoundary";
import { reconciliationPresentation, transferStatusPresentation } from "@/features/console/presentation";
import { FinancialSummary } from "@/ui/presentation/FinancialSummary";
import { RelativeTime } from "@/ui/presentation/RelativeTime";
import { TaskCard } from "@/ui/presentation/TaskCard";
import { TechnicalDetails } from "@/ui/presentation/TechnicalDetails";
import { TrustStrip } from "@/ui/presentation/TrustStrip";

type Props = Readonly<{
  accounts: Account[];
  accountsScopeComplete?: boolean;
  transfers: TransferSummary[];
  reconciliation: ReconciliationRun | null;
  accountsLoading: boolean;
  transfersLoading: boolean;
  reconciliationLoading: boolean;
  accountsError: string | null;
  transfersError: string | null;
  reconciliationError: string | null;
  accountsVerifiedAt?: string;
  transfersVerifiedAt?: string;
  reconciliationVerifiedAt?: string;
  online: boolean;
  capabilities: ConsoleCapabilities;
  orientation: LocalOrientation | null;
  orientationLoading: boolean;
  orientationError: string | null;
  orientationPreferenceError: string | null;
  orientationPreferenceSaving: boolean;
  canReadOrientation: boolean;
  canWriteOrientation: boolean;
  localWorkspace: boolean;
  forceOrientation?: boolean;
  onRefreshAccounts: () => void;
  onRefreshTransfers: () => void;
  onRefreshReconciliation: () => void;
  onRefreshAll: () => void;
  onRefreshOrientation: () => void;
  onUpdateOrientationPreferences: (change: Readonly<{ dismissed: boolean; completedStepIDs: OperatorPreferenceStepID[] }>) => Promise<boolean>;
}>;

export function OverviewView({ accounts, accountsScopeComplete = true, transfers, reconciliation, accountsLoading, transfersLoading, reconciliationLoading, accountsError, transfersError, reconciliationError, accountsVerifiedAt, transfersVerifiedAt, reconciliationVerifiedAt, online, capabilities, orientation, orientationLoading, orientationError, orientationPreferenceError, orientationPreferenceSaving, canReadOrientation, canWriteOrientation, localWorkspace, forceOrientation, onRefreshAccounts, onRefreshTransfers, onRefreshReconciliation, onRefreshAll, onRefreshOrientation, onUpdateOrientationPreferences }: Props) {
  const { mode } = useExperienceMode();
  const { currency, mixedCurrency, operatingMinor: operating, customerFundsMinor: customerFunds }=approvedCurrencyGroups(accounts);
  const asOf=accounts.map((account)=>account.as_of).filter(Boolean).sort().at(0);
  const busy = accountsLoading || transfersLoading || reconciliationLoading;
  const accountState = uiDataState({ loading: accountsLoading, hasData: accounts.length > 0, hasError: Boolean(accountsError), online, partial: Boolean(accountsError?.includes("partial")) });
  const transferState = uiDataState({ loading: transfersLoading, hasData: transfers.length > 0, hasError: Boolean(transfersError), online });
  const reconciliationState = uiDataState({ loading: reconciliationLoading, hasData: Boolean(reconciliation), hasError: Boolean(reconciliationError), online });
  const evidenceSettled =
    !accountsLoading &&
    !transfersLoading &&
    !reconciliationLoading &&
    !accountsError &&
    !transfersError &&
    !reconciliationError &&
    capabilities.accountsRead &&
    capabilities.transfersRead;
  const workspaceStage = deriveWorkspaceStage({
    accountCount: accounts.length,
    transfers,
    reconciliation,
    orientation,
    hasCriticalReadError: Boolean(accountsError || transfersError || reconciliationError || !online),
  });
  const newWorkspace = evidenceSettled && workspaceStage === "empty";
  const pendingTransfer = transfers.find((transfer) => transfer.financial_status === "pending");
  const pendingTransferStatus = pendingTransfer
    ? transferStatusPresentation(pendingTransfer.financial_status)
    : null;
  const reconciliationStatus = reconciliation ? reconciliationPresentation(reconciliation.status, reconciliation.mismatch_count) : null;

  if (mode === "simple") {
    const trustTone = !online || accountsError || transfersError || reconciliationError
      ? "unknown" as const
      : reconciliationStatus?.attention
        ? reconciliationStatus.tone
        : "positive" as const;
    const trustTitle = !online
      ? "Showing the last verified balances"
      : accountsError || transfersError || reconciliationError
        ? "Some information could not be refreshed"
        : reconciliationStatus?.attention
          ? reconciliationStatus.title
          : accountsLoading
            ? "Checking your balances"
            : accounts.length > 0
              ? "Balances checked"
              : "Your workspace is ready";
    const trustDetail = accountsVerifiedAt
      ? <RelativeTime value={accountsVerifiedAt} />
      : accountsLoading
        ? "We will keep the previous verified values visible while this finishes."
        : "Start by creating an account.";
    return <>
      <PageHeader eyebrow="Home" title="Your money at a glance" description="See what is available, what needs attention, and the safest next step." >
        {(capabilities.accountsRead || capabilities.transfersRead || capabilities.reconciliationRead) && <button className="button secondary" type="button" onClick={onRefreshAll} disabled={!online || busy}>{busy ? "Refreshing…" : "Refresh"}</button>}
      </PageHeader>
      <TrustStrip
        tone={trustTone}
        title={trustTitle}
        detail={trustDetail}
        action={<TechnicalDetails summary="View details" attention={Boolean(accountsError || transfersError || reconciliationError)}>
          {accountsVerifiedAt && <p>Account totals: <RelativeTime value={accountsVerifiedAt} />.</p>}
          {transfersVerifiedAt && <p>Transfer activity: <RelativeTime value={transfersVerifiedAt} />.</p>}
          {reconciliationVerifiedAt && <p>Balance check: <RelativeTime value={reconciliationVerifiedAt} />.</p>}
          {(accountsError || transfersError || reconciliationError) && <p>{accountsError ?? transfersError ?? reconciliationError}</p>}
        </TechnicalDetails>}
      />
      {localWorkspace && forceOrientation && (
        <LocalOrientationPanel
          evidence={orientation}
          loading={orientationLoading}
          error={orientationError}
          preferenceError={orientationPreferenceError}
          preferenceSaving={orientationPreferenceSaving}
          online={online}
          canRead={canReadOrientation}
          canWrite={canWriteOrientation}
          capabilities={capabilities}
          onRefresh={onRefreshOrientation}
          onUpdatePreferences={onUpdateOrientationPreferences}
        />
      )}
      {!capabilities.accountsRead && !capabilities.transfersRead && !capabilities.reconciliationRead && <StatePanel kind="denied" title="No accounts are available" message="Ask an administrator to give you access to the accounts you need."/>}

      {(pendingTransfer || reconciliationStatus?.attention || accountsError || transfersError || reconciliationError) && <>
        <div className="simple-section-heading"><div><h2>Needs your attention</h2><p>Handle these items before starting the same work again.</p></div></div>
        <div className="task-list">
          {pendingTransfer && pendingTransferStatus && <TaskCard title={pendingTransferStatus.title} explanation={pendingTransferStatus.explanation} tone={pendingTransferStatus.tone} action={{ label: "Check status", href: `/transfers/${pendingTransfer.transfer_id}` }} />}
          {reconciliation && reconciliationStatus?.attention && <TaskCard title={reconciliationStatus.title} explanation={reconciliationStatus.explanation} tone={reconciliationStatus.tone} action={{ label: "Review balance check", href: `/reconciliation/${reconciliation.run_id}` }} />}
          {accountsError && <TaskCard title="Account balances could not refresh" explanation="The last verified values remain visible. Refresh only the account balances when the connection is stable." tone="unknown" action={{ label: "View accounts", href: "/accounts" }} />}
          {transfersError && <TaskCard title="Transfer activity could not refresh" explanation="No empty history is assumed. Open transfers to try again." tone="unknown" action={{ label: "View transfers", href: "/transfers" }} />}
          {reconciliationError && <TaskCard title="The latest balance check is unavailable" explanation="Keep using the last verified result until the check can be loaded." tone="unknown" action={{ label: "View balance checks", href: "/reconciliation" }} />}
        </div>
      </>}

      {accounts.length > 0 && mixedCurrency && <StatePanel kind="error" title="Balances use different currencies" message="LedgerSync will not combine them. Open Accounts to review each currency separately."/>}
      {!accountsScopeComplete && <StatePanel kind="unknown" title="Workspace totals are not available" message="Only part of the account list was loaded. Open Accounts to review individual balances; no partial sum is presented as your full workspace." action={<Link className="button secondary" href="/accounts">View accounts</Link>} />}
      {accountsScopeComplete && accounts.length > 0 && !mixedCurrency && currency && <>
        <div className="simple-section-heading"><div><h2>Your money</h2><p>Exact balances from verified ledger records.</p></div><Link className="text-link" href="/accounts">View accounts</Link></div>
        <div className="simple-financial-grid">
          <FinancialSummary label="Operating balance" amount={<Money currency={currency} minorUnits={operating} />} explanation="Available for your organization’s operations." />
          <FinancialSummary label="Customer funds" amount={<Money currency={currency} minorUnits={customerFunds} />} explanation="Held separately and never counted as operating money." />
        </div>
      </>}

      {newWorkspace && <section className="friendly-empty-state home-empty-state"><h2>Start with your first account</h2><p>Create an account, add money, review it, then make a transfer. LedgerSync will guide you one step at a time.</p><div className="new-workspace-actions">{capabilities.accountsWrite && <Link className="button primary" href="/accounts/new">Create an account <ArrowRight aria-hidden="true" /></Link>}<Link className="button secondary" href="/guide">Open the guide</Link></div></section>}

      {!newWorkspace && transfers.length > 0 && <>
        <div className="simple-section-heading"><div><h2>Recent activity</h2><p>Your latest money movements.</p></div><Link className="text-link" href="/transfers">View all</Link></div>
        <ul className="simple-activity-list">
          {transfers.slice(0, 5).map((transfer) => {
            const status = transferStatusPresentation(transfer.financial_status);
            return <li key={transfer.transfer_id}><Link href={`/transfers/${transfer.transfer_id}`}><span><strong>{status.title}</strong><small><RelativeTime value={transfer.completed_at || transfer.created_at} /></small></span><Money currency={transfer.currency} minorUnits={transfer.amount_minor} /></Link></li>;
          })}
        </ul>
      </>}

      {evidenceSettled && workspaceStage === "account_ready" && <NextBestAction title="Add money to your account" message="Record where the money came from, then send it for review before making a transfer." action={capabilities.fundingWrite ? <Link className="button primary" href="/funding">Add money</Link> : <Link className="button secondary" href="/guide">Review the steps</Link>} />}
    </>;
  }
  return <>
    <PageHeader eyebrow="Operations / Authoritative ledger" title="Overview" description={newWorkspace ? "Your workspace is ready. Begin with one real account and let every later balance come from posted ledger records." : "Current balances, transfers, reconciliation results, and exceptions for this workspace."}>{(capabilities.accountsRead||capabilities.transfersRead||capabilities.reconciliationRead)&&<button className="button secondary" type="button" onClick={onRefreshAll} disabled={!online || busy}>{busy ? "Refreshing dashboard…" : "Refresh dashboard"}</button>}</PageHeader>
    {localWorkspace&&forceOrientation&&<LocalOrientationPanel evidence={orientation} loading={orientationLoading} error={orientationError} preferenceError={orientationPreferenceError} preferenceSaving={orientationPreferenceSaving} online={online} canRead={canReadOrientation} canWrite={canWriteOrientation} capabilities={capabilities} onRefresh={onRefreshOrientation} onUpdatePreferences={onUpdateOrientationPreferences}/>}
    {!capabilities.accountsRead&&!capabilities.transfersRead&&!capabilities.reconciliationRead&&<StatePanel kind="denied" title="No financial read capability" message="Your server-issued session has no account, transfer, or reconciliation read scope. Protected overview requests were not made."/>}
    {evidenceSettled && workspaceStage === "attention_required" && (
      <NextBestAction
        attention
        title="Resolve the current evidence warning"
        message="One or more authoritative reads, reconciliation results, or connectivity checks needs attention. Review the visible warning below before relying on normal metrics."
        action={reconciliation && !isAuthoritativelyReconciled(reconciliation)
          ? <Link className="button primary" href={`/reconciliation/${reconciliation.run_id}`}>Open reconciliation result</Link>
          : <button className="button primary" type="button" onClick={onRefreshAll} disabled={!online || busy}>Refresh evidence</button>}
      />
    )}
    {evidenceSettled && workspaceStage === "account_ready" && (
      <NextBestAction
        title="Add the first verified funding record"
        message="Your account structure is ready. Record external value evidence and send it through the existing approval flow before making a transfer."
        action={capabilities.fundingWrite
          ? <Link className="button primary" href="/funding">Open funding</Link>
          : <Link className="button secondary" href="/guide">Review the required steps</Link>}
      />
    )}
    {newWorkspace&&<section className="new-workspace" aria-labelledby="new-workspace-title">
      <div className="new-workspace-mark" aria-hidden="true"><Bank weight="fill" /></div>
      <div className="new-workspace-copy">
        <p className="eyebrow">New workspace</p>
        <h2 id="new-workspace-title">Start with four simple steps</h2>
        <p>Create an account, add a funding record, review it, then make a transfer.</p>
        <div className="new-workspace-actions">
          {capabilities.accountsWrite&&<Link className="button primary" href="/accounts/new">Create your first account <ArrowRight aria-hidden="true" /></Link>}
          <Link className="button secondary" href="/guide">Follow the guide</Link>
        </div>
      </div>
      <ol className="new-workspace-path" aria-label="First ledger steps">
        {capabilities.accountsWrite&&<li><Link href="/accounts/new"><span>01</span><strong>Create an account</strong><small>Set up where the money belongs.</small></Link></li>}
        {capabilities.fundingWrite&&<li><Link href="/funding"><span>02</span><strong>Add a funding record</strong><small>Add the payment reference and supporting document.</small></Link></li>}
        {capabilities.fundingApprove&&<li><Link href="/approvals"><span>03</span><strong>Review the record</strong><small>Check it before it can change a balance.</small></Link></li>}
        {capabilities.transfersWrite&&<li><Link href="/transfers"><span>04</span><strong>Make a transfer</strong><small>Move an exact amount between your accounts.</small></Link></li>}
      </ol>
    </section>}
    <section className="overview-data-state overview-account-state" data-data-state={accountState} aria-label="Account details state">
      {accountsError && <StatePanel
        kind="error"
        title="Account details unavailable"
        message={accountsError}
        action={<FocusedRetry label="Retry accounts only" onRetry={onRefreshAccounts} disabled={!online} busy={accountsLoading} />}
      />}
      {accountsVerifiedAt&&accounts.length>0&&<EvidenceFreshness state={accountsError||!online?"historical":accountsLoading?"refreshing":"current"} verifiedAt={accountsVerifiedAt} label="Account totals" reason={accountsError??(!online?"Reconnect before relying on totals.":undefined)}/>}
      {accountsLoading&&accounts.length===0&&<StatePanel title="Loading account details" message="Authoritative account balances are loading. No zero balance or empty tenant is inferred."/>}
      {!newWorkspace&&!accountsError&&!accountsLoading&&accounts.length===0&&<StatePanel title="No authorized accounts" message="The verified authorized scope is empty. Create an account or ask an administrator to grant account access."/>}
      {accounts.length>0&&mixedCurrency&&<StatePanel kind="error" title="Mixed-currency pilot data blocked" message="Loaded accounts are preserved, but LedgerSync will not combine balances across currencies. Investigate tenant provisioning before relying on overview totals."/>}
      {!accountsScopeComplete && accounts.length > 0 && <StatePanel kind="unknown" title="Workspace totals are not available" message="Only part of the authorized account list was loaded. Open Accounts to review individual balances." action={<Link href="/accounts">View accounts</Link>} />}
      {accountsScopeComplete&&accounts.length>0&&!mixedCurrency&&currency&&<div className="overview-metrics"><section className="balance-document"><div className="document-topline"><p>Operating-controlled balances</p><span>As of <Timestamp value={asOf} /></span></div><strong className="hero-amount"><Money currency={currency} minorUnits={operating} /></strong><p className="metric-definition">Excludes customer-funds category. Amounts with different ownership semantics are never combined silently.</p><Link className="text-link" href="/accounts">View account details →</Link></section><section className="balance-document secondary-balance"><div className="document-topline"><p>Customer funds</p><span>Separately classified</span></div><strong className="hero-amount"><Money currency={currency} minorUnits={customerFunds} /></strong><p className="metric-definition">Presented separately to avoid implying these funds are operating capital.</p></section></div>}
    </section>
    {!newWorkspace&&<section className="overview-data-state overview-reconciliation-state" data-data-state={reconciliationState} aria-label="Reconciliation results state">
      {reconciliationError && <StatePanel
        kind="error"
        title="Reconciliation results unavailable"
        message={reconciliationError}
        action={<FocusedRetry label="Retry reconciliation only" onRetry={onRefreshReconciliation} disabled={!online} busy={reconciliationLoading} />}
      />}
      {reconciliationVerifiedAt&&reconciliation&&<EvidenceFreshness state={reconciliationError||!online?"historical":reconciliationLoading?"refreshing":"current"} verifiedAt={reconciliationVerifiedAt} label="Reconciliation" reason={reconciliationError??(!online?"Reconnect before treating the run as current.":undefined)}/>}
      {reconciliationLoading&&!reconciliation&&<StatePanel title="Loading reconciliation results" message="No passing result or mismatch count is inferred while authoritative records load."/>}
      {reconciliation ? (
        <section className={`evidence-strip ${isAuthoritativelyReconciled(reconciliation) ? "" : "caution"}`}>
          <ShieldCheck weight="fill" aria-hidden="true" />
          <div>
            <strong>{isAuthoritativelyReconciled(reconciliation) ? "Latest reconciliation passed" : "Reconciliation requires attention"}</strong>
            <span>Run {reconciliation.run_id} · {reconciliation.mismatch_count} mismatches · <Timestamp value={reconciliation.completed_at} /></span>
          </div>
          <RecordLink href={`/reconciliation/${reconciliation.run_id}`} label="Open result" />
        </section>
      ) : !reconciliationError && !reconciliationLoading ? (
        <StatePanel kind="unknown" title="No reconciliation results" message="The verified history contains no authoritative run. No passing result is inferred." action={<Link className="text-link" href="/reconciliation">View reconciliation</Link>} />
      ) : null}
    </section>}
    {!newWorkspace&&<section className="overview-data-state overview-transfer-state" data-data-state={transferState} aria-label="Transfer history state">
      {transfersError && <StatePanel
        kind="error"
        title="Transfer history unavailable"
        message={transfersError}
        action={<FocusedRetry label="Retry transfers only" onRetry={onRefreshTransfers} disabled={!online} busy={transfersLoading} />}
      />}
      {transfersVerifiedAt&&transfers.length>0&&<EvidenceFreshness state={transfersError||!online?"historical":transfersLoading?"refreshing":"current"} verifiedAt={transfersVerifiedAt} label="Transfer history" reason={transfersError??(!online?"Reconnect before treating history as current.":undefined)}/>}
      {transfersLoading&&transfers.length===0&&<StatePanel title="Loading transfer history" message="No empty transfer history is inferred while immutable records load."/>}
      {!transfersError&&!transfersLoading&&transfers.length===0&&<StatePanel title="No transfer records" message="The verified authorized history is empty."/>}
      {transfers.length>0&&<TransferList variant="recent" transfers={transfers.slice(0,5)} returnTo="/"/>}
    </section>}
  </>;
}
