import type { Page, Route } from "@playwright/test";
import developerMetadataSource from "../../../contracts/developer-examples.v1.json";

export const sourceAccount = { account_id: "11111111-1111-4111-8111-111111111111", display_name:"Operating Reserve", category:"operating", external_reference:"OPS-RESERVE", currency: "INR", status: "active", available_minor: "125000", ledger_minor: "125000", account_version: "1", version: "8", as_of: "2026-08-19T12:00:00Z" };
export const destinationAccount = { account_id: "22222222-2222-4222-8222-222222222222", display_name:"Customer Funds", category:"customer_funds", external_reference:"CUSTOMER-FUNDS", currency: "INR", status: "active", available_minor: "25000", ledger_minor: "25000", account_version: "1", version: "4", as_of: "2026-08-19T12:00:00Z" };
export const transfer = { transfer_id:"33333333-3333-4333-8333-333333333333",source_account_id:sourceAccount.account_id,destination_account_id:destinationAccount.account_id,amount_minor:"500",currency:"INR",financial_status:"posted",delivery_status:"retrying",created_at:"2026-08-19T11:00:00Z",completed_at:"2026-08-19T11:00:01Z",journal_transaction_id:"44444444-4444-4444-8444-444444444444" };
export const fundingEvent = { funding_event_id:"abababab-abab-4bab-8bab-abababababab",destination_account_id:destinationAccount.account_id,external_reference:"BANK-REF-20260819",evidence_reference:"CASE-2026-0819",amount_minor:"125000",currency:"INR",status:"requested",requested_at:"2026-08-19T11:20:00Z",requester_subject_id:"operator-1",demo_policy:true };
export const approvalItems = [
  { domain:"funding",record_id:fundingEvent.funding_event_id,requester_subject_id:"operator-2",requested_at:"2026-08-07T09:00:00Z",age_seconds:"1036800",status:"requested",amount_minor:"125000",currency:"INR",related_account_id:destinationAccount.account_id,evidence_complete:true,self_approval_blocked:false,actionable_by_me:true,required_scope:"funding:approve",step_up_status:"not_required",safe_next_action:"review_decision" },
  { domain:"correction",record_id:"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",requester_subject_id:"operator-1",requested_at:"2026-08-08T10:00:00Z",age_seconds:"950400",status:"requested",amount_minor:"500",currency:"INR",related_transfer_id:transfer.transfer_id,evidence_complete:true,self_approval_blocked:true,actionable_by_me:false,required_scope:"corrections:approve",step_up_status:"required",approval_expires_at:"2026-09-01T10:00:00Z",safe_next_action:"wait_for_independent_approver" },
] as const;
export const run = { run_id:"55555555-5555-4555-8555-555555555555",status:"matched",correlation_id:"66666666-6666-4666-8666-666666666666",scope:"All authorized INR accounts",ledger_watermark:"8",application_version:"test",schema_version:"000008",checked_account_count:"2",posting_count:"2",mismatch_count:"0",started_at:"2026-08-19T11:59:58Z",completed_at:"2026-08-19T12:00:00Z" };
export const deliveryEvent = { event_id:"77777777-7777-4777-8777-777777777777",event_type:"account.balance.changed.v1",state:"retrying",aggregate_type:"account",aggregate_id:destinationAccount.account_id,aggregate_version:"4",attempt_count:"2",occurred_at:"2026-08-19T11:00:01Z",available_at:"2026-08-19T11:02:00Z",transfer_id:transfer.transfer_id,account_id:destinationAccount.account_id,correlation_id:"88888888-8888-4888-8888-888888888888",last_error_code:"redis_unavailable" };
export const webhookEndpoint = { endpoint_id:"12121212-1212-4212-8212-121212121212",label:"Settlement partner",origin:"https://partner.example.test",status:"active",subscribed_events:["transfer.posted"],recent_delivery_state:"dead",recent_attempt_count:"2",recent_dead_count:"1",verified_at:"2026-08-19T10:00:00Z",latest_delivery_at:"2026-08-19T11:02:00Z",updated_at:"2026-08-19T11:02:00Z" };
export const eventDetail = { ...deliveryEvent, delivery_attempts:[{attempt_id:"99999999-9999-4999-8999-999999999999",kind:"webhook",state:"retrying",attempt_number:"2",due_at:"2026-08-19T11:02:00Z",started_at:"2026-08-19T11:01:58Z",completed_at:"2026-08-19T11:02:00Z",response_class:"timeout",error_code:"timeout",endpoint_id:webhookEndpoint.endpoint_id,endpoint_label:webhookEndpoint.label,endpoint_origin:webhookEndpoint.origin}],delivery_attempts_truncated:false,timeline:[{kind:"committed",occurred_at:"2026-08-19T11:00:01Z"},{kind:"delivery_retrying",occurred_at:"2026-08-19T11:02:00Z"}] };
export const webhookEndpointDetail = { ...webhookEndpoint,delivery_attempts:[{attempt_id:eventDetail.delivery_attempts[0].attempt_id,event_id:deliveryEvent.event_id,transfer_id:transfer.transfer_id,state:"dead",attempt_number:"2",error_code:"timeout",due_at:"2026-08-19T11:01:00Z",completed_at:"2026-08-19T11:02:00Z"}],delivery_attempts_truncated:false };
export const diagnostics = { overall_state:"degraded",generated_at:"2026-08-19T12:00:00Z",application:{version:"0.1.0",commit:"abc123def456",environment:"local_demo",public_origin:"http://127.0.0.1:3100"},financial_authority:{postgres:{state:"reachable",schema_version:"000008"},latest_reconciliation:{state:"available",status:"matched",run_id:run.run_id,completed_at:run.completed_at}},delivery_cache:{outbox:{state:"reachable",pending_count:"1",dead_count:"0",worker_progress:"stalled",latest_published_at:"2026-08-19T11:00:00Z",oldest_pending_at:"2026-08-19T11:00:01Z"},redis:{state:"unavailable",label:"disposable_cache"}} };
export const recoveryEvidence = { format_version:"ledgersync-recovery-evidence-index/v1",generated_at_utc:"2026-08-19T12:05:00Z",latest_backup:{backup_id:"backup-20260819T115000Z-abcdef1",finalized_at_utc:"2026-08-19T11:50:00Z",size_bytes:1048576,schema_version:"000008_account_commands",digest_status:"verified",validation_status:"passed",source_commit:"0123456789abcdef0123456789abcdef01234567"},latest_restore:{backup_id:"backup-20260819T115000Z-abcdef1",completed_at_utc:"2026-08-19T11:57:00Z",status:"passed",reconciliation_status:"matched",mismatch_count:0,normal_project_unchanged:true,local_rto_seconds:42.5},retention:{valid_backup_count:3,ignored_entry_count:0,configured_keep_count:5} };
export const orientationEvidence = { generated_at:"2026-08-19T12:05:00Z",evidence_state:"partial",dismissed:false,preference_version:"0",operator_completed_step_ids:[],steps:[
  {id:"confirm_health",state:"missing",evidence_type:"local_health_confirmation",reason_code:"operator_confirmation_required"},
  {id:"understand_authority",state:"missing",evidence_type:"authority_acknowledgement",reason_code:"operator_confirmation_required"},
  {id:"inspect_accounts",state:"evidence_available",evidence_type:"account_record",evidence_id:sourceAccount.account_id,occurred_at:sourceAccount.as_of,reason_code:"operator_confirmation_required"},
  {id:"create_account",state:"completed",evidence_type:"account_created_audit",evidence_id:"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",occurred_at:"2026-08-19T10:00:00Z"},
  {id:"fund_account",state:"completed",evidence_type:"funding_journal",evidence_id:"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",occurred_at:"2026-08-19T10:30:00Z"},
  {id:"post_transfer",state:"completed",evidence_type:"posted_transfer",evidence_id:transfer.transfer_id,occurred_at:transfer.completed_at},
  {id:"retry_transfer",state:"evidence_available",evidence_type:"idempotency_outcome",evidence_id:transfer.transfer_id,occurred_at:transfer.completed_at,reason_code:"operator_confirmation_required"},
  {id:"inspect_postings",state:"evidence_available",evidence_type:"journal_postings",evidence_id:transfer.transfer_id,occurred_at:transfer.completed_at,reason_code:"operator_confirmation_required"},
  {id:"run_reconciliation",state:"completed",evidence_type:"reconciliation_run",evidence_id:run.run_id,occurred_at:run.completed_at},
  {id:"inspect_delivery",state:"evidence_available",evidence_type:"delivery_attempt",evidence_id:eventDetail.delivery_attempts[0].attempt_id,occurred_at:eventDetail.delivery_attempts[0].completed_at,reason_code:"operator_confirmation_required"},
  {id:"export_evidence",state:"evidence_available",evidence_type:"evidence_export",evidence_id:transfer.transfer_id,occurred_at:transfer.completed_at,reason_code:"operator_confirmation_required"},
  {id:"create_backup",state:"completed",evidence_type:"recovery_backup",evidence_id:recoveryEvidence.latest_backup.backup_id,occurred_at:recoveryEvidence.latest_backup.finalized_at_utc},
]};
export const transferExplainability = { transfer_id:transfer.transfer_id,generated_at:"2026-08-19T12:05:00Z",evidence_state:"complete",stages:[
  {sequence:1,kind:"request",state:"available",truncated:false,evidence:[{evidence_type:"idempotency_outcome",evidence_id:"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",status:"completed",occurred_at:transfer.created_at}]},
  {sequence:2,kind:"transfer",state:"available",truncated:false,evidence:[{evidence_type:"transfer",evidence_id:transfer.transfer_id,status:"posted",amount_minor:"500",currency:"INR",occurred_at:transfer.completed_at}]},
  {sequence:3,kind:"journal_postings",state:"available",truncated:false,evidence:[{evidence_type:"journal",evidence_id:transfer.journal_transaction_id,occurred_at:transfer.completed_at},{evidence_type:"posting",evidence_id:"aaaaaaaa-1111-4111-8111-111111111111",account_id:sourceAccount.account_id,status:"debit",direction:"debit",amount_minor:"500",currency:"INR",occurred_at:transfer.completed_at},{evidence_type:"posting",evidence_id:"aaaaaaaa-2222-4222-8222-222222222222",account_id:destinationAccount.account_id,status:"credit",direction:"credit",amount_minor:"500",currency:"INR",occurred_at:transfer.completed_at}]},
  {sequence:4,kind:"balance_versions",state:"available",truncated:false,evidence:[{evidence_type:"balance_version",account_id:sourceAccount.account_id,balance_version:"8",occurred_at:transfer.completed_at},{evidence_type:"balance_version",account_id:destinationAccount.account_id,balance_version:"4",occurred_at:transfer.completed_at}]},
  {sequence:5,kind:"outbox",state:"available",truncated:false,evidence:[{evidence_type:"outbox_event",evidence_id:deliveryEvent.event_id,status:"pending",occurred_at:transfer.completed_at}]},
  {sequence:6,kind:"delivery",state:"available",truncated:false,evidence:[{evidence_type:"delivery_attempt",evidence_id:eventDetail.delivery_attempts[0].attempt_id,status:"retrying",attempt_number:"2",occurred_at:eventDetail.delivery_attempts[0].completed_at}]},
  {sequence:7,kind:"reconciliation",state:"available",truncated:false,evidence:[{evidence_type:"reconciliation_run",evidence_id:run.run_id,status:"matched",related_id:run.ledger_watermark,occurred_at:run.completed_at}]},
]};
export const developerMetadata = developerMetadataSource;
export const investigationSearchPage = {
  results: [
    { record_type: "account", record_id: sourceAccount.account_id, safe_label: "Account", status: "active", occurred_at: sourceAccount.as_of, source: "postgresql", freshness: "search_snapshot" },
    { record_type: "request_reference", record_id: "88888888-8888-4888-8888-888888888888", related_record_type: "transfer", related_record_id: transfer.transfer_id, safe_label: "Request reference", status: "succeeded", occurred_at: transfer.completed_at, source: "postgresql", freshness: "search_snapshot" },
  ],
  query_kind: "immutable_id",
  generated_at: "2026-08-19T12:05:00Z",
  truncated: false,
} as const;
export const savedInvestigationView = {
  saved_view_id: "13131313-1313-4313-8313-131313131313",
  name: "Dead delivery events",
  filter_schema_version: "1",
  domain: "events",
  filters: { state: "dead" },
  target_path: "/events?state=dead",
  version: "1",
  created_at: "2026-08-19T12:04:00Z",
  updated_at: "2026-08-19T12:04:00Z",
} as const;
export const investigationWorkspace = {
  investigation_id: "15151515-1515-4515-8515-151515151515",
  title: "Transfer delivery review",
  taxonomy: "transfer_delivery",
  status: "open",
  version: "1",
  created_at: "2026-08-19T12:02:00Z",
  updated_at: "2026-08-19T12:02:00Z",
  historical_context: {
    query_context: { kind: "immutable_id", record_type: "transfer", value: transfer.transfer_id },
    references: [
      { relationship_type: "root", record_type: "transfer", record_id: transfer.transfer_id, target_path: `/transfers/${transfer.transfer_id}`, captured_at: "2026-08-19T12:02:00Z" },
      { relationship_type: "transfer_event", source_record_type: "transfer", source_record_id: transfer.transfer_id, record_type: "event", record_id: deliveryEvent.event_id, target_path: `/events/${deliveryEvent.event_id}`, captured_at: "2026-08-19T12:02:00Z" },
    ],
    withheld_reference_count: 0,
    history: [{ action: "created", actor_is_current_operator: true, version: "1", status: "open", occurred_at: "2026-08-19T12:02:00Z" }],
    history_truncated: false,
  },
  current_evidence: {
    root: { record_type: "transfer", record_id: transfer.transfer_id, safe_label: "Transfer", status: "posted", occurred_at: transfer.completed_at, source: "postgresql", freshness: "search_snapshot" },
    relationships: [
      { relationship_type: "transfer_journal", target_type: "journal", target_id: transfer.journal_transaction_id, safe_label: "Journal transaction", status: "recorded", occurred_at: transfer.completed_at, source: "postgresql", freshness: "relationship_snapshot" },
      { relationship_type: "transfer_event", target_type: "event", target_id: deliveryEvent.event_id, safe_label: deliveryEvent.event_type, status: "retrying", occurred_at: deliveryEvent.occurred_at, source: "postgresql", freshness: "relationship_snapshot" },
    ],
    generated_at: "2026-08-19T12:05:00Z",
    truncated: false,
    available: true,
  },
} as const;

