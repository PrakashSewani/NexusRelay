import type { NextConfig } from "next";
import { resolve } from "node:path";

const securityHeaders = [
  { key: "Referrer-Policy", value: "no-referrer" },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "X-Frame-Options", value: "DENY" },
  {
    key: "Permissions-Policy",
    value: "camera=(), geolocation=(), microphone=(), payment=(), usb=()",
  },
];

export function resolveBuildRevision(value: string | undefined) {
  if (value === undefined) {
    return "dev";
  }

  if (!/^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/.test(value)) {
    throw new Error(
      "NEXUSRELAY_BUILD_REVISION must be 1-128 ASCII letters, numbers, dots, underscores, or hyphens and start with a letter or number",
    );
  }

  return value;
}

const nextConfig: NextConfig = {
  generateBuildId: () => resolveBuildRevision(process.env.NEXUSRELAY_BUILD_REVISION),
  output: "standalone",
  outputFileTracingRoot: resolve(import.meta.dirname, "../.."),
  poweredByHeader: false,
  reactStrictMode: true,
  async headers() {
    return [
      {
        source: "/:path*",
        headers: securityHeaders,
      },
    ];
  },
};

export default nextConfig;
