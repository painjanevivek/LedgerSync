"use client";

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
        href: "/",        visible: always,
        experience: "both",
      },
      {
        section: "accounts" as const,
        label: "Accounts",
        href: "/accounts",        visible: (capabilities) => capabilities.accountsRead,
        experience: "both",
      },
      {
        section: "funding" as const,
        label: "Add money",
        href: "/funding",        visible: (capabilities) => capabilities.fundingRead,
        experience: "both",
      },
      {
        section: "transfers" as const,
        label: "Transfers",
        href: "/transfers",        visible: (capabilities) => capabilities.transfersRead,
        experience: "both",
      },
      {
        section: "tasks" as const,
        label: "Tasks",
        href: "/tasks",        visible: (capabilities) => canOpenApprovalInbox(capabilities) || capabilities.fundingRead || capabilities.correctionsRead || capabilities.reconciliationRead || capabilities.transfersRead || capabilities.eventsRead || capabilities.webhooksRead || capabilities.recoveryRead,
        experience: "both",
      },
      {
        section: "approvals" as const,
        label: "Approvals",
        href: "/approvals",        visible: canOpenApprovalInbox,
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
        href: "/search",        visible: canSearchInvestigations,
        experience: "expert",
      },
      {
        section: "corrections" as const,
        label: "Correct records",
        href: "/corrections",        visible: (capabilities) => capabilities.correctionsRead,
        experience: "expert",
      },
      {
        section: "reconciliation" as const,
        label: "Balance checks",
        href: "/reconciliation",        visible: (capabilities) => capabilities.reconciliationRead,
        experience: "expert",
      },
      {
        section: "events" as const,
        label: "Delivery activity",
        href: "/events",        visible: canOpenEventsAndWebhooks,
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
        href: "/developer",        visible: (capabilities) => capabilities.developerRead,
        experience: "expert",
      },
      {
        section: "recovery" as const,
        label: "Data recovery",
        href: "/recovery",        visible: (capabilities) => capabilities.recoveryRead,
        experience: "expert",
      },
      {
        section: "local-status" as const,
        label: "System status",
        href: "/local-status",        visible: (capabilities) => capabilities.localDiagnosticsRead,
        experience: "expert",
      },
    ],
  },
];