function relatedEvidencePage(url: string) {
  const parts = new URL(url).pathname.split("/");
  const sourceType = parts.at(-2) ?? "transfer";
  const sourceId = parts.at(-1) ?? transfer.transfer_id;
  const relationships = sourceType === "account" ? [
    { relationship_type: "account_transaction", target_type: "transfer", target_id: transfer.transfer_id, safe_label: "Transfer", status: "posted", occurred_at: transfer.completed_at, source: "postgresql", freshness: "relationship_snapshot" },
    { relationship_type: "account_event", target_type: "event", target_id: deliveryEvent.event_id, safe_label: deliveryEvent.event_type, status: "retrying", occurred_at: deliveryEvent.occurred_at, source: "postgresql", freshness: "relationship_snapshot" },
  ] : sourceType === "transfer" ? [
    { relationship_type: "transfer_journal", target_type: "journal", target_id: transfer.journal_transaction_id, safe_label: "Journal transaction", status: "recorded", occurred_at: transfer.completed_at, source: "postgresql", freshness: "relationship_snapshot" },
    { relationship_type: "transfer_event", target_type: "event", target_id: deliveryEvent.event_id, safe_label: deliveryEvent.event_type, status: "retrying", occurred_at: deliveryEvent.occurred_at, source: "postgresql", freshness: "relationship_snapshot" },
  ] : sourceType === "event" ? [
    { relationship_type: "event_transfer", target_type: "transfer", target_id: transfer.transfer_id, safe_label: "Transfer", status: "posted", occurred_at: transfer.completed_at, source: "postgresql", freshness: "relationship_snapshot" },
    { relationship_type: "event_account", target_type: "account", target_id: destinationAccount.account_id, safe_label: "Account", status: "active", occurred_at: sourceAccount.as_of, source: "postgresql", freshness: "relationship_snapshot" },
  ] : sourceType === "funding" ? [
    { relationship_type: "funding_approval", target_type: "approval", target_id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", safe_label: "Approval record", status: "requested", occurred_at: fundingEvent.requested_at, source: "postgresql", freshness: "relationship_snapshot" },
  ] : sourceType === "correction" ? [
    { relationship_type: "correction_original_transfer", target_type: "transfer", target_id: transfer.transfer_id, safe_label: "Transfer", status: "posted", occurred_at: transfer.completed_at, source: "postgresql", freshness: "relationship_snapshot" },
  ] : [];
  return {
    source_type: sourceType,
    source_id: sourceId,
    relationships,
    generated_at: "2026-08-19T12:05:00Z",
    truncated: false,
  };
}

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
}

