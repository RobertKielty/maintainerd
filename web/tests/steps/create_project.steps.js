const { Given, When, Then } = require("@cucumber/cucumber");
const { expect } = require("@playwright/test");

const signInAsStaff = async (world) => {
  const login = process.env.TEST_STAFF_LOGIN || "staff-tester";
  await world.page.goto(
    `${world.bffBaseUrl}/auth/test-login?login=${encodeURIComponent(login)}`,
    { waitUntil: "domcontentloaded" }
  );
  await world.page.goto(world.baseUrl, { waitUntil: "domcontentloaded" });
  world.currentRole = "staff";
};

const loadOnboardingIssue = async (world) => {
  const response = await world.page.request.get(`${world.bffBaseUrl}/api/onboarding/issues`);
  if (!response.ok()) {
    throw new Error(`Failed to load onboarding issues: ${response.status()}`);
  }
  const data = await response.json();
  const issues = data.issues || [];
  if (issues.length === 0) {
    throw new Error("No onboarding issues available for create project BDD test.");
  }
  const preferred = issues.find((issue) => issue.projectName) || issues[0];
  let projectName = preferred.projectName;
  if (!projectName) {
    const resolveResponse = await world.page.request.post(
      `${world.bffBaseUrl}/api/onboarding/resolve`,
      { data: { issueUrl: preferred.url } }
    );
    if (!resolveResponse.ok()) {
      throw new Error(`Failed to resolve onboarding issue: ${resolveResponse.status()}`);
    }
    const resolved = await resolveResponse.json();
    projectName = resolved.projectName;
  }
  if (!projectName) {
    throw new Error("Unable to resolve project name from onboarding issue.");
  }
  return { issue: preferred, projectName };
};

Given("I am signed in as a staff member", async function () {
  await signInAsStaff(this);
});

When("I open the Create Project dialog", async function () {
  await this.page.goto(`${this.baseUrl}/create/project`, { waitUntil: "domcontentloaded" });
  await expect(
    this.page.getByRole("heading", { name: "Create New Project" })
  ).toBeVisible({ timeout: 15000 });
});

When("I enter the onboarding issue URL for a sandbox project", async function () {
  const { issue, projectName } = await loadOnboardingIssue(this);
  this.onboardingIssue = issue.url;
  this.projectName = projectName;
  const input = this.page.getByPlaceholder("Search open onboarding issues or paste URL");
  await expect(input).toBeVisible({ timeout: 15000 });
  await input.fill(issue.url);
});

Then("the project name is resolved from the onboarding issue title", async function () {
  const resolving = this.page.getByText("Resolving issue title…");
  await resolving.waitFor({ state: "hidden", timeout: 15000 });
  await expect(
    this.page.getByText("Unable to extract project name from issue title.")
  ).toHaveCount(0);
  await expect(
    this.page.getByTestId("project-name")
  ).toHaveValue(this.projectName, { timeout: 15000 });
});

Then("I can provide either a legacy maintainer file or a dot project YAML URL", async function () {
  const legacyUrl = "https://github.com/example-org/example/blob/main/MAINTAINERS.md";
  this.expectedOrg = "example-org";
  await this.page.getByLabel("Legacy Maintainer File").fill(legacyUrl);
});

Then("the GitHub org is inferred from the provided file URL", async function () {
  await expect(
    this.page.getByText("GitHub Org could not be inferred from the file URLs.")
  ).toHaveCount(0);
  await expect(
    this.page.getByTestId("github-org")
  ).toHaveValue(this.expectedOrg, { timeout: 15000 });
});

When("I submit the form", async function () {
  const responsePromise = this.page.waitForResponse(
    (response) =>
      response.url().includes("/api/projects") && response.request().method() === "POST",
    { timeout: 20000 }
  );
  await this.page.getByRole("button", { name: "Create New Project" }).click();
  const response = await responsePromise;
  if (!response.ok()) {
    const body = await response.text();
    throw new Error(`Create project failed: ${response.status()} ${body}`);
  }
  await this.page.waitForURL(/\/projects\/\d+$/, { timeout: 20000 });
});

Then("a new project record is created", async function () {
  await expect(
    this.page.getByRole("heading", { name: this.projectName })
  ).toBeVisible({ timeout: 15000 });
});

When(/I enter an onboarding issue URL outside cncf\/sandbox/, async function () {
  if (!this.page.url().includes("/create/project")) {
    await this.page.goto(`${this.baseUrl}/create/project`, { waitUntil: "domcontentloaded" });
  }
  await expect(
    this.page.getByRole("heading", { name: "Create New Project" })
  ).toBeVisible({ timeout: 20000 });
  const input = this.page.getByPlaceholder("Search open onboarding issues or paste URL");
  await expect(input).toBeVisible({ timeout: 15000 });
  await input.fill("https://github.com/cncf/foundation/issues/1");
  await input.blur();
});

Then(/I am prompted to use a cncf\/sandbox issue URL/, async function () {
  await expect(
    this.page.getByText("Select a cncf/sandbox onboarding issue.")
  ).toBeVisible({ timeout: 15000 });
});
