"use client";

import {
  Archive,
  ArrowsCounterClockwise,
  ArrowsLeftRight,
  Bank,
  Broadcast,
  ChartDonut,
  CheckCircle,
  ClipboardText,
  Code,
  MagnifyingGlass,
  Pulse,
  Receipt,
  ShieldCheck,
} from "@phosphor-icons/react";
import Link from "next/link";
import { useState } from "react";

import type { ConsoleSection } from "@/features/console/ConsoleShell";
import {
  canOpenApprovalInbox,
  canOpenEventsAndWebhooks,
  canSearchInvestigations,
  type ConsoleCapabilities,
} from "@/features/console/capabilities";

type NavigationItem = Readonly<{
  section: ConsoleSection;
  label: string;
  href: string;
  icon: typeof Bank;
  visible: (capabilities: ConsoleCapabilities) => boolean;
  experience: "both" | "expert";
}>;

type NavigationGroupDefinition = Readonly<{
  id: "work" | "investigate" | "platform";
  label: string;
  priority: "primary" | "secondary";
  items: ReadonlyArray<NavigationItem>;
}>;

const always = () => true;

export const navigation: ReadonlyArray<NavigationGroupDefinition> = [
  {
    id: "work",
    label: "Workspace",
    priority: "primary",
    items: [
      {
        section: "overview" as const,
        label: "Home",
        href: "/",
        icon: ChartDonut,
        visible: always,
        experience: "both",
      },
      {
        section: "accounts" as const,
        label: "Accounts",
        href: "/accounts",
        icon: Bank,
        visible: (capabilities) => capabilities.accountsRead,
        experience: "both",
      },
      {
        section: "funding" as const,
        label: "Add money",
        href: "/funding",
        icon: Receipt,
        visible: (capabilities) => capabilities.fundingRead,
        experience: "both",
      },
      {
        section: "transfers" as const,
        label: "Transfers",
        href: "/transfers",
        icon: ArrowsLeftRight,
        visible: (capabilities) => capabilities.transfersRead,
        experience: "both",
      },
      {
        section: "tasks" as const,
        label: "Tasks",
        href: "/tasks",
        icon: ClipboardText,
        visible: (capabilities) => canOpenApprovalInbox(capabilities) || capabilities.fundingRead || capabilities.correctionsRead || capabilities.reconciliationRead || capabilities.transfersRead || capabilities.eventsRead || capabilities.webhooksRead || capabilities.recoveryRead,
        experience: "both",
      },
      {
        section: "approvals" as const,
        label: "Approvals",
        href: "/approvals",
        icon: CheckCircle,
        visible: canOpenApprovalInbox,
        experience: "expert",
      },
    ],
  },
  {
    id: "investigate",
    label: "Review & investigate",
    priority: "secondary",
    items: [
      {
        section: "search" as const,
        label: "Search records",
        href: "/search",
        icon: MagnifyingGlass,
        visible: canSearchInvestigations,
        experience: "expert",
      },
      {
        section: "corrections" as const,
        label: "Correct records",
        href: "/corrections",
        icon: ArrowsCounterClockwise,
        visible: (capabilities) => capabilities.correctionsRead,
        experience: "expert",
      },
      {
        section: "reconciliation" as const,
        label: "Balance checks",
        href: "/reconciliation",
        icon: ShieldCheck,
        visible: (capabilities) => capabilities.reconciliationRead,
        experience: "expert",
      },
      {
        section: "events" as const,
        label: "Delivery activity",
        href: "/events",
        icon: Broadcast,
        visible: canOpenEventsAndWebhooks,
        experience: "expert",
      },
    ],
  },
  {
    id: "platform",
    label: "System tools",
    priority: "secondary",
    items: [
      {
        section: "developer" as const,
        label: "Developer",
        href: "/developer",
        icon: Code,
        visible: (capabilities) => capabilities.developerRead,
        experience: "expert",
      },
      {
        section: "recovery" as const,
        label: "Data recovery",
        href: "/recovery",
        icon: Archive,
        visible: (capabilities) => capabilities.recoveryRead,
        experience: "expert",
      },
      {
        section: "local-status" as const,
        label: "System status",
        href: "/local-status",
        icon: Pulse,
        visible: (capabilities) => capabilities.localDiagnosticsRead,
        experience: "expert",
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

export function NavigationGroup({
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
