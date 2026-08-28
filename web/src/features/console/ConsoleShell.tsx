"use client";

import {
  ArrowsLeftRight,
  ArrowsCounterClockwise,
  Bank,
  ChartDonut,
  CheckCircle,
  Pulse,
  Broadcast,
  Code,
  Archive,
  ShieldCheck,
  SignOut,
  UserCircle,
  List,
  Compass,
  Receipt,
  X,
} from "@phosphor-icons/react";
import Link from "next/link";
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";

export type ConsoleSection =
  | "overview"
  | "accounts"
  | "funding"
  | "transfers"
  | "corrections"
  | "reconciliation"
  | "local-status"
  | "events"
  | "developer"
  | "recovery";

type Props = Readonly<{
  section: ConsoleSection;
  children: ReactNode;
  tenantLabel: string;
  tenantMeta: string;
  environmentLabel: string;
  operatorLabel: string;
  operatorMeta: string;
  preview?: boolean;
  onSignOut?: () => void;
}>;

const navigation = [
  {
    label: "Financial workspace",
    items: [
      {
        section: "overview" as const,
        label: "Overview",
        href: "/",
        icon: ChartDonut,
      },
      {
        section: "accounts" as const,
        label: "Accounts",
        href: "/accounts",
        icon: Bank,
      },
      {
        section: "funding" as const,
        label: "Funding evidence",
        href: "/funding",
        icon: Receipt,
      },
      {
        section: "transfers" as const,
        label: "Transfers",
        href: "/transfers",
        icon: ArrowsLeftRight,
      },
      {
        section: "corrections" as const,
        label: "Corrections",
        href: "/corrections",
        icon: ArrowsCounterClockwise,
      },
      {
        section: "reconciliation" as const,
        label: "Reconciliation",
        href: "/reconciliation",
        icon: ShieldCheck,
      },
    ],
  },
  {
    label: "Local tools",
    items: [
      {
        section: "local-status" as const,
        label: "Local status",
        href: "/local-status",
        icon: Pulse,
      },
      {
        section: "events" as const,
        label: "Events",
        href: "/events",
        icon: Broadcast,
      },
      {
        section: "developer" as const,
        label: "Developer",
        href: "/developer",
        icon: Code,
      },
      {
        section: "recovery" as const,
        label: "Recovery",
        href: "/recovery",
        icon: Archive,
      },
    ],
  },
];

