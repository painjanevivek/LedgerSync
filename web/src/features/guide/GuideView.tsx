import {
  Archive,
  ArrowsLeftRight,
  Bank,
  BookOpenText,
  CheckCircle,
  Receipt,
  ShieldCheck,
} from "@phosphor-icons/react/dist/ssr";
import Link from "next/link";

import { PageHeader } from "@/features/console/components";

const steps = [
  {
    title: "Create an account",
    description:
      "Name the account, choose its ledger category, confirm the fixed INR boundary, and review the command before creating it.",
    href: "/accounts/new",
    action: "Create account",
    icon: Bank,
  },
  {
    title: "Record incoming funds",
    description:
      "Use Funding records to add an external value reference. Review, approve, and post it so the balance is backed by a balanced journal.",
    href: "/funding",
    action: "Open funding",
    icon: Receipt,
  },
  {
    title: "Create the destination",
    description:
      "Create a second active INR account. Transfers require distinct source and destination accounts with compatible authorization.",
    href: "/accounts/new",
    action: "Create destination",
    icon: Bank,
  },
  {
    title: "Post an internal transfer",
    description:
      "Choose both accounts, enter the exact amount, review the intent, then confirm once. If the result is uncertain, retry the same request.",
    href: "/transfers",
    action: "Open transfers",
    icon: ArrowsLeftRight,
  },
  {
    title: "Run reconciliation",
    description:
      "Verify that account projections match immutable ledger postings. A matched run with zero mismatches is the completed result.",
    href: "/reconciliation",
    action: "Run reconciliation",
    icon: ShieldCheck,
  },
  {
    title: "Keep your records",
    description:
      "Inspect delivery events and the stored transfer proof chain, export the bounded CSV records you need, and create host backups outside the browser.",
    href: "/recovery",
    action: "Review recovery",
    icon: Archive,
  },
] as const;

export function GuideView() {
  return (
    <>
      <PageHeader
        eyebrow="Guide / First successful ledger path"
        title="Run LedgerSync with confidence"
        description="Follow this sequence once from an empty workspace. Each step creates the records required by the next, so you never need to invent balances or bypass ledger controls."
      >
        <Link className="button secondary" href="/">
          Return to overview
        </Link>
      </PageHeader>

      <section className="guide-principle" aria-label="LedgerSync operating principle">
        <BookOpenText weight="fill" aria-hidden="true" />
        <div>
          <p className="eyebrow">The efficient path</p>
          <h2>Account → funding → transfer → reconciliation</h2>
          <p>
            Work left to right. Funding establishes value, transfers move it,
            and reconciliation proves the resulting balances.
          </p>
        </div>
      </section>

      <ol className="guide-steps" aria-label="LedgerSync usage steps">
        {steps.map((step, index) => {
          const Icon = step.icon;
          return (
            <li key={`${index}-${step.title}`}>
              <span className="guide-index" aria-hidden="true">
                {String(index + 1).padStart(2, "0")}
              </span>
              <article>
                <Icon weight="fill" aria-hidden="true" />
                <div>
                  <h2>{step.title}</h2>
                  <p>{step.description}</p>
                </div>
                <Link className="text-link" href={step.href}>
                  {step.action} <span aria-hidden="true">→</span>
                </Link>
              </article>
            </li>
          );
        })}
      </ol>

      <section className="guide-safety" aria-labelledby="guide-safety-title">
        <CheckCircle weight="fill" aria-hidden="true" />
        <div>
          <p className="eyebrow">Remember</p>
          <h2 id="guide-safety-title">Stop safely; reset deliberately.</h2>
          <p>
            Normal stop preserves PostgreSQL and Redis volumes. Reset permanently
            removes the local ledger and should only be used when you intentionally
            want another empty workspace.
          </p>
        </div>
      </section>
    </>
  );
}
