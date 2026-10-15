"use client";

import {
  ArrowsLeftRight,
  ArrowsCounterClockwise,
  Bank,
  CaretDoubleLeft,
  CaretDoubleRight,
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
  useSyncExternalStore,
  type ReactNode,
} from "react";

import { useExperienceMode } from "@/features/console/ExperienceModeBoundary";
import { ExperienceModeSwitch } from "@/ui/presentation/ExperienceModeSwitch";
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
  | "tasks"
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
  id: "work" | "investigate" | "operations" | "developer";
  label: string;
  priority: "primary" | "secondary";
  items: ReadonlyArray<NavigationItem>;
}>;

const always = () => true;
const SIDEBAR_STORAGE_KEY = "ledgersync.sidebar.collapsed";
const SIDEBAR_CHANGE_EVENT = "ledgersync:sidebar-change";
let sidebarPreferenceFallback = false;

function readSidebarPreference() {
  try {
    return window.localStorage.getItem(SIDEBAR_STORAGE_KEY) === "true";
  } catch {
    return sidebarPreferenceFallback;
  }
}

function subscribeSidebarPreference(notify: () => void) {
  window.addEventListener("storage", notify);
  window.addEventListener(SIDEBAR_CHANGE_EVENT, notify);
  return () => {
    window.removeEventListener("storage", notify);
    window.removeEventListener(SIDEBAR_CHANGE_EVENT, notify);
  };
}

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
        section: "tasks" as const,
        label: "Tasks",
        href: "/tasks",
        icon: CheckCircle,
        visible: always,
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
        label: "Internal reconciliation",
        href: "/reconciliation",
        icon: ShieldCheck,
        visible: (capabilities) => capabilities.reconciliationRead,
      },
    ],
  },
  {
    id: "operations",
    label: "Operations",
    priority: "secondary",
    items: [
      {
        section: "local-status" as const,
        label: "Local status",
        href: "/local-status",
        icon: Pulse,
        visible: (capabilities) => capabilities.localDiagnosticsRead,
      },
      {
        section: "events" as const,
        label: "Events & webhooks",
        href: "/events",
        icon: Broadcast,
        visible: canOpenEventsAndWebhooks,
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
    id: "developer",
    label: "Developer",
    priority: "secondary",
    items: [
      {
        section: "developer" as const,
        label: "Developer",
        href: "/developer",
        icon: Code,
        visible: (capabilities) => capabilities.developerRead,
      },
    ],
  },
];

function NavigationLinks({
  items,
  section,
  closeNavigation,
  collapsed,
}: Readonly<{
  items: ReadonlyArray<NavigationItem>;
  section: ConsoleSection;
  closeNavigation: () => void;
  collapsed: boolean;
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
        aria-label={collapsed ? item.label : undefined}
        title={collapsed ? item.label : undefined}
      >
        <Icon
          weight={section === item.section ? "fill" : "regular"}
          aria-hidden="true"
        />
        <span className="nav-item-label">{item.label}</span>
      </Link>
    );
  });
}

