// ESLint for the frontend: the rules of Next.js (eslint-config-next), the
// type-aware rules of typescript-eslint, and the boundaries of the layout
// (eslint-plugin-boundaries). Prettier owns formatting. `make check` runs it,
// because Next.js 16 removed `next lint` and `next build` no longer lints.
// https://nextjs.org/docs/app/api-reference/config/eslint
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTypescript from "eslint-config-next/typescript";
import boundaries from "eslint-plugin-boundaries";
import { defineConfig, globalIgnores } from "eslint/config";
import tseslint from "typescript-eslint";

// A module that must never reach the browser lives in a `server/` directory
// and imports "server-only", so that Next.js fails the build if a client
// component imports it.
const serverModules = ["src/**/server/**/*.{ts,tsx}"];

export default defineConfig([
  globalIgnores([".next/**", "out/**", "build/**", "next-env.d.ts", "**/*.gen.ts"]),
  ...nextVitals,
  ...nextTypescript,
  {
    rules: {
      // The rule is for the Pages Router; this frontend uses the App Router only.
      "@next/next/no-html-link-for-pages": "off",
    },
  },
  {
    files: ["**/*.{ts,tsx,mts,cts}"],
    extends: [tseslint.configs.strictTypeChecked],
    languageOptions: {
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
  },
  {
    // The layout: routes use features and lib, a feature (one bounded context)
    // uses lib but never another feature, and lib never uses a feature.
    // src/lib/api holds the types generated from the OpenAPI contracts.
    files: ["src/**/*.{ts,tsx}"],
    plugins: { boundaries },
    settings: {
      "boundaries/elements": [
        { type: "app", pattern: "src/app", partialMatch: false },
        { type: "feature", pattern: "src/features/*", partialMatch: false, capture: ["context"] },
        { type: "api", pattern: "src/lib/api", partialMatch: false },
        { type: "lib", pattern: "src/lib", partialMatch: false },
      ],
    },
    rules: {
      // Imports inside one element (one route tree, one feature) are always allowed.
      "boundaries/dependencies": [
        "error",
        {
          default: "disallow",
          policies: [
            {
              from: { element: { type: "app" } },
              allow: { to: { element: { types: { anyOf: ["feature", "lib", "api"] } } } },
            },
            {
              from: { element: { type: "feature" } },
              allow: { to: { element: { types: { anyOf: ["lib", "api"] } } } },
            },
            {
              from: { element: { type: "lib" } },
              allow: { to: { element: { type: "api" } } },
            },
            {
              from: { element: { type: "api" } },
              allow: { to: { element: { type: "lib" } } },
            },
          ],
        },
      ],
      "no-restricted-syntax": [
        "error",
        {
          selector:
            'Program:has(> ExpressionStatement[directive="use client"]) ImportDeclaration[source.value=/(^|\\/)server(\\/|$)/]',
          message:
            "A client component must not import a server module. Pass the data from a server component instead.",
        },
      ],
    },
  },
  {
    files: serverModules,
    rules: {
      "no-restricted-syntax": [
        "error",
        {
          selector: 'Program:not(:has(> ImportDeclaration[source.value="server-only"]))',
          message: 'A module in a server/ directory must import "server-only".',
        },
      ],
    },
  },
]);
