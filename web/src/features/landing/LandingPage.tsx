import { ArrowRight, ArrowDown, Bank, ShieldCheck, ListChecks, Eye, Info, ArrowUpRight } from "@phosphor-icons/react/dist/ssr";
import { CommandSteps } from "@/ui/presentation/CommandSteps";

const questions = [
  ["Who is LedgerSync for?", "Business and finance operators who need clear balances, careful money workflows, and a reliable record of what happened."],
  ["Does Add money make a bank deposit?", "No. You record money confirmed outside LedgerSync and provide supporting information. Review and posting are separate steps; submitting a record does not mean a balance has been credited."],
  ["What happens if a transfer is still being confirmed?", "Do not create another transfer. Keep the original request and follow its recovery guidance. A missing response is not proof that no money moved."],
  ["What is Expert view?", "Expert view reveals additional filters, record references, and technical evidence. It never grants extra permissions or changes your money."],
];

export function LandingPage() {
  return <div className="public-site">
    <a href="#public-content" className="skip-link">Skip to main content</a>
    <header className="public-header">
      <a className="brand" href="/welcome" aria-label="LedgerSync introduction">Ledger<span>Sync</span></a>
      <nav className="public-links" aria-label="Page sections"><a href="#how-it-works">How it works</a><a href="#features">Features</a><a href="#questions">Questions</a></nav>
      <a className="button primary" href="/sign-in">Open workspace <ArrowUpRight aria-hidden="true" /></a>
      <details className="public-mobile-menu"><summary>Menu</summary><nav aria-label="Mobile page sections"><a href="#how-it-works">How it works</a><a href="#features">Features</a><a href="#questions">Questions</a></nav></details>
    </header>
    <main id="public-content">
      <section className="public-hero" aria-labelledby="public-title">
        <div className="public-hero-copy"><p className="eyebrow">CLEAR MONEY OPERATIONS</p><h1 id="public-title">Your money workflows.<br /><span>One clear step<br className="hero-break" /> at a time.</span></h1><p className="public-lede">See your balances, review each move, and know what needs your attention—without the technical noise.</p><div className="public-hero-actions"><a className="button primary" href="/sign-in">Open workspace <ArrowRight aria-hidden="true" /></a><a className="public-text-link" href="#how-it-works">See how it works <ArrowDown aria-hidden="true" /></a></div><p className="public-footnote">Built around review. Clear about the result.</p></div>
        <figure className="public-preview" aria-label="Illustrative transfer review, not connected to an account">
          <div className="preview-title"><span>Transfers / New transfer</span><span>Simple view</span></div>
          <h2>Check before you transfer.</h2><p>Nothing moves until you confirm.</p><CommandSteps stage="review" />
          <p className="preview-amount-label">You’re moving</p><strong className="preview-amount">₹2,500.00</strong>
          <div className="preview-account"><Bank aria-hidden="true" /><div><span>From</span><strong>Operating account</strong></div></div><div className="preview-account"><ShieldCheck aria-hidden="true" /><div><span>To</span><strong>Reserve account</strong></div></div>
          <div className="preview-explanation"><Info aria-hidden="true" /><span>Review the amount and both accounts before money moves.</span></div>
          <figcaption>Illustrative example · No money moves.</figcaption>
        </figure>
      </section>
      <section id="how-it-works" className="public-section" aria-labelledby="how-title"><p className="eyebrow">HOW IT WORKS</p><h2 id="how-title">A clear path from intent to outcome.</h2><div className="public-three-columns">{[["01", "Enter the details", "Choose your accounts and amount. Extra information stays out of the way until you need it."], ["02", "Review the effect", "Check what you’re asking to change before giving your confirmation."], ["03", "Understand the result", "See what completed, what needs attention, and the safe next step."]].map(([number, title, copy]) => <article key={number}><span className="public-section-number">{number}</span><h3>{title}</h3><p>{copy}</p></article>)}</div></section>
      <section id="features" className="public-section public-benefits" aria-labelledby="features-title"><div><p className="eyebrow">LESS TO INTERPRET. MORE CLARITY.</p><h2 id="features-title">The important things,<br />easy to find.</h2><p>Start with the outcome. Open the details when you need them.</p></div><div className="public-benefit-list">{[[Eye, "Understand your available money", "Read exact balances and see when they were checked."], [Bank, "Review every move", "Keep the amount, accounts, and expected effect together."], [ListChecks, "Find work needing attention", "Follow clear actions for reviews and uncertain results."]].map(([Icon, title, copy]) => { const Symbol = Icon as typeof Eye; return <article key={String(title)}><Symbol aria-hidden="true" /><div><h3>{String(title)}</h3><p>{String(copy)}</p></div></article>; })}</div></section>
      <section className="public-clarity" aria-labelledby="clarity-title"><ShieldCheck aria-hidden="true" /><div><h2 id="clarity-title">Clarity matters most when something needs checking.</h2><p>An incomplete response is not a completed transfer—or proof that nothing happened. LedgerSync keeps uncertainty visible and directs you back to the original request.</p></div></section>
      <section id="questions" className="public-section public-questions" aria-labelledby="questions-title"><div><p className="eyebrow">A FEW THINGS TO KNOW</p><h2 id="questions-title">Common questions.</h2></div><div>{questions.map(([question, answer]) => <details key={question}><summary>{question}</summary><p>{answer}</p></details>)}</div></section>
      <section className="public-closing"><h2>Ready to open your workspace?</h2><a href="/sign-in" className="button primary">Open workspace <ArrowRight aria-hidden="true" /></a></section>
    </main>
    <footer className="public-footer"><a className="brand" href="/welcome">Ledger<span>Sync</span></a><p>Software for internal money workflows. Not a bank or an external payment settlement service.</p><p>Only essential authentication and security cookies. No analytics or marketing tracking.</p><a href="#questions">Questions</a></footer>
  </div>;
}
