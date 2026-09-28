import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    environment: "jsdom",
    include: ["internal/handlers/static/js/*.test.js"],
  },
});
