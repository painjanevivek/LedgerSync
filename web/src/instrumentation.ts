import { readLocalAccessConfiguration } from "@/lib/local-access";
import { readPublicOrigin } from "@/lib/security";

export async function register() {
  if (process.env.NEXT_RUNTIME === "nodejs") {
    readPublicOrigin();
    readLocalAccessConfiguration();
  }
}