function NavigationGroup({
  definition,
  items,
  section,
  closeNavigation,
  collapsed,
}: Readonly<{
  definition: NavigationGroupDefinition;
  items: ReadonlyArray<NavigationItem>;
  section: ConsoleSection;
  closeNavigation: () => void;
  collapsed: boolean;
}>) {
  const active = items.some((item) => item.section === section);
  const [open, setOpen] = useState(false);

  if (collapsed) {
    return (
      <div className="nav-group nav-group-compact" role="group" aria-label={definition.label}>
        <NavigationLinks items={items} section={section} closeNavigation={closeNavigation} collapsed />
      </div>
    );
  }

  if (definition.priority === "primary") {
    return (
      <div className="nav-group nav-group-primary">
        <p className="nav-section-label">{definition.label}</p>
        <NavigationLinks
          items={items}
          section={section}
          closeNavigation={closeNavigation}
          collapsed={false}
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
      </summary>
      <div className="nav-group-links">
        <NavigationLinks
          items={items}
          section={section}
          closeNavigation={closeNavigation}
          collapsed={false}
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
  const { mode } = useExperienceMode();
  const [navigationOpen, setNavigationOpen] = useState(false);
  const [compactNavigation, setCompactNavigation] = useState(false);
  const sidebarCollapsed = useSyncExternalStore(
    subscribeSidebarPreference,
    readSidebarPreference,
    () => false,
  );
  const menuButton = useRef<HTMLButtonElement>(null);
  const drawer = useRef<HTMLElement>(null);
  const closeNavigation = useCallback(() => {
    setNavigationOpen(false);
    window.requestAnimationFrame(() => menuButton.current?.focus());
  }, []);
  const toggleSidebar = () => {
    const next = !sidebarCollapsed;
    sidebarPreferenceFallback = next;
    try {
      window.localStorage.setItem(SIDEBAR_STORAGE_KEY, String(next));
    } catch {
      // The toggle still works for this page when local storage is unavailable.
    }
    window.dispatchEvent(new Event(SIDEBAR_CHANGE_EVENT));
  };
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
          "a[href],button:not([disabled]),summary,[tabindex]:not([tabindex='-1'])",
        ) ?? [],
      ).filter((element) => !element.closest("[inert]") && element.checkVisibility({ visibilityProperty: true }));
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
    <div className={`app-shell${sidebarCollapsed ? " sidebar-collapsed" : ""}`}>
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
        className={`side-nav${navigationOpen ? " open" : ""}${sidebarCollapsed && !compactNavigation ? " collapsed" : ""}`}
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
        <div className="sidebar-heading">
          <Link className="brand" href="/" aria-label="LedgerSync overview">
            Ledger<span>Sync</span>
          </Link>
          <button
            className="sidebar-toggle"
            type="button"
            onClick={toggleSidebar}
            aria-label={sidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"}
            aria-pressed={sidebarCollapsed}
            title={sidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"}
          >
            {sidebarCollapsed ? <CaretDoubleRight aria-hidden="true" /> : <CaretDoubleLeft aria-hidden="true" />}
          </button>
        </div>

        <p className="nav-kicker">Financial operations</p>

        <nav className="primary-nav" aria-label="Primary navigation">
          {navigation.map((group) => {
            const items = group.items.filter((item) => item.visible(capabilities) && (mode === "expert" || group.priority === "primary" || item.section === section));
            if (items.length === 0) return null;
            return (
              <NavigationGroup
                key={group.id}
                definition={group}
                items={items}
                section={section}
                closeNavigation={() => setNavigationOpen(false)}
                collapsed={sidebarCollapsed && !compactNavigation}
              />
            );
          })}
        </nav>

      </aside>
      <div className="workspace-column">
        <nav
          className="workspace-topbar"
          aria-label="Workspace utilities"
          inert={navigationOpen ? true : undefined}
        >
          <div className="workspace-topbar-context" aria-label={`Tenant ${tenantLabel}; environment ${environmentLabel}`}>
            <strong>{tenantLabel}</strong>
            <span className="tenant-meta">{tenantMeta}</span>
            <span className="environment-chip">{environmentLabel}</span>
          </div>
          <div className="workspace-topbar-actions">
            <ExperienceModeSwitch />
            {canSearchInvestigations(capabilities) && (
              <Link href="/search" className="topbar-link">
                <MagnifyingGlass aria-hidden="true" />
                Search records
              </Link>
            )}
            <Link
              href="/guide"
              className={section === "guide" ? "topbar-link active" : "topbar-link"}
              aria-current={section === "guide" ? "page" : undefined}
            >
              <BookOpenText aria-hidden="true" />
              Guide
            </Link>
            <span className="topbar-utc-label">All times UTC</span>
            <div className="topbar-operator">
              <UserCircle weight="fill" aria-hidden="true" />
              <span>
                <strong>{operatorLabel}</strong>
                <small>{operatorMeta}</small>
              </span>
            </div>
            {session && (
              <button
                className="topbar-signout"
                type="button"
                onClick={() => void signOut()}
                disabled={signOutPending || !online}
                aria-label={signOutPending ? "Signing out" : "Sign out"}
                aria-describedby={signOutError ? "topbar-sign-out-error" : undefined}
              >
                <SignOut aria-hidden="true" />
              </button>
            )}
          </div>
          {signOutError && <p id="topbar-sign-out-error" className="session-action-error topbar-signout-error" role="alert">{signOutError}</p>}
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
      <span>LedgerSync · Internal ledger</span>
      <Link href="/guide">Help and guide</Link>
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
