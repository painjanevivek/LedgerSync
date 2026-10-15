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
import type { ReactNode } from "react";

import { PageHeader } from "@/ui/display/PageHeader";
import { DisclosureSection } from "@/ui/disclosure/DisclosureSection";

const steps = [
  {
    title: "Create an account",
    description: "Create a source account and a different destination account in the same currency. Set a unique reference and category for each. Each new account starts at zero.",
    href: "/accounts/new",
    action: "Create account",
    icon: Bank,
  },
  {
    title: "Add a funding record",
    description: "Select the account and exact amount. Record the payment reference and supporting evidence reference for value verified outside LedgerSync.",
    href: "/funding",
    action: "Open funding",
    icon: Receipt,
  },
  {
    title: "Review the funding record",
    description: "Ask a different authorized reviewer to check the payment evidence. The server applies the review policy. Approval alone does not change the balance.",
    href: "/approvals",
    action: "Review funding",
    icon: Receipt,
  },
  {
    title: "Post the approved funding",
    description: "Open the approved record and post its balanced journal. Check the posted receipt and account balance before a transfer.",
    href: "/funding",
    action: "Open approved funding",
    icon: Receipt,
  },
  {
    title: "Make a transfer",
    description: "Choose the source account, destination account, and amount. Review, then confirm once.",
    href: "/transfers",
    action: "Open transfers",
    icon: ArrowsLeftRight,
  },
  {
    title: "Verify the transfer result",
    description: "Open the receipt and its debit and credit entries. Delivery status is separate. If the result is unknown, keep the original request and check its status or retry the same intent.",
    href: "/transfers",
    action: "Inspect transfers",
    icon: CheckCircle,
  },
  {
    title: "Check ledger balances",
    description: "Compare posted ledger entries with stored balances. This does not check bank or provider statements.",
    href: "/reconciliation",
    action: "Open reconciliation",
    icon: ShieldCheck,
  },
  {
    title: "Keep your records",
    description:
      "Check transfer proof and event delivery. Export the required CSV records. Create host backups outside the browser.",
    href: "/recovery",
    action: "Review recovery",
    icon: Archive,
  },
] as const;

export function GuideView({ orientation }: Readonly<{ orientation?: ReactNode }>) {
  return (
    <>
      <PageHeader
        eyebrow="Guide / First steps"
        title="Get started"
        description="Follow the record from creation to recovery."
      >
        <Link className="button secondary" href="/">
          Back to overview
        </Link>
      </PageHeader>

      <section className="guide-principle" aria-label="LedgerSync operating principle">
        <BookOpenText weight="fill" aria-hidden="true" />
        <div>
          <h2>Follow the ledger path</h2>
          <p>Each action needs the records from the step before it.</p>
        </div>
      </section>

      {orientation}

      <DisclosureSection
        id="guide-reference-path"
        title="Full operating path"
        summary="Eight steps from account creation to recovery."
        defaultOpen={!orientation}
        lazy={Boolean(orientation)}
      >
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
      </DisclosureSection>

      <DisclosureSection
        id="guide-follow-up"
        title="Follow up on a record"
        summary="Find the original result, inspect delivery, or hand off an investigation."
        lazy
      >
        <ol className="guide-steps" aria-label="LedgerSync follow-up steps">
          <li><span className="guide-index" aria-hidden="true">01</span><article><BookOpenText aria-hidden="true" /><div><h2>Find the original record</h2><p>Search its complete ID or approved request reference. A lost response does not prove failure. If browser data is lost, sign in as the same user and open Recover a saved transfer. Review the saved accounts and amount before retrying.</p></div><Link className="text-link" href="/transfers#saved-transfer-recovery">Recover a saved transfer →</Link></article></li>
          <li><span className="guide-index" aria-hidden="true">02</span><article><Receipt aria-hidden="true" /><div><h2>Inspect event delivery</h2><p>Read the linked event and its attempts. A delivery failure does not reverse a posted journal. Reconciliation covers only its recorded snapshot.</p></div><Link className="text-link" href="/events">Open events →</Link></article></li>
          <li><span className="guide-index" aria-hidden="true">03</span><article><ShieldCheck aria-hidden="true" /><div><h2>Preserve and hand off an investigation</h2><p>Save an authorized search result as an investigation. The next operator must read current evidence with their own access. A saved reference grants no account access.</p></div><Link className="text-link" href="/search">Open investigations →</Link></article></li>
        </ol>
      </DisclosureSection>

      <DisclosureSection
        id="guide-ledger-terms"
        title="Ledger terms"
        summary="Five terms used in receipts and recovery."
        lazy
      >
        <dl>
          <dt>Minor unit</dt><dd>The smallest stored unit of a currency. For INR, 100 minor units equal INR 1.00. Enter the exact amount shown on the review screen.</dd>
          <dt>Journal</dt><dd>The linked debit and credit entries for one financial action. Posted entries remain in history.</dd>
          <dt>Request key</dt><dd>The saved identity of one intended action. Retry an unknown result with the same key and the same details.</dd>
          <dt>Unknown result</dt><dd>The response did not prove success or failure. Check the original request before starting another action.</dd>
          <dt>Internal reconciliation</dt><dd>A check of ledger entries against stored balances at a recorded point in time. It does not prove that a bank payment settled.</dd>
        </dl>
      </DisclosureSection>

      <section className="guide-safety" aria-labelledby="guide-safety-title">
        <CheckCircle weight="fill" aria-hidden="true" />
        <div>
          <h2 id="guide-safety-title">Stop preserves records. Reset removes them.</h2>
          <p>Use reset only when you want to remove the local ledger and start again.</p>
        </div>
      </section>
    </>
  );
}
