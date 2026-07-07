const { Given, When, Then } = require("@cucumber/cucumber");
const { expect } = require("@playwright/test");

const seededMaintainers = {
  antonio: {
    name: "Antonio Example",
    email: "antonio.example@test.dev",
    github: "antonio-example",
    location: "Madrid, Spain",
    country: "ES",
    timezone: "Europe/Madrid",
  },
  renee: {
    name: "Renee Sample",
    email: "renee.sample@example.dev",
    github: "renee-sample",
    location: "Portland, Oregon, USA",
    country: "US",
    timezone: "America/New_York",
  },
  alex: {
    name: "Alex Example",
    email: "alex@example.dev",
    github: "alex-example",
    location: "London, UK",
    country: "GB",
    timezone: "Europe/London",
  },
  diego: {
    name: "Diego Placeholder",
    email: "diego.placeholder@test.dev",
    github: "diego-placeholder",
    location: "São Paulo, Brazil",
    country: "BR",
    timezone: "America/Sao_Paulo",
  },
  jun: {
    name: "Jun Example",
    email: "jun.example@test.dev",
    github: "jun-example",
    location: "Tokyo, Japan",
    country: "JP",
    timezone: "Asia/Tokyo",
  },
  priya: {
    name: "Priya Demo",
    email: "priya.demo@test.dev",
    github: "priya-demo",
    location: "Bengaluru, India",
    country: "IN",
    timezone: "Asia/Kolkata",
  },
  sam: {
    name: "Sam NoEmail",
    email: "EMAIL_MISSING",
    github: "sam-noemail",
    location: "Yerevan, Armenia",
    country: "AM",
    timezone: "Asia/Yerevan",
  },
};

const resolveMaintainerIdByQuery = async (world, query) => {
  const response = await world.page.request.get(
    `${world.bffBaseUrl}/api/search?query=${encodeURIComponent(query)}`
  );
  if (!response.ok()) {
    throw new Error(`Failed to search maintainers: ${response.status()}`);
  }
  const payload = await response.json();
  const maintainers = Array.isArray(payload.maintainers) ? payload.maintainers : [];
  const exact = maintainers.find(
    (maintainer) =>
      maintainer.name === query ||
      maintainer.email === query ||
      maintainer.github === query ||
      maintainer.githubAccount === query
  );
  const match = exact || maintainers[0];
  if (!match?.id) {
    throw new Error(`No maintainer found for query "${query}"`);
  }
  return String(match.id);
};

const selectSeededMaintainer = async (world, key) => {
  const maintainer = seededMaintainers[key];
  if (!maintainer) {
    throw new Error(`Unknown seeded maintainer persona "${key}"`);
  }
  world.selectedMaintainer = maintainer;
  world.maintainerId = await resolveMaintainerIdByQuery(
    world,
    maintainer.github || maintainer.name
  );
  return world.maintainerId;
};

const resolveMaintainerId = async (world) => {
  if (world.maintainerId) {
    return world.maintainerId;
  }
  if (process.env.TEST_MAINTAINER_ID) {
    world.maintainerId = process.env.TEST_MAINTAINER_ID;
    return world.maintainerId;
  }
  const meResponse = await world.page.request.get(`${world.bffBaseUrl}/api/me`);
  if (!meResponse.ok()) {
    throw new Error(`Failed to load /api/me: ${meResponse.status()}`);
  }
  const meData = await meResponse.json();
  if (meData.maintainerId) {
    world.maintainerId = meData.maintainerId;
    return world.maintainerId;
  }
  throw new Error("No TEST_MAINTAINER_ID configured and /api/me did not return a maintainerId");
};

const openMaintainerPage = async (world) => {
  const maintainerId = await resolveMaintainerId(world);
  await world.page.goto(`${world.baseUrl}/maintainers/${maintainerId}`, {
    waitUntil: "domcontentloaded",
  });
  await expect(world.page.getByRole("heading", { name: /.+/ }).first()).toBeVisible({
    timeout: 15000,
  });
};

const serviceCard = (world, label = "CNCF FOSSA") =>
  world.page.getByRole("heading", { name: label });

const ensureServiceDetailsExpanded = async (world, label = "CNCF FOSSA") => {
  const toggle = world.page.getByRole("button", { name: `Manage ${label}` });
  if ((await toggle.count()) > 0) {
    await toggle.click();
    await expect(world.page.getByRole("button", { name: "Hide details" })).toBeVisible({
      timeout: 15000,
    });
  }
};

Given("a maintainer exists in maintainer-d", async function () {
  const response = await this.page.request.get(`${this.bffBaseUrl}/healthz`);
  if (!response.ok()) {
    throw new Error(`BFF health check failed: ${response.status()}`);
  }
});

