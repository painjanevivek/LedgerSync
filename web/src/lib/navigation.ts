export function safeInternalReturnPath(value: string | string[] | undefined) {
  if (
    typeof value !== "string" ||
    value.length > 2_048 ||
    !value.startsWith("/") ||
    value.startsWith("//") ||
    value.includes("\\")
  ) return undefined;
  try {
    const parsed = new URL(value, "https://ledgersync.invalid");
    if (parsed.origin !== "https://ledgersync.invalid") return undefined;
    return `${parsed.pathname}${parsed.search}${parsed.hash}`;
  } catch {
    return undefined;
  }
}
