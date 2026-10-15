# Owner answers and open decisions

**Prepared:** 2026-10-10. **Status:** draft from supplied names and repository evidence.

This file answers the questions in the user-supplied `Pasted text.txt`. Its phase numbers follow that attachment. They do not replace the M00–M17 phase IDs or status in the [master delivery register](ledgersync-master-progress.md).

`Recorded` means the user supplied the fact or the cited document records it. `Needed` means an answer or approval is still missing. `Proposed` means a suggested arrangement that has not been approved. This draft records the user's authority decision; it grants no additional authority.

## Needed now — Phase 0

| Question | Answer |
|---|---|
| 1. Contact routes for the owner and backup | **Recorded:** owner / external owner: **Vivek Painjane**. Backup name: **MasterVaik**. **Needed:** a project email, phone number, or other agreed route for each person. “Not recorded yet” is accurate for this draft; neither name establishes a tested contact route. |
| 2. Temporary decision forum and decision records | **Needed:** participants and the record location. **Proposed:** Vivek leads the review; MasterVaik is contacted when Vivek is unavailable. Record each decision with the question, decision, named decision-maker, UTC date, evidence link, affected scope, and review or expiry date. The proposed forum is not confirmed. |
| 3. MasterVaik's backup authority | **Recorded:** the user confirmed on 2026-10-10 that MasterVaik has delegated decision authority. **Needed:** the permitted decisions, exclusions, conditions, and expiry. Independent release approval and specialist sign-off are not established by this general authority statement. |
| 4. Owners of other critical areas | **Needed:** named people for finance / financial semantics, security and privacy, legal / compliance, platform / operations, accessibility, and partner coordination. **Proposed:** mark each assignment `OPEN` until appointed, and require appointment and approval evidence before its relevant gate. Do not assign those roles to Vivek or MasterVaik from their names alone. |

Phase 0 governance remains incomplete. The historical M00 canonical planning baseline is a different gate. Its `COMPLETE` status does not prove that these contact and authority decisions are complete. See the [master baseline record](../release-evidence/master-phase-0-baseline.md).

## Phase 1 — Repository and evidence reconciliation

**Recorded:** the [master completion plan](ledgersync-master-product-system-and-website-completion-plan.md) controls scope; the [master register](ledgersync-master-progress.md) controls current status. Older plans remain historical sources. Compare conflicting claims with their exact source commit and evidence. A previous test pass does not qualify a later change.

**Needed:** a product-owner decision only when a conflict remains after this evidence review. Record that conflict before asking the owner to choose. This draft does not treat the redesigned working tree as a qualified release candidate.

## Phase 2 — Partner problem and product boundary

| Question | Answer |
|---|---|
| 1. First business-partner type | **Recorded direction:** fintech and vertical-SaaS product teams. The [launch profile](../pilot/india-launch-profile.md) names a CTO or Head of Engineering as the initial buyer. **Needed:** one real prospect and its confirmed buyer. |
| 2. Specific operational problem | **Recorded current capability:** exact internal, same-currency ledger movement and investigation. **Recorded next hypothesis:** compare one mapped clearing account with an independent external statement, then let an operator review exceptions. **Needed:** a partner's recurring problem and sample evidence before approving that extension. |
| 3. Partner decision-maker, engineer, finance contact, and operator | **Needed:** names and agreed contact routes for all four roles. No named partner is established by the reviewed records. |
| 4. Prospects and introductions | **Needed:** real prospective businesses, interview or sandbox consent, and the person who will make each introduction. |
| 5. Evidence to continue or stop | **Proposed:** capture a real workflow, frequency, time spent, errors, source records, current alternative, and willingness to try the product. Change or stop the hypothesis if interviews and records do not show a recurring need. **Needed:** the owner-approved decision criteria. |
| 6. Useful pilot outcomes | **Recorded measurement categories:** integration effort, time to a safe first transfer, investigation time, support volume, and partner feedback. **Needed:** agreed measures, thresholds, sample size, observation window, and willingness-to-continue evidence. |

