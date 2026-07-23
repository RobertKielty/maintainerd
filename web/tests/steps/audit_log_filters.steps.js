const { When, Then } = require("@cucumber/cucumber");
const { expect } = require("@playwright/test");

const isoMinutes = (date) => date.toISOString().slice(0, 16);

const waitForAuditResponse = (world, action) => {
  const responsePromise = world.page.waitForResponse(
    (response) => response.url().includes("/api/audit") && response.request().method() === "GET",
    { timeout: 15000 }
  );
  return Promise.all([responsePromise, action()]);
};

When("I open the audit log page", async function () {
  await this.page.goto(`${this.baseUrl}/audit`, { waitUntil: "domcontentloaded" });
  await expect(this.page.getByRole("heading", { name: "Audit Log" })).toBeVisible({
    timeout: 15000,
  });
});

When("I open the sync runs page", async function () {
  await this.page.goto(`${this.baseUrl}/audit/sync-runs`, { waitUntil: "domcontentloaded" });
  await expect(this.page.getByRole("heading", { name: "Sync Runs" })).toBeVisible({
    timeout: 15000,
  });
});

When("I filter the audit log by action {string}", async function (action) {
  await waitForAuditResponse(this, () => this.page.getByLabel("Action").selectOption(action));
});

When("I filter the audit log by target {string}", async function (target) {
  const input = this.page.getByLabel("Target");
  await input.fill(target);
  await waitForAuditResponse(this, () => input.blur());
});

When("I filter the audit log to only include events since 1 day ago", async function () {
  const start = new Date(Date.now() - 24 * 60 * 60 * 1000);
  const input = this.page.getByLabel("Start time");
  await waitForAuditResponse(this, () => input.fill(isoMinutes(start)));
});

When("I filter the audit log to only include events before 1 day ago", async function () {
  const end = new Date(Date.now() - 24 * 60 * 60 * 1000);
  const input = this.page.getByLabel("End time");
  await waitForAuditResponse(this, () => input.fill(isoMinutes(end)));
});

const auditActionCells = (world) => world.page.locator("table tbody tr td:nth-child(2)");

// The table re-fetches after every filter change; wait for the table (or the
// "no results" empty state) to settle before reading its rows.
const waitForAuditTableSettled = async (world) => {
  await world.page
    .locator("table tbody tr, [class*='empty']")
    .first()
    .waitFor({ state: "visible", timeout: 15000 });
};

Then("every visible audit log row has action {string}", async function (expectedAction) {
  await waitForAuditTableSettled(this);
  const cells = auditActionCells(this);
  const count = await cells.count();
  expect(count).toBeGreaterThan(0);
  for (let index = 0; index < count; index += 1) {
    await expect(cells.nth(index)).toHaveText(expectedAction);
  }
});

Then("every visible audit log row has an action starting with {string}", async function (prefix) {
  await waitForAuditTableSettled(this);
  const cells = auditActionCells(this);
  const count = await cells.count();
  for (let index = 0; index < count; index += 1) {
    const text = await cells.nth(index).innerText();
    expect(text.startsWith(prefix)).toBe(true);
  }
});

Then("the audit log table shows no {string} rows", async function (action) {
  await waitForAuditTableSettled(this);
  const cells = auditActionCells(this);
  const count = await cells.count();
  for (let index = 0; index < count; index += 1) {
    await expect(cells.nth(index)).not.toHaveText(action);
  }
});

Then("the audit log table includes a row targeting project {string}", async function (projectName) {
  const links = this.page.locator("table tbody tr td").getByRole("link", {
    name: projectName,
    exact: true,
  });
  await expect(links.first()).toBeVisible({ timeout: 15000 });
});

Then(
  "the audit log table does not include a row targeting project {string}",
  async function (projectName) {
    const links = this.page.locator("table tbody tr td").getByRole("link", {
      name: projectName,
      exact: true,
    });
    await expect(links).toHaveCount(0);
  }
);

Then("every visible audit log row targets project {string}", async function (projectName) {
  await waitForAuditTableSettled(this);
  const targetCells = this.page.locator("table tbody tr td:nth-child(3)");
  const count = await targetCells.count();
  expect(count).toBeGreaterThan(0);
  for (let index = 0; index < count; index += 1) {
    await expect(targetCells.nth(index)).toContainText(projectName);
  }
});
