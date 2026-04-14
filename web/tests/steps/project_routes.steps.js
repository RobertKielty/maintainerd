const { Given, When, Then } = require("@cucumber/cucumber");
const { expect } = require("@playwright/test");

const routeProjectName = "Project Atlas";

const resolveProjectId = async (world, name) => {
  const response = await world.page.request.get(
    `${world.bffBaseUrl}/api/search?query=${encodeURIComponent(name)}`
  );
  if (!response.ok()) {
    throw new Error(`Failed to search project: ${response.status()}`);
  }
  const data = await response.json();
  const project = (data.projects || []).find((item) => item.name === name);
  if (!project) {
    throw new Error(`Project ${name} not found in search results.`);
  }
  return project.id;
};

Given("a project exists with reconciliation routes", function () {
  this.projectName = routeProjectName;
});

When("I open the project github route", async function () {
  this.projectId = await resolveProjectId(this, this.projectName);
  await this.page.goto(`${this.baseUrl}/projects/${this.projectId}/github`, {
    waitUntil: "domcontentloaded",
  });
  await expect(this.page.getByRole("heading", { name: this.projectName })).toBeVisible({
    timeout: 15000,
  });
});

Then("the project route navigation exposes these routes", async function (table) {
  const projectNav = this.page.locator('[class*="projectMenu"]');
  const routeHeader = this.page.locator('[class*="collapsibleHeader"]');
  const contentColumn = this.page.locator('[class*="contentColumn"]');
  for (const row of table.hashes()) {
    const link = projectNav.getByRole("link", { name: row.label, exact: true });
    await expect(link).toBeVisible({ timeout: 15000 });
    await link.click();
    await expect(this.page).toHaveURL(
      new RegExp(`/projects/${this.projectId}${row["path-suffix"]}$`),
      { timeout: 15000 }
    );
    await expect(routeHeader.getByRole("heading", { name: row.heading, exact: true })).toBeVisible({
      timeout: 15000,
    });
    await expect(contentColumn.getByText(row["body-snippet"], { exact: false })).toBeVisible({
      timeout: 15000,
    });
  }
});
