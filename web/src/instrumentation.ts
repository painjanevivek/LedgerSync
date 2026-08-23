import { readDemoConfiguration } from "@/lib/demo";

export async function register() {
  if (process.env.NEXT_RUNTIME === "nodejs") readDemoConfiguration();
}
