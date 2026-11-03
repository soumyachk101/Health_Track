import path from "node:path";
import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

// tsconfig has `jsx: "preserve"` (Next.js convention), so plain esbuild
// transforms would leave JSX untouched — the react plugin does the babel
// transform for .tsx under React 19.
const root = fileURLToPath(new URL(".", import.meta.url));

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: [path.resolve(root, "vitest.setup.ts")],
  },
  resolve: {
    // Matches the tsconfig paths / Next "@/" convention.
    alias: { "@": root },
  },
});
