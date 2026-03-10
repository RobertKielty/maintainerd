const {
  BeforeAll,
  AfterAll,
  Before,
  After,
  setDefaultTimeout,
} = require("@cucumber/cucumber");
const { chromium } = require("playwright");
const fs = require("node:fs");
const path = require("node:path");

let browser;
let keepOpen = false;
let keepOpenOnFailure = false;

setDefaultTimeout(20 * 1000);

BeforeAll(async () => {
  const headless = process.env.WEB_TEST_HEADLESS !== "false";
  keepOpen = process.env.WEB_TEST_KEEP_OPEN === "true";
  browser = await chromium.launch({ headless });
});

AfterAll(async () => {
  if (browser && !keepOpenOnFailure) {
    await browser.close();
  }
});

Before(async function () {
  const artifactsDir =
    process.env.WEB_TEST_ARTIFACTS_DIR || path.join(process.cwd(), "testdata", "web-artifacts");
  fs.mkdirSync(artifactsDir, { recursive: true });
  this.context = await browser.newContext({
    recordVideo: { dir: artifactsDir },
  });
  this.page = await this.context.newPage();
  this.artifactsDir = artifactsDir;
});

After(async function (scenario) {
  const failed = scenario?.result?.status === "FAILED";
  try {
    if (failed && this.page && !this.page.isClosed()) {
      const safeName = scenario.pickle.name.replace(/[^a-z0-9-_]+/gi, "_").slice(0, 80);
      const screenshot = await this.page.screenshot({
        path: path.join(this.artifactsDir, `${safeName}.png`),
        fullPage: true,
      });
      if (this.attach) {
        await this.attach(screenshot, "image/png");
      }
    }
  } finally {
    if (failed && keepOpen) {
      keepOpenOnFailure = true;
      return;
    }
    if (this.page && !this.page.isClosed()) {
      await this.page.close();
    }
    if (this.context) {
      await this.context.close();
    }
  }
});
