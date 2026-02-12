import { defineConfig, PluginOption } from "vite";
import { NodeGlobalsPolyfillPlugin } from "@esbuild-plugins/node-globals-polyfill";
import { nodePolyfills } from "vite-plugin-node-polyfills";
import fixReactVirtualized from "esbuild-plugin-react-virtualized";
import react from "@vitejs/plugin-react";

// https://vitejs.dev/config/
// Set VITE_BASE env var to override (e.g. VITE_BASE=/ npm run build for root)
export default defineConfig({
  base: process.env.VITE_BASE ?? "/plans/",
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  plugins: [
    react(),
    nodePolyfills({
      include: ["timers"],
      globals: {
        process: true,
      },
    }),
  ] as PluginOption[],
  optimizeDeps: {
    esbuildOptions: {
      define: {
        global: "globalThis",
      },
      plugins: [
        NodeGlobalsPolyfillPlugin({
          buffer: true,
          process: true,
        }),
        fixReactVirtualized,
      ],
    },
  },
});
