// @ts-check
const { defineConfig, devices } = require("@playwright/test");

const baseURL =
  process.env.PLAYWRIGHT_BASE_URL ||
  process.env.SRA_TEST_BASE_URL ||
  "http://127.0.0.1:18081";

/** Default viewports (mobile / tablet / desktop). */
const viewportProjects = [
  { name: "mobile", width: 390, height: 844 },
  { name: "tablet", width: 820, height: 1180 },
  { name: "desktop", width: 1280, height: 800 },
];

const guiAgentConfig =
  process.env.SRA_TEST_CONFIG ||
  "scripts/test/fixtures/config.gui-layout.json";

/** @type {import('@playwright/test').PlaywrightTestConfig} */
module.exports = defineConfig({
  testDir: "tests/gui",
  timeout: 30_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  workers: 1,
  retries: 1,
  reporter: process.env.CI ? "line" : "list",
  webServer: {
    command: "bash scripts/test/start-gui-layout-agent.sh",
    url: `${baseURL}/api/v1/health`,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    env: {
      SRA_TEST_CONFIG: guiAgentConfig,
      SRA_TEST_TOKEN:
        process.env.SRA_TEST_TOKEN || "integration-test-token",
    },
  },
  use: {
    baseURL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    // Service worker only in pwa.spec.js: page.route stubs must not bypass the SW.
    serviceWorkers: "block",
    ...devices["Desktop Chrome"],
    // Existing specs assert the German texts; i18n.spec.js covers English.
    locale: "de-DE",
  },
  projects: viewportProjects.map(({ name, width, height }) => ({
    name,
    use: {
      viewport: { width, height },
    },
  })),
});
