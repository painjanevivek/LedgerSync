"use client";
import { ArrowLeft, Info } from "@phosphor-icons/react";
import Link from "next/link";
import type { ReactNode } from "react";
import { useFocusedWorkspace } from "@/features/console/WorkspaceLayoutContext";
import { CommandSteps, type CommandStage } from "./CommandSteps";

export function CommandFrame({ title, description, stage, children, help, returnTo, returnLabel }: Readonly<{ title: string; description: string; stage: CommandStage; children: ReactNode; help?: ReactNode; returnTo: string; returnLabel: string }>) {
  useFocusedWorkspace(title, returnTo, returnLabel);
  return <div className="command-frame"><div className="command-primary"><Link className="command-back" href={returnTo}><ArrowLeft aria-hidden="true" />{returnLabel}</Link><h1>{title}</h1><p className="command-description">{description}</p><CommandSteps stage={stage} />{children}</div><aside className="command-help" aria-label="Contextual guidance"><h2>{stage === "result" ? "Your next step" : "Before you confirm"}</h2>{help ?? <><p>Check the details and the financial effect before confirming.</p><p>Extra record information is available in the details below.</p></>}<Link href="/guide">Need help?</Link><div className="command-help-note"><Info aria-hidden="true" /><p>If we cannot confirm the result, return to the original request before trying again.</p></div></aside></div>;
}