export function ConsoleShell({
  section,
  children,
  tenantLabel,
  tenantMeta,
  environmentLabel,
  operatorLabel,
  operatorMeta,
  preview = false,
  onSignOut,
}: Props) {
  const [navigationOpen, setNavigationOpen] = useState(false);
  const [compactNavigation, setCompactNavigation] = useState(false);
  const menuButton = useRef<HTMLButtonElement>(null);
  const drawer = useRef<HTMLElement>(null);
  const closeNavigation = useCallback(() => {
    setNavigationOpen(false);
    window.requestAnimationFrame(() => menuButton.current?.focus());
  }, []);
  useEffect(() => {
    const query = window.matchMedia("(max-width: 760px)");
    const update = () => {
      setCompactNavigation(query.matches);
      if (!query.matches) setNavigationOpen(false);
    };
    update();
    query.addEventListener("change", update);
    return () => query.removeEventListener("change", update);
  }, []);
  useEffect(() => {
    if (!navigationOpen || !compactNavigation) return;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    drawer.current?.querySelector<HTMLElement>("a,button")?.focus();
    const containFocus = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        closeNavigation();
        return;
      }
      if (event.key !== "Tab") return;
      const focusable = Array.from(
        drawer.current?.querySelectorAll<HTMLElement>(
          "a[href],button:not([disabled]),[tabindex]:not([tabindex='-1'])",
        ) ?? [],
      ).filter((element) => !element.hasAttribute("inert"));
      if (focusable.length === 0) {
        event.preventDefault();
        return;
      }
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };
    document.addEventListener("keydown", containFocus);
    return () => {
      document.body.style.overflow = previousOverflow;
      document.removeEventListener("keydown", containFocus);
    };
  }, [closeNavigation, compactNavigation, navigationOpen]);
  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">
        Skip to main content
      </a>
      <header className="mobile-bar" inert={navigationOpen ? true : undefined}>
        <Link className="brand" href="/">
          Ledger<span>Sync</span>
        </Link>
        <span className="mobile-context">
          {tenantLabel}
          <small>{environmentLabel}</small>
        </span>
        <button
          ref={menuButton}
          className="menu-button"
          type="button"
          aria-expanded={navigationOpen}
          aria-controls="workspace-navigation"
          onClick={() => setNavigationOpen(true)}
        >
          <List aria-hidden="true" />
          <span>Menu</span>
        </button>
      </header>
      {navigationOpen && (
        <button
          className="nav-backdrop"
          type="button"
          aria-label="Close navigation"
          onClick={closeNavigation}
        />
      )}
      <aside
        ref={drawer}
        id="workspace-navigation"
        className={navigationOpen ? "side-nav open" : "side-nav"}
        aria-label="LedgerSync workspace"
        aria-hidden={compactNavigation && !navigationOpen ? true : undefined}
        inert={compactNavigation && !navigationOpen ? true : undefined}
        role={compactNavigation && navigationOpen ? "dialog" : undefined}
        aria-modal={compactNavigation && navigationOpen ? true : undefined}
      >
        <button
          className="drawer-close"
          type="button"
          aria-label="Close navigation"
          onClick={closeNavigation}
        >
          <X aria-hidden="true" />
        </button>
        <Link className="brand" href="/" aria-label="LedgerSync overview">
          Ledger<span>Sync</span>
        </Link>

        <p className="nav-kicker">Operator workspace</p>
        <p className="context-label">Environment</p>
        <div
          className="workspace-card"
          aria-label={`${tenantLabel}, ${tenantMeta}`}
        >
          <div className="workspace-mark" aria-hidden="true">
            <Bank weight="fill" />
          </div>
          <span>
            <strong>{tenantLabel}</strong>
            <small>{tenantMeta}</small>
          </span>
        </div>

        <nav className="primary-nav" aria-label="Primary navigation">
          {navigation.map((group) => (
            <div className="nav-group" key={group.label}>
              <p className="nav-section-label">{group.label}</p>
              {group.label === "Local tools" && preview && (
                <Link
                  href="/?guide=1"
                  onClick={() => setNavigationOpen(false)}
                  className="nav-item"
                >
                  <Compass aria-hidden="true" />
                  <span>Local guide</span>
                </Link>
              )}
              {group.items.map((item) => {
                const Icon = item.icon;
                return (
                  <Link
                    key={item.section}
                    href={item.href}
                    prefetch={group.label === "Local tools" ? false : undefined}
                    onClick={() => setNavigationOpen(false)}
                    className={
                      section === item.section ? "nav-item active" : "nav-item"
                    }
                    aria-current={section === item.section ? "page" : undefined}
                  >
                    <Icon
                      weight={section === item.section ? "fill" : "regular"}
                      aria-hidden="true"
                    />
                    <span>{item.label}</span>
                  </Link>
                );
              })}
            </div>
          ))}
        </nav>

        <div className="nav-footer">
          <div className="environment-row">
            <CheckCircle weight="fill" aria-hidden="true" />
            <span>{environmentLabel}</span>
            {preview && <span className="preview-chip">Preview</span>}
          </div>
          <div className="operator-row">
            <UserCircle weight="fill" aria-hidden="true" />
            <span>
              <strong>{operatorLabel}</strong>
              <small>{operatorMeta}</small>
            </span>
            {onSignOut && (
              <button
                className="icon-button"
                type="button"
                onClick={onSignOut}
                aria-label="Sign out"
              >
                <SignOut aria-hidden="true" />
              </button>
            )}
          </div>
        </div>
      </aside>
      <main
        id="main-content"
        className={`console-main section-${section}`}
        inert={navigationOpen ? true : undefined}
      >
        {children}
      </main>
    </div>
  );
}

export function ConsoleFooter({ pending = false }: { pending?: boolean } = {}) {
  return (
    <footer
      className={`console-footer${pending ? " is-pending" : ""}`}
      aria-hidden={pending || undefined}
    >
      <span>
        PostgreSQL alone supplies customer-visible balances. Redis is
        disposable.
      </span>
      <span>All times shown in UTC.</span>
      <span>© 2026 LedgerSync, Inc.</span>
    </footer>
  );
}
