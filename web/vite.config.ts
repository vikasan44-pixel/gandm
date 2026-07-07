import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Dev server proxies /api to the Go backend so the browser sees everything
// as same-origin — no CORS headers needed on the backend for local dev.
// Adjust the target if SERVER_PORT in the backend's .env isn't 8080.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
});
