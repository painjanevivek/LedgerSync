"use client";

import { WarningCircle } from "@phosphor-icons/react";
import dynamic from "next/dynamic";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";

import type { ConsoleSession } from "@/features/accounts/types";
import { ConsoleFooter, ConsoleShell } from "@/features/console/ConsoleShell";
import { PageHeader, StatePanel } from "@/features/console/components";
import type { DeveloperMetadata } from "@/lib/api/developer";
import { readJSON, unavailableMessage } from "@/lib/api/client";

const DeveloperView = dynamic(() => import("@/features/developer/DeveloperViews").then((module) => module.DeveloperView), {
  loading: () => <StatePanel title="Loading developer reference" message="Preparing the bounded contract examples without delaying the operator shell." />,
});

export function DeveloperConsole() {
  const router = useRouter();
  const [session,setSession] = useState<ConsoleSession|null>(null);
  const [sessionLoading,setSessionLoading] = useState(true);
  const [metadata,setMetadata] = useState<DeveloperMetadata|null>(null);
  const [loading,setLoading] = useState(false);
  const [error,setError] = useState<string|null>(null);
  const [online,setOnline] = useState(true);
  const [publicOrigin,setPublicOrigin] = useState("http://127.0.0.1:3000");
  const generation = useRef(0);

  const loadMetadata = useCallback(async () => {
    const request = ++generation.current;
    setLoading(true);
    const response = await readJSON<DeveloperMetadata>("/api/developer/metadata");
    if (request !== generation.current) return;
    if (response.ok&&response.data.schema_version==="1") { setMetadata(response.data);setError(null); }
    else setError(unavailableMessage(response.status,"developer contract metadata"));
    setLoading(false);
  },[]);

  useEffect(()=>{ let active=true;(async()=>{const response=await readJSON<ConsoleSession>("/api/session");if(!active)return;if(response.ok&&response.data.tenant_id)setSession(response.data);setSessionLoading(false);})();return()=>{active=false;};},[]);
  useEffect(()=>{const update=()=>{setOnline(navigator.onLine);setPublicOrigin(window.location.origin);};update();window.addEventListener("online",update);window.addEventListener("offline",update);return()=>{window.removeEventListener("online",update);window.removeEventListener("offline",update);};},[]);
  useEffect(()=>{if(!session||!online||!session.scopes.includes("developer:read"))return;const timer=window.setTimeout(()=>void loadMetadata(),0);return()=>{window.clearTimeout(timer);generation.current+=1;};},[loadMetadata,online,session]);

  async function signOut(){if(!session)return;await fetch("/api/auth/sign-out",{method:"POST",headers:{"X-CSRF-Token":session.csrf_token}});router.refresh();}

  if(sessionLoading)return <ConsoleShell section="developer" tenantLabel="Verifying tenant" tenantMeta="Secure session" environmentLabel="Checking environment" operatorLabel="Verifying operator" operatorMeta="Authorization pending"><PageHeader eyebrow="API-first local workspace" title="Verifying access" description="Checking the authorized developer scope before contract examples are displayed."/><StatePanel title="Loading contract boundary" message="No endpoint or retry behavior is being inferred while the session is verified."/><ConsoleFooter/></ConsoleShell>;
  if(!session)return <main className="boot-screen"><p className="eyebrow">Authentication required</p><h1>Developer workspace unavailable</h1><StatePanel kind="denied" title="No authorized session" message="Configure the approved OIDC provider, or explicitly enable the isolated local demo environment. No contract metadata is displayed."/></main>;
  const canRead=session.scopes.includes("developer:read");
  return <ConsoleShell section="developer" tenantLabel={session.tenant_label??"Ledger tenant"} tenantMeta={session.tenant_id} environmentLabel={session.environment==="demo"?"Isolated demo":"Verified production"} operatorLabel={session.operator_label??session.subject_id} operatorMeta={session.environment==="demo"?"Non-production data":"Authorized operator"} preview={session.environment==="demo"} onSignOut={()=>void signOut()}>{!online&&<div className="offline-banner" role="status"><WarningCircle weight="fill" aria-hidden="true"/><span><strong>You are offline.</strong> Download and refresh are disabled.</span></div>}<DeveloperView metadata={metadata} loading={loading} error={error} online={online} canRead={canRead} publicOrigin={publicOrigin} onRefresh={()=>void loadMetadata()}/><ConsoleFooter/></ConsoleShell>;
}
