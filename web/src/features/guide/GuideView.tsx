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
    description: "Give the account a clear name, reference, and category. Creating an account starts it at INR 0.00.",
    href: "/accounts/new",
    action: "Create account",
    icon: Bank,
  },
  {
    title: "Add a funding record",
    description: "Add the account, amount, payment reference, and supporting document. This records what you checked outside LedgerSync.",
    href: "/funding",
    action: "Open funding",
    icon: Receipt,
  },
  {
    title: "Review the funding record",
    description: "Check the details before approving it. A funding record changes a balance only after review and posting.",
    href: "/funding",
    action: "Review funding",
    icon: Receipt,
  },
  {
    title: "Make a transfer",
    description: "Choose the account money leaves, the account it goes to, and the amount. Review the details, then confirm once.",
    href: "/transfers",
    action: "Open transfers",
    icon: ArrowsLeftRight,
  },
  {
    title: "Check your records",
    description: "Use reconciliation after transfers to check that account balances match the ledger records.",
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
        eyebrow="Guide / First steps"
        title="Use LedgerSync step by step"
        description="Start at the top. Each step unlocks the next one."
      >
        <Link className="button secondary" href="/">
          Return to overview
        </Link>
      </PageHeader>

      <section className="guide-principle" aria-label="LedgerSync operating principle">
        <BookOpenText weight="fill" aria-hidden="true" />
        <div>
          <p className="eyebrow">Your first path</p>
          <h2>Account → funding → review → transfer</h2>
          <p>Start with an account. Then add and review a funding record before making a transfer.</p>
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
          <h2 id="guide-safety-title">Keep your records safe.</h2>
          <p>Normal stop keeps your local records. Reset removes the local ledger, so use it only when you want to start again.</p>
        </div>
      </section>
    </>
  );
}
