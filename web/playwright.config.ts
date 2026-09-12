import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./e2e",
  workers: 1,
  use: {
    baseURL: process.env.RDC_TEST_URL,
    locale: "en-US",
    ignoreHTTPSErrors: true,
    viewport: { width: 1440, height: 1000 },
  },
  reporter: "list",
});
