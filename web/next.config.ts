import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  distDir: process.env.NEXT_DIST_DIR || ".next",
  turbopack: {
    root: process.env.TURBOPACK_ROOT || "./",
  },
  outputFileTracingRoot: process.env.OUTPUT_FILE_TRACING_ROOT,
  images: {
    remotePatterns: [
      {
        protocol: "https",
        hostname: "github.com",
        pathname: "/**.png",
      },
    ],
  },
};

export default nextConfig;