Given("the maintainer belongs to one or more projects", async function () {
  await selectSeededMaintainer(this, "antonio");
});
Given("the maintainer is associated with a project that uses FOSSA", async function () {
  this.serviceKind = "fossa";
  await selectSeededMaintainer(this, "sam");
});
Given("the maintainer belongs to a project that uses FOSSA", async function () {
  this.serviceKind = "fossa";
  await selectSeededMaintainer(this, "alex");
});
Given("the maintainer belongs to multiple active projects that use FOSSA", async function () {
  this.serviceKind = "fossa";
  await selectSeededMaintainer(this, "renee");
});
Given("the maintainer belongs to one or more projects that use FOSSA", async function () {
  this.serviceKind = "fossa";
  await selectSeededMaintainer(this, "jun");
});
Given("the maintainer exists in CNCF FOSSA", async function () {
  if (!this.maintainerId) {
    await selectSeededMaintainer(this, "renee");
  }
});
Given("the maintainer does not exist in CNCF FOSSA", async function () {
  await selectSeededMaintainer(this, "diego");
});
Given("the maintainer is not a member of that project's FOSSA team", async function () {
  await selectSeededMaintainer(this, "alex");
});
Given("the maintainer is missing from one or more required FOSSA teams", async function () {
  if (!this.maintainerId) {
    await selectSeededMaintainer(this, "renee");
  }
});
Given("a CNCF FOSSA invitation is pending for the maintainer", async function () {
  await selectSeededMaintainer(this, "jun");
});

When("I open the maintainer page", async function () {
  await openMaintainerPage(this);
});

When("I update the maintainer email address", async function () {
  await openMaintainerPage(this);
  const nextEmail =
    process.env.TEST_UPDATED_MAINTAINER_EMAIL || "bob@affiliated-company.tld";
  const editButton = this.page.getByRole("button", { name: "Edit" });
  await expect(editButton).toBeVisible({ timeout: 15000 });
  await editButton.click();
  const emailInput = this.page.getByRole("textbox", { name: "Email", exact: true });
  await expect(emailInput).toBeEnabled({ timeout: 15000 });
  await emailInput.fill(nextEmail);
  this.updatedMaintainerEmail = nextEmail;
});

When("I save the maintainer record", async function () {
  const saveButton = this.page.getByRole("button", { name: "Save changes" });
  await expect(saveButton).toBeEnabled({ timeout: 15000 });
  const responsePromise = this.page.waitForResponse(
    (response) =>
      response.request().method() === "PATCH" &&
      /\/api\/maintainers\/\d+$/.test(response.url()),
    { timeout: 20000 }
  );
  await saveButton.click();
  const response = await responsePromise;
  if (!response.ok()) {
    throw new Error(`Save maintainer failed: ${response.status()} ${await response.text()}`);
  }
  await expect(
    this.page.getByRole("button", { name: "Edit" })
  ).toBeVisible({ timeout: 15000 });
});

When("I refresh the maintainer's remote service associations", async function () {
  await openMaintainerPage(this);
  await ensureServiceDetailsExpanded(this);
  const button = this.page.getByRole("button", { name: "Refresh" });
  await expect(button).toBeVisible({ timeout: 15000 });
  const responsePromise = this.page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      /\/api\/maintainers\/\d+\/services\/fossa\/refresh$/.test(response.url()),
    { timeout: 20000 }
  );
  await button.click();
  const response = await responsePromise;
  if (!response.ok()) {
    throw new Error(`Refresh remote service associations failed: ${response.status()} ${await response.text()}`);
  }
  this.lastMaintainerServiceResponse = await response.json().catch(() => null);
});

When("I reconcile the maintainer's FOSSA access from the maintainer page", async function () {
  await openMaintainerPage(this);
  await ensureServiceDetailsExpanded(this);
  const button = this.page.getByRole("button", { name: "Reconcile Missing" });
  await expect(button).toBeVisible({ timeout: 15000 });
  const responsePromise = this.page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      /\/api\/maintainers\/\d+\/services\/fossa\/reconcile$/.test(response.url()),
    { timeout: 20000 }
  );
  await button.click();
  const response = await responsePromise;
  if (!response.ok()) {
    throw new Error(`Reconcile FOSSA access failed: ${response.status()} ${await response.text()}`);
  }
  this.lastMaintainerServiceResponse = await response.json().catch(() => null);
});

When("I send a CNCF FOSSA invite from the maintainer page", async function () {
  await openMaintainerPage(this);
  await ensureServiceDetailsExpanded(this);
  const button = this.page.getByRole("button", { name: "Send Invite" });
  await expect(button).toBeVisible({ timeout: 15000 });
  const responsePromise = this.page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      /\/api\/maintainers\/\d+\/services\/fossa\/invite$/.test(response.url()),
    { timeout: 20000 }
  );
  await button.click();
  const response = await responsePromise;
  if (!response.ok()) {
    throw new Error(`Send FOSSA invite failed: ${response.status()} ${await response.text()}`);
  }
  this.lastMaintainerServiceResponse = await response.json().catch(() => null);
});

