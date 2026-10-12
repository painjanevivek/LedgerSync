import type { ReactNode } from "react";

export function FinancialSummary({ label, amount, explanation, action }: Readonly<{ label: string; amount: ReactNode; explanation: string; action?: ReactNode }>) {
  return <article className="financial-summary"><p>{label}</p><strong>{amount}</strong><span>{explanation}</span>{action}</article>;
}
