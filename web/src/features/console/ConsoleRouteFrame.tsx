"use client";

import { WarningCircle } from "@phosphor-icons/react";
import type { ReactNode } from "react";

import { LoginScreen } from "@/features/auth/LoginScreen";
import { ConsoleFooter, ConsoleShell, type ConsoleSection } from "@/features/console/ConsoleShell";
import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { PageHeader } from "@/ui/display/PageHeader";
import { StatePanel } from "@/ui/display/StatePanel";

export function ConsoleRouteFrame({
  section,
  loadingLabel,
  children,
  pending = false,
}: Readonly<{
  section: ConsoleSection;
  loadingLabel: string;
  children: ReactNode;
  pending?: boolean;
}>) {
  const { session, sessionError, sessionLoading, online } = useConsoleSession();

  if (sessionLoading) {
    return (
      <ConsoleShell section={section} tenantLabel="Verifying tenant" tenantMeta="Secure session" environmentLabel="Checking environment" operatorLabel="Verifying operator" operatorMeta="Authorization pending">
        <div className="session-loading" aria-busy="true">
          <PageHeader eyebrow={`${loadingLabel} · LedgerSync operator workspace`} title="Verifying access" description="Verifying the authorized tenant scope before financial records are displayed." />
          <StatePanel title="Loading financial records" message="Balances, transfer history, and reconciliation results are loading from their authoritative sources." />
        </div>
        <ConsoleFooter pending />
      </ConsoleShell>
    );
  }
  if (!session) return <LoginScreen unavailableMessage={sessionError} />;

  return (
    <ConsoleShell
      section={section}
      tenantLabel={session.tenant_label ?? "Ledger tenant"}
      tenantMeta={session.tenant_id}
      environmentLabel={session.environment === "local" ? "Local workspace" : "Verified production"}
      operatorLabel={session.operator_label ?? session.subject_id}
      operatorMeta={session.environment === "local" ? "This workstation" : "Authorized operator"}
    >
      {!online && (
        <div className="offline-banner" role="status">
          <WarningCircle weight="fill" aria-hidden="true" />
          <span><strong>You are offline.</strong> Writes are disabled and no unverified result is shown.</span>
        </div>
      )}
      {children}
      <ConsoleFooter pending={pending} />
    </ConsoleShell>
  );
}
