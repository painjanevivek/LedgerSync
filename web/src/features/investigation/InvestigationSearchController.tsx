"use client";

import { MagnifyingGlass } from "@phosphor-icons/react";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

import { ConsoleRouteFrame } from "@/features/console/ConsoleRouteFrame";
import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import { canSearchInvestigations, deriveConsoleCapabilities } from "@/features/console/capabilities";
import { SavedViewsPanel } from "@/features/investigation/SavedViewsPanel";
import { WorkspaceCapture } from "@/features/investigation/WorkspaceCapture";
import { WorkspaceListPanel } from "@/features/investigation/WorkspaceListPanel";
import { sanitizeInvestigationSearch, type InvestigationRecordType, type InvestigationSearchPage, type InvestigationSearchResult } from "@/lib/api/investigation-search";
import { investigationSearchURL, parseInvestigationSearchPageQuery } from "@/lib/page-query/investigation-search";
import { FormField } from "@/ui/forms/FormField.client";
import { Identifier } from "@/ui/display/Identifier";
import { PageHeader } from "@/ui/display/PageHeader";
import { RecordLink } from "@/ui/display/RecordLink";
import { StatePanel } from "@/ui/display/StatePanel";
import { StatusBadge, type StatusTone } from "@/ui/display/StatusBadge";
import { Timestamp } from "@/ui/display/Timestamp";
import { DisclosureSection } from "@/ui/disclosure/DisclosureSection";

function resultPath(type: InvestigationRecordType, id: string): string | null {
  switch (type) {
    case "account": return `/accounts/${encodeURIComponent(id)}`;
    case "transfer": return `/transfers/${encodeURIComponent(id)}`;
    case "funding": return `/funding/${encodeURIComponent(id)}`;
    case "event": return `/events/${encodeURIComponent(id)}`;
    case "reconciliation_run": return `/reconciliation/${encodeURIComponent(id)}`;
    case "correction": return `/corrections/${encodeURIComponent(id)}`;
    default: return null;
  }
}

export function investigationResultHref(result: InvestigationSearchResult): string | null {
  if (result.record_type === "reconciliation_mismatch" || result.record_type === "request_reference") {
    return result.related_record_type && result.related_record_id ? resultPath(result.related_record_type, result.related_record_id) : null;
  }
  return resultPath(result.record_type, result.record_id);
}

function tone(status: string): StatusTone {
  if (["active", "posted", "published", "matched", "approved", "succeeded"].includes(status)) return "success";
  if (["rejected", "dead", "failed", "mismatch", "denied", "closed", "cancelled", "expired"].includes(status)) return "danger";
  if (["pending", "requested", "retrying", "frozen"].includes(status)) return "warning";
  return "neutral";
}

function SearchResultCard({ result, query, queryKind }: Readonly<{ result: InvestigationSearchResult; query: string; queryKind: "immutable_id" | "approved_reference" }>) {
  const href = investigationResultHref(result);
  return <li className="investigation-result">
    <div className="investigation-result-heading">
      <div><p className="eyebrow">{result.record_type.replaceAll("_", " ")}</p><h2>{result.safe_label}</h2></div>
      <StatusBadge tone={tone(result.status)}>{result.status}</StatusBadge>
    </div>
    <dl className="investigation-result-facts">
      <div><dt>Record ID</dt><dd><Identifier value={result.record_id} /></dd></div>
      <div><dt>Recorded UTC</dt><dd><Timestamp value={result.occurred_at} /></dd></div>
      <div><dt>Source and freshness</dt><dd>PostgreSQL · search snapshot</dd></div>
    </dl>
    {href ? <RecordLink href={href} label={result.record_type === "request_reference" ? "Open referenced evidence" : "Open authoritative detail"} /> : <p className="muted">This locator has no released detail route. Use the exact identifier with an authorized operations team.</p>}
    <WorkspaceCapture result={result} query={query} queryKind={queryKind} />
  </li>;
}

