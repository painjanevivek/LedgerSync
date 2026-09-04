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
  Receipt,
  X,
  BookOpenText,
  MagnifyingGlass,
} from "@phosphor-icons/react";
import Link from "next/link";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";

import { useConsoleSession } from "@/features/console/ConsoleSessionBoundary";
import {
  canOpenApprovalInbox,
  canOpenEventsAndWebhooks,
  canSearchInvestigations,
  deriveConsoleCapabilities,
  type ConsoleCapabilities,
} from "@/features/console/capabilities";
import { COMPACT_NAVIGATION_MEDIA_QUERY } from "@/lib/responsive";

export type ConsoleSection =
  | "overview"
  | "accounts"
  | "funding"
  | "transfers"
  | "approvals"
  | "corrections"
  | "reconciliation"
  | "local-status"
  | "events"
  | "developer"
  | "recovery"
  | "guide"
  | "search";

type Props = Readonly<{
  section: ConsoleSection;
  children: ReactNode;
  tenantLabel: string;
  tenantMeta: string;
  environmentLabel: string;
  operatorLabel: string;
  operatorMeta: string;
}>;

type NavigationItem = Readonly<{
  section: ConsoleSection;
  label: string;
  href: string;
  icon: typeof Bank;
  visible: (capabilities: ConsoleCapabilities) => boolean;
}>;

type NavigationGroupDefinition = Readonly<{
  id: "work" | "investigate" | "platform" | "environment";
  label: string;
  priority: "primary" | "secondary";
  items: ReadonlyArray<NavigationItem>;
}>;

const always = () => true;

const navigation: ReadonlyArray<NavigationGroupDefinition> = [
  {
    id: "work",
    label: "Work",
    priority: "primary",
    items: [
      {
        section: "overview" as const,
        label: "Overview",
        href: "/",
        icon: ChartDonut,
        visible: always,
      },
      {
        section: "accounts" as const,
        label: "Accounts",
        href: "/accounts",
        icon: Bank,
        visible: (capabilities) => capabilities.accountsRead,
      },
      {
        section: "funding" as const,
        label: "Funding records",
        href: "/funding",
        icon: Receipt,
        visible: (capabilities) => capabilities.fundingRead,
      },
      {
        section: "transfers" as const,
        label: "Transfers",
        href: "/transfers",
        icon: ArrowsLeftRight,
        visible: (capabilities) => capabilities.transfersRead,
      },
      {
        section: "approvals" as const,
        label: "Approvals",
        href: "/approvals",
        icon: CheckCircle,
        visible: canOpenApprovalInbox,
      },
    ],
  },
  {
    id: "investigate",
    label: "Investigate",
    priority: "secondary",
    items: [
      {
        section: "search" as const,
        label: "Search records",
        href: "/search",
        icon: MagnifyingGlass,
        visible: canSearchInvestigations,
      },
      {
        section: "corrections" as const,
        label: "Corrections",
        href: "/corrections",
        icon: ArrowsCounterClockwise,
        visible: (capabilities) => capabilities.correctionsRead,
      },
      {
        section: "reconciliation" as const,
        label: "Reconciliation",
        href: "/reconciliation",
        icon: ShieldCheck,
        visible: (capabilities) => capabilities.reconciliationRead,
      },
      {
        section: "events" as const,
        label: "Events & webhooks",
        href: "/events",
        icon: Broadcast,
        visible: canOpenEventsAndWebhooks,
      },
    ],
  },
  {
    id: "platform",
    label: "Platform",
    priority: "secondary",
    items: [
      {
        section: "developer" as const,
        label: "Developer",
        href: "/developer",
        icon: Code,
        visible: (capabilities) => capabilities.developerRead,
      },
      {
        section: "recovery" as const,
        label: "Recovery",
        href: "/recovery",
        icon: Archive,
        visible: (capabilities) => capabilities.recoveryRead,
      },
    ],
  },
  {
    id: "environment",
    label: "Environment",
    priority: "secondary",
    items: [
      {
        section: "local-status" as const,
        label: "Local status",
        href: "/local-status",
        icon: Pulse,
        visible: (capabilities) => capabilities.localDiagnosticsRead,
      },
    ],
  },
];

