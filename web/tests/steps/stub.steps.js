const { Given, When, Then } = require("@cucumber/cucumber");

const pending = function () {
  return "pending";
};

Given("the maintainer-d database contains staff and maintainer GitHub accounts", pending);
Given("a staff GitHub account {string} exists in the database", pending);
Given("a maintainer GitHub account {string} exists in the database", pending);
Given("no GitHub account {string} exists in the database", pending);
Given("I have recently visited projects, maintainers, and services", pending);
Given("I have logged out", pending);

Given("I am signed in as staff {string}", async function (login) {
  await this.page.goto(
    `${this.bffBaseUrl}/auth/test-login?login=${encodeURIComponent(login)}`,
    { waitUntil: "domcontentloaded" }
  );
  await this.page.goto(this.baseUrl);
});

Given("I am signed in as {string} {string}", async function (_role, login) {
  await this.page.goto(
    `${this.bffBaseUrl}/auth/test-login?login=${encodeURIComponent(login)}`,
    { waitUntil: "domcontentloaded" }
  );
  await this.page.goto(this.baseUrl);
});

Given("I am signed in as a maintainer for project {string}", async function (_project) {
  void _project;
  const login = process.env.TEST_MAINTAINER_LOGIN || "antonio-example";
  await this.page.goto(
    `${this.bffBaseUrl}/auth/test-login?login=${encodeURIComponent(login)}`,
    { waitUntil: "domcontentloaded" }
  );
  await this.page.goto(this.baseUrl);
});

When("I view project {string}", pending);
When("I view the staff dashboard", pending);
When("I view the staff dashboard", pending);
When("I attempt to view project {string}", pending);
When("I attempt to edit project {string}", pending);
When(
  "I edit maintainer {string} for project {string} with field {string} set to {string}",
  pending
);
When("I click the log out button", pending);
When("I attempt to access a protected page", pending);


Then("I can see all projects", pending);
Then("I can see all maintainers", pending);
Then("maintainer {string} is visible", pending);
Then("I can see all services the project is setup on", pending);
Then("I can see project {string} data", pending);
Then("I can see maintainer records for project {string}", pending);
Then("I am denied access to project {string}", pending);
Then("the change is rejected", pending);
Then("the project {string} record is not updated in the database", pending);
Then("no audit log entry is recorded for project {string}", pending);
Then("my maintainer profile is updated in the database", pending);
Then("the maintainer record is updated in the database", pending);
Then("I see search results that include {string}", pending);
Then("I can open the {string} result {string}", pending);
Then("I see a list of recently visited projects", pending);
Then("I see a list of recently visited maintainers", pending);
Then("I see a list of recently visited services", pending);
Then("the recently visited projects include {string}", pending);
Then("the recently visited maintainers include {string}", pending);
Then("the recently visited services include {string}", pending);
Then(
  "I see a summary of the full maintainer count, full project count, graduated project count, incubating project count, sandbox project count",
  pending
);
Then("the user is authenticated", pending);
Then("the user is authorized as staff", pending);
Then("the user is authorized as a project maintainer", pending);
Then("the user is denied access", pending);
Then("the user is shown an unauthorized message", pending);
Then("my session is terminated", pending);
Then("I am returned to the sign-in page", pending);
Then("I am redirected to the sign-in page", pending);