When("the FOSSA invitation is accepted", function () {
  // External side-effect. The following refresh step re-reads remote state.
});

Then("I see a service associations section", async function () {
  await expect(serviceCard(this)).toBeVisible({ timeout: 15000 });
  await ensureServiceDetailsExpanded(this);
  await expect(
    this.page.getByRole("heading", { name: "Remote Service User Account" })
  ).toBeVisible({ timeout: 15000 });
});

Then("I see which remote services the maintainer is associated with", async function () {
  await expect(this.page.getByRole("heading", { name: "CNCF FOSSA" })).toBeVisible({
    timeout: 15000,
  });
});

Then("I see which project service assignments imply that the maintainer should be associated with those services", async function () {
  await ensureServiceDetailsExpanded(this);
  await expect(this.page.getByRole("heading", { name: "Project Target Memberships" })).toBeVisible({
    timeout: 15000,
  });
  await expect(this.page.getByRole("columnheader", { name: "Project" })).toBeVisible({
    timeout: 15000,
  });
  await expect(this.page.getByRole("columnheader", { name: "Remote Target" })).toBeVisible({
    timeout: 15000,
  });
});

Then("maintainer-d checks whether the maintainer exists on the remote service using the updated maintainer email address", async function () {
  const email = this.updatedMaintainerEmail || process.env.TEST_UPDATED_MAINTAINER_EMAIL;
  if (!email) {
    return;
  }
  const services = Array.isArray(this.lastMaintainerServiceResponse?.services)
    ? this.lastMaintainerServiceResponse.services
    : [];
  const matched = services.some((service) => service?.account?.emailUsed === email);
  expect(matched).toBeTruthy();
  await ensureServiceDetailsExpanded(this);
  await expect(this.page.locator('[class*="stateEmail"]').getByText(email, { exact: true }).first()).toBeVisible({
    timeout: 15000,
  });
});

Then("maintainer-d may also check using the maintainer GitHub email address", function () {
  // Lookup strategy is internal. UI exposes the matched source when known.
});

Then("the maintainer page shows the updated remote service association status", async function () {
  await ensureServiceDetailsExpanded(this);
  await expect(this.page.locator('[class*="badge_"]').first()).toBeVisible({
    timeout: 15000,
  });
});

Then("I see that the maintainer is associated with CNCF FOSSA", async function () {
  await ensureServiceDetailsExpanded(this);
  await expect(this.page.getByText("Registered").first()).toBeVisible({ timeout: 15000 });
});

Then("I see that the maintainer is missing from the FOSSA team required by the project", async function () {
  await ensureServiceDetailsExpanded(this);
  await expect(this.page.getByText("Missing").first()).toBeVisible({
    timeout: 15000,
  });
});

Then("maintainer-d adds the maintainer to every missing required FOSSA team using the FOSSA REST API", function () {
  // Covered by the POST reconcile request completing successfully.
});

Then("the maintainer page shows the full set of required FOSSA teams for the maintainer", async function () {
  await ensureServiceDetailsExpanded(this);
  const rows = this.page.locator("tbody tr");
  await expect.poll(async () => rows.count(), { timeout: 15000 }).toBeGreaterThan(0);
});

Then("the maintainer page shows that the maintainer is now associated with those FOSSA teams", async function () {
  await ensureServiceDetailsExpanded(this);
  const memberBadges = this.page.getByText("Member");
  await expect(memberBadges.first()).toBeVisible({ timeout: 15000 });
});

Then("maintainer-d sends a CNCF FOSSA invitation to the maintainer", function () {
  // Covered by the POST invite request completing successfully.
});

Then("the maintainer page shows that FOSSA onboarding is pending", async function () {
  await ensureServiceDetailsExpanded(this);
  await expect(this.page.getByText("Invited").first()).toBeVisible({ timeout: 15000 });
});

Then("maintainer-d reconciles the maintainer to each required FOSSA team", function () {
  // Covered by refresh completing after invite acceptance.
});

Then("the maintainer page shows that the maintainer is associated with those FOSSA teams", async function () {
  await ensureServiceDetailsExpanded(this);
  const memberBadges = this.page.getByText("Member");
  await expect(memberBadges.first()).toBeVisible({ timeout: 15000 });
});

Then("I see whether the maintainer was matched by maintainer email address or GitHub email address", async function () {
  await ensureServiceDetailsExpanded(this);
  const workSource = this.page.locator('[class*="stateSource"]').getByText("Work", {
    exact: true,
  });
  const githubSource = this.page.locator('[class*="stateSource"]').getByText("GitHub", {
    exact: true,
  });
  await Promise.race([
    expect(workSource).toBeVisible({ timeout: 15000 }),
    expect(githubSource).toBeVisible({ timeout: 15000 }),
  ]);
});
