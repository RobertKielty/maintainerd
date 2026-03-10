const { Given, When, Then } = require("@cucumber/cucumber");
const { expect } = require("@playwright/test");

const normalizeEnvKey = (value) =>
  value.toUpperCase().replace(/[^A-Z0-9]+/g, "_");

const resolveMaintainerLogin = (label) => {
  if (label === "self") {
    return process.env.TEST_MAINTAINER_LOGIN || "antonio-example";
  }
  const envKey = `TEST_MAINTAINER_LOGIN_${normalizeEnvKey(label)}`;
  return (
    process.env[envKey] ||
    process.env.TEST_OTHER_MAINTAINER_LOGIN ||
    label
  );
};

const resolveMaintainerID = async (world, label) => {
  if (/^\d+$/.test(label)) {
    return label;
  }
  const envKey = `TEST_MAINTAINER_ID_${normalizeEnvKey(label)}`;
  if (process.env[envKey]) {
    return process.env[envKey];
  }

  if (label === "self") {
    if (world.selfMaintainerId) {
      return world.selfMaintainerId;
    }
    const meResponse = await world.page.request.get(`${world.bffBaseUrl}/api/me`);
    if (!meResponse.ok()) {
      throw new Error(`Failed to load /api/me: ${meResponse.status()}`);
    }
    const meData = await meResponse.json();
    if (!meData.maintainerId) {
      throw new Error("No maintainerId returned from /api/me");
    }
    world.selfMaintainerId = meData.maintainerId;
    return meData.maintainerId;
  }

  if (label === "other") {
    if (world.otherMaintainerId) {
      return world.otherMaintainerId;
    }
    const selfId = world.currentRole === "maintainer" ? await resolveMaintainerID(world, "self") : null;
    const projectsResponse = await world.page.request.get(
      `${world.bffBaseUrl}/api/projects?limit=10`
    );
    if (!projectsResponse.ok()) {
      throw new Error(`Failed to load /api/projects: ${projectsResponse.status()}`);
    }
    const projectsData = await projectsResponse.json();
    for (const project of projectsData.projects || []) {
      for (const maintainer of project.maintainers || []) {
        if (maintainer.id && (!selfId || maintainer.id !== selfId)) {
          world.otherMaintainerId = maintainer.id;
          return maintainer.id;
        }
      }
    }
    throw new Error("No other maintainer found in project list");
  }

  throw new Error(
    `No maintainer ID configured for "${label}". Set TEST_MAINTAINER_ID or TEST_OTHER_MAINTAINER_ID.`
  );
};

const resolveProjectName = (label) => {
  if (label === "maintainer-d" && process.env.TEST_PROJECT_NAME) {
    return process.env.TEST_PROJECT_NAME;
  }
  return label;
};

Given("the maintainer-d database contains staff, maintainers, and projects", async function () {
  const response = await this.page.request.get(`${this.bffBaseUrl}/healthz`);
  if (!response.ok()) {
    throw new Error(`BFF health check failed: ${response.status()}`);
  }
});

Given("I am signed in as maintainer {string}", async function (label) {
  const login = resolveMaintainerLogin(label);
  await this.page.goto(
    `${this.bffBaseUrl}/auth/test-login?login=${encodeURIComponent(login)}`,
    { waitUntil: "domcontentloaded" }
  );
  await this.page.goto(this.baseUrl);
  this.currentRole = "maintainer";
});

When("I open the maintainer profile for {string}", async function (label) {
  const maintainerID = await resolveMaintainerID(this, label);
  await this.page.goto(`${this.baseUrl}/maintainers/${maintainerID}`, {
    waitUntil: "domcontentloaded",
  });
  await this.page.getByRole("heading", { name: /.+/ }).first().waitFor({
    state: "visible",
    timeout: 15000,
  });
});

When("I view the projects list", async function () {
  await this.page.goto(this.baseUrl, { waitUntil: "domcontentloaded" });
  await this.page.getByRole("link").first().waitFor({ state: "visible", timeout: 15000 });
});

When("I edit my maintainer profile with field {string} set to {string}", async function (field, value) {
  const editCard = this.page
    .getByRole("heading", { name: "Update maintainer" })
    .first()
    .locator("..");
  await expect(editCard).toBeVisible({ timeout: 15000 });
  const editButton = editCard.getByRole("button", { name: "Edit" });
  if (await editButton.isVisible()) {
    await editButton.click();
  }

  if (field === "email") {
    const emailInput = this.page.getByRole("textbox", { name: "Email" });
    await expect(emailInput).toBeEnabled({ timeout: 15000 });
    await emailInput.fill(value);
    return;
  }

  if (field === "company") {
    const companySelect = this.page.getByRole("combobox", { name: "Company" });
    await expect(companySelect).toBeEnabled({ timeout: 15000 });
    await companySelect.selectOption({ label: value });
    return;
  }

  throw new Error(`Unsupported maintainer field: ${field}`);
});

When("I save my maintainer profile changes", async function () {
  const saveButton = this.page.getByRole("button", { name: "Save changes" });
  await expect(saveButton).toBeEnabled({ timeout: 15000 });
  const responsePromise = this.page.waitForResponse(
    (response) =>
      response.request().method() === "PATCH" &&
      response.url().includes("/api/maintainers/"),
    { timeout: 20000 }
  );
  await saveButton.click();
  const response = await responsePromise;
  if (!response.ok()) {
    const body = await response.text();
    throw new Error(`Update maintainer failed: ${response.status()} ${body}`);
  }
  await expect(this.page.getByRole("button", { name: "Edit" })).toBeVisible({
    timeout: 15000,
  });
});

Then("project {string} is visible", async function (label) {
  const projectName = resolveProjectName(label);
  await expect(
    this.page.getByRole("link", { name: projectName }).first()
  ).toBeVisible({ timeout: 15000 });
});

Then("the maintainer email is visible", async function () {
  await expect(this.page.getByRole("button", { name: "Copy email" })).toBeVisible({
    timeout: 15000,
  });
});

Then("the maintainer email is hidden", async function () {
  await expect(this.page.getByRole("button", { name: "Copy email" })).toHaveCount(0);
  await expect(this.page.getByText("Email", { exact: true })).toHaveCount(0);
});

Then("my maintainer profile shows email {string}", async function (email) {
  await expect(this.page.getByText(email)).toBeVisible({ timeout: 15000 });
});

Then("my maintainer profile shows no company", async function () {
  const companySelect = this.page.getByRole("combobox", { name: "Company" });
  await expect(companySelect).toHaveValue("");
});
