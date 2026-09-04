import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Container images consume Next's standalone server. Vercel performs its
  // own output tracing and packaging; forcing standalone there can make the
  // adapter look for artifacts that Next intentionally did not emit.
  output: process.env.VERCEL ? undefined : "standalone",
  poweredByHeader: false,
};

export default nextConfig;
