# Master Phase 0 baseline evidence

**Captured:** 2026-08-28 (Asia/Calcutta)

**Baseline branch:** `main`

**Baseline commit:** `1fa770936472dd1089f8ec0997b25bdbbd6c20fa`

**Remote:** `origin` (`https://github.com/painjanevivek/Real-Time-Balance-Visibility-in-Microservice-Based-Money-Transfers.git`)

**Phase outcome:** canonical planning baseline established; Phase 1 quality reconvergence is next

## Preserved work

The master plan described four uncommitted responsive-layout changes at `f87245f`. They were already preserved in the immediately following commit:

```text
1fa7709 fix(layout) : eliminate detached blank viewport space
```

The baseline working tree was clean before this phase. No existing product change was overwritten or discarded.

## Planning reconciliation

- The supplied 1,201-line master plan is committed at `docs/plans/ledgersync-master-product-system-and-website-completion-plan.md` and is content-equivalent to the supplied source after newline normalization.
- `docs/plans/ledgersync-master-progress.md` owns current phase status, dependencies, evidence pointers, owner categories, next actions, and stop-ship rules.
- The earlier implementation plan, future-scope roadmap, roadmap register, and detailed pilot gate register remain available as historical evidence and explicitly defer current status to the master register.
- Spec Kit contains 121 tasks: 118 checked and three deliberately open external/manual gates (`T094`, `T095`, and `T121`).
- The requirements-quality checklist is 16/16 checked.

## Host and tool baseline

| Item | Observed value |
|---|---|
| Host shell | PowerShell 7.6.4 |
| Git | 2.51.1.windows.1 |
| Go | 1.26.6 windows/amd64 |
| Node.js | 24.12.0 |
| npm | 11.6.2 |
| Docker CLI | 29.5.3 |
| Docker Compose | 5.1.4 |
| Local environment files | `.env.example` only; no local `.env` was present |
| Relevant listening ports | No listener observed on 3000, 8080, 5432, or 6379 |

Docker Desktop was installed but its Linux engine was stopped. Compose interpolation also correctly rejected the absent required local API token. These are recorded baseline conditions, not a ready-runtime result. Phase 2 must turn each condition into clear doctor/start guidance; Phase 1 real-stack qualification requires starting the engine and creating isolated runtime secrets.

## Exact-baseline workflow truth

GitHub Actions for `1fa7709` reported:

| Workflow | Result |
|---|---|
| Production-path CI | Passed |
| Quality gates | Failed |
| Supply-chain and security gates | Failed |

Therefore Phase 1 is active and the current commit is not qualified. Earlier green evidence remains historical and is not reused as proof for this baseline.

## Repository setup verification

- `.gitignore` covers local secrets, Go/Node/Python outputs, editor state, local tools, caches, and agent-only instructions.
- `.dockerignore` excludes Git, local agent/index state, dependencies, builds, secrets, logs, specifications, and test inputs from image contexts.
- `web/eslint.config.mjs` contains global ignores for generated output, dependencies, coverage, Playwright reports/results, and minified assets.
- No Prettier configuration, publishable npm package, Terraform module, or Helm chart exists at this baseline, so no additional ignore file is required in Phase 0.
- `git diff --check` passed for the Phase 0 documentation changes.

## Phase 0 gate decision

Phase 0 is complete because prior work is preserved, the master plan and one status register now agree with repository evidence, historical sources are retained without competing authority, and future phases have IDs, dependencies, owner categories, evidence paths, next actions, and stop-ship rules. This does not claim that Phase 1 or any provider/human gate has passed.
