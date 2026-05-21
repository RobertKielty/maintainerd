const { Given, When, Then } = require("@cucumber/cucumber");
const { expect } = require("@playwright/test");

const rollCallProjectName = "Project Atlas";

const presentInDatabaseSection = (world) =>
  world.page.getByRole("heading", { name: "PRESENT IN CNCF DATABASE" }).locator("..").locator("..");

const rollCallCheckboxes = (world) =>
  presentInDatabaseSection(world).locator("ul input[type='checkbox']");

const rollCallCheckboxForMaintainer = (world, name) =>
  presentInDatabaseSection(world)
    .locator("li", { has: world.page.getByRole("link", { name }) })
    .locator("input[type='checkbox']");

Given("a project has multiple active maintainers in the maintainer roll call", function () {
  this.projectName = rollCallProjectName;
});

Then("I see the {string} section", async function (label) {
  await expect(this.page.getByRole("heading", { name: label })).toBeVisible({
    timeout: 15000,
  });
});

When("I select all maintainers in the roll call", async function () {
  const checkbox = presentInDatabaseSection(this).getByLabel("Select all");
  await expect(checkbox).toBeVisible({ timeout: 15000 });
  await checkbox.check();
});

Then("all maintainer checkboxes in the roll call are selected", async function () {
  const checkboxes = rollCallCheckboxes(this);
  const count = await checkboxes.count();
  expect(count).toBeGreaterThan(1);
  for (let index = 0; index < count; index += 1) {
    await expect(checkboxes.nth(index)).toBeChecked({ timeout: 15000 });
  }
  await expect(presentInDatabaseSection(this).getByText(`${count} selected`)).toBeVisible({
    timeout: 15000,
  });
});

Then("the {string} bulk action is enabled for the roll call", async function (label) {
  await expect(
    presentInDatabaseSection(this).getByRole("button", { name: label })
  ).toBeEnabled({ timeout: 15000 });
});

When("I clear the roll call selection", async function () {
  const button = presentInDatabaseSection(this).getByRole("button", { name: "Clear" });
  await expect(button).toBeVisible({ timeout: 15000 });
  await button.click();
});

When("I select maintainer {string} in the roll call", async function (name) {
  const checkbox = rollCallCheckboxForMaintainer(this, name);
  await expect(checkbox).toBeVisible({ timeout: 15000 });
  await checkbox.check();
});

Then("only maintainer {string} is selected in the roll call", async function (name) {
  const rows = presentInDatabaseSection(this).locator("li");
  const count = await rows.count();
  expect(count).toBeGreaterThan(1);
  for (let index = 0; index < count; index += 1) {
    const row = rows.nth(index);
    const checkbox = row.locator("input[type='checkbox']");
    const rowText = (await row.textContent()) || "";
    if (rowText.includes(name)) {
      await expect(checkbox).toBeChecked({ timeout: 15000 });
    } else {
      await expect(checkbox).not.toBeChecked({ timeout: 15000 });
    }
  }
  await expect(presentInDatabaseSection(this).getByText("1 selected")).toBeVisible({
    timeout: 15000,
  });
});
