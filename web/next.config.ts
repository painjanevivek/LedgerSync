import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Containers consume the standalone server. Vercel owns its output tracing.
  output: process.env.VERCEL ? undefined : "standalone",
  poweredByHeader: false,
  experimental: {
    // Bound production page-generation workers on local and container builds.
    cpus: 2,
    // Keep shared console modules reusable across the growing set of routes.
    // Thresholds are raw source bytes; emitted-asset and browser budgets stay fixed.
    turbopackChunking: { minChunkSize: 20_000, maxMergeChunkSize: 50_000 },
  },
};

export default nextConfig;
