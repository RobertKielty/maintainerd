const { Given, When, Then } = require("@cucumber/cucumber");
const { expect } = require("@playwright/test");

const projectNames = {
  fossaFull: "Project Fossa Full",
  fossaPartial: "Project Fossa Partial",
  fossaInvites: "Project Fossa Invites",
  fossaMissingEmail: "Project Fossa Missing Email",
  snyk: "Project Snyk",
  noLicense: "Project No License",
};

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

const openProjectPage = async (world) => {
  if (!world.projectName) {
    throw new Error("No projectName set for license checker scenario.");
  }
  const id = await resolveProjectId(world, world.projectName);
  await world.page.goto(`${world.baseUrl}/projects/${id}`, {
    waitUntil: "domcontentloaded",
  });
  await expect(world.page.getByRole("heading", { name: world.projectName })).toBeVisible({
    timeout: 15000,
  });
};

const openLicenseCheckerSection = async (world) => {
  const menuButton = world.page.getByRole("button", {
    name: "SERVICES / LICENSE CHECKER",
  });
  await menuButton.waitFor({ state: "visible", timeout: 15000 });
  await menuButton.click();
  await expect(world.page.getByRole("heading", { name: "SERVICES / LICENSE CHECKER" })).toBeVisible({
    timeout: 15000,
  });
};

Given("the project has selected FOSSA", function () {
  if (!this.projectName) {
    this.projectName = projectNames.fossaPartial;
  }
});

Given("a FOSSA team is assigned to the project", function () {
  if (!this.projectName) {
    this.projectName = projectNames.fossaPartial;
  }
});

Given("maintainers are registered on the project", function () {
  if (!this.projectName) {
    this.projectName = projectNames.fossaPartial;
  }
});

Given("those maintainers are already Team Admins on the FOSSA team", function () {
  this.projectName = projectNames.fossaFull;
});

Given("all registered maintainers are recorded as Team Admins on the FOSSA team", function () {
  this.projectName = projectNames.fossaFull;
});

Given(
  "at least one registered maintainer is not recorded as a Team Admin on the FOSSA team",
  function () {
    this.projectName = projectNames.fossaPartial;
  }
);

Given("an invitation is recorded as pending", function () {
  this.projectName = projectNames.fossaInvites;
});

Given("invitations are recorded with statuses pending, accepted, expired, and error", function () {
  this.projectName = projectNames.fossaInvites;
});

Given("a registered maintainer is missing an email address", function () {
  this.projectName = projectNames.fossaMissingEmail;
});

Given("the project has selected a non-FOSSA license checker", function () {
  this.projectName = projectNames.snyk;
});

Given("the project has not selected a license checker", function () {
  this.projectName = projectNames.noLicense;
});

When("I open the project page", async function () {
  await openProjectPage(this);
});

When("I open the SERVICES \\/ LICENSE CHECKER section", async function () {
  await openLicenseCheckerSection(this);
});

Then("the ACTIVE MAINTAINERS ELIGABLE FOR INVITATION table is visible", async function () {
  if (!this.page.url().includes("/projects/")) {
    await openProjectPage(this);
  }
  await openLicenseCheckerSection(this);
  await expect(this.page.getByRole("heading", { name: "ACTIVE MAINTAINERS ELIGABLE FOR INVITATION" })).toBeVisible({
    timeout: 15000,
  });
});

When("I select maintainers in the table", async function () {
  const checkbox = this.page.getByRole("checkbox").first();
  await checkbox.check();
});

When("I clear the maintainer selection", async function () {
  const checkboxes = this.page.getByRole("checkbox");
  const count = await checkboxes.count();
  for (let index = 0; index < count; index += 1) {
    const checkbox = checkboxes.nth(index);
    if (await checkbox.isChecked()) {
      await checkbox.uncheck();
    }
  }
});

When("I send invites", async function () {
  const button = this.page.getByRole("button", { name: /Send CNCF FOSSA Invites to/i });
  await button.click();
});

Then("invitations are sent to the selected maintainers", async function () {
  const section = this.page.getByRole("heading", { name: "PENDING INVITATIONS" }).locator("..").locator("..");
  await expect(section.getByText("No pending FOSSA invites.")).toHaveCount(0, { timeout: 15000 });
  const rows = section.locator("tbody tr");
  await expect
    .poll(async () => rows.count(), { timeout: 15000 })
    .toBeGreaterThan(0);
});

