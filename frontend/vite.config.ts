import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig, type Plugin } from "vite";
import vue from "@vitejs/plugin-vue";
import tailwindcss from "@tailwindcss/vite";
import wails from "@wailsio/runtime/plugins/vite";

const rootDir = path.dirname(fileURLToPath(import.meta.url));

/**
 * Vite's SPA fallback would serve index.html for /api/*, so a probe of
 * /api/meta on wails.localhost:9245 looks like "success" HTML.
 * Answer JSON instead. Desktop IPC is Wails bindings, not this host.
 */
function lrssApiNotWeb(): Plugin {
  return {
    name: "lrss-api-not-web",
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const url = (req.url ?? "").split("?")[0];
        if (!url.startsWith("/api")) {
          next();
          return;
        }
        res.setHeader("Content-Type", "application/json; charset=utf-8");
        if ((req.method ?? "GET") === "GET" && url === "/api/meta") {
          res.end(JSON.stringify({ mode: "wails", web: false, desktop: true }));
          return;
        }
        res.statusCode = 404;
        res.end(JSON.stringify({ error: "web API is not on the Vite/Wails origin" }));
      });
    },
  };
}

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
  plugins: [lrssApiNotWeb(), vue(), tailwindcss(), wails("./bindings")],
  resolve: {
    alias: {
      "@": path.resolve(rootDir, "./src"),
    },
  },
});
