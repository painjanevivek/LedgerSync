import type { Page, Route } from "@playwright/test";
import developerMetadataSource from "../../../contracts/developer-examples.v1.json";

export const sourceAccount = { account_id: "11111111-1111-4111-8111-111111111111", display_name:"Operating Reserve", category:"operating", external_reference:"OPS-RESERVE", currency: "INR", status: "active", available_minor: "125000", ledger_minor: "125000", account_version: "1", version: "8", as_of: "2026-08-19T12:00:00Z" };
export const destinationAccount = { account_id: "22222222-2222-4222-8222-222222222222", display_name:"Customer Funds", category:"customer_funds", external_reference:"CUSTOMER-FUNDS", currency: "INR", status: "active", available_minor: "25000", ledger_minor: "25000", account_version: "1", version: "4", as_of: "2026-08-19T12:00:00Z" };
export const transfer = { transfer_id:"33333333-3333-4333-8333-333333333333",source_account_id:sourceAccount.account_id,destination_account_id:destinationAccount.account_id,amount_minor:"500",currency:"INR",financial_status:"posted",delivery_status:"retrying",created_at:"2026-08-19T11:00:00Z",completed_at:"2026-08-19T11:00:01Z",journal_transaction_id:"44444444-4444-4444-8444-444444444444" };
export const run = { run_id:"55555555-5555-4555-8555-555555555555",status:"matched",correlation_id:"66666666-6666-4666-8666-666666666666",scope:"All authorized INR accounts",ledger_watermark:"8",application_version:"test",schema_version:"000008",checked_account_count:"2",posting_count:"2",mismatch_count:"0",started_at:"2026-08-19T11:59:58Z",completed_at:"2026-08-19T12:00:00Z" };
export const deliveryEvent = { event_id:"77777777-7777-4777-8777-777777777777",event_type:"account.balance.changed.v1",state:"retrying",aggregate_type:"account",aggregate_id:destinationAccount.account_id,aggregate_version:"4",attempt_count:"2",occurred_at:"2026-08-19T11:00:01Z",available_at:"2026-08-19T11:02:00Z",transfer_id:transfer.transfer_id,account_id:destinationAccount.account_id,correlation_id:"88888888-8888-4888-8888-888888888888",last_error_code:"redis_unavailable" };
export const eventDetail = { ...deliveryEvent, delivery_attempts:[{attempt_id:"99999999-9999-4999-8999-999999999999",kind:"notification",state:"retrying",attempt_number:"2",due_at:"2026-08-19T11:02:00Z",started_at:"2026-08-19T11:01:58Z",completed_at:"2026-08-19T11:02:00Z",response_class:"timeout",error_code:"timeout"}],delivery_attempts_truncated:false,timeline:[{kind:"committed",occurred_at:"2026-08-19T11:00:01Z"},{kind:"delivery_retrying",occurred_at:"2026-08-19T11:02:00Z"}] };
export const diagnostics = { overall_state:"degraded",generated_at:"2026-08-19T12:00:00Z",application:{version:"0.1.0",commit:"abc123def456",environment:"local_demo",public_origin:"http://127.0.0.1:3100"},financial_authority:{postgres:{state:"reachable",schema_version:"000008"},latest_reconciliation:{state:"available",status:"matched",run_id:run.run_id,completed_at:run.completed_at}},delivery_cache:{outbox:{state:"reachable",pending_count:"1",dead_count:"0",worker_progress:"stalled",latest_published_at:"2026-08-19T11:00:00Z",oldest_pending_at:"2026-08-19T11:00:01Z"},redis:{state:"unavailable",label:"disposable_cache"}} };
export const recoveryEvidence = { format_version:"ledgersync-recovery-evidence-index/v1",generated_at_utc:"2026-08-19T12:05:00Z",latest_backup:{backup_id:"backup-20260819T115000Z-abcdef1",finalized_at_utc:"2026-08-19T11:50:00Z",size_bytes:1048576,schema_version:"000008_account_commands",digest_status:"verified",validation_status:"passed",source_commit:"0123456789abcdef0123456789abcdef01234567"},latest_restore:{backup_id:"backup-20260819T115000Z-abcdef1",completed_at_utc:"2026-08-19T11:57:00Z",status:"passed",reconciliation_status:"matched",mismatch_count:0,normal_project_unchanged:true,local_rto_seconds:42.5},retention:{valid_backup_count:3,ignored_entry_count:0,configured_keep_count:5} };
export const orientationEvidence = { generated_at:"2026-08-19T12:05:00Z",evidence_state:"complete",steps:[
  {id:"inspect_account",state:"evidence_available",evidence_type:"account_record",evidence_id:sourceAccount.account_id,occurred_at:sourceAccount.as_of},
  {id:"create_account",state:"completed",evidence_type:"account_created_audit",evidence_id:"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",occurred_at:"2026-08-19T10:00:00Z"},
  {id:"fund_account",state:"completed",evidence_type:"posted_transfer",evidence_id:transfer.transfer_id,occurred_at:transfer.completed_at},
  {id:"inspect_transfer",state:"evidence_available",evidence_type:"transfer_record",evidence_id:transfer.transfer_id,occurred_at:transfer.completed_at},
  {id:"run_reconciliation",state:"completed",evidence_type:"reconciliation_run",evidence_id:run.run_id,occurred_at:run.completed_at},
  {id:"inspect_delivery",state:"evidence_available",evidence_type:"delivery_attempt",evidence_id:deliveryEvent.event_id,occurred_at:deliveryEvent.occurred_at},
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

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });
}

