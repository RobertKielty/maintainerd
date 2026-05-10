const { Given, When, Then } = require("@cucumber/cucumber");
const { expect } = require("@playwright/test");

Given("I am on the home page", async function () {
  await this.page.goto(this.baseUrl, { waitUntil: "domcontentloaded" });
  await this.page
    .locator("table")
    .first()
    .waitFor({ state: "visible", timeout: 15000 });
});

When("I filter the maintainer column by {string}", async function (value) {
  const input = this.page.getByPlaceholder(
    "Filter maintainer, location, or timezone"
  );
  await input.waitFor({ state: "visible", timeout: 10000 });
  await input.fill(value);
  // Allow debounce + fetch to settle
  await this.page.waitForTimeout(600);
});

When("I filter the project list by dot-project repo {string}", async function (value) {
  const button = this.page.getByRole("button", { name: value, exact: true });
  await button.waitFor({ state: "visible", timeout: 10000 });
  await button.click();
  await this.page.waitForTimeout(300);
});

Then(
  "the project list shows projects with maintainers in {string}",
  async function (location) {
    const rows = this.page.locator("tbody tr");
    await expect(rows.first()).toBeVisible({ timeout: 10000 });
    await expect(
      this.page.getByText("No projects match these filters.")
    ).not.toBeVisible();
    await this.page.screenshot({
      path: `${this.artifactsDir}/location-filter-${location.replace(/[^a-z0-9-_]+/gi, "_")}.png`,
      fullPage: true,
    });
  }
);

// Sam NoEmail (Yerevan, Armenia) is assigned only to "Project Fossa Missing Email"
// in the seed. Other projects (e.g. Project Atlas) have no Armenian maintainers.
Then(
  "the project list does not show projects without maintainers in {string}",
  async function (location) {
    await expect(
      this.page.getByRole("link", { name: "Project Atlas" }),
      `Expected no projects without maintainers in ${location}`
    ).not.toBeVisible({
      timeout: 5000,
    });
  }
);

Then("the project list shows no results", async function () {
  await expect(
    this.page.locator("tbody").getByText("No projects match these filters.")
  ).toBeVisible({ timeout: 10000 });
});

Then("the project list shows projects with dot-project repos", async function () {
  await expect(this.page.getByRole("link", { name: "Project Atlas" })).toBeVisible({
    timeout: 10000,
  });
  await expect(this.page.getByRole("link", { name: "Project Beacon" })).not.toBeVisible({
    timeout: 5000,
  });
});

Then("the project list shows projects without dot-project repos", async function () {
  await expect(this.page.getByRole("link", { name: "Project Beacon" })).toBeVisible({
    timeout: 10000,
  });
  await expect(this.page.getByRole("link", { name: "Project Atlas" })).not.toBeVisible({
    timeout: 5000,
  });
});
