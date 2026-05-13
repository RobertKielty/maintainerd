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

When("I open the DOT-PROJECT ROLL CALL route", async function () {
  const link = this.page.locator('[class*="projectMenu"]').getByRole("link", {
    name: "DOT-PROJECT ROLL CALL",
    exact: true,
  });
  await link.click();
  await expect(this.page).toHaveURL(new RegExp(`/projects/${this.projectId}/dot-project$`), {
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

Then("the legacy roll call shows a dot-project migration note", async function () {
  await expect(
    this.page.getByText(
      "This project has a maintainer file in its .project repo. Use DOT-PROJECT ROLL CALL to track the migration from the legacy maintainer file.",
      { exact: false }
    )
  ).toBeVisible({ timeout: 15000 });
});

Then("the project route navigation shows a red X on LEGACY ROLL CALL", async function () {
  const link = this.page
    .locator('[class*="projectMenu"]')
    .getByRole("link", { name: "LEGACY ROLL CALL", exact: true });
  await expect(link.locator("text=✕")).toBeVisible({ timeout: 15000 });
});

Then("the project route navigation shows a green tick on DOT-PROJECT ROLL CALL", async function () {
  const link = this.page
    .locator('[class*="projectMenu"]')
    .getByRole("link", { name: "DOT-PROJECT ROLL CALL", exact: true });
  await expect(link.locator("text=✓")).toBeVisible({ timeout: 15000 });
});

Then("the dot-project roll call shows the persisted discovery summary", async function () {
  const contentColumn = this.page.locator('[class*="contentColumn"]');
  await expect(contentColumn.getByText("Persisted dot-project roll call", { exact: true })).toBeVisible({
    timeout: 15000,
  });
  await expect(contentColumn.getByText("Schema version", { exact: true })).toBeVisible({ timeout: 15000 });
  await expect(contentColumn.getByText("1.0.0", { exact: true })).toBeVisible({ timeout: 15000 });
  await expect(contentColumn.getByText("Maintainer count", { exact: true })).toBeVisible({ timeout: 15000 });
  await expect(contentColumn.getByText("3 maintainers", { exact: true })).toBeVisible({ timeout: 15000 });
});

Then("the dot-project roll call lists the tracked .project files", async function () {
  const contentColumn = this.page.locator('[class*="contentColumn"]');
  const trackedFilesTable = contentColumn.locator("table").first();
  for (const label of [
    ".project repo",
    "project.yaml",
    "MAINTAINERS.yaml",
    "SECURITY.md",
    "CONTRIBUTING.md",
    "GOVERNANCE.md",
  ]) {
    await expect(trackedFilesTable.getByRole("cell", { name: label, exact: true })).toBeVisible({
      timeout: 15000,
    });
  }
});

Then("the dot-project roll call renders the cached maintainer file as formatted YAML", async function () {
  const contentColumn = this.page.locator('[class*="contentColumn"]');
  const viewer = contentColumn.locator('[class*="dotProjectYamlViewer"]');

  await expect(viewer.getByRole("heading", { name: "Formatted maintainer file", exact: true })).toBeVisible({
    timeout: 15000,
  });
  await expect(viewer.locator('[class*="dotProjectTeamName"]').getByText("project-maintainers")).toBeVisible({
    timeout: 15000,
  });
  await expect(viewer.locator('[class*="dotProjectMemberChip"]').getByText("antonio-example")).toBeVisible({
    timeout: 15000,
  });
  await expect(viewer.locator('[class*="yamlTokenComment"]')).toContainText(
    "# Project Atlas dot-project maintainers"
  );
});