Sources: [master product focus](ledgersync-master-progress.md), [partner readiness](../pilot/design-partner-readiness.md), [partner record template](../pilot/partner-evidence-template.md). These define a direction and evidence requirements; they do not establish customer demand.

## Phase 3 — Financial, legal, privacy, and operating semantics

| Question | Answer |
|---|---|
| 1. Who approves financial wording? | **Recorded required roles:** finance with product, support, operations, and legal where relevant. **Needed:** named reviewers and approval references for balances, transfer outcomes, corrections, internal reconciliation, fees, limits, and timestamps. |
| 2. Which states and sources must stay distinct? | **Recorded engineering semantics:** posted means the journal and balances committed; rejected means no posting; result not confirmed means the client cannot prove the outcome. Retrying an unknown outcome keeps the same complete intent and key. Delivery is separate. PostgreSQL is financial authority; cache is disposable. Internal reconciliation compares postings with LedgerSync's projections. A passing result needs a completed stored matched run and zero mismatches. |
| 3. Data that may or may not be collected | **Recorded:** investigation uses authorized record IDs and source evidence. Partner notes and support records must not contain tokens, secrets, raw financial exports, or unnecessary personal data. **Needed:** an approved inventory, purpose, collection policy, and product-measurement policy. |
| 4. Retention, access, export, deletion, and legal holds | **Recorded:** an engineering retention default exists; protected financial and audit records are retained. **Needed:** written legal, privacy, and finance policy by data class, with named access/export owners and legal-hold rules. The default is not production approval. |
| 5. Reviewed jurisdiction, entity, relationship, and claims | **Recorded product target:** India and an internal ledger boundary. **Needed:** legal entity, partner relationship, reviewed claims, and approval from a qualified legal/compliance owner. A target jurisdiction is not a completed legal review. |
| 6. Support and incident communications | **Needed:** support route, hours, response expectations, escalation route, and incident communications owner. |

Sources: [financial UI semantics](../product/financial-ui-semantics.md), [pilot decisions](../product/pilot-decision-register.md), [data retention](../runbooks/data-retention.md), [master product boundary](ledgersync-master-product-system-and-website-completion-plan.md).

## Phase 4 — User journeys and redesign

| Question | Answer |
|---|---|
| 1. First roles and workflows | **Recorded direction:** API integration engineers and finance/operations investigators. The current redesign covers Overview, accounts, funding records and review, internal transfers, internal reconciliation, linked records, and the guide. Access follows server-issued scopes. |
| 2. Safe actions for each transfer outcome | **Recorded:** posted users inspect the committed record; rejected users inspect the reason before preparing a new intent; unknown users check or retry the same intent and key. Delivery retry does not change posted money. |
| 3. Visible outcome references | **Recorded:** exact currency/amount; full account and transfer IDs; posting and delivery states; journal/posting references where returned; balance versions and UTC source times; reconciliation run references; last successful verification time when records are old. Missing links remain explicit. |
| 4. Confirmation and approval controls | **Recorded:** transfer preparation leads to review and explicit confirmation. Funding/correction controls require their scoped review paths and separation of requester and approver where defined. Step-up approval controls require production identity qualification. **Needed:** named owners to approve the enabled roles, policy, and high-risk action wording. |
| 5. Representative users and wording reviewers | **Needed:** named engineers/operators, physical-device and assistive-technology reviewers, finance reviewer, and security reviewer. Automated checks do not provide their approval. |
| 6. What to say when current records cannot be verified | **Recorded:** show unavailable, unknown, or historical state; show the last successful verification time when retained records exist; offer a focused retry when permitted. Do not claim empty data, zero, a current balance, successful posting, or passing reconciliation without its source record. |

Sources: [system design](../architecture/ledgersync-system-design.md), [UI system alignment](../design/ui-system-alignment.md), [financial UI semantics](../product/financial-ui-semantics.md).

## Phase 5 — Release-candidate scope and qualification

