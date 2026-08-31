import { headers } from "next/headers";
import Link from "next/link";
import { notFound } from "next/navigation";

const attemptPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/u;
const completedAttempts = new Set<string>();

type SearchParams = Promise<Record<string, string | string[] | undefined>>;

export default async function RouteErrorProbe({ searchParams }: Readonly<{ searchParams: SearchParams }>) {
  const requestHeaders = await headers();
  const query = await searchParams;
  const attempt = typeof query.attempt === "string" ? query.attempt : "";
  const enabled = process.env.LEDGERSYNC_ENABLE_TEST_RENDER_FAILURE === "true"
    && process.env.LEDGERSYNC_DEPLOYMENT_ENV === "development"
    && requestHeaders.get("host") === "127.0.0.1:3100"
    && attemptPattern.test(attempt);

  if (!enabled) notFound();

  if (!completedAttempts.has(attempt)) {
    if (completedAttempts.size >= 32) completedAttempts.clear();
    completedAttempts.add(attempt);
    throw new Error("confidential-render-probe-must-never-reach-the-browser");
  }

  return (
    <main className="route-boundary" aria-labelledby="route-recovery-heading">
      <section className="route-boundary-card">
        <p className="eyebrow">Safe retry complete</p>
        <h1 id="route-recovery-heading">The page rendered on retry.</h1>
        <p>No financial record was created or changed by this test-only recovery check.</p>
        <div className="route-boundary-actions">
          <Link className="button primary" href="/">Return to overview</Link>
        </div>
      </section>
    </main>
  );
}