export async function mockOperatorConsole(page: Page, { sessionDelayMilliseconds = 0 }: { sessionDelayMilliseconds?: number } = {}) {
  await page.route("**/api/session", async (route) => {
    if (sessionDelayMilliseconds > 0) await new Promise((resolve) => setTimeout(resolve, sessionDelayMilliseconds));
    return json(route, { subject_id: "operator-1", tenant_id: "tenant-1", csrf_token: "csrf-test-token", scopes: ["accounts:read", "accounts:write", "transactions:read", "transfers:read", "transfers:write", "reconciliation:read", "reconciliation:write", "local:read", "events:read", "explainability:read", "developer:read", "recovery:read", "exports:read"], environment:"demo",tenant_label:"Meridian Labs · Test",operator_label:"Test operator" });
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
  await page.route("**/api/reconciliation/runs?*", (route) => json(route, { runs:[run],next_cursor:"" }));
  await page.route("**/api/reconciliation/runs/*", (route) => json(route, run));
  await page.route("**/api/local/diagnostics", (route) => json(route, diagnostics));
  await page.route("**/api/local/orientation", (route) => json(route, orientationEvidence));
  await page.route("**/api/transfers/*/explainability", (route) => json(route, transferExplainability));
  await page.route(/\/api\/events\?(?:.*)/, (route) => json(route, { events:[deliveryEvent],next_cursor:"" }));
  await page.route(/\/api\/events\/[^/?]+$/, (route) => json(route, eventDetail));
  await page.route("**/api/developer/metadata", (route) => json(route, developerMetadata));
  await page.route("**/api/developer/openapi", (route) => route.fulfill({ status:200,contentType:"application/yaml",headers:{"Content-Disposition":`attachment; filename="ledgersync-openapi.yaml"`},body:"openapi: 3.1.0\ninfo:\n  title: LedgerSync\npaths:\ncomponents:\n" }));
  await page.route("**/api/recovery/manifests", (route) => json(route, recoveryEvidence));
  await page.route(/\/api\/exports\/.*\.csv(?:\?.*)?$/, (route) => {
    const path=new URL(route.request().url()).pathname;
    const family=path.includes("/accounts/")?"account-ledger":path.includes("reconciliation")?"reconciliation":"transfers";
    return route.fulfill({status:200,contentType:"text/csv; charset=utf-8",headers:{"Content-Disposition":`attachment; filename="ledgersync-${family}-20260819T120000Z-v1.csv"`,"X-LedgerSync-Export-Schema":"1"},body:"schema_version,record_id,amount_minor,currency\r\n1,record-1,\"500\",INR\r\n"});
  });
}
