"use client";
import Link from "next/link";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { ConsoleRouteFrame } from "@/features/console/ConsoleRouteFrame";
import type { ConsoleSession } from "@/features/accounts/types";
import { parseReservationBody, type Reservation, type ReservationCreate, type ReservationReceipt } from "@/lib/api/reservations";
import { CopyControl } from "@/ui/controls/CopyControl.client";
import { CsvEvidenceDownload } from "@/ui/controls/CsvEvidenceDownload.client";
import { DataTableRegion } from "@/ui/display/DataTableRegion";
import { Money } from "@/ui/display/Money";
import { PageHeader } from "@/ui/display/PageHeader";
import { StatePanel } from "@/ui/display/StatePanel";
import { Timestamp } from "@/ui/display/Timestamp";
import { FormField } from "@/ui/forms/FormField.client";
import { useReservationCommand } from "./useReservationCommand";
import { useReservationEvidence } from "./useReservationEvidence";
import styles from "./reservations.module.css";
type Review = Readonly<{
    kind: "create";
    body: ReservationCreate;
} | {
    kind: "decision";
    hold: Reservation;
    decision: "capture" | "release";
}>;
export function ReservationsController({ invalid = false }: Readonly<{
    invalid?: boolean;
}>) {
    const { session, online, hasScope } = useConsoleSession();
    return <ConsoleRouteFrame section="accounts" loadingLabel="Internal reservations">{session && (invalid || !hasScope("transfers:read") ? <><PageHeader title="Internal reservations" description="Read and manage authorized internal reservations."/><StatePanel kind={invalid ? "error" : "denied"} title={invalid ? "Invalid reservation link" : "Reservation access required"} message={invalid ? "This workspace does not accept query parameters." : "Your session cannot read reservation evidence."}/></> : <ReservationWorkspace key={`${session.tenant_id}:${session.subject_id}`} session={session} online={online} canWrite={hasScope("transfers:write")}/>)}</ConsoleRouteFrame>;
}
function ReservationWorkspace({ session, online, canWrite }: Readonly<{
    session: ConsoleSession;
    online: boolean;
    canWrite: boolean;
}>) {
    const evidence = useReservationEvidence(online), command = useReservationCommand(session.tenant_id, session.subject_id, session.csrf_token, online);
    const canExport = session.scopes.includes("exports:read");
    const [source, setSource] = useState(""), [destination, setDestination] = useState(""), [amount, setAmount] = useState(""), [ttl, setTTL] = useState("3600"), [review, setReview] = useState<Review | null>(null), [formError, setFormError] = useState<string | null>(null);
    const heading = useRef<HTMLHeadingElement>(null), alive = useRef(true), returnFocus = useRef<string | null>(null);
    useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
    useEffect(() => {
        if (review)
            heading.current?.focus();
    }, [review]);
    const disabled = !online || !canWrite || command.pending || command.blocked || !!command.intent;
    async function confirmed(result: ReservationReceipt | null) {
        if (!result || !alive.current)
            return;
        setReview(null);
        await evidence.read("", result.id);
    }
    function prepare(event: FormEvent<HTMLFormElement>) {
        event.preventDefault();
        setFormError(null);
        try {
            if (!/^[1-9][0-9]{0,4}$/.test(ttl))
                throw Error("Use an expiry from 1 to 86400 seconds.");
            const body = parseReservationBody({ debit_account_id: source, credit_account_id: destination, amount_minor: amount, currency: "INR", ttl_seconds: Number(ttl) }, false) as ReservationCreate;
            returnFocus.current = "reservation-review-create";
            setReview({ kind: "create", body });
        }
        catch (cause) {
            setFormError(cause instanceof Error ? cause.message : "Check the reservation fields.");
        }
    }
    function closeReview() {
        if (command.pending || command.intent)
            return;
        setReview(null);
        window.requestAnimationFrame(() => document.getElementById(returnFocus.current ?? "")?.focus());
    }
    function reviewDecision(hold: Reservation, decision: "capture" | "release") { returnFocus.current = `reservation-${decision}-${hold.id}`; setReview({ kind: "decision", hold, decision }); }
    async function sendReview() {
        if (!review || disabled)
            return;
        if (review.kind === "create")
            await confirmed(await command.execute("/api/reservations", review.body));
        else
            await confirmed(await command.execute(`/api/reservations/${review.hold.id}/decisions`, { decision: review.decision }));
    }
    return <div className={styles.workspace}>
  <PageHeader title="Internal reservations" description="Reserve spendable funds for one internal destination. Capture the full amount, release it, or let the database expiry worker release it."><Link className="button secondary" href="/accounts">Accounts</Link><button className="button secondary" type="button" disabled={!online || evidence.busy} onClick={() => void evidence.read(evidence.cursor, evidence.current?.id)}>Refresh reservation evidence</button></PageHeader>
  <p>Posted ledger minus active reservations equals available funds. A reservation does not post a journal. Capture posts one balanced internal transfer. Version 1 supports INR, full capture and full release.</p>
  <p>Export one authorized database snapshot of at most 2,000 holds. Active obligations remain reserved after their deadline until a terminal transaction commits. The CSV is evidence, not a backup. Import exact minor-unit columns as text in a spreadsheet.</p>
  {canExport && <CsvEvidenceDownload path="/api/reservations/export.csv" filename="reservation-obligations.csv" label="Export reservation obligations CSV" maxBytes={2097152} disabled={!online || evidence.busy || !!evidence.error}/>}
  {!online && <StatePanel kind="offline" title="Connection unavailable" message="Earlier evidence is historical. Commands wait for a connection."/>}
  {command.error && <StatePanel kind={command.intent ? "unknown" : "error"} title={command.receipt ? "Command confirmed; recovery cleanup needs attention" : command.intent ? "Original reservation request retained" : "Reservation request needs attention"} message={command.error}/>}
  {command.intent && <section className={`surface ${styles.panel}`} aria-labelledby="reservation-recovery"><h2 id="reservation-recovery">Continue the original reservation request</h2><p>Its original destination, exact payload and key stay fixed. Do not replace a key to resolve an unknown result.</p><CopyControl value={command.intent.key} label="Copy original reservation request key"/><div className={styles.actions}><button className="button primary" type="button" disabled={!online || !canWrite || command.pending || command.blocked} onClick={() => void command.execute().then(confirmed)}>Retry original reservation request</button>{command.rejected && <button className="button secondary" type="button" disabled={command.pending} onClick={command.closeRejected}>Close this rejected reservation request</button>}</div></section>}
  {command.receipt && <section className={`surface ${styles.panel}`} role="status"><h2>Reservation command confirmed</h2><p>{command.receipt.replayed ? "The original receipt was recovered." : "The server confirmed the command."} Original outcome: {command.receipt.state}.</p>{command.receipt.rejection_code && <p>Capture was denied ({command.receipt.rejection_code.replaceAll("_", " ")}). The hold stays active and funds stay reserved. This key keeps that denied result. Read the current hold before another decision.</p>}<p>This is the original command receipt. The current hold can have a later state.</p><CopyControl value={command.receipt.id} label="Copy confirmed reservation identifier"/>{command.receipt.transfer_id && <Link className="button secondary" href={`/transfers/${command.receipt.transfer_id}`}>Open captured transfer</Link>}</section>}
  {evidence.error && <StatePanel kind="error" title="Reservation evidence unavailable" message={evidence.error} action={<button className="button secondary" type="button" disabled={!online || evidence.busy} onClick={() => void evidence.read(evidence.cursor, evidence.current?.id)}>Retry reservation read</button>}/>}
  {evidence.current && <section className={`surface ${styles.panel}`} aria-labelledby="reservation-current"><h2 id="reservation-current">Current reservation evidence</h2><p>{evidence.error ? "Historical read: " : "Authorized database read: "}{evidence.current.state}. Hold version {evidence.current.version}.</p><CopyControl value={evidence.current.id}/><p>Expiry: <Timestamp value={evidence.current.expires_at}/>. An active hold stays reserved until a terminal database transaction commits.</p></section>}
  <section className={`surface ${styles.panel}`} aria-labelledby="reservation-create"><h2 id="reservation-create">Reserve funds</h2><p>Use two authorized, active INR customer accounts. The server checks ownership, destination permission and current available funds. Expiry is measured by the database clock.</p>{!canWrite ? <StatePanel kind="denied" title="Reservation write access required" message="You can inspect evidence. Your session cannot reserve or decide funds."/> : <form onSubmit={prepare}><fieldset disabled={disabled} className={styles.fields}><legend>Reservation details</legend><FormField label="Reservation source account" requirement="required"><input className="evidence-input" value={source} onChange={e => setSource(e.target.value)} required spellCheck={false}/></FormField><FormField label="Reservation destination account" requirement="required"><input className="evidence-input" value={destination} onChange={e => setDestination(e.target.value)} required spellCheck={false}/></FormField><FormField label="Reservation amount in minor units" requirement="required" hint="100 minor units equals INR 1.00. Use an exact positive integer."><input className="evidence-input" value={amount} inputMode="numeric" pattern="[1-9][0-9]*" onChange={e => setAmount(e.target.value)} required/></FormField><FormField label="Reservation expiry in seconds" requirement="required" hint="From 1 to 86400 seconds. This value cannot be changed after acceptance."><input className="evidence-input" value={ttl} inputMode="numeric" pattern="[1-9][0-9]*" onChange={e => setTTL(e.target.value)} required/></FormField><button id="reservation-review-create" className="button primary" type="submit">Review reservation</button></fieldset>{formError && <p role="alert">{formError}</p>}</form>}</section>
  {review && <section className={`surface ${styles.panel}`} aria-labelledby="reservation-review-title" onKeyDown={e => {
                if (e.key === "Escape")
                    closeReview();
            }}><h2 id="reservation-review-title" ref={heading} tabIndex={-1}>{review.kind === "create" ? "Confirm reservation" : review.decision === "capture" ? "Confirm full capture" : "Confirm full release"}</h2>{review.kind === "create" ? <dl className={styles.details}><div><dt>Source</dt><dd><code>{review.body.debit_account_id}</code></dd></div><div><dt>Destination</dt><dd><code>{review.body.credit_account_id}</code></dd></div><div><dt>Reserved amount</dt><dd><Money currency="INR" minorUnits={review.body.amount_minor}/></dd></div><div><dt>Database-clock lifetime</dt><dd>{review.body.ttl_seconds} seconds</dd></div></dl> : <><CopyControl value={review.hold.id}/><p><Money currency={review.hold.currency} minorUnits={review.hold.amount_minor}/> · full {review.decision}.</p><p>{review.decision === "capture" ? "The server rechecks the deadline, account status, current authority and transfer limits. A denial keeps this reservation." : "Release returns the full reserved amount to available funds. It does not post a journal."}</p></>}<div className={styles.actions}><button className="button primary" type="button" disabled={disabled} onClick={() => void sendReview()}>{command.pending ? "Sending original request…" : review.kind === "create" ? "Confirm reserve funds" : review.decision === "capture" ? "Capture full amount" : "Release full amount"}</button><button className="button secondary" type="button" disabled={command.pending || !!command.intent} onClick={closeReview}>Return without sending</button></div></section>}
  <section className={`surface ${styles.panel}`} aria-labelledby="reservation-list" aria-busy={evidence.busy}><h2 id="reservation-list">Authorized reservations</h2>{!evidence.page ? (!evidence.error && <p>Loading authorized evidence. No empty result is inferred.</p>) : evidence.page.holds.length === 0 ? <p>No reservations are present on this authorized page.</p> : <DataTableRegion label="Authorized reservation evidence"><table><thead><tr><th scope="col">Reservation</th><th scope="col">Exact amount</th><th scope="col">Current state / expiry</th><th scope="col">Action</th></tr></thead><tbody>{evidence.page.holds.map(item => <tr key={item.id}><td><CopyControl value={item.id}/><details><summary>Accounts</summary><p>Source <code>{item.debit_account_id}</code></p><p>Destination <code>{item.credit_account_id}</code></p></details></td><td><Money currency={item.currency} minorUnits={item.amount_minor}/></td><td>{item.state} · version {item.version}<br /><Timestamp value={item.expires_at}/></td><td>{item.state === "active" ? <div className={styles.actions}><button id={`reservation-capture-${item.id}`} className="button secondary" type="button" disabled={disabled || evidence.busy || !!evidence.error} onClick={() => reviewDecision(item, "capture")}>Review capture</button><button id={`reservation-release-${item.id}`} className="button secondary" type="button" disabled={disabled || evidence.busy || !!evidence.error} onClick={() => reviewDecision(item, "release")}>Review release</button></div> : item.transfer_id ? <Link className="button secondary" href={`/transfers/${item.transfer_id}`}>View captured transfer</Link> : "Terminal evidence"}</td></tr>)}</tbody></table></DataTableRegion>}<div className={styles.actions}>{evidence.cursor && <button type="button" className="button secondary" disabled={!online || evidence.busy} onClick={() => void evidence.read()}>First reservation page</button>}{evidence.page?.next_cursor && <button type="button" className="button secondary" disabled={!online || evidence.busy} onClick={() => void evidence.read(evidence.page!.next_cursor)}>Next reservation page</button>}</div></section>
 </div>;
}
