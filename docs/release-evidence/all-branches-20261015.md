# Branch consolidation — 15 October 2026

## Scope and preserved source

The owner requested all development branches and verified local work in main. Local source was frozen while the other task paused. Full recovery commit: `32d8038d4236f71be6c0c3d0ce2aba5e6f9776b7`. Curated candidate: `17c7c250a5985df9a2e7cd302ba11ccaadef164f`. The 122 excluded paths contain local `.impeccable`, video output and generated report artifacts. They remain in the original checkout and full recovery commit. Runtime secrets, financial data, private acceptance evidence, backup branches and stash records are not publication inputs.

## Reviewed conflict resolutions

- The ten split PR branches (#13–#22) were already squash merged. The final split tree `81e1a1d` exactly equals the pre-consolidation main tree `d4991cc`. Retain the current tree and record their original ancestry in this integration branch.
- Original hardening branches through controlled financial commands are ancestors of the local portfolio branch. Retain the newer actor-bound functions, scoped repositories, worker fences and immutable evidence.
- Approved opening imports were missing. Port `1cfadec` to migration **000064**, because 000037–000063 are already assigned. Keep immutable manifests, independent finance approval, exact content hashes, zero-state reconciliation, atomic execution, replay safety and provisioning-only execution. Add forced tenant policies, actor context, current rollback protection and the complete function ACL inventory.
- Older correction and financial-DML contractions are replaced by current migrations 000048–000060 and the R03 restricted-role matrix. Do not replay old numbered migrations or restore obsolete function names and broad writes.
- Older tenant RLS uses `ledgersync.tenant_id`. The current implementation uses both `app.tenant_id` and `app.actor_id`, clears pooled state and forces scoped policies. Keep it, including migration 000063 for sessions/preferences and the new opening evidence policies.
- The integration branch's Vercel packaging fix was missing. Preserve conditional output tracing for Vercel and standalone output for containers.
- The simple-first and fork UI work is represented by the latest guided UI and scoped sessions. Retain current migration 000062/000063 rather than duplicate old migration 000037. Current system design, diagrams, exact money, concurrency, idempotency, consistency, delivery and recovery documentation supersede the older README layout. Keep the current README's explicit local-only boundary and links.
- Dispatcher adapters moved from application to platform packages. Preserve that boundary and its current tests; do not restore removed adapters.
- SQL files use LF in Git and in checkouts. This prevents Windows CRLF from breaking exact function-body migration rewrites.

The merge below records branch ancestry **in the integration branch**. Main requires linear history. A GitHub squash merge preserves the reviewed final content and links this evidence; original branch tips remain available as recovery references. No branch deletion is part of this change.

## Remote head coverage

| Reference | Frozen head | In integration ancestry |
|---|---|---|
| `origin/codex/003-simple-first-hardening` | `b36b3289e7532556e869ff26eaa42a6a0509fd41` | Yes |
| `origin/codex/integrate-progressive-disclosure` | `4ecaef9b272660145d766a1ef85cadec04e97005` | Yes |
| `origin/codex/main-production-hardening` | `8a6177aec99aac1fbfdf48e648f3ace1c6997c2e` | Yes |
| `origin/codex/portfolio-restructuring-r00-r10` | `65e45ae1ac399c2c09f612941fa2e2c6ed178269` | Yes |
| `origin/codex/pr11-01-worker-response-ci` | `3520861181258fb19066dbe860c5dfe20de7451e` | Yes |
| `origin/codex/pr11-02-identifiers` | `483e18d360f435c8de182c554a07c4a6859581f1` | Yes |
| `origin/codex/pr11-03-controlled-ledger` | `7948df3105f421b197e61b3390eb0ee97cd22b6b` | Yes |
| `origin/codex/pr11-04-progressive-disclosure` | `4139a5717f091ab4539668df3f86a0b428eaa461` | Yes |
| `origin/codex/pr11-05-deployment` | `df604a7f43aff0a33bf3960dcb699307175e407e` | Yes |
| `origin/codex/pr11-06-opaque-sessions` | `76832764f8ff8a25ecfdf3fca3122083b939c58a` | Yes |
| `origin/codex/pr11-07-bff-boundaries` | `ed085da0eba1cc41f481fac6d1e8a062b93bb77e` | Yes |
| `origin/codex/pr11-08-ui-foundations` | `80a9200ab46eb07bf512c7b40098aa50fdcf9f92` | Yes |
| `origin/codex/pr11-09-guided-workflows` | `dd65cbd9e08dca4e3d92fb2afa0296552717f994` | Yes |
| `origin/codex/pr11-10-study-documentation` | `81e1a1d61f109d3e66c6aad9d03277f13407255c` | Yes |
| `origin/codex/production-hardening` | `49ab793ff3036802e65f164336f1e12c9ba4784a` | Yes |
| `origin/codex/production-hardening-pr003` | `be3638593dbceaa30f392080d6f8a717c67a72ec` | Yes |
| `origin/codex/production-hardening-pr003-api` | `c61d32a8ce2bd3470c863a024108a2f72b1b43d6` | Yes |
| `origin/codex/production-hardening-pr004-identifiers` | `116a243d83ef638a14e1135ed53d1a7b5b42f50c` | Yes |
| `origin/codex/production-hardening-pr005-role-harness` | `329ce24c7d9936734191d220f000b7f3c898a22e` | Yes |
| `origin/codex/production-hardening-pr006-ledger-semantic-keys` | `9aa285fa0587ceed13db523bce143ce6b459c31e` | Yes |
| `origin/codex/production-hardening-pr007-ledger-semantic-validation` | `4b368afbc4130ed9cced199a2cb4735640b46bcc` | Yes |
| `origin/codex/production-hardening-pr008-controlled-financial-commands` | `777f90827c5637b57a3455c7466bf772af343485` | Yes |
| `origin/codex/production-hardening-pr008-funding-corrections` | `1cfadec01e8dd1a7cb6ca8bb19f2935c382a1fbe` | Yes |
| `origin/codex/production-hardening-pr009-revoke-financial-dml` | `b9edecbd8bd9a5281bd64ad1f6e15fe401943e9f` | Yes |
| `origin/codex/production-hardening-pr010-tenant-rls` | `52bf5c0df6ecec7b52cecceeb921cb81f6b1d323` | Yes |
| `origin/codex/progressive-disclosure` | `5b4654ba8bf71d9b9c29b1edfd739c1fb4e2dee1` | Yes |
| `origin/codex/secure-transfer-foundation` | `57ef4afca9c2a91259375837915e9979cc74d28a` | Yes |
| `origin/codex/secure-webrtc-production-hardening` | `ed1662ebc52fbff49bd75bc957b5d2fa637fa73c` | Yes |
| `origin/pr-11-head` | `f453431713ce2342423f76be8f6d70d503cc8d75` | Yes |

The screenshot fork `yashmahajan11234567/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers:main` resolves to `f453431713ce2342423f76be8f6d70d503cc8d75`, fetched as `origin/pr-11-head`. Its ten non-ancestor commits are now recorded in the integration ancestry. The old ahead/behind counter is not a measure of missing feature content after a squash merge.

## Verification and limits

- Go deterministic production, unit, contract, architecture and model packages passed before history consolidation.
- Disposable full integration and fault inventories passed at source `b3b8a99`; no normal container or source changes were made during that run.
- Linux visuals, complete browser checks, final image journeys and hosted CI remain pending at this checkpoint. A pending check is not a pass.
- R13 remains in progress. R14-R17 are not completed by this consolidation. No managed environment, real provider, independent human accessibility review or production approval is claimed.
