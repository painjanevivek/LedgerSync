"use client";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { parseStatementBody, type StatementReview, type StatementReviewInput } from "@/lib/api/statement-reconciliation";
import { FormField } from "@/ui/forms/FormField.client";
import { Money } from "@/ui/display/Money";
import styles from "./statement.module.css";
export function StatementReviewEditor({ item, actor, busy, blocked, onSave, onClose }: Readonly<{
    item: StatementReview;
    actor: string;
    busy: boolean;
    blocked: boolean;
    onSave: (body: StatementReviewInput) => Promise<void>;
    onClose: () => void;
}>) {
    const [status, setStatus] = useState(item.status), [assignee, setAssignee] = useState(item.assignee), [reason, setReason] = useState(item.reason || "investigation"), [comment, setComment] = useState(item.comment), [evidence, setEvidence] = useState(item.evidence), [posting, setPosting] = useState(item.linked_posting_id), [error, setError] = useState<string | null>(null);
    const heading = useRef<HTMLHeadingElement>(null), fixedMatch = item.status === "matched" && !!item.linked_posting_id;
    useEffect(() => { heading.current?.focus(); }, []);
    async function save(event: FormEvent<HTMLFormElement>) {
        event.preventDefault();
        if (busy || blocked)
            return;
        setError(null);
        const body: StatementReviewInput = { kind: item.kind, item_id: item.item_id, expected_version: item.version, status, assignee, reason, comment, evidence, ...(status === "matched" ? { posting_id: posting } : {}) };
        try {
            parseStatementBody(body, "review");
            await onSave(body);
        }
        catch {
            setError("Check the operator, reason, bounded comment, internal evidence link and exact ledger item identifier.");
        }
    }
    return <section className={`surface ${styles.panel}`} aria-labelledby="statement-review-heading" onKeyDown={e => {
            if (e.key === "Escape" && !busy && !blocked) {
                e.preventDefault();
                onClose();
            }
        }}>
  <h2 id="statement-review-heading" ref={heading} tabIndex={-1}>Review {item.kind === "statement" ? "statement row" : "ledger item"}</h2>
  <p><code>{item.item_id}</code> · <Money currency={item.currency} minorUnits={item.amount_minor}/> · {item.direction}</p>
  <p>The original classification remains <strong>{item.classification.replaceAll("_", " ")}</strong>. A reviewed difference does not become a financial adjustment.</p>
  <form onSubmit={event => void save(event)}><fieldset disabled={busy || blocked} className={styles.fields}><legend>Decision at review version {item.version}</legend>
   <FormField label="Review status" requirement="required"><select className="evidence-input" value={status} onChange={event => setStatus(event.target.value as StatementReview["status"])}>{!fixedMatch && <><option value="open">Keep open</option><option value="reviewed_exception">Reviewed — difference remains</option></>}{(item.kind === "statement" || fixedMatch) && <option value="matched">Link exact ledger item</option>}</select></FormField>
   {status === "matched" && <FormField label="Ledger posting identifier" requirement="required" hint="Use an item in this frozen comparison. Amount, currency and direction must agree. Only the reference may differ."><input className="evidence-input" value={posting} readOnly={fixedMatch} required onChange={event => setPosting(event.target.value)}/></FormField>}
   <FormField label="Assigned operator" requirement="optional" hint={`Your operator identifier is ${actor}. Leave blank to keep this unassigned.`}><input className="evidence-input" value={assignee} maxLength={256} onChange={event => setAssignee(event.target.value)}/></FormField>
   <button className="button secondary" type="button" onClick={() => setAssignee(actor)}>Assign to me</button>
   <FormField label="Reason" requirement="required"><select className="evidence-input" value={reason} onChange={event => setReason(event.target.value)}>{["investigation", "confirmed", "external_missing", "amount_difference", "reference_override", "other"].map(value => <option value={value} key={value}>{value.replaceAll("_", " ")}</option>)}</select></FormField>
   <FormField label="Comment" requirement="optional" hint="Maximum 512 UTF-8 bytes. Comments stay with the decision history."><textarea className="evidence-input" value={comment} maxLength={512} onChange={event => setComment(event.target.value)}/></FormField>
   <FormField label="Internal evidence link" requirement="optional" hint="Use a /transfers/, /funding/ or /reconciliation/ record path."><input className="evidence-input" value={evidence} maxLength={256} onChange={event => setEvidence(event.target.value)}/></FormField>
  </fieldset>{error && <p role="alert">{error}</p>}<div className={styles.actions}><button className="button primary" type="submit" disabled={busy || blocked}>{busy ? "Saving original decision…" : "Save review decision"}</button><button className="button secondary" type="button" disabled={busy || blocked} onClick={onClose}>Cancel edit</button></div></form>
 </section>;
}
