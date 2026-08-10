import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import tailwindcss from "@tailwindcss/vite";
import wails from "@wailsio/runtime/plugins/vite";

const rootDir = path.dirname(fileURLToPath(import.meta.url));

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    // Use IPv4 loopback. Wails app.preRun() and reverse proxy force tcp4 for
    // localhost/127.0.0.1; binding IPv6-only localhost causes connect failures on Windows.
    // FRONTEND_DEVSERVER_URL is set to http://127.0.0.1:<port> in Taskfile run.
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  plugins: [vue(), tailwindcss(), wails("./bindings")],
  resolve: {
    alias: {
      "@": path.resolve(rootDir, "./src"),
    },
  },
});