export function InvestigationSearchController({ initialQuery, invalidQuery }: Readonly<{ initialQuery: string; invalidQuery: boolean }>) {
  const router = useRouter();
  const { session, online } = useConsoleSession();
  const capabilities = deriveConsoleCapabilities(session);
  const canSearch = canSearchInvestigations(capabilities);
  const [input, setInput] = useState(initialQuery);
  const [inputError, setInputError] = useState<string | null>(null);
  const [page, setPage] = useState<InvestigationSearchPage | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const search = useCallback(async (signal: AbortSignal) => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch(`/api/investigation/search?q=${encodeURIComponent(initialQuery)}&limit=10`, { cache: "no-store", signal });
      const payload = await response.json() as unknown;
      const sanitized = sanitizeInvestigationSearch(response.status, payload);
      if (sanitized.status < 200 || sanitized.status >= 300) {
        setPage(null);
        setError(sanitized.status === 429 ? "Search is temporarily rate limited. Wait before retrying the identical lookup." : sanitized.status === 403 ? "Your current role cannot search across investigation records." : "Authorized search evidence is unavailable. No missing record is inferred.");
        return;
      }
      setPage(sanitized.body as InvestigationSearchPage);
    } catch (cause) {
      if ((cause as Error).name !== "AbortError") {
        setPage(null);
        setError("Authorized search evidence is unavailable. No missing record is inferred.");
      }
    } finally {
      if (!signal.aborted) setLoading(false);
    }
  }, [initialQuery]);

  useEffect(() => {
    if (!session || !online || !canSearch || invalidQuery || !initialQuery) return;
    const controller = new AbortController();
    const timer = window.setTimeout(() => void search(controller.signal), 0);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [canSearch, initialQuery, invalidQuery, online, search, session]);

  const submit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const candidate = input.trim();
    if (!parseInvestigationSearchPageQuery({ q: candidate })) {
      setInputError("Enter one complete immutable ID or an approved reference of 8–128 characters. Partial search is not allowed.");
      return;
    }
    setInputError(null);
    setPage(null);
    setError(null);
    router.push(investigationSearchURL(candidate));
  };

  return <ConsoleRouteFrame section="search" loadingLabel="Search" pending={loading}>
    <div className="investigation-search-workspace">
      <PageHeader eyebrow="Investigate / Exact lookup" title="Search records" description="Locate authorized evidence by one complete immutable ID or approved external reference. LedgerSync does not perform broad or cross-tenant discovery." />
      {!canSearch && session ? <StatePanel kind="denied" title="Investigation authority required" message="Your server-issued role and scopes do not permit cross-domain lookup. No protected search was made." /> : invalidQuery ? <StatePanel kind="error" title="Invalid search URL" message="The shared URL contains an unknown, repeated, empty, oversized, partial, or malformed lookup. No protected search was made." action={<button className="button secondary" type="button" onClick={() => router.replace("/search")}>Clear invalid lookup</button>} /> : <>
        <form className="investigation-search-form" role="search" onSubmit={submit}>
          <FormField label="Exact ID or approved reference" requirement="required" hint="Examples: a complete account/transfer UUID or an exact approved account or funding reference. Minimum reference length: 8." error={inputError}>
            <input type="search" value={input} onChange={(event) => { setInput(event.target.value); setInputError(null); }} maxLength={128} autoComplete="off" spellCheck={false} placeholder="11111111-1111-4111-8111-111111111111" />
          </FormField>
          <button className="button primary" type="submit" disabled={!online || loading}><MagnifyingGlass aria-hidden="true" />{loading ? "Searching…" : "Search exact evidence"}</button>
        </form>
        <p className="investigation-search-boundary">Exact lookup only · maximum 10 typed locators · current tenant and server-issued scopes · no balances or payloads copied into results</p>
        {!initialQuery && <StatePanel title="Start with known evidence" message="Enter a complete record ID, request reference, or approved external reference. Partial names and broad discovery are intentionally unavailable." />}
        {initialQuery && !online && !page && <StatePanel kind="offline" title="Search unavailable offline" message="Reconnect to query current tenant-scoped evidence. No empty result is inferred." />}
        {error && <StatePanel kind="error" title="Search evidence unavailable" message={error} action={online ? <button className="button secondary" type="button" onClick={() => { const controller = new AbortController(); void search(controller.signal); }}>Retry exact lookup</button> : undefined} />}
        {loading && !page && <StatePanel announce="polite" title="Searching authorized evidence" message="No result is inferred until the bounded server lookup completes." />}
        {page && !loading && page.results.length === 0 && <StatePanel title="No authorized match" message="No matching evidence is visible in the current tenant and scopes. LedgerSync does not distinguish a missing record from one outside your authority." />}
        {page && page.results.length > 0 && <section className="investigation-results" aria-labelledby="investigation-results-heading">
          <div className="investigation-results-summary"><div><p className="eyebrow">Bounded lookup</p><h2 id="investigation-results-heading">{page.results.length} authorized locator{page.results.length === 1 ? "" : "s"}</h2></div><p>Generated <Timestamp value={page.generated_at} />{page.truncated ? " · Additional matches withheld by the result bound." : ""}</p></div>
          <ol>{page.results.map((result) => <SearchResultCard key={`${result.record_type}-${result.record_id}-${result.related_record_id ?? "direct"}`} result={result} query={initialQuery} queryKind={page.query_kind} />)}</ol>
        </section>}
        {(Boolean(page?.results.length) || Boolean(initialQuery)) && <DisclosureSection id="investigation-case-tools" title="Case tools" summary="Open saved views and investigation workspaces after locating evidence." lazy><WorkspaceListPanel /><SavedViewsPanel /></DisclosureSection>}
      </>}
    </div>
  </ConsoleRouteFrame>;
}
