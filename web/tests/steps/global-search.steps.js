const { When, Then } = require("@cucumber/cucumber");
const { expect } = require("@playwright/test");

const getMaintainersSection = (page) =>
  page.getByRole("heading", { name: "Maintainers" }).locator("..").locator("..");

When("I search globally for maintainer email {string}", async function (email) {
  const searchInput = this.page.getByPlaceholder("Search projects");
  await searchInput.fill(email);
  await searchInput.press("Enter");
  await this.page.waitForURL(/\/search\?query=/, { timeout: 15000 });
  await Promise.race([
    this.page.getByRole("heading", { name: "Maintainers" }).waitFor({
      state: "visible",
      timeout: 15000,
    }),
    this.page.getByText("No results found.").waitFor({
      state: "visible",
      timeout: 15000,
    }),
  ]);
});

Then("I see maintainer {string} in the results", async function (name) {
  const section = getMaintainersSection(this.page);
  await expect(section.getByRole("link", { name })).toBeVisible({ timeout: 15000 });
});

Then("the result shows the maintainer email {string}", async function (email) {
  const section = getMaintainersSection(this.page);
  const row = section.getByText(email, { exact: false }).first();
  await expect(row).toBeVisible({ timeout: 15000 });
});

Then("I see an empty state indicating no maintainers were found", async function () {
  await expect(this.page.getByText("No results found.")).toBeVisible({ timeout: 15000 });
});

Then(
  "the results include the projects associated with {string}",
  async function (name) {
    const section = getMaintainersSection(this.page);
    const row = section.getByRole("link", { name }).locator("..");
    await expect(row.getByRole("link", { name: "Project Atlas" })).toBeVisible({
      timeout: 15000,
    });
  }
);

When("I try to access global maintainer search by email", async function () {
  this.searchResponse = await this.page.request.get(
    `${this.bffBaseUrl}/api/search?query=${encodeURIComponent("alex@example.dev")}`
  );
});

Then("I am denied access to global maintainer search", async function () {
  expect(this.searchResponse.status()).toBe(403);
});
