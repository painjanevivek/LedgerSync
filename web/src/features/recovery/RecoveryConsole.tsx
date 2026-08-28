"use client";

import { WarningCircle } from "@phosphor-icons/react";
import dynamic from "next/dynamic";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";

import type { ConsoleSession } from "@/features/accounts/types";
import { ConsoleFooter, ConsoleShell } from "@/features/console/ConsoleShell";
import { PageHeader, StatePanel } from "@/features/console/components";
import type { LocalDiagnostics } from "@/lib/api/operations";
import type { RecoveryEvidenceIndex } from "@/lib/api/recovery";
import { readJSON, unavailableMessage } from "@/lib/api/client";

const RecoveryView = dynamic(() => import("@/features/recovery/RecoveryView").then((module) => module.RecoveryView), {
  loading: () => <StatePanel title="Loading recovery reference" message="Preparing the protected evidence view without delaying the operator shell." />,
});

export function RecoveryConsole() {
  const router = useRouter();
  const [session,setSession]=useState<ConsoleSession|null>(null);
  const [sessionLoading,setSessionLoading]=useState(true);
  const [online,setOnline]=useState(true);
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
    if(database.ok&&database.data.overall_state){setDiagnostics(database.data);setDiagnosticsError(null);}else setDiagnosticsError(unavailableMessage(database.status,"current database evidence"));
    if(index.ok&&index.data.format_version==="ledgersync-recovery-evidence-index/v1"){setRecovery(index.data);setRecoveryError(null);}else setRecoveryError(unavailableMessage(index.status,"protected recovery evidence"));
    setLoading(false);
  },[]);

  useEffect(()=>{let active=true;(async()=>{const response=await readJSON<ConsoleSession>("/api/session");if(!active)return;if(response.ok&&response.data.tenant_id)setSession(response.data);setSessionLoading(false);})();return()=>{active=false;};},[]);
  useEffect(()=>{const update=()=>setOnline(navigator.onLine);update();window.addEventListener("online",update);window.addEventListener("offline",update);return()=>{window.removeEventListener("online",update);window.removeEventListener("offline",update);};},[]);
  useEffect(()=>{if(!session||!online||!session.scopes.includes("recovery:read"))return;const timer=window.setTimeout(()=>void load(),0);return()=>{window.clearTimeout(timer);generation.current+=1;};},[load,online,session]);
  async function signOut(){if(!session)return;await fetch("/api/auth/sign-out",{method:"POST",headers:{"X-CSRF-Token":session.csrf_token}});router.refresh();}

  if(sessionLoading)return <ConsoleShell section="recovery" tenantLabel="Verifying tenant" tenantMeta="Secure session" environmentLabel="Checking environment" operatorLabel="Verifying operator" operatorMeta="Authorization pending"><PageHeader eyebrow="Local tools / Recovery evidence" title="Verifying access" description="Checking recovery and database read scopes before protected evidence is displayed."/><StatePanel title="Loading custody boundary" message="No backup, restore, or current database state is inferred while authorization is verified."/><ConsoleFooter/></ConsoleShell>;
  if(!session)return <main className="boot-screen"><p className="eyebrow">Authentication required</p><h1>Recovery Center unavailable</h1><StatePanel kind="denied" title="No authorized session" message="Configure the approved OIDC provider, or explicitly enable the isolated local demo environment. No recovery evidence is displayed."/></main>;
  const canRead=session.scopes.includes("recovery:read");
  return <ConsoleShell section="recovery" tenantLabel={session.tenant_label??"Ledger tenant"} tenantMeta={session.tenant_id} environmentLabel={session.environment==="demo"?"Isolated demo":"Verified production"} operatorLabel={session.operator_label??session.subject_id} operatorMeta={session.environment==="demo"?"Non-production data":"Authorized operator"} preview={session.environment==="demo"} onSignOut={()=>void signOut()}>
    {!online&&<div className="offline-banner" role="status"><WarningCircle weight="fill" aria-hidden="true"/><span><strong>You are offline.</strong> Existing evidence is historical until it can be refreshed.</span></div>}
    <RecoveryView diagnostics={diagnostics} recovery={recovery} diagnosticsError={diagnosticsError} recoveryError={recoveryError} loading={loading} online={online} canRead={canRead} canReadDiagnostics={session.scopes.includes("local:read")} onRefresh={()=>void load()}/>
    <ConsoleFooter/>
  </ConsoleShell>;
}
