"use client";

import { WarningCircle } from "@phosphor-icons/react";
import dynamic from "next/dynamic";
import { useCallback, useEffect, useRef, useState } from "react";

import { ConsoleFooter, ConsoleShell } from "@/features/console/ConsoleShell";
import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { PageHeader, StatePanel } from "@/features/console/components";
import type { LocalDiagnostics } from "@/lib/api/operations";
import type { RecoveryEvidenceIndex } from "@/lib/api/recovery";
import { readJSON, unavailableMessage } from "@/lib/api/client";

const RecoveryView = dynamic(() => import("@/features/recovery/RecoveryView").then((module) => module.RecoveryView), {
  loading: () => <StatePanel title="Loading recovery reference" message="Preparing the protected evidence view without delaying the operator shell." />,
});

export function RecoveryConsole() {
  const { session, sessionError, sessionLoading, online, hasScope } = useConsoleSession();
  const [diagnostics,setDiagnostics]=useState<LocalDiagnostics|null>(null);
  const [recovery,setRecovery]=useState<RecoveryEvidenceIndex|null>(null);
  const [diagnosticsError,setDiagnosticsError]=useState<string|null>(null);
  const [recoveryError,setRecoveryError]=useState<string|null>(null);
  const [loading,setLoading]=useState(false);
  const generation=useRef(0);

  const load=useCallback(async()=>{
    const request=++generation.current; setLoading(true);
    const [database,index]=await Promise.all([readJSON<LocalDiagnostics>("/api/local/diagnostics"),readJSON<RecoveryEvidenceIndex>("/api/recovery/manifests")]);
    if(request!==generation.current)return;
    if(database.ok&&database.data.overall_state){setDiagnostics(database.data);setDiagnosticsError(null);}else setDiagnosticsError(unavailableMessage(database.status,"current database evidence",database.requestReference));
    if(index.ok&&index.data.format_version==="ledgersync-recovery-evidence-index/v1"){setRecovery(index.data);setRecoveryError(null);}else setRecoveryError(unavailableMessage(index.status,"protected recovery evidence",index.requestReference));
    setLoading(false);
  },[]);

  useEffect(()=>{if(!session||!online||!hasScope("recovery:read"))return;const timer=window.setTimeout(()=>void load(),0);return()=>{window.clearTimeout(timer);generation.current+=1;};},[hasScope,load,online,session]);

  if(sessionLoading)return <ConsoleShell section="recovery" tenantLabel="Verifying tenant" tenantMeta="Secure session" environmentLabel="Checking environment" operatorLabel="Verifying operator" operatorMeta="Authorization pending"><PageHeader eyebrow="Local tools / Recovery evidence" title="Verifying access" description="Checking recovery and database read scopes before protected evidence is displayed."/><StatePanel title="Loading custody boundary" message="No backup, restore, or current database state is inferred while authorization is verified."/><ConsoleFooter/></ConsoleShell>;
  if(!session)return <main className="boot-screen"><p className="eyebrow">Access not verified</p><h1>Recovery Center unavailable</h1><StatePanel kind={sessionError?"error":"denied"} title={sessionError?"Session evidence unavailable":"No authorized session"} message={sessionError??"Log in to the local workspace or configure the approved OIDC provider. No recovery evidence is displayed."}/></main>;
  const canRead=hasScope("recovery:read");
  return <ConsoleShell section="recovery" tenantLabel={session.tenant_label??"Ledger tenant"} tenantMeta={session.tenant_id} environmentLabel={session.environment==="local"?"Local workspace":"Verified production"} operatorLabel={session.operator_label??session.subject_id} operatorMeta={session.environment==="local"?"This workstation":"Authorized operator"}>
    {!online&&<div className="offline-banner" role="status"><WarningCircle weight="fill" aria-hidden="true"/><span><strong>You are offline.</strong> Existing evidence is historical until it can be refreshed.</span></div>}
    <RecoveryView diagnostics={diagnostics} recovery={recovery} diagnosticsError={diagnosticsError} recoveryError={recoveryError} loading={loading} online={online} canRead={canRead} canReadDiagnostics={hasScope("local:read")} onRefresh={()=>void load()}/>
    <ConsoleFooter/>
  </ConsoleShell>;
}
