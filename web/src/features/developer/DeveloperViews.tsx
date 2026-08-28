"use client";

import { ArrowRight, Check, CheckCircle, Copy, DownloadSimple, Key, LockKey, ShieldCheck, WarningCircle } from "@phosphor-icons/react";
import Link from "next/link";
import { useState } from "react";

import { DataTableRegion, PageHeader, StatePanel, StatusBadge } from "@/features/console/components";
import type { DeveloperExample, DeveloperMetadata } from "@/lib/api/developer";

function CopyCode({ value, label }: Readonly<{ value:string; label:string }>) {
  const [status, setStatus] = useState<"idle"|"copied"|"failed">("idle");
  async function copy() {
    try { await navigator.clipboard.writeText(value); setStatus("copied"); }
    catch { setStatus("failed"); }
    window.setTimeout(() => setStatus("idle"), 1800);
  }
  return <div className="developer-code"><div><span>{label}</span><button type="button" className="code-copy-button" onClick={() => void copy()} aria-label={`Copy ${label}`}>{status === "copied" ? <Check aria-hidden="true"/> : <Copy aria-hidden="true"/>}{status === "copied" ? "Copied" : "Copy"}</button></div><pre tabIndex={0} aria-label={label}><code>{value}</code></pre><span className="sr-only" role="status" aria-live="polite">{status === "copied" ? `${label} copied` : status === "failed" ? `${label} copy failed` : ""}</span></div>;
}

function browserRequest(example: DeveloperExample) {
  const path = example.id === "create_account" ? "/api/me/accounts" : `/api${example.path}`;
  return `const session = await fetch("/api/session", { cache: "no-store" })\n  .then((response) => response.json());\n\nconst response = await fetch("${path}", {\n  method: "${example.method}",\n  headers: {\n    "Content-Type": "${example.headers["Content-Type"]}",\n    "Idempotency-Key": "${example.headers["Idempotency-Key"]}",\n    "X-CSRF-Token": session.csrf_token\n  },\n  body: JSON.stringify(${JSON.stringify(example.body, null, 2).replaceAll("\n", "\n  ")})\n});\n\nconst result = await response.json();`;
}

function RequestProof({ example }: Readonly<{example:DeveloperExample}>) {
  return <article className={`request-proof-sheet ${example.id}`} aria-labelledby={`${example.id}-heading`}>
    <header><div><p className="eyebrow">Versioned contract example</p><h3 id={`${example.id}-heading`}>{example.title}</h3><code>{example.request_schema}</code></div><StatusBadge tone="info">{example.method}</StatusBadge></header>
    <div className="request-proof-line"><span>01 · Browser BFF route</span><strong><code>{example.method} {example.id === "create_account" ? "/api/me/accounts" : `/api${example.path}`}</code></strong><small>The private contract path is <code>{example.path}</code>.</small></div>
    <div className="request-proof-line"><span>02 · Idempotent intent</span><strong><code>{example.headers["Idempotency-Key"]}</code></strong><small>The key binds the complete normalized body.</small></div>
    {example.id === "create_transfer" ? <div className="request-proof-line exact-money"><span>03 · Exact string money</span><strong><code>{`"amount": "${example.body.amount}"`}</code></strong><small>Never send a JSON number for money.</small></div> : <div className="request-proof-line zero-boundary"><span>03 · Financial boundary</span><strong><code>INR {example.result_facts?.available_minor ?? "0"}.00</code></strong><small><code>{`"available_minor":"${example.result_facts?.available_minor ?? "0"}"`}</code> · <code>{`"ledger_minor":"${example.result_facts?.ledger_minor ?? "0"}"`}</code>. No opening amount is accepted.</small></div>}
    <CopyCode value={browserRequest(example)} label={`${example.title} browser example`} />
    <div className="safe-retry-rule"><ShieldCheck weight="fill" aria-hidden="true"/><div><strong>Safe retry rule</strong><p>{example.retry_summary}</p></div></div>
  </article>;
}

