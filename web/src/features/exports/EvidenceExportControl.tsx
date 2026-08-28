"use client";

import { DownloadSimple, FileCsv, WarningCircle } from "@phosphor-icons/react";
import { FormEvent, useEffect, useId, useRef, useState } from "react";

export type ExportDisclosure = Readonly<{ label: string; value: string }>;
type State = "idle" | "generating" | "downloading" | "error";

export function isSafeExportEndpoint(endpoint:string){
  try { const url=new URL(endpoint,"http://ledgersync.local"); return url.origin==="http://ledgersync.local"&&url.pathname.startsWith("/api/exports/")&&!url.username&&!url.password&&!url.hash; }
  catch{return false;}
}

export function EvidenceExportControl({label,subject,endpoint,scope,filters,columns,online,canExport}:Readonly<{label:string;subject:string;endpoint:string;scope:string;filters:ExportDisclosure[];columns:string;online:boolean;canExport:boolean;}>) {
  const dialog=useRef<HTMLDialogElement>(null); const dialogHeading=useRef<HTMLHeadingElement>(null); const trigger=useRef<HTMLButtonElement>(null); const resultHeading=useRef<HTMLHeadingElement>(null); const started=useRef(false); const resetTimer=useRef<number|undefined>(undefined);
  const frameName=`export-${useId().replace(/[^a-z0-9]/gi,"")}`;
  const [state,setState]=useState<State>("idle"); const [error,setError]=useState("");
  const busy=state==="generating"||state==="downloading";
  useEffect(()=>()=>{if(resetTimer.current)window.clearTimeout(resetTimer.current);},[]);
  useEffect(()=>{if(state!=="idle")resultHeading.current?.focus();},[state]);

  function open(){setError("");setState("idle");dialog.current?.showModal();window.requestAnimationFrame(()=>dialogHeading.current?.focus());}
  function download(event:FormEvent){
    event.preventDefault();
    if(!online){setError("The browser is offline. Reconnect before generating a new export file.");setState("error");dialog.current?.close();return;}
    if(!isSafeExportEndpoint(endpoint)){setError("The fixed export route is unavailable. No download was started.");setState("error");dialog.current?.close();return;}
    const url=new URL(endpoint,window.location.origin); if(url.origin!==window.location.origin){setError("The export route crossed the local browser boundary. No download was started.");setState("error");dialog.current?.close();return;}
    dialog.current?.close(); setState("generating"); started.current=true;
    window.setTimeout(()=>{
      const form=document.createElement("form"); form.method="GET"; form.action=url.pathname; form.target=frameName; form.hidden=true;
      url.searchParams.forEach((value,name)=>{const input=document.createElement("input");input.type="hidden";input.name=name;input.value=value;form.append(input);});
      document.body.append(form); form.submit(); form.remove(); setState("downloading");
      resetTimer.current=window.setTimeout(()=>{started.current=false;setState("idle");},4000);
    },0);
  }
  function inspectError(){
    if(!started.current)return;
    const frame=document.querySelector<HTMLIFrameElement>(`iframe[name="${frameName}"]`); const text=frame?.contentDocument?.body.textContent?.trim();
    if(!text)return;
    try { const value=JSON.parse(text) as {error?:{code?:string}}; if(value.error?.code){if(resetTimer.current)window.clearTimeout(resetTimer.current);started.current=false;setError(`Export was not generated (${value.error.code}). Refresh the underlying records and try again.`);setState("error");} } catch { /* Attachment responses are handled by the browser download manager. */ }
  }
  return <div className="evidence-export-control">
    <button ref={trigger} className="button secondary export-trigger" type="button" onClick={open} disabled={!online||!canExport||busy}><FileCsv aria-hidden="true"/>{busy?"Preparing export…":label}</button>
    {!canExport&&<span className="export-permission-note">Export requires the authorized exports:read scope.</span>}
    <iframe className="export-response-frame" name={frameName} title={`${subject} export response`} onLoad={inspectError}/>
    {state!=="idle"&&<div className="export-progress" role={state==="error"?"alert":"status"} aria-live="polite"><h3 ref={resultHeading} tabIndex={-1}>{state==="generating"?"Generating exact CSV":state==="downloading"?"Downloading exact CSV":"Export not generated"}</h3><p>{state==="generating"?"The server is iterating the authorized scope within its fixed row and time limits.":state==="downloading"?"The bounded stream was handed to the browser download manager. No row set is assembled in this page.":error}</p>{state==="error"&&<button className="button secondary" type="button" onClick={open}>Review and retry</button>}</div>}
    <dialog ref={dialog} className="confirmation-dialog export-review-dialog" aria-labelledby={`${frameName}-heading`} aria-describedby={`${frameName}-description`} onClose={()=>{if(!busy){setState("idle");trigger.current?.focus();}}}>
      <form method="dialog" onSubmit={download}><p className="eyebrow">Exact records export</p><h2 ref={dialogHeading} tabIndex={-1} id={`${frameName}-heading`}>Review {subject} export</h2><p id={`${frameName}-description`}>Confirm the authorized scope before generating a bounded, spreadsheet-safe records file.</p>
        <div className="export-review-proof"><div><span>Scope</span><strong>{scope}</strong></div><div><span>Applied filters</span><strong>{filters.length?filters.map((item)=>`${item.label}: ${item.value}`).join(" · "):"None — all authorized records"}</strong></div><div><span>Maximum records</span><strong>10,000</strong></div><div><span>Exact format</span><strong>UTF-8 CSV · schema version 1</strong></div><div><span>Identifiers</span><strong>{columns}</strong></div></div>
        <div className="export-not-backup"><WarningCircle weight="fill" aria-hidden="true"/><p><strong>This export is not a backup.</strong> It contains bounded operational records, not database state, restore metadata, credentials, or consistency tokens.</p></div>
        <div className="action-row"><button className="button secondary guarded-control" value="cancel" type="button" onClick={()=>dialog.current?.close()}>Cancel</button><button className="button primary guarded-control" type="submit"><DownloadSimple aria-hidden="true"/>Download CSV</button></div>
      </form>
    </dialog>
  </div>;
}
