# Guided Workspace delivery record

This supersedes the visual direction in the earlier simple-first plan, not its financial invariants. Existing checked tasks are historical and do not establish acceptance of this redesign.

Visual target: the third displayed concept (Guided Workspace), `exec-b142f8ba-7a55-49f7-ae17-d7f61464624a.png`. Implement in the existing Next application; no prototype scaffold, deployment, secrets, commits, or pushes.

- [ ] G01 Inventory routes/states and establish public/authenticated regression tests.
- [x] G02 Public landing, illustrative hero, welcome/sign-in, isolated provider boundary.
- [x] G03 Semantic design tokens, top navigation/Profile, responsive and focused frames.
- [x] G04 Shared three-stage controls and transfer review/result with exact expected effects and safe uncertainty.
- [ ] G05 Home/Accounts/Tasks source coverage, priorities, freshness, and shared presentation.
- [ ] G06 Funding, approval, correction, lifecycle, reconciliation, and replay guided workflows.
- [ ] G07 Expert tools, beginner Help, modular decomposition and obsolete-style cleanup.
- [ ] G08 Artifact filtering, dependencies, full frontend/build/configuration qualification.
- [ ] G09 Real-stack financial lifecycle, Go integration/race/migration/retention/recovery qualification.
- [ ] G10 Same-state visual comparison, responsive/accessibility evidence and completion matrix.
- [ ] G11 Five moderated operator sessions and remediation of repeated confusion (human acceptance).

## Route inventory

Public: `/welcome`, anonymous `/`, `/sign-in`. Workspace: authenticated `/`, `/accounts`, `/accounts/new`, account detail, `/funding` and detail, `/transfers` and detail, `/tasks`, `/approvals`, `/corrections` and detail, `/reconciliation` and detail, `/search`, investigation detail, `/events` and detail, `/webhooks` and detail, `/developer`, `/recovery`, `/local-status`, `/guide`. `/admin` stays unreleased. Error/denied/loading/empty/offline/partial/unknown states are required, not optional designs.

## Contract decisions

Public pages never mount account/task/preference providers. The anonymous root may resolve the session but cannot read financial data. Welcome is public regardless of session. Existing protected route URLs and server authorization remain unchanged.

Details → Review → Result is presentation, not a replacement transaction engine. Expected balances are exact projections, never guaranteed outcomes. Unknown submission retains exact intent and original key. A replay POST must never be labelled as a status read.

## Completion evidence

Record only freshly executed checks here. No implementation or acceptance gate is complete merely because a legacy test was green. Production release and human usability acceptance remain distinct from local code delivery.

See `design-qa.md` at the integration-worktree root for the current result and explicit blockers. G01 and G05 are partially implemented; G06 and G07 are not complete. G08–G11 remain acceptance gates, not implied passes from the implemented landing page.

## Compatibility review: empty correction pages

Real-stack UI inspection found successful empty correction lists serializing `events: null`. The existing public schema requires an array. The Go HTTP handler now normalizes a nil event slice to `[]` only after authorization and repository success. Errors remain errors; scopes, row authorization, commands, schema, and financial state transitions are unchanged. `TestCorrectionEmptyListUsesContractArray` verifies the wire contract. The frontend continues rejecting malformed list data rather than treating arbitrary missing data as an empty result.
