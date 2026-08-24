import { readFile } from "node:fs/promises";

function assertUsableToken(raw: string): string {
  const token = raw.trim();
  if (token.length < 16 || token.length > 16_384 || /\s/.test(token)) {
    throw new Error("private API workload credential is malformed");
  }
  const parts = token.split(".");
  if (parts.length === 3) {
    try {
      const claims = JSON.parse(Buffer.from(parts[1], "base64url").toString("utf8")) as { exp?: unknown };
      if (typeof claims.exp !== "number" || !Number.isSafeInteger(claims.exp) || claims.exp <= Math.floor(Date.now() / 1000) + 5) {
        throw new Error("private API workload credential is expired or near expiry");
      }
    } catch (error) {
      throw error instanceof Error ? error : new Error("private API workload credential is malformed");
    }
  }
  return token;
}

// Production expects a managed workload agent to atomically refresh this
// file. Reading per request makes rotation visible without restarting the BFF.
export async function getPrivateAPIWorkloadCredential(): Promise<string> {
  const tokenFile = process.env.LEDGERSYNC_PRIVATE_API_TOKEN_FILE?.trim();
  if (tokenFile) return assertUsableToken(await readFile(tokenFile, "utf8"));

  const staticToken = process.env.LEDGERSYNC_PRIVATE_API_TOKEN?.trim();
  if (process.env.LEDGERSYNC_ENV === "production" || process.env.LEDGERSYNC_DEPLOYMENT_ENV === "production") {
    throw new Error("production requires LEDGERSYNC_PRIVATE_API_TOKEN_FILE managed renewal");
  }
  if (!staticToken) throw new Error("private API workload credential is unavailable");
  return assertUsableToken(staticToken);
}
