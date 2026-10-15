"use client";

import Link from "next/link";
import type { ReactNode } from "react";
import type { TransferDetail } from "@/features/accounts/types";
import { transferStatusPresentation } from "@/features/console/presentation";
import { EvidenceFreshness } from "@/ui/display/Evidence";
import { Money } from "@/ui/display/Money";
import { PageHeader } from "@/ui/display/PageHeader";
import { StatusSummary } from "@/ui/presentation/StatusSummary";
import { TechnicalDetails } from "@/ui/presentation/TechnicalDetails";
import { Timestamp } from "@/ui/display/Timestamp";

/** The posted outcome is immutable. Delivery and later reads remain separate. */
export function SimpleTransferReceipt({detail,verifiedAt,historical,returnTo,evidence}:Readonly<{detail:TransferDetail;verifiedAt?:string;historical:boolean;returnTo:string;evidence:ReactNode}>) {
  const presentation=transferStatusPresentation(detail.financial_status);
  return <>
    <PageHeader eyebrow="Transfers" title="Transfer result" description="Check whether the money moved and choose the next safe step." />
    <StatusSummary status={presentation} action={<Link className="button secondary" href={returnTo}>{returnTo.startsWith("/transfers") ? "Back to transfers" : "Back to previous view"}</Link>} />
    {verifiedAt&&<EvidenceFreshness state={historical?"historical":"current"} verifiedAt={verifiedAt} label="Transfer record" />}
    <section className="simple-transfer-result" aria-label="Exact transfer result">
      <p>Exact amount</p><strong><Money currency={detail.currency} minorUnits={detail.amount_minor}/></strong>
      <p>Created <Timestamp value={detail.created_at}/></p>
      <p>Delivery status: {detail.delivery_status.replaceAll("_"," ")}. Delivery does not change the recorded financial outcome.</p>
      <TechnicalDetails summary="View transfer details" attention={presentation.attention||historical}>{evidence}</TechnicalDetails>
    </section>
  </>;
}
