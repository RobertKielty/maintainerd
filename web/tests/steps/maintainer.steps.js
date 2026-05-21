const { When, Then } = require("@cucumber/cucumber");
const { expect } = require("@playwright/test");

When("I search for {string}", async function (query) {
  const searchInput = this.page.getByPlaceholder(
    "Search projects, maintainers, companies, or roster URLs"
  );
  await searchInput.fill(query);
  await searchInput.press("Enter");
  await this.page.screenshot({
    path: `${this.artifactsDir}/after-search-${query.replace(/[^a-z0-9-_]+/gi, "_")}.png`,
    fullPage: true,
  });
  const match = new RegExp(query, "i");
  await this.page.getByRole("link", { name: match }).first().waitFor({
    state: "visible",
    timeout: 15000,
  });
});

When("I click on maintainer {string}", async function (name) {
  const maintainerLink = this.page.getByRole("link", { name }).first();
  await maintainerLink.waitFor({ state: "visible", timeout: 15000 });
  await Promise.all([
    this.page.waitForURL(/\/maintainers\/\d+/, {
      timeout: 15000,
      waitUntil: "domcontentloaded",
    }),
    maintainerLink.click(),
  ]);
});

Then("I see the maintainer card for {string}", async function (name) {
  await Promise.race([
    this.page.getByRole("heading", { name }).waitFor({ timeout: 15000 }),
    this.page.getByText("Unable to load maintainer").waitFor({ timeout: 15000 }),
  ]);
  await expect(this.page.getByRole("heading", { name })).toBeVisible({
    timeout: 15000,
  });
});

Then("I see the maintainer location {string}", async function (location) {
  await expect(this.page.getByText(location)).toBeVisible({ timeout: 10000 });
});
