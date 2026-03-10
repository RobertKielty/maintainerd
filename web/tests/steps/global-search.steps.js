const { When, Then } = require("@cucumber/cucumber");
const { expect } = require("@playwright/test");

const getMaintainersSection = (page) =>
  page.getByRole("heading", { name: "Maintainers" }).locator("..").locator("..");

When("I search globally for maintainer email {string}", async function (email) {
  await this.page.goto(
    `${this.baseUrl}/search?query=${encodeURIComponent(email)}`,
    { waitUntil: "domcontentloaded" }
  );
  await this.page.waitForURL(/\/search\?query=/, { timeout: 15000 });
  const response = await this.page.waitForResponse(
    (response) =>
      response.request().method() === "GET" && response.url().includes("/api/search?"),
    { timeout: 15000 }
  );
  if (!response.ok()) {
    const body = await response.text();
    console.error(
      `global-search: api/search failed status=${response.status()} url=${response.url()} body=${body}`
    );
    throw new Error(`global-search: api/search failed status=${response.status()}`);
  }
  try {
    const payload = await response.json();
    const maintainers = Array.isArray(payload.maintainers) ? payload.maintainers : [];
    console.log(
      `global-search: api/search ok maintainers=${maintainers.length} maintainersTotal=${payload.maintainersTotal} projectsTotal=${payload.projectsTotal} companiesTotal=${payload.companiesTotal}`
    );
    console.log(
      `global-search: sample maintainers=${maintainers
        .slice(0, 3)
        .map((m) => `${m.name} <${m.email || "EMAIL_MISSING"}>`)
        .join(", ")}`
    );
  } catch (err) {
    const body = await response.text();
    console.error(`global-search: api/search json parse failed err=${err} body=${body}`);
    throw err;
  }
  await Promise.race([
    this.page.getByRole("heading", { name: "Maintainers" }).waitFor({
      state: "visible",
      timeout: 15000,
    }),
    this.page.getByRole("heading", { name: "Companies" }).waitFor({
      state: "visible",
      timeout: 15000,
    }),
    this.page.getByText("No results found.").waitFor({
      state: "visible",
      timeout: 15000,
    }),
    this.page.getByText("Unable to load search results.").waitFor({
      state: "visible",
      timeout: 15000,
    }),
    this.page.getByText("You do not have access to global search.").waitFor({
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
  await expect(this.page.getByRole("heading", { name: "Maintainers" })).toHaveCount(0);
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
