const exponents: Readonly<Record<string, number>> = { EUR: 2, GBP: 2, INR: 2, JPY: 0, KWD: 3, USD: 2 };
const maximumMinorUnits = "9223372036854775807";

function exceedsSigned64BitMinorUnits(value: string): boolean {
  return value.length > maximumMinorUnits.length || (value.length === maximumMinorUnits.length && value > maximumMinorUnits);
}

export function minorUnitsFromDecimal(currency: string, input: string): string {
  const code = currency.trim().toUpperCase();
  const exponent = exponents[code];
  const value = input.trim();
  if (exponent === undefined || !/^\d+(?:\.\d+)?$/.test(value)) throw new Error("Enter a positive amount using the account currency precision.");
  const [whole, fractional = ""] = value.split(".");
  if (fractional.length > exponent) throw new Error(`${code} supports at most ${exponent} decimal places.`);
  const normalized = `${whole.replace(/^0+(?=\d)/, "")}${fractional.padEnd(exponent, "0")}`.replace(/^0+/, "") || "0";
  if (normalized === "0") throw new Error("Amount must be greater than zero.");
  if (exceedsSigned64BitMinorUnits(normalized)) throw new Error("Amount exceeds the supported exact minor-unit range.");
  return normalized;
}

export function formatMinorUnits(
  currency: string,
  minorUnits: string,
  options: Readonly<{ groupThousands?: boolean; showCurrencyCode?: boolean }> = {},
): string {
  const code = currency.trim().toUpperCase();
  const exponent = exponents[code];
  const value = minorUnits.trim();
  if (exponent === undefined || !/^\d+$/.test(value) || exceedsSigned64BitMinorUnits(value)) return "Unavailable";
  let whole: string;
  let fractional: string | undefined;
  if (exponent === 0) {
    whole = value;
  } else {
    const padded = value.padStart(exponent + 1, "0");
    whole = padded.slice(0, -exponent).replace(/^0+(?=\d)/, "");
    fractional = padded.slice(-exponent);
  }
  if (options.groupThousands) whole = whole.replace(/\B(?=(\d{3})+(?!\d))/g, ",");
  const amount = fractional === undefined ? whole : `${whole}.${fractional}`;
  return options.showCurrencyCode === false ? amount : `${code} ${amount}`;
}

/** Locale grouping without converting any financial value through a floating-point number. */
export function formatCurrencyMinorUnits(currency: string, minorUnits: string, locale = currency === "INR" ? "en-IN" : "en-US"): string {
  const canonical = formatMinorUnits(currency, minorUnits);
  if (canonical === "Unavailable") return canonical;
  const code = currency.trim().toUpperCase();
  const [whole, fraction] = canonical.slice(code.length + 1).split(".");
  const formatter = new Intl.NumberFormat(locale, { style: "currency", currency: code, currencyDisplay: code === "USD" ? "code" : "symbol", minimumFractionDigits: exponents[code], maximumFractionDigits: exponents[code] });
  return formatter.formatToParts(BigInt(whole)).map(part => part.type === "fraction" ? fraction : part.value).join("");
}
