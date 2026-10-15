# Independent financial model

R08 compares the application with a small independent ledger. The model uses only the Go standard library and wide integer arithmetic. It does not call the production money parser, fingerprint function, financial SQL or posting calculations. An architecture check enforces this separation.

The model starts with two synthetic USD accounts. Their balances are 10,000 and 2,000 minor units. Each balance version starts at zero. Funding adds a separate zero-opening clearing account. Its negative balance offsets the funded customer account. The total ledger value therefore stays 12,000 minor units. No external money moves.

## What the tests compare

The model selects an expected outcome before a financial command runs. It predicts exact balances, account versions, journal count, replay behavior and conflicts. The application runs through an actual restricted API database login. Test setup and invariant inspection use the disposable database owner. Setup authority is not presented as application authority.

After each step, the test compares every account balance and version with the model. It checks that each journal has two entries in one currency and a net total of zero. It also compares hashes of all previous posted transfers, journals and postings. A later operation must not edit or delete an old financial record.

The five default seeds are `17`, `29`, `101`, `404`, and `20261015`. Each seed produces 64 steps. The initial steps guarantee the main paths. Later steps select a bounded mix of:

- A new transfer in either direction.
- A replay of an existing posted or rejected intent.
- A changed financial intent with an existing key.
- Funding with an independent reviewer, posting, and replay.
- An exact correction with its normal independent approval and step-up checks.
- A lost caller result followed by reconstruction and a same-intent retry.
- An injected pre-commit outbox failure, full rollback, and a same-key retry.
- Four concurrent calls with the same intent. One original result and three replays must remain.
- A request larger than the available customer balance.

A correction is attempted only when a posted original remains eligible and its exact reverse amount is available. Otherwise that generated step is a checked no-op. The suite does not weaken a product rule to force an invalid correction through. The receipt comparison follows the public contract: the debit-account version is required. A recipient read capability is not implied. Both stored account versions are still checked independently.

## Run and reproduce

Run `node scripts/portfolio/run-model.mjs`. It reuses the R07 guarded host harness with a fixed financial-model profile. It creates new temporary PostgreSQL and Redis containers, applies the candidate migrations and roles there, compiles the actual integration test binary, and runs the named generated-model test. It removes only those exact temporary containers after recording the evidence. Normal services and volumes remain separate.

The profile has a ten-minute wall bound, four-command concurrency bound, and a 2,048-command allowance. A single sequence has a 90-second database deadline. The suite is also part of the normal integration test run. To select one default seed in an already verified disposable harness, use the Go subtest name `TestGeneratedFinancialModel/seed_17`.

Every run logs a `MODEL_REPORT` JSON record with its seed and full operation vector. On an invariant mismatch, the suite retains the failing prefix and tries at most 20 deletion replays. A smaller vector is accepted only when it reproduces the same mismatch class. Infrastructure errors are not silently converted into financial counterexamples. A unit test checks the minimizer with a known two-step cause.

Save one report under the repository's private `data/portfolio/` tree. Run `node scripts/portfolio/run-model.mjs data/portfolio/<report>.json` to replay it in a new disposable project. The runner selects the recorded minimal vector when present. It validates kinds, integer values, references, size and count before starting containers. Unknown commands, arbitrary shell operations and normal database targets are not inputs. The replay vector becomes a hashed artifact in the new bundle.

## Fuzzing and limits

The independent oracle has a bounded ten-second fuzz run. It varies amounts, direction, funding, replay and conflict sequences. Separate tests cover canonical amounts, durable rejected outcomes and signed-64-bit overflow. CI includes this package in unit/race/vet checks and runs its fuzz target. No hosted CI result is claimed from local execution.

The database comparison covers a bounded USD subset. It does not prove every interleaving, every policy configuration or all possible input values. Permissions and malformed caller context remain R02/R03 test responsibilities. Dependency and process failures also have the R07 evidence. R16 must extend the model for reservations and capture before qualifying that new balance meaning.

The first model run found a harness assumption: it required a recipient version in the sender's receipt. The minimizer reduced this to one transfer. The public contract requires only the debit-account version. The harness was corrected, and the original failed reports remain. This observation is not a product money-movement defect. No production financial code changed in R08.
