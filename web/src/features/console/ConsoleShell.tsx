"use client";

import { List, X, SignOut, CaretDown, UserCircle } from "@phosphor-icons/react";
import Link from "next/link";
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useConsoleSession } from "./ConsoleSessionBoundary";
import { deriveConsoleCapabilities } from "./capabilities";
import { useExperienceMode } from "./ExperienceModeBoundary";
import { ExperienceModeSwitch } from "@/ui/presentation/ExperienceModeSwitch";
import { navigation } from "./ConsoleNavigation";
import { WorkspaceLayoutContext, type FocusedWorkspace } from "./WorkspaceLayoutContext";
import { COMPACT_NAVIGATION_MEDIA_QUERY } from "@/lib/responsive";

export type ConsoleSection = "overview" | "accounts" | "funding" | "transfers" | "approvals" | "corrections" | "reconciliation" | "local-status" | "events" | "developer" | "recovery" | "guide" | "search" | "tasks";
type Props = Readonly<{ section: ConsoleSection; children: ReactNode; tenantLabel: string; tenantMeta: string; environmentLabel: string; operatorLabel: string; operatorMeta: string }>;

export function ConsoleShell({ section, children, tenantLabel, environmentLabel, operatorLabel, operatorMeta }: Props) {
  const { session, online, signOut, signOutPending, signOutError } = useConsoleSession();
  const capabilities = useMemo(() => deriveConsoleCapabilities(session), [session]);
  const { mode } = useExperienceMode();
  const [focused, setFocused] = useState<FocusedWorkspace | null>(null);
  const [navigationOpen, setNavigationOpen] = useState(false);
  const menuButton = useRef<HTMLButtonElement>(null);
  const drawer = useRef<HTMLElement>(null);
  const closeNavigation = useCallback(() => { setNavigationOpen(false); requestAnimationFrame(() => menuButton.current?.focus()); }, []);
  useEffect(() => {
    if (!navigationOpen) return;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    drawer.current?.querySelector<HTMLElement>("button,a")?.focus();
    const containFocus = (event: KeyboardEvent) => {
      if (event.key === "Escape") { event.preventDefault(); closeNavigation(); return; }
      if (event.key !== "Tab") return;
      const items = Array.from(drawer.current?.querySelectorAll<HTMLElement>("a[href],button:not([disabled]),summary") ?? []).filter(item => item.checkVisibility() && !item.closest("details:not([open]) .workspace-popover-content"));
      const first = items[0], last = items.at(-1);
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
    };
    document.addEventListener("keydown", containFocus);
    return () => { document.body.style.overflow = previousOverflow; document.removeEventListener("keydown", containFocus); };
  }, [closeNavigation, navigationOpen]);
  useEffect(() => {
    const compact = window.matchMedia(COMPACT_NAVIGATION_MEDIA_QUERY);
    const closeOnDesktop = () => { if (!compact.matches) setNavigationOpen(false); };
    compact.addEventListener("change", closeOnDesktop);
    return () => compact.removeEventListener("change", closeOnDesktop);
  }, []);
  const primary = navigation.flatMap(group => group.items).filter(item => item.experience === "both" && item.visible(capabilities));
  const expert = navigation.map(group => ({ ...group, items: group.items.filter(item => item.experience === "expert" && item.visible(capabilities)) })).filter(group => group.items.length);
  const links = primary.map(item => <Link key={item.section} href={item.href} prefetch={false} aria-current={section === item.section ? "page" : undefined} onClick={() => setNavigationOpen(false)}>{item.label}</Link>);
  const expertLinks = mode === "expert" && expert.length > 0 ? <details className="workspace-popover"><summary>Expert tools <CaretDown aria-hidden="true" /></summary><div className="workspace-popover-content">{expert.map(group => <section key={group.id}><h2>{group.label}</h2>{group.items.map(item => <Link key={item.section} href={item.href} prefetch={false} onClick={() => setNavigationOpen(false)} aria-current={section === item.section ? "page" : undefined}>{item.label}</Link>)}</section>)}</div></details> : null;
  return <WorkspaceLayoutContext.Provider value={setFocused}><div className={`app-shell guided-shell${focused ? " is-focused" : ""}`}>
    <a className="skip-link" href="#main-content">Skip to main content</a>
    <header className="guided-topbar" inert={navigationOpen || undefined}>
      <Link className="brand" href="/" aria-label="LedgerSync overview">Ledger<span>Sync</span></Link>
      {focused ? <nav className="guided-breadcrumb" aria-label="Breadcrumb"><Link href={focused.returnTo}>{focused.returnLabel.replace(/^Back to /, "")}</Link><span aria-hidden="true">/</span><span>{focused.title}</span></nav> : <span className="guided-workspace-name">{tenantLabel}</span>}
      {!focused && <nav className="guided-desktop-nav" aria-label="Primary navigation">{links}<Link href="/guide" aria-current={section === "guide" ? "page" : undefined}>Help</Link>{expertLinks}</nav>}
      {focused && <span className="guided-mode-indicator">{mode === "simple" ? "Simple view" : "Expert view"}</span>}
      <details className="workspace-popover profile-popover" onKeyDown={event => { if (event.key === "Escape") { event.currentTarget.open = false; event.currentTarget.querySelector("summary")?.focus(); } }}><summary><UserCircle aria-hidden="true" /><span>Profile</span><CaretDown aria-hidden="true" /></summary><div className="workspace-popover-content"><strong>{operatorLabel}</strong><p>{operatorMeta} · {environmentLabel}</p><p>{mode === "simple" ? "Simple view" : "Expert view"}</p><ExperienceModeSwitch /><Link href="/welcome">About LedgerSync</Link>{session && <button className="button secondary" type="button" disabled={signOutPending || !online} onClick={() => void signOut()} aria-describedby={!online ? "sign-out-offline" : signOutError ? "sign-out-error" : undefined}><SignOut aria-hidden="true" />{signOutPending ? "Signing out" : "Sign out"}</button>}{!online && <p id="sign-out-offline">Reconnect to sign out safely.</p>}{signOutError && <p id="sign-out-error" role="alert">{signOutError}</p>}</div></details>
      {!focused && <button className="guided-menu-button button secondary" ref={menuButton} type="button" aria-expanded={navigationOpen} aria-controls="workspace-navigation" onClick={() => setNavigationOpen(true)}><List aria-hidden="true" />Menu</button>}
    </header>
    {navigationOpen && <><button className="guided-nav-backdrop" aria-label="Close navigation" onClick={closeNavigation} /><aside className="guided-nav-drawer" ref={drawer} id="workspace-navigation" role="dialog" aria-modal="true" aria-label="LedgerSync workspace"><button className="button secondary" onClick={closeNavigation} aria-label="Close navigation"><X aria-hidden="true" />Close</button><nav aria-label="Primary navigation">{links}<Link href="/guide" onClick={() => setNavigationOpen(false)}>Help</Link>{expertLinks}</nav></aside></>}
    <div className="workspace-column" inert={navigationOpen || undefined}><main id="main-content" className={`console-main section-${section}`}>{children}</main></div>
  </div></WorkspaceLayoutContext.Provider>;
}
export function ConsoleFooter({ pending = false }: { pending?: boolean } = {}) {
  return (
    <footer
      className={`console-footer${pending ? " is-pending" : ""}`}
      aria-hidden={pending || undefined}
    >
      <span>
        Balances come from verified ledger records.
      </span>
      <span className="expert-only">PostgreSQL alone supplies customer-visible balances. Redis is disposable. Exact times use UTC.</span>
      <span>© 2026 LedgerSync, Inc.</span>
    </footer>
  );
}

type OperatorWorkspaceProps = Readonly<{
  children: ReactNode;
  className?: string;
  footer?: ReactNode;
  rail?: ReactNode;
  railLabel?: string;
}>;

/**
 * A page-level composition contract for operational screens. The primary
 * document stays readable while an optional, non-interactive context rail can
 * occupy otherwise accidental wide-screen whitespace.
 */
export function OperatorWorkspace({
  children,
  className = "",
  footer,
  rail,
  railLabel = "Contextual information",
}: OperatorWorkspaceProps) {
  return (
    <div className={`operator-workspace ${className}`.trim()}>
      <div className="operator-workspace-primary">{children}</div>
      {rail ? (
        <aside className="operator-workspace-rail" aria-label={railLabel}>
          {rail}
        </aside>
      ) : null}
      {footer ? (
        <div className="operator-workspace-footer">{footer}</div>
      ) : null}
    </div>
  );
}
