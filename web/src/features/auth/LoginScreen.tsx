import {
  ArrowRight,
  CheckCircle,
  Database,
  LockKey,
} from "@phosphor-icons/react/dist/ssr";

type Props = Readonly<{
  returnTo?: string;
  unavailableMessage?: string | null;
}>;

export function LoginScreen({
  returnTo = "/",
  unavailableMessage = null,
}: Props) {
  const loginHref = `/api/auth/sign-in?return_to=${encodeURIComponent(returnTo)}`;
  return (
    <main className="login-shell">
      <header className="login-topbar">
        <span className="login-brand" aria-label="LedgerSync">
          Ledger<span>Sync</span>
        </span>
        <span className="login-context">Local ledger workspace</span>
      </header>

      <section className="login-intro" aria-labelledby="login-title">
        <p className="eyebrow">Exact internal ledgers</p>
        <h1 id="login-title">Your ledger starts empty.</h1>
        <p className="login-lede">
          Sign in to build a workspace from the first account onward. LedgerSync
          adds no sample balances, transfers, or reconciliation results.
        </p>
        <div className="login-boundary">
          <Database weight="fill" aria-hidden="true" />
          <p>
            <strong>Your local records persist.</strong>
            PostgreSQL remains the financial authority when you stop and restart
            the application.
          </p>
        </div>
      </section>

      <section className="login-entry" aria-labelledby="login-entry-title">
        <div className="login-entry-rule" aria-hidden="true">
          <span>01</span>
          <span>02</span>
          <span>03</span>
        </div>
        <div className="login-entry-content">
          <LockKey weight="fill" aria-hidden="true" />
          <p className="eyebrow">Local access</p>
          <h2 id="login-entry-title">Open your workspace</h2>
          <p>
            For this workstation, one click creates a short-lived local session.
            The production OIDC boundary remains unchanged.
          </p>
          {unavailableMessage && (
            <p className="login-error" role="alert">
              {unavailableMessage}
            </p>
          )}
          <a className="button primary login-button" href={loginHref}>
            Log in <ArrowRight weight="bold" aria-hidden="true" />
          </a>
          <div className="login-assurance">
            <CheckCircle weight="fill" aria-hidden="true" />
            <span>Loopback-only · 30-minute session · HttpOnly cookie</span>
          </div>
        </div>
      </section>
    </main>
  );
}
