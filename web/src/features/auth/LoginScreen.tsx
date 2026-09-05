import { ArrowLeft, ArrowRight, LockKey } from "@phosphor-icons/react/dist/ssr";
import { safeInternalReturnPath } from "@/lib/navigation";
import { TechnicalDetails } from "@/ui/presentation/TechnicalDetails";

type Props = Readonly<{ returnTo?: string; unavailableMessage?: string | null; localAccess?: boolean }>;

export function LoginScreen({ returnTo = "/", unavailableMessage = null, localAccess = false }: Props) {
  const loginHref = `/api/auth/sign-in?return_to=${encodeURIComponent(safeInternalReturnPath(returnTo) ?? "/")}`;
  return <main className="guided-sign-in">
    <header><a className="brand" href="/welcome" aria-label="LedgerSync introduction">Ledger<span>Sync</span></a></header>
    <section aria-labelledby="login-title"><LockKey aria-hidden="true" /><h1 id="login-title">Open your workspace.</h1><p>See your money, review each move, and take the next clear step.</p>
      {unavailableMessage && <div role="alert"><p>We couldn’t open your workspace. No financial information is shown.</p><TechnicalDetails summary="View sign-in details"><p>{unavailableMessage}</p></TechnicalDetails></div>}
      <a className="button primary" href={loginHref}>Sign in <ArrowRight aria-hidden="true" /></a>
      <p className="sign-in-method">{localAccess ? "This local workspace uses a short-lived development session." : "Continue with the sign-in method configured for your workspace."}</p>
      <p className="sign-in-privacy">Only essential authentication and security cookies. No marketing or analytics tracking.</p>
      <a className="command-back" href="/welcome"><ArrowLeft aria-hidden="true" />About LedgerSync</a>
    </section>
  </main>;
}
