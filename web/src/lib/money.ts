const exponents: Readonly<Record<string, number>> = { EUR: 2, GBP: 2, INR: 2, JPY: 0, KWD: 3, USD: 2 };

export function minorUnitsFromDecimal(currency: string, input: string): string {
  const code = currency.trim().toUpperCase();
  const exponent = exponents[code];
  const value = input.trim();
  if (exponent === undefined || !/^\d+(?:\.\d+)?$/.test(value)) throw new Error("Enter a positive amount using the account currency precision.");
  const [whole, fractional = ""] = value.split(".");
  if (fractional.length > exponent) throw new Error(`${code} supports at most ${exponent} decimal places.`);
  const normalized = `${whole.replace(/^0+(?=\d)/, "")}${fractional.padEnd(exponent, "0")}`.replace(/^0+/, "") || "0";
  if (normalized === "0") throw new Error("Amount must be greater than zero.");
  return normalized;
}

export function formatMinorUnits(currency: string, minorUnits: string): string {
  const code = currency.trim().toUpperCase();
  const exponent = exponents[code];
  const value = minorUnits.trim();
  if (exponent === undefined || !/^\d+$/.test(value)) return "Unavailable";
  if (exponent === 0) return `${code} ${value}`;
  const padded = value.padStart(exponent + 1, "0");
  const whole = padded.slice(0, -exponent).replace(/^0+(?=\d)/, "");
  const fractional = padded.slice(-exponent);
  return `${code} ${whole}.${fractional}`;
}
