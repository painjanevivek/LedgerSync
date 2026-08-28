import { readDemoConfiguration } from "@/lib/demo";
import { readPublicOrigin } from "@/lib/security";

export async function register() {
  if (process.env.NEXT_RUNTIME === "nodejs") {
    readPublicOrigin();
    readDemoConfiguration();
  }
}
