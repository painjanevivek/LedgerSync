"use client";

import Link from "next/link";

export default function RouteError({ reset }: Readonly<{ reset: () => void }>) {
  const retryCurrentRoute = () => {
    reset();
    window.location.reload();
  };

  return (
    <main className="route-boundary" aria-labelledby="route-error-heading">
      <section className="route-boundary-card route-boundary-card-error" role="alert">
        <p className="eyebrow">Render interrupted</p>
        <h1 id="route-error-heading">This page could not be shown safely.</h1>
        <p>
          LedgerSync has not inferred a financial result. Retry the same page, or return to the overview and verify the latest recorded evidence.
        </p>
        <div className="route-boundary-actions">
          <button className="button primary" type="button" onClick={retryCurrentRoute}>
            Try again safely
          </button>
          <Link className="button secondary" href="/">Return to overview</Link>
        </div>
      </section>
    </main>
  );
}
