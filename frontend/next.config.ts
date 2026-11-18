// Next.js settings. https://nextjs.org/docs/app/api-reference/config/next-config-js
import path from "node:path";

import type { NextConfig } from "next";

const config: NextConfig = {
  // The build writes .next/standalone, a server with only the files that it
  // needs, which the container image runs without node_modules
  // (frontend/Dockerfile). The frontend is a package of the pnpm workspace,
  // so its dependencies are traced from the root of the repository.
  output: "standalone",
  outputFileTracingRoot: path.join(import.meta.dirname, ".."),

  // The model of new Next.js apps: a page is static by default, and what
  // depends on a request or on data that is not cached streams in behind a
  // <Suspense> boundary. Every screen here shows live data of the contexts,
  // so each one reads its data inside such a boundary.
  cacheComponents: true,
  partialPrefetching: true,

  // A link to a route that does not exist fails the type check.
  typedRoutes: true,

  reactStrictMode: true,
  poweredByHeader: false,
};

export default config;
