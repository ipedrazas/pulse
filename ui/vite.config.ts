import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/healthz": "http://localhost:8080",
      "/status": "http://localhost:8080",
      "/nodes": "http://localhost:8080",
    },
  },
});
