# Rendered system diagrams

All 19 Mermaid blocks in [the system design](../ledgersync-system-design.md) were parsed and rendered with Mermaid 12.1.0. [Open the gallery](index.html). Each diagram has an SVG, PNG, and Mermaid source file. The [manifest](manifest.json) records source hashes, source lines, and rendered dimensions.

## Rebuild

From the repository root, with the frontend dependencies and Playwright Chromium installed:

```powershell
npm install --prefix .tmp/diagram-renderer --save-exact mermaid@12.1.0
node scripts/render-system-diagrams.mjs
```

This is local rendering. It does not publish files or certify the proposed cloud deployment.

- [2. User journey and screen responsibilities](01-2-user-journey-and-screen-responsibilities.svg) · [PNG](01-2-user-journey-and-screen-responsibilities.png) · [source](01-2-user-journey-and-screen-responsibilities.mmd)
- [3. System context and trust boundaries](02-3-system-context-and-trust-boundaries.svg) · [PNG](02-3-system-context-and-trust-boundaries.png) · [source](02-3-system-context-and-trust-boundaries.mmd)
- [4. Component architecture](03-4-component-architecture.svg) · [PNG](03-4-component-architecture.png) · [source](03-4-component-architecture.mmd)
- [5. Financial data model and invariants](04-5-financial-data-model-and-invariants.svg) · [PNG](04-5-financial-data-model-and-invariants.png) · [source](04-5-financial-data-model-and-invariants.mmd)
- [6.1 Managed OIDC flow: implemented, requires provider configuration](05-6-1-managed-oidc-flow-implemented-requires-provider-configuration.svg) · [PNG](05-6-1-managed-oidc-flow-implemented-requires-provider-configuration.png) · [source](05-6-1-managed-oidc-flow-implemented-requires-provider-configuration.mmd)
- [7.1 Create an account: implemented](06-7-1-create-an-account-implemented.svg) · [PNG](06-7-1-create-an-account-implemented.png) · [source](06-7-1-create-an-account-implemented.mmd)
- [7.2 Record and post funding: implemented controlled workflow](07-7-2-record-and-post-funding-implemented-controlled-workflow.svg) · [PNG](07-7-2-record-and-post-funding-implemented-controlled-workflow.png) · [source](07-7-2-record-and-post-funding-implemented-controlled-workflow.mmd)
- [7.3 Internal transfer with verified success: implemented](08-7-3-internal-transfer-with-verified-success-implemented.svg) · [PNG](08-7-3-internal-transfer-with-verified-success-implemented.png) · [source](08-7-3-internal-transfer-with-verified-success-implemented.mmd)
- [7.4 Unknown transfer result and follow-up: implemented safety rule](09-7-4-unknown-transfer-result-and-follow-up-implemented-safety-rule.svg) · [PNG](09-7-4-unknown-transfer-result-and-follow-up-implemented-safety-rule.png) · [source](09-7-4-unknown-transfer-result-and-follow-up-implemented-safety-rule.mmd)
- [7.5 Read after a committed write: current primary-read design](10-7-5-read-after-a-committed-write-current-primary-read-design.svg) · [PNG](10-7-5-read-after-a-committed-write-current-primary-read-design.png) · [source](10-7-5-read-after-a-committed-write-current-primary-read-design.mmd)
- [8.1 Outbox, cache projection, and webhook delivery: implemented processors](11-8-1-outbox-cache-projection-and-webhook-delivery-implemented-processors.svg) · [PNG](11-8-1-outbox-cache-projection-and-webhook-delivery-implemented-processors.png) · [source](11-8-1-outbox-cache-projection-and-webhook-delivery-implemented-processors.mmd)
- [8.2 Reconciliation: implemented comparison, no automatic repair](12-8-2-reconciliation-implemented-comparison-no-automatic-repair.svg) · [PNG](12-8-2-reconciliation-implemented-comparison-no-automatic-repair.png) · [source](12-8-2-reconciliation-implemented-comparison-no-automatic-repair.mmd)
- [9. Investigation and handoff](13-9-investigation-and-handoff.svg) · [PNG](13-9-investigation-and-handoff.png) · [source](13-9-investigation-and-handoff.mmd)
- [10. Recovery and retention](14-10-recovery-and-retention.svg) · [PNG](14-10-recovery-and-retention.png) · [source](14-10-recovery-and-retention.mmd)
- [11. Local deployment: actual repository topology](15-11-local-deployment-actual-repository-topology.svg) · [PNG](15-11-local-deployment-actual-repository-topology.png) · [source](15-11-local-deployment-actual-repository-topology.mmd)
- [12.1 Vercel: configured target, deployment not established by this review](16-12-1-vercel-configured-target-deployment-not-established-by-this-review.svg) · [PNG](16-12-1-vercel-configured-target-deployment-not-established-by-this-review.png) · [source](16-12-1-vercel-configured-target-deployment-not-established-by-this-review.mmd)
- [12.2 AWS container deployment: proposed target](17-12-2-aws-container-deployment-proposed-target.svg) · [PNG](17-12-2-aws-container-deployment-proposed-target.png) · [source](17-12-2-aws-container-deployment-proposed-target.mmd)
- [12.3 Proposed release sequence](18-12-3-proposed-release-sequence.svg) · [PNG](18-12-3-proposed-release-sequence.png) · [source](18-12-3-proposed-release-sequence.mmd)
- [13. Deferred provider payout sequence: proposed only](19-13-deferred-provider-payout-sequence-proposed-only.svg) · [PNG](19-13-deferred-provider-payout-sequence-proposed-only.png) · [source](19-13-deferred-provider-payout-sequence-proposed-only.mmd)
