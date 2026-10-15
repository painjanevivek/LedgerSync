"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { DisclosureSection } from "@/ui/disclosure/DisclosureSection";
import { StatePanel } from "@/ui/display/StatePanel";
import { Money } from "@/ui/display/Money";
import { Timestamp } from "@/ui/display/Timestamp";
import { CopyControl } from "@/ui/controls/CopyControl.client";
import { EvidenceFreshness } from "@/ui/display/Evidence";
import type { TransferResult } from "@/lib/api/transfers";

type SavedRequest = Readonly<{
 request_reference: string; source_account_id: string; destination_account_id: string;
 currency: string; amount_minor: string; created_at: string; state: "unresolved" | "posted" | "rejected"; transfer_id?: string;
}>;

function RecoveryInbox() {
 const {session, online, hasScope}=useConsoleSession();
 const [items,setItems]=useState<SavedRequest[]>([]);
 const [cursor,setCursor]=useState<string>();
 const [nextCursor,setNextCursor]=useState<string>();
 const [verifiedAt,setVerifiedAt]=useState<string>();
 const [error,setError]=useState<string>();
 const [loading,setLoading]=useState(false);
 const [selected,setSelected]=useState<SavedRequest>();
 const [pending,setPending]=useState(false);
 const [result,setResult]=useState<TransferResult>();
 const [notice,setNotice]=useState<string>();
 const requestGeneration=useRef({version:0});
 const posting=useRef(false);
 const review=useRef<HTMLDialogElement>(null);
 const originButton=useRef<HTMLButtonElement | null>(null);
 const canWrite=hasScope("transfers:write");

 const load=useCallback(async()=>{
  if(!online) return;
  const generation=++requestGeneration.current.version;
  setLoading(true);setError(undefined);setItems([]);setVerifiedAt(undefined);
  try {
   const response=await fetch(`/api/transfers/recovery${cursor?`?cursor=${encodeURIComponent(cursor)}`:""}`,{cache:"no-store"});
   if(!response.ok) throw new Error("Saved requests could not be loaded. Check your access and refresh.");
   const page=await response.json() as {requests?: SavedRequest[];next_cursor?: string};
   if(!Array.isArray(page.requests)) throw new Error("Saved request evidence is unavailable.");
   if(generation!==requestGeneration.current.version) return;
   setItems(page.requests);setNextCursor(page.next_cursor);setVerifiedAt(new Date().toISOString());
  } catch(error) {if(generation===requestGeneration.current.version) setError(error instanceof Error?error.message:"Saved requests unavailable.");}
  finally {if(generation===requestGeneration.current.version) setLoading(false);}
 },[cursor,online]);
 useEffect(()=>{const coordinator=requestGeneration.current;const timer=setTimeout(()=>void load(),0);return()=>{clearTimeout(timer);coordinator.version++;};},[load]);
 useEffect(()=>{if(selected) review.current?.showModal();},[selected]);

 async function retry(){
  if(!selected || !session || !online || !canWrite || loading || error || posting.current) return;
  posting.current=true;setPending(true);setNotice(undefined);setResult(undefined);
  try {
   const response=await fetch(`/api/transfers/recovery/${selected.request_reference}`,{method:"POST",headers:{"X-CSRF-Token":session.csrf_token}});
   const payload=await response.json().catch(()=>({})) as Partial<TransferResult>;
   if(!response.ok || !payload.transfer_id || !["posted","rejected"].includes(payload.status??"")) {
    setNotice("The result is not confirmed. Refresh saved requests before another retry. The original request remains unchanged.");
   } else {
    setResult(payload as TransferResult);
    setNotice(payload.status==="posted"?"The original transfer is posted. Open its receipt to verify the journal.":"The original transfer was rejected. No money moved.");
   }
  } catch {setNotice("The result is not confirmed. Refresh saved requests before another retry. The original request remains unchanged.");}
  finally {posting.current=false;setPending(false);review.current?.close();setSelected(undefined);originButton.current?.focus();await load();}
 }

 return <section className="transfer-recovery" aria-label="Saved transfer requests">
  <p>Recover a confirmed request after browser data is lost. Sign in as the same user in the same tenant. Opening this list does not move money.</p>
  <div className="action-row"><button className="button secondary" disabled={!online || loading || pending} onClick={()=>void load()}>Refresh saved requests</button>{cursor&&<button className="button secondary" disabled={loading || pending} onClick={()=>setCursor(undefined)}>Newest saved requests</button>}</div>
  {(verifiedAt || loading)&&<EvidenceFreshness state={!online || error ? "historical" : loading ? "refreshing" : "current"} verifiedAt={verifiedAt} label="Saved request evidence" />}
  {error&&<StatePanel kind="error" title="Saved requests unavailable" message={error}/>}
  {!online&&<StatePanel kind="unknown" title="Connect to recover" message="Reconnect and refresh before acting on a saved request."/>}
  {loading&&<p role="status">Loading saved requests…</p>}
  {verifiedAt&&!loading&&!error&&items.length===0&&<StatePanel title="No saved requests in this page" message="This authorized page contains no saved intent. This is not proof that a request from an older client failed. Check transfer history and exact references before creating another transfer."/>}
  {notice&&<StatePanel kind={result?.status==="posted"?undefined:result?.status==="rejected"?"error":"unknown"} title={result?.status==="posted"?"Recovered transfer posted":result?.status==="rejected"?"Recovered transfer rejected":"Check the original result"} message={notice} action={result?.transfer_id?<Link className="text-link" href={`/transfers/${result.transfer_id}`}>Open recovered receipt</Link>:undefined}/>}
  <ul className="evidence-list">{items.map(item=><li key={item.request_reference}>
   <p><CopyControl value={item.request_reference}/> · <Money currency={item.currency} minorUnits={item.amount_minor}/> · {item.state==="unresolved"?"Result not yet confirmed":item.state}</p>
   <p>From <code>{item.source_account_id}</code> to <code>{item.destination_account_id}</code></p>
   <p>Saved <Timestamp value={item.created_at}/></p>
   {item.transfer_id?<Link className="text-link" href={`/transfers/${item.transfer_id}`}>Open original {item.state} receipt</Link>:<button className="button secondary" aria-label={`Review saved request ${item.request_reference}`} disabled={!online||!canWrite||loading||pending||Boolean(error)} onClick={event=>{originButton.current=event.currentTarget;setSelected(item);}}>Review saved request</button>}
  </li>)}</ul>
  {nextCursor&&<button className="button secondary" disabled={loading||pending||!online} onClick={()=>setCursor(nextCursor)}>Older saved requests</button>}
  {!canWrite&&<p className="permission-note">Read-only access. Transfer write authority is required to retry.</p>}
  <dialog className="confirmation-dialog" ref={review} aria-labelledby="saved-request-review" onCancel={event=>{if(pending)event.preventDefault();}} onClose={()=>{setSelected(undefined);originButton.current?.focus();}}>
   <form onSubmit={event=>{event.preventDefault();void retry();}}>
   <h2 id="saved-request-review">Recover the original transfer?</h2>
   {selected&&<><p>Verify the original accounts and exact amount. If posting already committed, the server returns the saved result. If it did not commit, this action can post the original transfer after current checks pass.</p>
    <dl className="evidence-list"><div><dt>Request reference</dt><dd>{selected.request_reference}</dd></div><div><dt>From</dt><dd>{selected.source_account_id}</dd></div><div><dt>To</dt><dd>{selected.destination_account_id}</dd></div><div><dt>Exact amount</dt><dd><Money currency={selected.currency} minorUnits={selected.amount_minor}/></dd></div></dl>
    <p>The server reuses the original key. Editing is not permitted.</p></>}
   <div className="action-row"><button type="button" className="button secondary" disabled={pending} onClick={()=>review.current?.close()}>Cancel recovery</button><button type="submit" className="button primary" disabled={pending||!online||!canWrite||loading||Boolean(error)}>{pending?"Checking original transfer…":"Retry original request"}</button></div>
   </form>
  </dialog>
 </section>;
}

export function TransferRecovery(){
 return <DisclosureSection id="saved-transfer-recovery" title="Recover a saved transfer" summary="Find the original request after browser data is lost." lazy><RecoveryInbox/></DisclosureSection>;
}
