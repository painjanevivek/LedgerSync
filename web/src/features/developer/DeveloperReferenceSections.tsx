import { ArrowRight, CheckCircle, ShieldCheck, WarningCircle } from "@phosphor-icons/react";

import { DataTableRegion } from "@/features/console/components";
import { DeveloperCopyCode } from "@/features/developer/DeveloperCopyCode";
import { buildTransferRecipes } from "@/features/developer/developer-recipes";
import type { DeveloperExample, DeveloperMetadata } from "@/lib/api/developer";

const referenceLinks = [
  ["authentication-heading", "Authentication"],
  ["schema-accounts", "Accounts"],
  ["schema-funding", "Funding"],
  ["schema-transfers", "Transfers"],
  ["schema-corrections", "Corrections"],
  ["schema-reconciliation", "Reconciliation"],
  ["schema-operations", "Events"],
  ["schema-webhooks", "Webhooks"],
  ["safe-retries-heading", "Errors & retries"],
] as const;

export function DeveloperReferenceNavigation() {
  return (
    <nav className="developer-reference-nav" aria-label="Developer reference sections">
      <div>
        <p className="eyebrow">Contract map</p>
        <strong>Jump to one integration responsibility</strong>
      </div>
      <ul>{referenceLinks.map(([target, label]) => <li key={target}><a href={`#${target}`}>{label}</a></li>)}</ul>
    </nav>
  );
}

export function ExactMoneyAndRetryGuide({ metadata }: Readonly<{ metadata: DeveloperMetadata }>) {
  const mutationFamilies = metadata.endpoint_groups
    .map((group) => ({ family: group.label, operations: group.operations.filter((operation) => operation.method !== "GET") }))
    .filter((group) => group.operations.length > 0);

  return (
    <section className="developer-section" aria-labelledby="exact-money-guide-heading">
      <div className="section-heading"><div><p className="eyebrow">One rule across every financial command</p><h2 id="exact-money-guide-heading">Exact money and replay protection</h2></div></div>
      <div className="developer-rule-grid">
        <article><span>01</span><h3>Decimal input stays text</h3><p>Send a transfer amount such as <code>&quot;125.50&quot;</code>. Never parse or calculate it with a browser floating-point number.</p></article>
        <article><span>02</span><h3>Minor-unit output stays text</h3><p>A response such as <code>&quot;12550&quot;</code> means 125.50 INR. Store and calculate it with an integer/decimal library, not JavaScript <code>number</code>.</p></article>
        <article><span>03</span><h3>Unknown is not failed</h3><p>If a timeout happens after send, the command may have committed. Retry the identical normalized body with the identical idempotency key.</p></article>
      </div>
      <DataTableRegion label="Mutation replay protection matrix">
        <table className="data-table developer-error-table developer-replay-table">
          <thead><tr><th scope="col">Family</th><th scope="col">Mutation</th><th scope="col">Replay rule</th></tr></thead>
          <tbody>{mutationFamilies.map((group) => <tr key={group.family}><td>{group.family}</td><td><ul className="developer-operation-list">{group.operations.map((operation) => <li key={operation.operation_id}><strong><code>{operation.operation_id}</code></strong></li>)}</ul></td><td>Persist a new key with the complete intent before send. Same key + same intent replays; same key + changed intent conflicts; an unknown response keeps the original key.</td></tr>)}</tbody>
        </table>
      </DataTableRegion>
    </section>
  );
}

export function IntegrationRecipeSection({ example }: Readonly<{ example: DeveloperExample }>) {
  const recipes = buildTransferRecipes(example);
  return (
    <section className="developer-section" aria-labelledby="integration-recipes-heading">
      <div className="section-heading"><div><p className="eyebrow">Generated from the canonical transfer example</p><h2 id="integration-recipes-heading">Integration recipes</h2></div><span>curl · TypeScript · Go · Postman</span></div>
      <p className="developer-section-intro">Choose the language your server uses. These are static examples, not an HTTP runner, and no credential is read, accepted, or stored by this screen.</p>
      <div className="developer-recipe-list">{recipes.map((recipe, index) => <details key={recipe.id} open={index === 0}><summary><span>{recipe.label}</span><small>{recipe.summary}</small></summary><DeveloperCopyCode value={recipe.code} label={`${recipe.label} transfer recipe`} /></details>)}</div>
    </section>
  );
}

export function PartnerJourneySection() {
  const steps = [
    ["Authenticate", "Choose the browser BFF for local console work or a protected server token for a partner service."],
    ["Provision", "Create accounts one at a time with createAccount. No public bulk API is currently contracted."],
    ["Fund", "Request controlled funding evidence, obtain the required approval, then post only the reviewed event."],
    ["Transfer", "Send exact decimal text and persist the idempotency key with the complete request."],
    ["Inspect", "Use the returned identifier and X-Request-ID; never infer success from a timeout."],
    ["Reconcile", "Run or inspect reconciliation and resolve mismatches from authoritative evidence."],
    ["Consume webhooks", "Verify signatures, deduplicate event IDs, acknowledge quickly, and process asynchronously."],
  ] as const;
  return (
    <section className="developer-section" aria-labelledby="partner-journey-heading">
      <div className="section-heading"><div><p className="eyebrow">Documentation-only qualification path</p><h2 id="partner-journey-heading">From authentication to webhook evidence</h2></div></div>
      <ol className="developer-journey">{steps.map(([title, detail], index) => <li key={title}><span>{String(index + 1).padStart(2, "0")}</span><div><strong>{title}</strong><p>{detail}</p></div></li>)}</ol>
      <div className="developer-scope-boundary"><WarningCircle aria-hidden="true" /><p><strong>Ledger boundary:</strong> these operations record closed-loop ledger activity and evidence. They do not move external funds, prove bank settlement, or create custody.</p></div>
    </section>
  );
}

export function WebhookVerificationSection() {
  return (
    <section className="developer-section" aria-labelledby="webhook-verification-heading">
      <div className="section-heading"><div><p className="eyebrow">Server-initiated ownership proof</p><h2 id="webhook-verification-heading">Webhook endpoint verification</h2></div></div>
      <div className="webhook-verification-flow">
        <article><span>01</span><ShieldCheck aria-hidden="true" /><div><h3>Register a key reference</h3><p>Store the signing key in the approved secret manager. Send only its reference and key ID; LedgerSync never returns the secret or challenge.</p></div></article>
        <ArrowRight aria-hidden="true" />
        <article><span>02</span><ShieldCheck aria-hidden="true" /><div><h3>Answer LedgerSync</h3><p>LedgerSync posts an expiring challenge to the HTTPS endpoint. Return <code>X-LedgerSync-Verification</code> with the required HMAC proof.</p></div></article>
        <ArrowRight aria-hidden="true" />
        <article><span>03</span><CheckCircle aria-hidden="true" /><div><h3>Wait for active</h3><p>Poll endpoint metadata until it becomes <code>active</code>. A failed or expired proof never activates delivery.</p></div></article>
      </div>
    </section>
  );
}
