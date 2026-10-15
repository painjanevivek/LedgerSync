# ADR-006: Version evidence and bind it to the executing candidate

Date: 14 October 2026. Status: accepted for evidence tooling.

Keep source identity, image identity, schema identity, runtime configuration, collection time, and test outcome together. HEAD is insufficient when the working tree is dirty. Store a manifest of file hashes and the Git index digest. Hash source, not secret values. Record only presence/size metadata for protected local configuration.

Separate source tests from runtime tests. R00 started existing containers without rebuilding. Its runtime smoke qualifies those recorded images; its Go and frontend tests qualify the captured source. Do not imply that these are the same candidate.

The R01 demonstration contract is `ledgersync-demo-run/v1`. It defines bounded result fields, artefact paths, hashes, scenario results, and limitations. A digest detects change but does not prove independent authorship. The example is explicitly synthetic contract data, not a completed run.

Only a completed scenario with recorded checks can be passed. Failed, skipped, and incomplete outcomes require a reason. Runtime truth remains independent of test evidence. A historical passing run cannot become current service health.

R00 preserves existing files and data, with explicit additions to the master progress register and external plan. Check all original source hashes at phase completion. Do not stage, commit, push, or reset pre-existing work as part of this packet.

R13 owns combined qualification. R17 owns publication and managed production decisions. Portfolio completion does not supply legal, customer, managed identity, or independent review approval.