function NavigationLinks({
  items,
  section,
  closeNavigation,
}: Readonly<{
  items: ReadonlyArray<NavigationItem>;
  section: ConsoleSection;
  closeNavigation: () => void;
}>) {
  return items.map((item) => {
    const Icon = item.icon;
    return (
      <Link
        key={item.section}
        href={item.href}
        prefetch={false}
        onClick={closeNavigation}
        className={section === item.section ? "nav-item active" : "nav-item"}
        aria-current={section === item.section ? "page" : undefined}
      >
        <Icon
          weight={section === item.section ? "fill" : "regular"}
          aria-hidden="true"
        />
        <span>{item.label}</span>
      </Link>
    );
  });
}

function NavigationGroup({
  definition,
  items,
  section,
  closeNavigation,
}: Readonly<{
  definition: NavigationGroupDefinition;
  items: ReadonlyArray<NavigationItem>;
  section: ConsoleSection;
  closeNavigation: () => void;
}>) {
  const active = items.some((item) => item.section === section);
  const [open, setOpen] = useState(definition.priority === "primary" || active);

  if (definition.priority === "primary") {
    return (
      <div className="nav-group nav-group-primary">
        <p className="nav-section-label">{definition.label}</p>
        <NavigationLinks
          items={items}
          section={section}
          closeNavigation={closeNavigation}
        />
      </div>
    );
  }

  return (
    <details
      className="nav-group nav-group-secondary"
      open={active || open}
      onToggle={(event) => setOpen(event.currentTarget.open)}
    >
      <summary>
        <span>{definition.label}</span>
        <small>{items.length}</small>
      </summary>
      <div className="nav-group-links">
        <NavigationLinks
          items={items}
          section={section}
          closeNavigation={closeNavigation}
        />
      </div>
    </details>
  );
}

export function ConsoleShell({
  section,
  children,
  tenantLabel,
  tenantMeta,
  environmentLabel,
  operatorLabel,
  operatorMeta,
}: Props) {
  const { session, online, signOut, signOutPending, signOutError } =
    useConsoleSession();
  const capabilities = useMemo(
    () => deriveConsoleCapabilities(session),
    [session],
  );
  const [navigationOpen, setNavigationOpen] = useState(false);
  const [compactNavigation, setCompactNavigation] = useState(false);
  const menuButton = useRef<HTMLButtonElement>(null);
  const drawer = useRef<HTMLElement>(null);
  const closeNavigation = useCallback(() => {
    setNavigationOpen(false);
    window.requestAnimationFrame(() => menuButton.current?.focus());
  }, []);
  useEffect(() => {
    const query = window.matchMedia(COMPACT_NAVIGATION_MEDIA_QUERY);
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
          {navigation.map((group) => {
            const items = group.items.filter((item) => item.visible(capabilities));
            if (items.length === 0) return null;
            return (
              <NavigationGroup
                key={group.id}
                definition={group}
                items={items}
                section={section}
                closeNavigation={() => setNavigationOpen(false)}
              />
            );
          })}
        </nav>

        <div className="nav-footer">
          <div className="environment-row">
            <CheckCircle weight="fill" aria-hidden="true" />
            <span>{environmentLabel}</span>
          </div>
          <div className="operator-row">
            <UserCircle weight="fill" aria-hidden="true" />
            <span>
              <strong>{operatorLabel}</strong>
              <small>{operatorMeta}</small>
            </span>
            {session && (
              <button
                className="icon-button"
                type="button"
                onClick={() => void signOut()}
                disabled={signOutPending || !online}
                aria-label={signOutPending ? "Signing out" : "Sign out"}
                aria-describedby={signOutError ? "sign-out-error" : undefined}
              >
                <SignOut aria-hidden="true" />
              </button>
            )}
          </div>
          {signOutError && (
            <p id="sign-out-error" className="session-action-error" role="alert">
              {signOutError}
            </p>
          )}
        </div>
      </aside>
      <div className="workspace-column">
        <nav
          className="workspace-topbar"
          aria-label="Workspace utilities"
          inert={navigationOpen ? true : undefined}
        >
          <span>LedgerSync workspace</span>
          <Link
            href="/guide"
            className={section === "guide" ? "topbar-link active" : "topbar-link"}
            aria-current={section === "guide" ? "page" : undefined}
          >
            <BookOpenText aria-hidden="true" />
            Guide
          </Link>
        </nav>
        <main
          id="main-content"
          className={`console-main section-${section}`}
          inert={navigationOpen ? true : undefined}
        >
          {children}
        </main>
      </div>
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
