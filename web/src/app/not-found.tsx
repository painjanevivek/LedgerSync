import Link from "next/link";

export default function NotFound() {
  return (
    <main className="route-boundary" aria-labelledby="route-not-found-heading">
      <section className="route-boundary-card">
        <p className="eyebrow">Route unavailable</p>
        <h1 id="route-not-found-heading">Page unavailable</h1>
        <p>
          This page could not be found. The address may be invalid, unavailable, or outside this application. No record or access status is disclosed.
        </p>
        <div className="route-boundary-actions">
          <Link className="button primary" href="/">Return to overview</Link>
        </div>
      </section>
    </main>
  );
}