export function DeveloperView({ metadata, loading, error, online, canRead, publicOrigin, onRefresh }: Readonly<{ metadata:DeveloperMetadata|null; loading:boolean; error:string|null; online:boolean; canRead:boolean; publicOrigin:string; onRefresh:()=>void }>) {
  const transfer = metadata?.examples.find((example) => example.id === "create_transfer");
  const account = metadata?.examples.find((example) => example.id === "create_account");
  return <>
    <PageHeader eyebrow="API-first local workspace" title="Developer" description="Build one exact request, preserve its retry identity, and trace the authorized result.">
      <div className="header-actions"><button className="button secondary guarded-control" type="button" disabled={!online||loading||!canRead} onClick={onRefresh}>Refresh contract</button>{online&&canRead&&metadata ? <a className="button primary guarded-control" href="/api/developer/openapi" download="ledgersync-openapi.yaml"><DownloadSimple aria-hidden="true"/>Download OpenAPI YAML</a> : <span className="permission-note">Download unavailable until the current authorized contract is loaded.</span>}</div>
    </PageHeader>
    {!canRead && <StatePanel kind="denied" title="Developer contract not authorized" message="This session does not include developer:read. No contract metadata or download has been requested."/>}
    {!online && <StatePanel kind="offline" title="Offline — contract freshness is not verified" message={metadata ? "The last loaded version remains visible. Reconnect before using it as the current contract." : "Reconnect to retrieve the versioned local API contract."}/>} 
    {error && <StatePanel kind="error" title="Developer contract unavailable" message={error}/>} 
    {loading&&!metadata && <StatePanel title="Loading versioned contract" message="Retrieving bounded developer metadata from the authorized private contract source."/>}
    {metadata && <div className="developer-workspace" aria-busy={loading}>
      <section className="developer-boundary" aria-labelledby="local-boundary-heading"><div className="boundary-mark" aria-hidden="true"><LockKey weight="fill"/></div><div><p className="eyebrow">{metadata.boundary.network.replaceAll("_", " ")}</p><h2 id="local-boundary-heading">{metadata.boundary.title}</h2><p>{metadata.boundary.summary}</p></div><dl><div><dt>Browser base URL</dt><dd><code>{publicOrigin}{metadata.base_url}</code></dd></div><div><dt>Contract</dt><dd><code>v{metadata.contract_version}</code></dd></div><div><dt>Metadata schema</dt><dd><code>v{metadata.schema_version}</code></dd></div></dl></section>

      <section className="developer-section" aria-labelledby="authentication-heading"><div className="section-heading"><div><p className="eyebrow">Two boundaries, never one shared token</p><h2 id="authentication-heading">Authentication</h2></div></div><div className="authentication-ledger">{metadata.authentication.map((authentication,index) => <article key={authentication.id}><span className="auth-order">0{index+1}</span>{authentication.id === "browser_bff_session" ? <ShieldCheck aria-hidden="true"/> : <Key aria-hidden="true"/>}<div><h3>{authentication.label}</h3><p>{authentication.summary}</p>{authentication.id === "browser_bff_session" ? <p className="auth-rule"><strong>Use:</strong> same-origin paths, signed cookie, CSRF header for writes. The BFF adds private credentials server-side.</p> : <><p className="auth-rule"><strong>Use:</strong> protected host tooling only. Never paste revealed output into this browser, source, screenshots, or logs.</p><CopyCode value=".\\scripts\\local-api-credential.ps1" label="credential fingerprint command"/><CopyCode value=".\\scripts\\local-api-credential.ps1 -Reveal" label="deliberate credential reveal command"/></>}</div></article>)}</div></section>

      <section className="developer-section" aria-labelledby="endpoint-groups-heading"><div className="section-heading"><div><p className="eyebrow">Private API contract</p><h2 id="endpoint-groups-heading">Endpoint groups</h2></div><span>{metadata.endpoint_groups.reduce((count,group)=>count+group.operations.length,0)} operations</span></div><div className="endpoint-catalogue">{metadata.endpoint_groups.map((group,index)=><details key={group.id} open={index===0}><summary><span>{group.label}</span><small>{group.operations.length} operations</small></summary><div>{group.operations.map((operation)=><div className="endpoint-row" key={operation.operation_id}><span className={`method-label ${operation.method.toLowerCase()}`}>{operation.method}</span><code>{metadata.base_url}{operation.path}</code><span>{operation.scope}</span><small>{operation.operation_id}</small></div>)}</div></details>)}</div></section>

      <section className="developer-section examples-section" aria-labelledby="exact-requests-heading"><div className="section-heading"><div><p className="eyebrow">Copyable · Non-secret · Schema-linked</p><h2 id="exact-requests-heading">Exact request proofs</h2></div></div>{transfer&&<RequestProof example={transfer}/>} {account&&<RequestProof example={account}/>}</section>

      <section className="developer-section" aria-labelledby="operational-contracts-heading"><div className="section-heading"><div><p className="eyebrow">Read and verify</p><h2 id="operational-contracts-heading">Reconciliation and operational evidence</h2></div></div><div className="developer-capability-index"><div><span>Reconciliation</span><strong>Prove PostgreSQL balances match postings.</strong><Link href="/reconciliation">Open reconciliation <ArrowRight aria-hidden="true"/></Link></div><div><span>Events</span><strong>Investigate delivery separately from financial status.</strong><Link href="/events">Open event evidence <ArrowRight aria-hidden="true"/></Link></div><div><span>Diagnostics</span><strong>Separate financial authority, delivery, and disposable cache.</strong><Link href="/local-status">Open local status <ArrowRight aria-hidden="true"/></Link></div></div></section>

      <section className="developer-section" aria-labelledby="safe-retries-heading"><div className="section-heading"><div><p className="eyebrow">Outcome-aware recovery</p><h2 id="safe-retries-heading">Errors and safe retries</h2></div></div><DataTableRegion label="Safe retry outcomes"><table className="data-table developer-error-table"><thead><tr><th scope="col">Outcome code</th><th scope="col">Safe action</th></tr></thead><tbody>{metadata.retry_outcomes.map((outcome)=><tr key={outcome.code}><td><strong><code>{outcome.code}</code></strong></td><td>{outcome.safe_action}</td></tr>)}</tbody></table></DataTableRegion><DataTableRegion label="Developer error catalogue"><table className="data-table developer-error-table"><thead><tr><th scope="col">Error code</th><th scope="col">Meaning</th></tr></thead><tbody>{metadata.error_catalogue.map((entry)=><tr key={entry.code}><td><strong><code>{entry.code}</code></strong></td><td>{entry.meaning}</td></tr>)}</tbody></table></DataTableRegion></section>

      <section className="developer-correlation" aria-labelledby="correlation-heading"><div><p className="eyebrow">Authorized lookup</p><h2 id="correlation-heading">Trace one request</h2><p>{metadata.record_lookup.summary}</p></div><div className="correlation-header"><span>Request header</span><strong><code>{metadata.record_lookup.correlation_header}</code></strong></div><ul>{metadata.record_lookup.safe_fields.map((field)=><li key={field}><code>{field}</code></li>)}</ul><p><WarningCircle aria-hidden="true"/>There is no browser raw-log search and no global object-disclosure endpoint. Use the tenant-authorized detail views above.</p></section>

      <section className="openapi-download" aria-labelledby="openapi-heading"><CheckCircle weight="fill" aria-hidden="true"/><div><p className="eyebrow">Complete machine-readable contract</p><h2 id="openapi-heading">OpenAPI YAML</h2><p>Download the full private API contract served from the same versioned source as this metadata. The browser download contains schemas and examples, never a credential.</p></div>{online&&canRead ? <a className="button primary guarded-control" href="/api/developer/openapi" download="ledgersync-openapi.yaml"><DownloadSimple aria-hidden="true"/>Download YAML</a> : <span className="permission-note">Download unavailable until connectivity and developer scope are verified.</span>}</section>
    </div>}
  </>;
}