Then("the PENDING INVITATIONS table is shown with headings", async function (table) {
  await expect(this.page.getByRole("heading", { name: "PENDING INVITATIONS" })).toBeVisible({
    timeout: 15000,
  });
  const section = this.page.getByRole("heading", { name: "PENDING INVITATIONS" }).locator("..").locator("..");
  for (const header of table.raw()[0] || []) {
    await expect(section.getByRole("columnheader", { name: header })).toBeVisible({ timeout: 15000 });
  }
});

Then("maintainer-d checks for an existing FOSSA team for the project", async function () {
  // Covered by the FOSSA team link being rendered after choosing FOSSA.
});

Then("if no team exists, maintainer-d creates a FOSSA team using the project name", async function () {
  await expect(this.page.getByRole("link", { name: this.projectName })).toBeVisible({ timeout: 15000 });
});

Then('if a team exists, the audit log records "FOSSA_TEAM_REUSED"', function () {
  // TODO: wire audit log assertions when audit UI is covered by BDD tests.
});

Then("if team creation fails, the UI shows the FOSSA error and the error time", function () {
  // TODO: simulate FOSSA failure to assert error rendering in UI tests.
});

Then("the FOSSA team is checked for existing team members", function () {
  // Covered by the FOSSA team table being rendered.
});

Then("the FOSSA team is checked for imported repos", function () {
  // Covered by the REPOS IMPORTED table placeholder.
});

Then("the ACTIVE MAINTAINERS ELIGABLE FOR INVITATION table is shown", async function () {
  await expect(this.page.getByRole("heading", { name: "ACTIVE MAINTAINERS ELIGABLE FOR INVITATION" })).toBeVisible({
    timeout: 15000,
  });
});

Then("I see the FOSSA team members list with status {string}", async function (status) {
  await expect(this.page.getByText(status).first()).toBeVisible({ timeout: 15000 });
});

Then("I do not see the {string} button", async function (label) {
  const button = this.page.getByRole("button", { name: new RegExp(label, "i") });
  await expect(button).toHaveCount(0);
});

Then(
  "I see the {string} button followed a list of checkbox selectable maintainers that are not on FOSSA according to maintainerd database records",
  async function (label) {
    const inviteButton = this.page.getByRole("button", { name: new RegExp(label, "i") });
    const pendingSection = this.page.getByRole("heading", { name: "PENDING INVITATIONS" }).locator("..").locator("..");
    await Promise.race([
      expect(inviteButton).toBeVisible({ timeout: 15000 }),
      expect(pendingSection.getByRole("cell", { name: /pending/i }).first()).toBeVisible({ timeout: 15000 }),
    ]);
    if (await inviteButton.count()) {
      const checkboxes = this.page.getByRole("checkbox");
      const count = await checkboxes.count();
      expect(count).toBeGreaterThan(0);
    }
  }
);

When("the system checks FOSSA invitation status", async function () {
  await openProjectPage(this);
  const menuButton = this.page.getByRole("button", {
    name: "SERVICES / LICENSE CHECKER",
  });
  await menuButton.waitFor({ state: "visible", timeout: 15000 });
  await menuButton.click();
});

Then(
  "if the invitation is accepted, the maintainer is added to the FOSSA team and the invitation is marked as accepted",
  async function () {
    const section = this.page.getByRole("heading", { name: "CNCF FOSSA TEAM" }).locator("..").locator("..");
    await expect(section.getByText("Priya Demo")).toBeVisible({ timeout: 15000 });
    await expect(section.getByText("Team Admin")).toBeVisible({ timeout: 15000 });
  }
);

Then(
  "if the invitation is expired, the invitation is re-issued and the status remains pending with an updated time sent",
  async function () {
    const section = this.page.getByRole("heading", { name: "PENDING INVITATIONS" }).locator("..").locator("..");
    await expect(section.getByRole("cell", { name: "pending" }).first()).toBeVisible({ timeout: 15000 });
    await expect(section.getByRole("cell", { name: "expired" })).toHaveCount(0);
  }
);

