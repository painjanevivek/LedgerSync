import summary from "@/lib/portfolio-demo.generated.json";
import { CopyControl } from "@/ui/controls/CopyControl.client";
import { DataTableRegion } from "@/ui/display/DataTableRegion";

export function SystemsLabEvidence() {
  return <section aria-labelledby="local-demo-proof-heading">
    <h3 id="local-demo-proof-heading">Last local failure demonstration</h3>
    <p>This is a retained result from synthetic tests on one local host. It is separate from current service health.</p>
    <dl className="evidence-list">
      <div><dt>Recorded outcome</dt><dd>{summary.passed ? "All named demonstration checks passed" : "Demonstration has failed or incomplete checks"}</dd></div>
      <div><dt>Completed</dt><dd><time dateTime={summary.completed_at}>{summary.completed_at.replace("T", " ").replace("Z", " UTC")}</time></dd></div>
      <div><dt>Run</dt><dd><CopyControl value={summary.run_id} label="Copy demonstration run ID" /></dd></div>
      <div><dt>Source snapshot</dt><dd><CopyControl value={summary.source.manifest_sha256} label="Copy demonstration source hash" />{summary.source.dirty ? "Includes recorded local changes." : "Recorded committed source."}</dd></div>
      <div><dt>Verified artifacts</dt><dd>{summary.artifact_count} artifacts checked when this summary was made. Test binaries and private logs stay on the host.</dd></div>
      <div><dt>Contract and schema</dt><dd>API {summary.api_contract_version}; {summary.migration.count} applied migrations.</dd></div>
    </dl>
    <DataTableRegion label="Local failure demonstration checks"><table className="data-table"><thead><tr><th scope="col">Check</th><th scope="col">Recorded result</th></tr></thead><tbody>{summary.scenarios.map(scenario => <tr key={scenario.id}><td>{scenario.id.replaceAll("-", " ")}</td><td>{scenario.status}</td></tr>)}</tbody></table></DataTableRegion>
    <p>A host operator can run the fixed scenarios with <code>node scripts/portfolio/run-demo.mjs</code>. This page can only read the saved summary. It cannot start faults or access Docker.</p>
  </section>;
}
