import { formatMinorUnits } from "@/lib/money";

export function Money({ currency, minorUnits, className, groupThousands, showCurrencyCode }: Readonly<{ currency: string; minorUnits: string; className?: string; groupThousands?: boolean; showCurrencyCode?: boolean }>) {
  const formatted = formatMinorUnits(currency, minorUnits, { groupThousands, showCurrencyCode });
  return <span className={className} data-currency={currency.trim().toUpperCase()} data-minor-units={minorUnits}>{formatted}</span>;
}