export async function mockOperatorConsole(page: Page, { sessionDelayMilliseconds = 0 }: { sessionDelayMilliseconds?: number } = {}) {
  let savedViews: Array<Record<string, unknown>> = [{ ...savedInvestigationView }];
  let workspace: Record<string, unknown> | null = structuredClone(investigationWorkspace) as unknown as Record<string, unknown>;
  await page.route("**/api/session", async (route) => {
    if (sessionDelayMilliseconds > 0) await new Promise((resolve) => setTimeout(resolve, sessionDelayMilliseconds));
    return json(route, { subject_id: "operator-1", tenant_id: "tenant-1", csrf_token: "csrf-test-token", scopes: ["accounts:read", "accounts:write", "transactions:read", "transfers:read", "transfers:write", "funding:read", "funding:write", "funding:approve", "corrections:read", "corrections:write", "corrections:approve", "reconciliation:read", "reconciliation:write", "local:read", "local:write", "events:read", "investigation:read", "investigation:write", "explainability:read", "developer:read", "credentials:read", "credentials:write", "webhooks:read", "webhooks:write", "webhooks:replay", "recovery:read", "exports:read"], environment:"local",tenant_label:"My Ledger Workspace",operator_label:"Test operator" });
  });
  await page.route("**/api/me/accounts?*", (route) => json(route, { accounts: [sourceAccount, destinationAccount], next_cursor: "" }));
  await page.route(/\/api\/accounts\/[^/?]+(?:\?.*)?$/, (route) => {
    const account = route.request().url().includes(sourceAccount.account_id) ? sourceAccount : destinationAccount;
    return json(route, account);
  });
  await page.route("**/api/accounts/*/balance", (route) => {
    const account = route.request().url().includes(sourceAccount.account_id) ? sourceAccount : destinationAccount;
    return json(route, account);
  });
  await page.route("**/api/accounts/*/transactions?*", (route) => json(route, { transactions: [{ transfer_id: "transfer-existing", direction: "credit", amount: "500", currency: "INR", status: "posted", occurred_at: "2026-08-19T11:00:00Z" }] }));
  await page.route("**/api/transfers?*", (route) => json(route, { transfers:[transfer],next_cursor:"" }));
  await page.route("**/api/transfers/*", (route) => json(route, { ...transfer,actor_subject_id:"operator-1",postings:[{posting_id:"posting-1",account_id:sourceAccount.account_id,direction:"debit",amount_minor:"500",currency:"INR",occurred_at:transfer.completed_at},{posting_id:"posting-2",account_id:destinationAccount.account_id,direction:"credit",amount_minor:"500",currency:"INR",occurred_at:transfer.completed_at}],timeline:[] }));
  await page.route("**/api/funding-events?*", (route) => json(route, { events:[fundingEvent],next_cursor:"" }));
  await page.route("**/api/funding-events/*", (route) => json(route, fundingEvent));
  await page.route("**/api/approvals?*", (route) => json(route, { items:approvalItems,page_count:approvalItems.length,next_cursor:"approval-page-2" }));
  await page.route("**/api/reconciliation/runs?*", (route) => json(route, { runs:[run],next_cursor:"" }));
  await page.route("**/api/reconciliation/runs/*", (route) => json(route, run));
  await page.route("**/api/local/diagnostics", (route) => json(route, diagnostics));
  await page.route("**/api/local/orientation", (route) => json(route, orientationEvidence));
  await page.route("**/api/transfers/*/explainability", (route) => json(route, transferExplainability));
  await page.route(/\/api\/events\?(?:.*)/, (route) => json(route, { events:[deliveryEvent],next_cursor:"" }));
  await page.route(/\/api\/events\/[^/?]+$/, (route) => json(route, eventDetail));
  await page.route(/\/api\/webhook-endpoints\?(?:.*)/, (route) => json(route, { items:[webhookEndpoint],next_cursor:"" }));
  await page.route(/\/api\/webhook-endpoints\/[^/?]+$/, (route) => json(route, webhookEndpointDetail));
  await page.route("**/api/developer/metadata", (route) => json(route, developerMetadata));
  await page.route("**/api/developer/openapi", (route) => route.fulfill({ status:200,contentType:"application/yaml",headers:{"Content-Disposition":`attachment; filename="ledgersync-openapi.yaml"`},body:"openapi: 3.1.0\ninfo:\n  title: LedgerSync\npaths:\ncomponents:\n" }));
  await page.route("**/api/recovery/manifests", (route) => json(route, recoveryEvidence));
  await page.route("**/api/investigation/search?*", (route) => json(route, investigationSearchPage));
  await page.route("**/api/investigation/related/**", (route) => json(route, relatedEvidencePage(route.request().url())));
  await page.route("**/api/investigation/saved-views", async (route) => {
    const method = route.request().method();
    if (method === "GET") return json(route, { views: savedViews, generated_at: "2026-08-19T12:05:00Z" });
    if (method === "POST") {
      const input = route.request().postDataJSON() as Record<string, unknown>;
      const created = { saved_view_id: "14141414-1414-4414-8414-141414141414", ...input, target_path: input.domain === "events" && (input.filters as Record<string, string>).state === "dead" ? "/events?state=dead" : "/events?state=published", version: "1", created_at: "2026-08-19T12:06:00Z", updated_at: "2026-08-19T12:06:00Z" };
      savedViews = [created, ...savedViews];
      return json(route, created, 201);
    }
    return json(route, { error: { code: "invalid_request" } }, 400);
  });
  await page.route("**/api/investigation/saved-views/*", async (route) => {
    const id = new URL(route.request().url()).pathname.split("/").at(-1);
    const current = savedViews.find((view) => view.saved_view_id === id);
    if (!current) return json(route, { error: { code: "not_found" } }, 404);
    if (route.request().method() === "PUT") {
      const input = route.request().postDataJSON() as { name: string };
      const updated = { ...current, name: input.name, version: String(Number(current.version) + 1), updated_at: "2026-08-19T12:07:00Z" };
      savedViews = savedViews.map((view) => view.saved_view_id === id ? updated : view);
      return json(route, updated);
    }
    if (route.request().method() === "DELETE") {
      savedViews = savedViews.filter((view) => view.saved_view_id !== id);
      return route.fulfill({ status: 204 });
    }
    return json(route, { error: { code: "invalid_request" } }, 400);
  });
  await page.route("**/api/investigation/workspaces", async (route) => {
    if (route.request().method() === "GET") {
      const summary = workspace ? Object.fromEntries(Object.entries(workspace).filter(([key]) => !["historical_context", "current_evidence"].includes(key))) : null;
      return json(route, { investigations: summary ? [summary] : [], generated_at: "2026-08-19T12:05:00Z" });
    }
    if (route.request().method() === "POST") {
      const input = route.request().postDataJSON() as { title: string; taxonomy: string; query_context: { kind: string; record_type: string; value: string }; root_record: { record_type: string; record_id: string } };
      workspace = structuredClone(investigationWorkspace) as unknown as Record<string, unknown>;
      workspace.investigation_id = "16161616-1616-4616-8616-161616161616";
      workspace.title = input.title; workspace.taxonomy = input.taxonomy;
      workspace.historical_context = { ...(workspace.historical_context as Record<string, unknown>), query_context: input.query_context, references: [{ relationship_type: "root", record_type: input.root_record.record_type, record_id: input.root_record.record_id, target_path: input.root_record.record_type === "account" ? `/accounts/${input.root_record.record_id}` : `/transfers/${input.root_record.record_id}`, captured_at: "2026-08-19T12:06:00Z" }] };
      workspace.current_evidence = { ...(workspace.current_evidence as Record<string, unknown>), root: { record_type: input.root_record.record_type, record_id: input.root_record.record_id, safe_label: "Account", status: "active", occurred_at: sourceAccount.as_of, source: "postgresql", freshness: "search_snapshot" }, relationships: [] };
      return json(route, workspace, 201);
    }
    return json(route, { error: { code: "invalid_request" } }, 400);
  });
  await page.route("**/api/investigation/workspaces/*/*", async (route) => {
    if (!workspace || route.request().method() !== "POST") return json(route, { error: { code: "not_found" } }, 404);
    const action = new URL(route.request().url()).pathname.split("/").at(-1);
    const nextVersion = String(Number(workspace.version) + 1);
    if (action === "evidence-bundle") {
      const body = Buffer.from("PK\u0003\u0004bounded-test-archive", "utf8");
      return route.fulfill({ status: 200, contentType: "application/zip", headers: { "Content-Disposition": `attachment; filename="ledgersync-investigation-${workspace.investigation_id}-20260819T120500Z-v1.zip"`, "Content-Length": String(body.byteLength), "X-LedgerSync-Bundle-Schema": "1", "X-LedgerSync-Bundle-SHA256": "a".repeat(64), "X-LedgerSync-Bundle-Expires-At": "2026-08-19T12:20:00Z" }, body });
    }
    if (action === "handoff") { const id = workspace.investigation_id as string; workspace = null; return json(route, { investigation_id: id, outcome: "handed_off", version: nextVersion, occurred_at: "2026-08-19T12:08:00Z" }); }
    if (action === "close" || action === "reopen") {
      workspace.version = nextVersion; workspace.status = action === "close" ? "closed" : "open"; workspace.updated_at = "2026-08-19T12:08:00Z";
      if (action === "close") workspace.closed_at = "2026-08-19T12:08:00Z"; else delete workspace.closed_at;
      return json(route, { investigation_id: workspace.investigation_id, outcome: action === "close" ? "closed" : "reopened", version: nextVersion, occurred_at: "2026-08-19T12:08:00Z" });
    }
    return json(route, { error: { code: "invalid_request" } }, 400);
  });
  await page.route("**/api/investigation/workspaces/*", (route) => workspace ? json(route, workspace) : json(route, { error: { code: "not_found" } }, 404));
  await page.route(/\/api\/exports\/.*\.csv(?:\?.*)?$/, (route) => {
    const path=new URL(route.request().url()).pathname;
    const family=path.includes("/accounts/")?"account-ledger":path.includes("reconciliation")?"reconciliation":"transfers";
    return route.fulfill({status:200,contentType:"text/csv; charset=utf-8",headers:{"Content-Disposition":`attachment; filename="ledgersync-${family}-20260819T120000Z-v2.csv"`,"X-LedgerSync-Export-Schema":"2"},body:"schema_version,record_id,amount_minor,currency\r\n2,record-1,\"500\",INR\r\n"});
  });
}