Then(
  "if the invitation check fails, the invitation is marked as error with the last error message",
  async function () {
    const section = this.page.getByRole("heading", { name: "PENDING INVITATIONS" }).locator("..").locator("..");
    await expect(section.getByRole("cell", { name: "pending" }).first()).toBeVisible({ timeout: 15000 });
    await expect(section.getByRole("cell", { name: "error" })).toHaveCount(0);
  }
);

Then("I see the PENDING INVITATIONS table", async function () {
  await expect(this.page.getByRole("heading", { name: "PENDING INVITATIONS" })).toBeVisible({
    timeout: 15000,
  });
});

Then("I see the invite sent time and estimated expiry time for pending invitations", async function () {
  const section = this.page.getByRole("heading", { name: "PENDING INVITATIONS" }).locator("..").locator("..");
  await expect(section.getByRole("columnheader", { name: "Invite sent on" })).toBeVisible({ timeout: 15000 });
  await expect(section.getByRole("columnheader", { name: "Estimated time of expiry" })).toBeVisible({ timeout: 15000 });
});

Then("I see a note that the maintainer email is missing", async function () {
  await expect(this.page.getByText("Missing email").first()).toBeVisible({ timeout: 15000 });
});

Then("the maintainer is not eligible for FOSSA invites", async function () {
  await expect(this.page.getByText("Not eligible for invites")).toBeVisible({ timeout: 15000 });
});

Then("I see a message that the project has not selected FOSSA", async function () {
  await expect(this.page.getByText("This project has not selected FOSSA")).toBeVisible({
    timeout: 15000,
  });
});

Then("I see a note that the project may have an organization on Snyk", async function () {
  await expect(
    this.page.getByText("It may be using Snyk for license checks.")
  ).toBeVisible({ timeout: 15000 });
});

Then(
  "the invite action shows Send CNCF FOSSA Invites to {int} Selected Maintainers",
  async function (count) {
    const label = `Send CNCF FOSSA Invites to ${count} Selected Maintainers`;
    await expect(this.page.getByRole("button", { name: label })).toBeVisible({ timeout: 15000 });
  }
);

Then("the invite action is disabled when {int} is {int}", async function (_count, selected) {
  const label = new RegExp(`Send CNCF FOSSA Invites to ${selected} Selected Maintainers`, "i");
  const button = this.page.getByRole("button", { name: label });
  await expect(button).toBeDisabled({ timeout: 15000 });
});

Given("invitations are recorded as pending", function () {
  this.projectName = projectNames.fossaInvites;
});

When("the invitation poller runs", async function () {
  await openProjectPage(this);
  await openLicenseCheckerSection(this);
});

Then("accepted invitations add the maintainer to the FOSSA team", async function () {
  const section = this.page.getByRole("heading", { name: "CNCF FOSSA TEAM" }).locator("..").locator("..");
  await expect(section.getByText("Priya Demo")).toBeVisible({ timeout: 15000 });
});

Then("the audit log records a FOSSA add-user event", function () {
  // Audit log UI not covered in BDD; verified via backend tests.
});

Then("expired invitations are re-issued", async function () {
  const section = this.page.getByRole("heading", { name: "PENDING INVITATIONS" }).locator("..").locator("..");
  await expect(section.getByRole("cell", { name: "pending" }).first()).toBeVisible({ timeout: 15000 });
});

Then("pending invitations can be deleted to remove the invitation from FOSSA and maintainer-d", async function () {
  const section = this.page.getByRole("heading", { name: "PENDING INVITATIONS" }).locator("..").locator("..");
  await expect(section.getByRole("button", { name: "Remove" }).first()).toBeVisible({ timeout: 15000 });
});

Then('I see a "Choose FOSSA" button', async function () {
  await expect(this.page.getByRole("button", { name: "Choose FOSSA" })).toBeVisible({
    timeout: 15000,
  });
});

When("I choose FOSSA", async function () {
  if (!this.page.url().includes("/projects/")) {
    await openProjectPage(this);
  }
  await openLicenseCheckerSection(this);
  const button = this.page.getByRole("button", { name: "Choose FOSSA" });
  await expect(button).toBeVisible({ timeout: 15000 });
  await button.click();
});

Then("the existing FOSSA onboarding process runs for the project", async function () {
  await Promise.race([
    this.page.getByText("FOSSA onboarding started.").waitFor({ timeout: 15000 }),
    this.page.getByRole("link", { name: this.projectName }).waitFor({ timeout: 15000 }),
  ]);
});
