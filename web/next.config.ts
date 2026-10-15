import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Containers consume the standalone server. Vercel owns its output tracing.
  output: process.env.VERCEL ? undefined : "standalone",
  poweredByHeader: false,
};

export default nextConfig;
