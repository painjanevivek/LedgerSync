import { formatCurrencyMinorUnits, formatMinorUnits } from "@/lib/money";

export function Money({ currency, minorUnits, className, groupThousands, showCurrencyCode, localized = false }: Readonly<{ currency: string; minorUnits: string; className?: string; groupThousands?: boolean; showCurrencyCode?: boolean; localized?: boolean }>) {
  const formatted = localized ? formatCurrencyMinorUnits(currency, minorUnits) : formatMinorUnits(currency, minorUnits, { groupThousands, showCurrencyCode });
  return <span className={className} data-currency={currency.trim().toUpperCase()} data-minor-units={minorUnits}>{formatted}</span>;
}
