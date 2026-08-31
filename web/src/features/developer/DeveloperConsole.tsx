"use client";

import { WarningCircle } from "@phosphor-icons/react";
import dynamic from "next/dynamic";
import { useCallback, useEffect, useRef, useState } from "react";

import { ConsoleFooter, ConsoleShell } from "@/features/console/ConsoleShell";
import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { PageHeader, StatePanel } from "@/features/console/components";
import type { DeveloperMetadata } from "@/lib/api/developer";
import { readJSON, unavailableMessage } from "@/lib/api/client";

const DeveloperView = dynamic(() => import("@/features/developer/DeveloperViews").then((module) => module.DeveloperView), {
  loading: () => <StatePanel title="Loading developer reference" message="Preparing the bounded contract examples without delaying the operator shell." />,
});

export function DeveloperConsole() {
  const { session, sessionError, sessionLoading, online, publicOrigin, hasScope } = useConsoleSession();
  const [metadata,setMetadata] = useState<DeveloperMetadata|null>(null);
  const [loading,setLoading] = useState(false);
  const [error,setError] = useState<string|null>(null);
  const generation = useRef(0);

  const loadMetadata = useCallback(async () => {
    const request = ++generation.current;
    setLoading(true);
    const response = await readJSON<DeveloperMetadata>("/api/developer/metadata");
    if (request !== generation.current) return;
    if (response.ok&&response.data.schema_version==="1") { setMetadata(response.data);setError(null); }
    else setError(unavailableMessage(response.status,"developer contract metadata",response.requestReference));
    setLoading(false);
  },[]);

  useEffect(()=>{if(!session||!online||!hasScope("developer:read"))return;const timer=window.setTimeout(()=>void loadMetadata(),0);return()=>{window.clearTimeout(timer);generation.current+=1;};},[hasScope,loadMetadata,online,session]);

  if(sessionLoading)return <ConsoleShell section="developer" tenantLabel="Verifying tenant" tenantMeta="Secure session" environmentLabel="Checking environment" operatorLabel="Verifying operator" operatorMeta="Authorization pending"><PageHeader eyebrow="API-first local workspace" title="Verifying access" description="Checking the authorized developer scope before contract examples are displayed."/><StatePanel title="Loading contract boundary" message="No endpoint or retry behavior is being inferred while the session is verified."/><ConsoleFooter/></ConsoleShell>;
  if(!session)return <main className="boot-screen"><p className="eyebrow">Access not verified</p><h1>Developer workspace unavailable</h1><StatePanel kind={sessionError?"error":"denied"} title={sessionError?"Session evidence unavailable":"No authorized session"} message={sessionError??"Log in to the local workspace or configure the approved OIDC provider. No contract metadata is displayed."}/></main>;
  const canRead=hasScope("developer:read");
  return <ConsoleShell section="developer" tenantLabel={session.tenant_label??"Ledger tenant"} tenantMeta={session.tenant_id} environmentLabel={session.environment==="local"?"Local workspace":"Verified production"} operatorLabel={session.operator_label??session.subject_id} operatorMeta={session.environment==="local"?"This workstation":"Authorized operator"}>{!online&&<div className="offline-banner" role="status"><WarningCircle weight="fill" aria-hidden="true"/><span><strong>You are offline.</strong> Download and refresh are disabled.</span></div>}<DeveloperView metadata={metadata} loading={loading} error={error} online={online} canRead={canRead} publicOrigin={publicOrigin} onRefresh={()=>void loadMetadata()}/><ConsoleFooter/></ConsoleShell>;
}
