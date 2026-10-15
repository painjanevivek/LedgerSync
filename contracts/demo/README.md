# Demonstration evidence contract

Version: `ledgersync-demo-run/v1`. Owner: portfolio evidence tooling.

The JSON Schema defines the permitted fields. The validator also checks result consistency, unique identifiers, time order, safe relative artifact paths, and file size. Use both checks. Install the locked web toolchain with `npm ci` in `web` first. The validator uses its existing Ajv dependency. It does not add a runtime dependency to the API or console.

Run `node --test scripts/portfolio/demo-contract.test.mjs` from the repository root. Validate a file with `node scripts/portfolio/demo-contract.mjs contracts/demo/run-manifest.v1.example.json`.

The example is synthetic. It proves that the contract is usable. It does not prove that a financial, failure, restore, or performance test ran. Validation checks structure and internal consistency. It does not execute a scenario, open referenced artifacts, authenticate an author, verify file digests, or prove isolation. R07 must implement those runtime checks before it accepts a bundle as evidence.

Only an allowlisted disposable project name is valid. The runner must also verify container labels, volume ownership, ports, and target identities before it performs a fault. A name alone is insufficient. Never run fault scenarios against the normal local stack.

Use SHA-256 for source and artifact identity. Use the executing image's immutable digest. Record schema and API contract versions separately. Dirty source requires a content manifest. Keep a run historical when any relevant source, image, migration, or configuration identity differs. Do not translate an old passing result into current service health.

Free-text reasons and limitations must contain no secrets or personal data. Structural validation cannot detect all sensitive text. Review artifacts before sharing them. Future incompatible changes require a new schema version and reader compatibility tests.