| Question | Answer |
|---|---|
| 1. Candidate scope and feature freeze | **Recorded current boundary:** internal same-currency ledger, controlled funding and compensating corrections. Provider payouts, FX, cards, and broad balance lifecycle expansion are deferred. External reconciliation is a discovery hypothesis. **Needed:** the exact candidate SHA and an approved capability/feature-flag list. |
| 2. Changes since the qualified commit | **Recorded:** earlier evidence is tied to specific commits; the current redesign and other working-tree changes need their own qualification. **Needed:** a candidate diff and retained/rolled-back/separately-reviewed decisions after the evidence review. |
| 3. Release-risk acceptance | **Needed:** named accepting authority, permitted finding categories, compensating controls, and expiry. The register's financial/security stop-ship rules remain binding. No backup-name entry provides independent approval. |
| 4. Candidate evidence package | **Recorded requirement:** source SHA, build/image identity, migrations and contract versions, exact test/CI results, deployment configuration references, findings and exceptions with owner/expiry, and signed gate decisions. Include provider or human evidence where the gate requires it. |

Source: [master register](ledgersync-master-progress.md), including its evidence protocol and stop-ship rules.

## Phases 6–9 — Environment, identity, operations, and recovery

| Question | Answer |
|---|---|
| 1. AWS account, credentials, budget, and provision authority | **Recorded target:** Mumbai, `ap-south-1`. **Needed:** account/billing owner, approved credentials process, budget, and time-bounded provision authority. |
| 2. Monthly cost envelope | **Needed:** reviewed monthly budget for compute, databases, monitoring, retention, backups, and recovery exercises. No approved cost figure was found in the reviewed records. |
| 3. Real identity provider and permission owners | **Recorded target:** Amazon Cognito, invited operators, tenant-bound scopes, separate operator and workload identities. **Needed:** live provider configuration/evidence, named role owners, and approved grant/revocation processes. |
| 4. On-call route and stop/restart authority | **Needed:** responders, coverage, tested contact routes, escalation, and named authority to stop or resume operations. |
| 5. Service targets, traffic, and support hours | **Recorded launch-profile targets:** at least 99.9% monthly API availability; transfer p95 below 500 ms; balance-read p95 below 200 ms; at most 25 new transfer journeys/second at launch. **Needed:** partner-specific acceptance, managed proof, capacity budget, and support hours. These are targets, not service guarantees from this draft. |
| 6. RPO/RTO and proof | **Recorded profile targets:** committed-transaction RPO 0 and RTO at most 15 minutes for an availability-zone failure; RPO at most 5 minutes and RTO at most 60 minutes for corruption/operator error. **Needed:** named approval and provider-backed failover/PITR drills with a reconciled restored ledger. Local restore results do not prove these managed objectives. |
| 7. Telemetry approval | **Needed:** named operations/security/privacy approvers, data inventory, access, region, and retention rules. |
| 8. Independent activation review | **Needed:** independent reviewers and evidence for infrastructure, tenant isolation, credentials, recovery, and production access. The current register identifies database authority and tenant isolation as an immediate release gate. |

Sources: [launch profile](../pilot/india-launch-profile.md), [master register](ledgersync-master-progress.md), [partner readiness](../pilot/design-partner-readiness.md). The secondary India backup region is a design target, not a second financial writer.

## Phases 10–13 — Developer experience, human review, rehearsal, and sandbox

| Question | Answer |
|---|---|
| 1. Sandbox users and onboarding owner | **Needed:** partner engineers/operators and a named LedgerSync onboarding owner. |
| 2. Independent integration | **Recorded handoff:** API contract/examples, exact string amounts, idempotency and lost-response guidance, scoped credentials through an approved secret channel, support route, and explicit exclusions. **Needed:** partner-run integration evidence without help from the implementation team. |
| 3. Required scenarios | **Recorded categories:** validation and authorization; same-key retries and changed-intent conflicts; lost responses; temporary dependency failure; rate limits; credential lifecycle; webhook delivery/replay; and reconciliation/investigation. **Needed:** an agreed per-partner checklist and its pass evidence. |
| 4. Human review matrix | **Recorded proposal:** current/previous Chrome and Edge, current Firefox, Safari and iOS Safari. **Needed:** approved devices, browser versions, assistive technologies, representative users, reviewers, and dated results. |
| 5. Incident/onboarding rehearsal | **Recorded sequence:** sandbox, internal rehearsal, then one limited partner. **Needed:** named rehearsal lead, participants, date, findings, and closure before access. |
| 6. Partner support and escalation | **Needed:** support channel, coverage, response target, escalation route, and communications owner. |
| 7. Sandbox exit approval | **Recorded required roles:** product, finance, security, operations, and partner owner. **Needed:** named signers, decision, UTC date, and evidence reference. |

