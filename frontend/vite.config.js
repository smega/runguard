import { defineConfig, loadEnv } from "vite";
import preact from "@preact/preset-vite";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig(({ mode }) => {
  // Vite config runs in Node, not the browser, so it has to load .env
  // files itself via loadEnv rather than import.meta.env. Set
  // VITE_API_PROXY_TARGET in frontend/.env (see .env.example) to point the
  // dev server at your own backend; defaults to the port
  // backend/config.sample.yml itself defaults to, for local dev with both
  // halves running on one machine.
  const env = loadEnv(mode, process.cwd(), "");
  const apiProxyTarget = env.VITE_API_PROXY_TARGET || "http://localhost:8888";

  return {
    plugins: [preact(), tailwindcss()],
    server: {
      proxy: {
        "/api": apiProxyTarget,
      },
    },
  };
});
