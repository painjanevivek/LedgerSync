"use client";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useId, useRef, useState, type ChangeEvent, type FormEvent } from "react";
import type { ConsoleSession } from "@/features/accounts/types";
import { ConsoleRouteFrame } from "@/features/console/ConsoleRouteFrame";
import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { utcDateTime } from "@/features/console/format";
import { isStatementPreview, maxStatementSourceBytes, parseStatementBody, type StatementPreview, type StatementReview, type StatementReviewInput } from "@/lib/api/statement-reconciliation";
import { readStatementBytes } from "@/lib/statement-stream";
import { CopyControl } from "@/ui/controls/CopyControl.client";
import { DataTableRegion } from "@/ui/display/DataTableRegion";
import { Money } from "@/ui/display/Money";
import { PageHeader } from "@/ui/display/PageHeader";
import { StatePanel } from "@/ui/display/StatePanel";
import { FormField } from "@/ui/forms/FormField.client";
import { StatementDownload } from "./StatementDownload";
import { StatementReviewEditor } from "./StatementReviewEditor";
import { statementDigest } from "./statementIntent";
import { useStatementCommand, type StatementCommandResult } from "./useStatementCommand";
import { useStatementWorkspace } from "./useStatementWorkspace";
import styles from "./statement.module.css";
export function StatementReconciliationController({ runID, invalid = false }: Readonly<{
    runID?: string;
    invalid?: boolean;
}>) {
    const { session, online, hasScope } = useConsoleSession();
    return <ConsoleRouteFrame section="reconciliation" loadingLabel="Statement comparison">
  {session && !hasScope("reconciliation:read") ? <><PageHeader title="External statement comparison" description="Compare supplied evidence with a saved ledger snapshot."/><StatePanel kind="denied" title="Statement access required" message="Your session cannot read statement comparisons."/></> : session && invalid ? <><PageHeader title="External statement comparison" description="Compare supplied evidence with a saved ledger snapshot."/><StatePanel kind="error" title="Invalid statement link" message="The comparison identifier or query is invalid. No statement result was inferred." action={<Link className="button secondary" href="/reconciliation/statements">Open statement workspace</Link>}/></> : session && <AuthorizedStatementWorkspace key={`${session.tenant_id}:${session.subject_id}:${runID ?? "workspace"}`} session={session} online={online} canWrite={hasScope("reconciliation:write")} canExport={hasScope("exports:read")} runID={runID}/>}
 </ConsoleRouteFrame>;
}
function AuthorizedStatementWorkspace({ session, online, canWrite, canExport, runID }: Readonly<{
    session: ConsoleSession;
    online: boolean;
    canWrite: boolean;
    canExport: boolean;
    runID?: string;
}>) {
    const router = useRouter(), accountsList = useId(), workspace = useStatementWorkspace(), command = useStatementCommand(session.tenant_id, session.subject_id, session.csrf_token, online);
    const { refresh, loadRun } = workspace;
    const [account, setAccount] = useState(""), [sourceRef, setSourceRef] = useState("synthetic-clearing"), [mappingReview, setMappingReview] = useState<{
        account_id: string;
        source_account_ref: string;
    } | null>(null), [mappingError, setMappingError] = useState<string | null>(null);
    const [source, setSource] = useState<Uint8Array | null>(null), [sourceSHA, setSourceSHA] = useState<string | null>(null), [preview, setPreview] = useState<StatementPreview | null>(null), [fileBusy, setFileBusy] = useState(false), [fileError, setFileError] = useState<string | null>(null), [review, setReview] = useState<StatementReview | null>(null);
    const fileGeneration = useRef(0), fileInFlight = useRef(false), alive = useRef(true);
    useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
    const disabled = !online || !canWrite || command.pending || command.blocked || !!command.intent || fileBusy;
    useEffect(() => {
        if (!online)
            return;
        const timer = window.setTimeout(() => void refresh(), 0);
        return () => window.clearTimeout(timer);
    }, [online, refresh]);
    useEffect(() => {
        if (!online || !runID)
            return;
        const timer = window.setTimeout(() => void loadRun(runID), 0);
        return () => window.clearTimeout(timer);
    }, [loadRun, online, runID]);
    const afterCommand = useCallback(async (result: StatementCommandResult | null) => {
        if (!result || !alive.current)
            return;
        if (result.kind === "mapping")
            setMappingReview(null);
        if (result.kind === "import") {
            setPreview(null);
            setSource(null);
            setSourceSHA(null);
        }
        if (result.kind === "review")
            setReview(null);
        if (result.kind === "run") {
            router.replace(`/reconciliation/statements?run=${result.receipt.id}`);
            await refresh();
        }
        else if (runID)
            await Promise.all([refresh(), loadRun(runID)]);
        else
            await refresh();
    }, [loadRun, refresh, router, runID]);
    function prepareMapping(event: FormEvent<HTMLFormElement>) {
        event.preventDefault();
        setMappingError(null);
        try {
            const body = { account_id: account, source_account_ref: sourceRef };
            parseStatementBody(body, "mapping");
            setMappingReview(body);
        }
        catch {
            setMappingError("Choose an active INR account and a source reference with letters, digits, dots, dashes, slashes or colons.");
        }
    }
    async function chooseFile(event: ChangeEvent<HTMLInputElement>) {
        const file = event.target.files?.[0];
        if (!file || fileInFlight.current)
            return;
        fileInFlight.current = true;
        const generation = ++fileGeneration.current;
        setFileBusy(true);
        setFileError(null);
        setPreview(null);
        setSource(null);
        setSourceSHA(null);
        try {
            if (!file.size || file.size > maxStatementSourceBytes)
                throw new Error("Choose one non-empty CSV file of at most 1 MiB.");
            const bytes = new Uint8Array(await file.arrayBuffer());
            if (bytes.byteLength !== file.size || bytes.byteLength > maxStatementSourceBytes)
                throw new Error("The selected file size changed. Choose it again.");
            const digest = await statementDigest(bytes);
            if (command.intent?.kind === "import") {
                if (digest !== command.intent.sourceSHA256)
                    throw new Error("This is not the original file. Its digest differs. The saved request was preserved.");
                if (generation === fileGeneration.current) {
                    setSource(bytes);
                    setSourceSHA(digest);
                }
                return;
            }
            let binary = "";
            for (let n = 0; n < bytes.length; n += 32768)
                binary += String.fromCharCode(...bytes.subarray(n, n + 32768));
            const response = await fetch("/api/statement-reconciliation/preview", { method: "POST", cache: "no-store", headers: { "Content-Type": "application/json", "X-CSRF-Token": session.csrf_token, "Idempotency-Key": crypto.randomUUID() }, body: JSON.stringify({ source_base64: btoa(binary) }), signal: AbortSignal.timeout(15000) });
            const value = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(await readStatementBytes(response.body, 16384))) as unknown;
            if (!response.ok) {
                const message = (value as {
                    error?: {
                        message?: unknown;
                    };
                })?.error?.message;
                throw new Error(typeof message === "string" ? message : "The complete file failed validation. Check CSV v1, exact amounts, period and totals.");
            }
            if (!isStatementPreview(value) || value.sha256 !== digest)
                throw new Error("The preview does not identify the selected original file.");
            if (value.source_account_ref !== workspace.workspace?.mapping?.source_account_ref)
                throw new Error("The file source reference does not agree with the fixed mapping.");
            if (generation === fileGeneration.current) {
                setSource(bytes);
                setSourceSHA(digest);
                setPreview(value);
            }
        }
        catch (cause) {
            if (generation === fileGeneration.current)
                setFileError(cause instanceof Error ? cause.message : "The file could not be validated. Nothing was accepted.");
        }
        finally {
            if (generation === fileGeneration.current)
                setFileBusy(false);
            fileInFlight.current = false;
        }
    }
    function closeReview() {
        const previous = review;
        setReview(null);
        if (previous)
            window.requestAnimationFrame(() => document.getElementById(`statement-review-${previous.kind}-${previous.item_id}`)?.focus());
    }
    const data = workspace.workspace, page = workspace.page;
    return <div className={styles.workspace}>
  <PageHeader title="External statement comparison" description="Compare supplied CSV evidence with a fixed ledger snapshot. Review differences without posting an adjustment."><Link className="button secondary" href="/reconciliation">Internal reconciliation</Link><button className="button secondary" type="button" disabled={!online || workspace.workspaceLoading} onClick={() => void refresh()}>Refresh workspace</button></PageHeader>
  <p className={styles.boundary}>Synthetic CSV v1 · INR · one fixed clearing mapping. A matching file does not prove bank settlement.</p>
  {!online && <StatePanel kind="offline" title="Connection unavailable" message="Saved evidence stays visible. New requests and downloads wait for a connection."/>}
  {command.error && <StatePanel kind={command.intent ? "unknown" : "error"} title={command.result ? "Command confirmed; follow-up needs attention" : command.intent ? "Original statement request retained" : "Statement request needs attention"} message={command.error}/>}
  {command.result && <section className={`surface ${styles.panel}`} role="status"><h2>Command confirmed</h2><p>{command.result.receipt.replayed ? "The original result was recovered." : "The server confirmed this command."} No financial journal was changed.</p><CopyControl value={command.result.receipt.id} label="Copy confirmed statement resource"/></section>}
  {command.intent && <section className={`surface ${styles.panel}`} aria-labelledby="statement-recovery-title"><h2 id="statement-recovery-title">Continue the original request</h2><p>Keep its original destination, payload and key. A new key is not an automatic recovery action.</p><CopyControl value={command.intent.key} label="Copy original statement request key"/>
   {command.intent.kind === "import" && <><p>Reselect the exact original file if it is no longer in memory. Raw source bytes are not saved in browser recovery storage.</p><FormField label="Original CSV for recovery" requirement="required"><input className="evidence-input" type="file" accept=".csv,text/csv" onChange={event => void chooseFile(event)} disabled={fileBusy || command.pending}/></FormField><p>{sourceSHA === command.intent.sourceSHA256 ? "The selected file matches the saved digest." : "The original file is required before retry."}</p>{fileError && <p role="alert">{fileError}</p>}</>}
   <div className={styles.actions}><button className="button primary" type="button" disabled={!online || !canWrite || command.pending || command.blocked || command.intent.kind === "import" && sourceSHA !== command.intent.sourceSHA256} onClick={() => void command.retry(source ?? undefined).then(afterCommand)}>{command.pending ? "Sending original request…" : "Retry original request"}</button>{command.rejected && <button className="button secondary" type="button" disabled={command.pending} onClick={command.closeRejected}>Close this rejected request</button>}</div>
  </section>}
  {workspace.workspaceError && <StatePanel kind="error" title="Statement workspace unavailable" message={workspace.workspaceError} action={<button className="button secondary" type="button" disabled={!online || workspace.workspaceLoading} onClick={() => void refresh()}>Retry workspace read</button>}/>}
  {!data ? (!workspace.workspaceError && <StatePanel title="Loading statement evidence" message="No empty import list or matching result is inferred while the authorized read is pending."/>) : <>
   <section className={`surface ${styles.panel}`} aria-labelledby="statement-mapping-title"><h2 id="statement-mapping-title">Clearing account mapping</h2>
    {data.mapping ? <><p><strong>{data.mapping.source_account_ref}</strong> is mapped to one INR account. Mapping version {data.mapping.version} stays fixed.</p><CopyControl value={data.mapping.account_id} label="Copy mapped clearing account"/></> : !canWrite ? <StatePanel kind="denied" title="Mapping write access required" message="Your session can inspect evidence but cannot create a mapping."/> : mappingReview ? <><p>Confirm this fixed mapping. Version 1 does not provide a change or delete action.</p><dl className={styles.details}><div><dt>Source reference</dt><dd>{mappingReview.source_account_ref}</dd></div><div><dt>INR account</dt><dd>{mappingReview.account_id}</dd></div></dl><div className={styles.actions}><button type="button" className="button primary" disabled={disabled} onClick={() => void command.run("mapping", "/api/statement-reconciliation/mappings", mappingReview).then(afterCommand)}>Confirm fixed mapping</button><button type="button" className="button secondary" disabled={command.pending || !!command.intent} onClick={() => setMappingReview(null)}>Return to edit</button></div></> : <form onSubmit={prepareMapping}><fieldset disabled={disabled} className={styles.fields}><legend>One source and one INR account</legend><FormField label="Mapped INR account" requirement="required" hint="Choose a listed account or enter its full identifier. This mapping does not move funds."><input className="evidence-input" list={accountsList} value={account} required onChange={event => setAccount(event.target.value)}/></FormField><datalist id={accountsList}>{data.accounts.map(option => <option key={option.id} value={option.id}>{option.display_name || option.id} · {option.kind}</option>)}</datalist>{data.more_accounts && <p>The list shows the first 100 active INR accounts. Enter an exact authorized account identifier if needed.</p>}<FormField label="Statement source account reference" requirement="required" hint="It must agree with every row in the CSV."><input className="evidence-input" value={sourceRef} maxLength={128} required onChange={event => setSourceRef(event.target.value)}/></FormField></fieldset>{mappingError && <p role="alert">{mappingError}</p>}<button type="submit" className="button primary" disabled={disabled}>Review fixed mapping</button></form>}
   </section>
   {data.mapping && canWrite && <section className={`surface ${styles.panel}`} aria-labelledby="statement-source-title"><h2 id="statement-source-title">Validate and accept a statement</h2><p>Preview validates the full file. Acceptance retains its immutable bytes, digest and rows. No partial import is accepted.</p><p><a href="/samples/statement-v1.csv" download>Download synthetic CSV v1 sample</a></p><FormField label="Statement CSV" requirement="required" hint="Unquoted UTF-8, eleven fixed columns, up to 1 MiB and 1,000 rows. Use UTC seconds and exact integer minor units."><input className="evidence-input" type="file" accept=".csv,text/csv" disabled={disabled} onChange={event => void chooseFile(event)}/></FormField>{fileBusy && <p role="status">Validating the complete source…</p>}{fileError && <p role="alert">{fileError}</p>}{preview && <><dl className={styles.details}><div><dt>Validated rows</dt><dd>{preview.row_count}</dd></div><div><dt>Period, UTC</dt><dd>{utcDateTime(preview.period_start)} to {utcDateTime(preview.period_end)}; end excluded</dd></div><div><dt>Opening / closing</dt><dd><Money currency="INR" minorUnits={preview.opening_minor}/> / <Money currency="INR" minorUnits={preview.closing_minor}/></dd></div></dl><details><summary>Original source checksum</summary><CopyControl value={preview.sha256} label="Copy statement source digest"/></details><button className="button primary" type="button" disabled={disabled || !source} onClick={() => void command.run("import", "/api/statement-reconciliation/imports", { mapping_id: data.mapping!.id }, source ?? undefined, preview.sha256).then(afterCommand)}>Accept immutable source</button></>}</section>}
   <section className={`surface ${styles.panel}`} aria-labelledby="statement-imports-title"><h2 id="statement-imports-title">Accepted statement sources</h2><p>At most 100 imports are retained in this bounded version. Identical bytes recover the accepted import. Source row identities cannot be reused across files.</p>{data.imports.length === 0 ? <StatePanel title="No accepted statements" message="A validated preview has not yet been accepted for this mapping."/> : <DataTableRegion label="Accepted statement source records"><table><thead><tr><th scope="col">Source period, UTC</th><th scope="col">Rows</th><th scope="col">Source identity</th><th scope="col">Comparison</th><th scope="col">Actions</th></tr></thead><tbody>{data.imports.map(item => <tr key={item.id}><td>{utcDateTime(item.period_start)}<br />{utcDateTime(item.period_end)}</td><td>{item.row_count}</td><td><code>{item.sha256}</code></td><td>{item.run_state ?? "Not prepared"}</td><td><div className={styles.actions}>{item.run_state === "completed" ? <Link className="button secondary" href={`/reconciliation/statements?run=${item.run_id}`}>View comparison</Link> : <button className="button secondary" type="button" disabled={disabled} onClick={() => void command.run("run", "/api/statement-reconciliation/runs", { import_id: item.id }).then(afterCommand)}>{item.run_state === "prepared" ? "Resume original snapshot" : "Compare fixed snapshot"}</button>}{canExport && <StatementDownload id={item.id} path={`/api/statement-reconciliation/imports/${item.id}/source.csv`} label="Download original" source disabled={!online}/>}</div></td></tr>)}</tbody></table></DataTableRegion>}</section>
  </>}
  {runID && <section className={`surface ${styles.panel}`} aria-labelledby="statement-result-title" aria-busy={workspace.runLoading}><h2 id="statement-result-title">Statement comparison and review</h2>{workspace.runError && <StatePanel kind="error" title="Comparison read unavailable" message={workspace.runError} action={<button className="button secondary" type="button" disabled={!online || workspace.runLoading} onClick={() => void loadRun(runID, workspace.cursor)}>Retry comparison read</button>}/>} {!page ? <p>No result is inferred until this authorized comparison loads.</p> : <>
   <p>Frozen cutoff: {utcDateTime(page.run.cutoff)}. Rule version {page.run.rule_version}; mapping version {page.run.mapping_version}. Later or backdated records cannot enter this saved snapshot.</p>
   {page.run.state !== "completed" ? <StatePanel kind="unknown" title="Prepared snapshot awaits completion" message="The original run can resume. No matching result is inferred yet."/> : <><dl className={styles.counts}><div><dt>Statement rows</dt><dd>{page.run.statement_row_count}</dd></div><div><dt>Eligible ledger items</dt><dd>{page.run.snapshot_item_count}</dd></div><div><dt>Linked rows</dt><dd>{page.run.matched_row_count}</dd></div><div><dt>Unlinked statement / ledger</dt><dd>{page.run.unmatched_statement_count} / {page.run.unmatched_ledger_count}</dd></div></dl><p>Reviewed exceptions remain unlinked differences. Links compare supplied evidence; they do not post an adjustment or prove external settlement.</p>{canExport && <StatementDownload id={page.run.id} path={`/api/statement-reconciliation/runs/${page.run.id}/comparison.csv`} label="Export comparison CSV" disabled={!online || workspace.runLoading}/>}
    <DataTableRegion label="Statement comparison review queue"><table><thead><tr><th scope="col">Evidence</th><th scope="col">Exact amount</th><th scope="col">Original classification</th><th scope="col">Review</th><th scope="col">Action</th></tr></thead><tbody>{page.items.map(item => <tr key={`${item.kind}:${item.item_id}`}><td>{item.kind}<br /><code>{item.item_id}</code><br /><code>{item.external_reference}</code></td><td><Money currency={item.currency} minorUnits={item.amount_minor}/><br />{item.direction}</td><td>{item.classification.replaceAll("_", " ")}</td><td>{item.status.replaceAll("_", " ")}<br />{item.assignee || "Unassigned"}<br />Version {item.version}</td><td><button id={`statement-review-${item.kind}-${item.item_id}`} className="button secondary" type="button" disabled={disabled || workspace.runLoading || item.kind === "ledger" && item.status === "matched"} onClick={() => setReview(item)}>Review item</button>{item.linked_posting_id && <details><summary>Linked ledger item</summary><CopyControl value={item.linked_posting_id}/></details>}</td></tr>)}</tbody></table></DataTableRegion>
    <div className={styles.actions}>{workspace.cursor && <button className="button secondary" type="button" disabled={!online || workspace.runLoading} onClick={() => void loadRun(runID)}>First review page</button>}{page.next_cursor && <button className="button secondary" type="button" disabled={!online || workspace.runLoading} onClick={() => void loadRun(runID, page.next_cursor)}>Next review page</button>}</div>
   </>}
  </>}</section>}
  {review && runID && <StatementReviewEditor key={`${review.kind}:${review.item_id}:${review.version}`} item={review} actor={session.subject_id} busy={command.pending} blocked={disabled} onClose={closeReview} onSave={async (body: StatementReviewInput) => { await afterCommand(await command.run("review", `/api/statement-reconciliation/runs/${runID}/reviews`, body)); }}/>}
 </div>;
}
