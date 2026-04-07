import type { NextConfig } from "next";

const apiUrl = process.env.API_URL || "http://localhost:8080";

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    // `fallback` rewrites only fire when no filesystem route, no static page,
    // and no dynamic route matched. This lets us ship local Route Handlers
    // (e.g. the SSE proxy at app/api/projects/[id]/agent/stream/route.ts)
    // that take precedence over the catch-all forge-core proxy. The dev
    // server's rewrite layer gzip-buffers proxied responses, which breaks
    // EventSource — so the SSE endpoint must be served locally.
    return {
      beforeFiles: [],
      afterFiles: [],
      fallback: [
        {
          source: "/api/:path*",
          destination: `${apiUrl}/api/:path*`,
        },
      ],
    };
  },
};

export default nextConfig;