Sources: [partner readiness](../pilot/design-partner-readiness.md), [partner template](../pilot/partner-evidence-template.md), [pilot decisions](../product/pilot-decision-register.md).

## Phases 14–15 — Live pilot and graduation

| Question | Answer |
|---|---|
| 1. Approved partner and signers | **Needed:** one approved first partner and named signers on both sides. The repository records a sequential 2–3-partner target; it does not record a live partner approval. |
| 2. Cohort, accounts, limits, hours, and dates | **Recorded baseline:** internal same-tenant INR only, no overdraft; the launch profile records amount/velocity limits. **Needed:** the approved partner-specific account set, cohort size, limits, hours, dates, and observation window. |
| 3. Immediate stop events and restart authority | **Recorded stop conditions:** duplicate movement, unbalanced journal, unexplained reconciliation mismatch, tenant breach, stale-current balance/RYEW violation, missing audit/recovery proof, or unowned critical alert. **Needed:** the named incident/stop authority, containment route, and independent restart signers. |
| 4. Daily measures and reviewers | **Recorded categories:** posting/rejection/retries; reconciliation; balance/RYEW; outbox/delivery; access denials; latency/errors/backup age; support and integration findings. **Needed:** named reviewers, dashboards/source records, review window, and product-value measures. |
| 5. Graduate, extend, remediate, or stop thresholds | **Recorded scorecard:** graduate only with complete safety gates and approved partner outcomes; extend when safe but evidence is insufficient; remediate a failed correctable gate; stop when trust/legal/value/operations evidence does not support continuing. **Needed:** signed sample sizes, observation period, SLOs, and partner-value thresholds. |
| 6. Offboarding | **Needed:** owner and approved process for access/credential revocation, authorized export, retention, and legal holds. Preserve immutable posted records. |

Sources: [graduation scorecard](../pilot/graduation-scorecard.md), [partner readiness](../pilot/design-partner-readiness.md), [partner template](../pilot/partner-evidence-template.md).

## Phase 16 — Public communication

| Question | Answer |
|---|---|
| 1. Publication approval | **Recorded in the supplied questions:** the project is not to be published online now. Keep publication gated. **Needed later:** a new explicit publication decision and owner. |
| 2. Approved claims | **Needed:** current evidence and named legal/security approval for each product, security, availability, recovery, privacy, and accessibility claim. Local evidence must be labeled local. |
| 3. Requests, consent, retention, and response | **Needed:** intake owner, support route, lawful-purpose/consent process where applicable, retention/deletion policy, abuse controls, and response expectations. |
| 4. Surface separation | **Recorded architecture:** separate public website and authenticated operator console/BFF; no public self-service onboarding in the initial pilot. **Needed:** approved implementation and deployment evidence before publication. |

Source: [master plan](ledgersync-master-product-system-and-website-completion-plan.md).

## Phase 17 — Add-ons

**Recorded:** AI, extra analytics vendors, data warehouses, message brokers, Kubernetes, and multi-region financial writes are not current prerequisites. Keep them outside the current scope until evidence and a separate decision support them.

For each proposal, the answers still **needed** are: (1) the observed partner/operator problem and source; (2) the expected measurable benefit and evaluation; (3) added security, privacy, operations, and cost duties; and (4) named owner, exit/rollback path, and required approval. No such approval is supplied by the owner names.

## First owner follow-up

Resolve these four items first:

1. Contact route for Vivek Painjane and contact route for MasterVaik.
2. Decision-forum participants and approved decision-record location.
3. The scope, conditions, exclusions, and expiry of MasterVaik's confirmed delegated authority.
4. Named critical-area owners, with approval scope and appointment dates.

Keep every later external gate open until its required person, decision, and evidence are recorded. This draft does not modify the master register.
