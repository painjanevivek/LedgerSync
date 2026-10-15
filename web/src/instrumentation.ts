import { readLocalAccessConfiguration } from "@/lib/local-access";
import { readPublicOrigin } from "@/lib/security";
import { readOptionalFeatures } from "@/lib/feature-configuration";

export async function register() {
  if (process.env.NEXT_RUNTIME === "nodejs") {
    readPublicOrigin();
    readLocalAccessConfiguration();
    readOptionalFeatures(process.env);
    const { registerBFFTelemetry } = await import("@/lib/bff-telemetry");
    registerBFFTelemetry();
  }
}
