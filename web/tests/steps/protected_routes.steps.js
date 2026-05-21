const { Given, When, Then } = require("@cucumber/cucumber");
const { expect } = require("@playwright/test");

const routeProjectName = "Project Atlas";
const companyName = "Example Labs";
const maintainerName = "Antonio Example";
const protectedSearchQuery = "alex@example.dev";

const signInAsStaffRequest = async (world) => {
  const login = process.env.TEST_STAFF_LOGIN || "staff-tester";
  const response = await world.page.request.get(
    `${world.bffBaseUrl}/auth/test-login?login=${encodeURIComponent(login)}`
  );
  if (!response.ok()) {
    throw new Error(`Failed to sign in test staff user: ${response.status()}`);
  }
};

const signOutRequest = async (world) => {
  await world.page.request.post(`${world.bffBaseUrl}/auth/logout`);
  await world.context.clearCookies();
};

const resolveSearchEntityId = async (world, query, field, expectedValue) => {
  await signInAsStaffRequest(world);
  const response = await world.page.request.get(
    `${world.bffBaseUrl}/api/search?query=${encodeURIComponent(query)}`
  );
  if (!response.ok()) {
    throw new Error(`Failed to search for ${field}: ${response.status()}`);
  }
  const data = await response.json();
  const collection = Array.isArray(data[field]) ? data[field] : [];
  const entity = collection.find((item) => item.name === expectedValue);
  await signOutRequest(world);
  if (!entity || !entity.id) {
    throw new Error(`Unable to resolve ${field} ID for ${expectedValue}`);
  }
  return entity.id;
};

const captureAuthRedirect = async (world, protectedPath) => {
  let authLoginRequestURL = null;
  const handleRequest = (request) => {
    if (request.url().startsWith(`${world.bffBaseUrl}/auth/login`)) {
      authLoginRequestURL = request.url();
    }
  };

  await world.page.route("https://github.com/**", async (route) => {
    await route.abort();
  });
  world.page.on("request", handleRequest);

  try {
    await world.page.goto(`${world.baseUrl}${protectedPath}`, {
      waitUntil: "domcontentloaded",
      timeout: 15000,
    });
  } catch {
    // The browser may abort once the OAuth redirect leaves the local app.
  }

  await expect
    .poll(() => authLoginRequestURL, {
      timeout: 15000,
      message: "Expected the app to request /auth/login for the protected route",
    })
    .toBeTruthy();

  world.protectedPath = protectedPath;
  world.authLoginRequestURL = authLoginRequestURL;

  world.page.off("request", handleRequest);
  await world.page.unroute("https://github.com/**");
};

Given("I am signed out", async function () {
  await signOutRequest(this);
});

When("I open a protected project route while signed out", async function () {
  this.projectId = await resolveSearchEntityId(this, routeProjectName, "projects", routeProjectName);
  await captureAuthRedirect(this, `/projects/${this.projectId}/github`);
});

When("I open a protected maintainer page while signed out", async function () {
  this.maintainerId = await resolveSearchEntityId(this, maintainerName, "maintainers", maintainerName);
  await captureAuthRedirect(this, `/maintainers/${this.maintainerId}`);
});

When("I open a protected company page while signed out", async function () {
  this.companyId = await resolveSearchEntityId(this, companyName, "companies", companyName);
  await captureAuthRedirect(this, `/companies/${this.companyId}`);
});

When("I open a protected search page with a query while signed out", async function () {
  await captureAuthRedirect(this, `/search?query=${encodeURIComponent(protectedSearchQuery)}`);
});

Then("I am redirected to sign in with a next parameter for the protected route", async function () {
  expect(this.authLoginRequestURL).toContain("/auth/login?next=");
  expect(this.authLoginRequestURL).toContain(
    `next=${encodeURIComponent(this.protectedPath)}`
  );
});

When("I complete sign-in for the protected route", async function () {
  await signInAsStaffRequest(this);
  await this.page.goto(`${this.baseUrl}${this.protectedPath}`, {
    waitUntil: "domcontentloaded",
  });
});

Then("I land on the original protected project route", async function () {
  await expect(this.page).toHaveURL(
    new RegExp(`/projects/${this.projectId}/github$`),
    { timeout: 15000 }
  );
  await expect(
    this.page.getByRole("heading", { name: routeProjectName })
  ).toBeVisible({ timeout: 15000 });
});

Then("I land on the original protected maintainer page", async function () {
  await expect(this.page).toHaveURL(
    new RegExp(`/maintainers/${this.maintainerId}$`),
    { timeout: 15000 }
  );
  await expect(
    this.page.getByRole("heading", { name: maintainerName })
  ).toBeVisible({ timeout: 15000 });
});

Then("I land on the original protected company page", async function () {
  await expect(this.page).toHaveURL(
    new RegExp(`/companies/${this.companyId}$`),
    { timeout: 15000 }
  );
  await expect(
    this.page.getByRole("heading", { name: companyName })
  ).toBeVisible({ timeout: 15000 });
});

Then("I land on the original protected search page with the query intact", async function () {
  await expect(this.page).toHaveURL(
    new RegExp(`/search\\?query=${encodeURIComponent(protectedSearchQuery)}`),
    { timeout: 15000 }
  );
  await expect(
    this.page.getByText(`Query: “${protectedSearchQuery}”`)
  ).toBeVisible({ timeout: 15000 });
  await expect(
    this.page.getByRole("link", { name: "Alex Example" })
  ).toBeVisible({ timeout: 15000 });
});
