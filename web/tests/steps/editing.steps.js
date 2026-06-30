const { When, Then } = require("@cucumber/cucumber");
const { expect } = require("@playwright/test");

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Resolve a maintainer's numeric id by their GitHub login.
 * Uses GET /api/search since that endpoint returns id + github/githubAccount.
 */
const resolveMaintainerIdByLogin = async (world, login) => {
  const response = await world.page.request.get(
    `${world.bffBaseUrl}/api/search?query=${encodeURIComponent(login)}`
  );
  if (!response.ok()) {
    throw new Error(`Failed to search for maintainer "${login}": ${response.status()}`);
  }
  const payload = await response.json();
  const maintainers = Array.isArray(payload.maintainers) ? payload.maintainers : [];
  const match = maintainers.find(
    (m) =>
      m.github === login ||
      m.githubAccount === login ||
      m.name === login
  ) || maintainers[0];
  if (!match?.id) {
    throw new Error(`No maintainer found for login "${login}"`);
  }
  return String(match.id);
};

/**
 * Resolve a project's numeric id by its exact name.
 * Uses GET /api/projects?namePrefix= — picks the first exact name match.
 */
const resolveProjectIdByName = async (world, name) => {
  const response = await world.page.request.get(
    `${world.bffBaseUrl}/api/projects?namePrefix=${encodeURIComponent(name)}&limit=10`
  );
  if (!response.ok()) {
    throw new Error(`Failed to list projects for "${name}": ${response.status()}`);
  }
  const payload = await response.json();
  const projects = Array.isArray(payload.projects) ? payload.projects : [];
  const match = projects.find((p) => p.name === name) || projects[0];
  if (!match?.id) {
    throw new Error(`No project found with name "${name}"`);
  }
  return String(match.id);
};

/**
 * Fetch recent audit logs (staff-only endpoint).
 * Returns the logs array from GET /api/audit.
 */
const fetchAuditLogs = async (world) => {
  const response = await world.page.request.get(
    `${world.bffBaseUrl}/api/audit?limit=200`
  );
  if (!response.ok()) {
    throw new Error(`Failed to fetch audit log: ${response.status()}`);
  }
  const payload = await response.json();
  return Array.isArray(payload.logs) ? payload.logs : [];
};

// ---------------------------------------------------------------------------
// Field name → accessible label mapping for maintainer edit form inputs
// ---------------------------------------------------------------------------
const maintainerFieldLabel = {
  email: { name: "Email", exact: true },
  githubEmail: { name: "GitHub Email", exact: true },
  location: { name: "Location", exact: true },
  name: { name: "Name", exact: true },
  github: { name: "GitHub account", exact: true },
};

// ---------------------------------------------------------------------------
// Steps
// ---------------------------------------------------------------------------

When(
  "I edit the {string} record {string} with field {string} set to {string}",
  async function (recordType, recordName, field, value) {
    if (recordType === "maintainer") {
      const id = await resolveMaintainerIdByLogin(this, recordName);
      await this.page.goto(`${this.baseUrl}/maintainers/${id}`);

      // Open the edit panel
      const editCard = this.page
        .getByRole("heading", { name: "Update maintainer" })
        .first()
        .locator("..");
      await expect(editCard).toBeVisible({ timeout: 15000 });
      const editButton = editCard.getByRole("button", { name: "Edit" });
      if (await editButton.isVisible()) {
        await editButton.click();
      }

      // Fill the field
      const labelOpts = maintainerFieldLabel[field];
      if (!labelOpts) {
        throw new Error(`Unsupported maintainer field for editing: "${field}"`);
      }
      const input = this.page.getByRole("textbox", labelOpts);
      await expect(input).toBeEnabled({ timeout: 15000 });
      await input.fill(value);

      // Save and wait for PATCH response
      const saveButton = editCard.getByRole("button", { name: "Save changes" });
      await expect(saveButton).toBeEnabled({ timeout: 15000 });
      const responsePromise = this.page.waitForResponse(
        (response) =>
          response.request().method() === "PATCH" &&
          /\/api\/maintainers\/\d+$/.test(response.url()),
        { timeout: 20000 }
      );
      await saveButton.click();
      const patchResponse = await responsePromise;
      if (!patchResponse.ok()) {
        const body = await patchResponse.text();
        throw new Error(
          `PATCH /api/maintainers/${id} failed: ${patchResponse.status()} ${body}`
        );
      }

      this.lastEdit = { recordType, recordName, id, field, value };
    } else if (recordType === "project") {
      const id = await resolveProjectIdByName(this, recordName);
      await this.page.goto(`${this.baseUrl}/projects/${id}`);

      if (field === "maturity") {
        // Maturity is edited via a modal: click "MOVE LEVEL" → modal opens → click the target level button
        const moveLevelButton = this.page.getByRole("button", { name: "MOVE LEVEL" });
        await expect(moveLevelButton).toBeVisible({ timeout: 15000 });

        const responsePromise = this.page.waitForResponse(
          (response) =>
            response.request().method() === "PATCH" &&
            response.url().includes(`/api/projects/${id}/maturity`),
          { timeout: 20000 }
        );
        await moveLevelButton.click();

        // Modal opens — click the button with the target maturity label
        const modal = this.page.getByRole("dialog");
        await expect(modal).toBeVisible({ timeout: 10000 });
        const targetButton = modal.getByRole("button", { name: value, exact: true });
        await expect(targetButton).toBeEnabled({ timeout: 10000 });
        await targetButton.click();

        const patchResponse = await responsePromise;
        if (!patchResponse.ok()) {
          const body = await patchResponse.text();
          throw new Error(
            `PATCH /api/projects/${id}/maturity failed: ${patchResponse.status()} ${body}`
          );
        }
      } else {
        throw new Error(`Unsupported project field for editing: "${field}"`);
      }

      this.lastEdit = { recordType, recordName, id, field, value };
    } else {
      throw new Error(`Unsupported record type: "${recordType}"`);
    }
  }
);

Then(
  "the {string} record {string} is updated in the database",
  async function (recordType, _recordName) {
    if (!this.lastEdit) {
      throw new Error("No lastEdit stored — did the When step run?");
    }
    const { id, field, value } = this.lastEdit;

    if (recordType === "maintainer") {
      const response = await this.page.request.get(
        `${this.bffBaseUrl}/api/maintainers/${id}`
      );
      if (!response.ok()) {
        throw new Error(`GET /api/maintainers/${id} returned ${response.status()}`);
      }
      const data = await response.json();
      const actual = data[field];
      if (actual !== value) {
        throw new Error(
          `Expected maintainer field "${field}" to be "${value}" but got "${actual}"`
        );
      }
    } else if (recordType === "project") {
      const response = await this.page.request.get(
        `${this.bffBaseUrl}/api/projects/${id}`
      );
      if (!response.ok()) {
        throw new Error(`GET /api/projects/${id} returned ${response.status()}`);
      }
      const data = await response.json();
      const actual = data[field];
      if (actual !== value) {
        throw new Error(
          `Expected project field "${field}" to be "${value}" but got "${actual}"`
        );
      }
    } else {
      throw new Error(`Unsupported record type: "${recordType}"`);
    }
  }
);

Then(
  "an audit log entry is recorded for {string} {string}",
  async function (recordType, _recordName) {
    if (!this.lastEdit) {
      throw new Error("No lastEdit stored — did the When step run?");
    }
    const { id } = this.lastEdit;
    const logs = await fetchAuditLogs(this);

    const idNum = Number(id);
    let found;
    if (recordType === "maintainer") {
      found = logs.find((l) => l.maintainerId === idNum);
    } else if (recordType === "project") {
      found = logs.find((l) => l.projectId === idNum);
    } else {
      throw new Error(`Unsupported record type: "${recordType}"`);
    }

    if (!found) {
      throw new Error(
        `No audit log entry found for ${recordType} id=${id}`
      );
    }
  }
);

Then(
  "an audit log entry is recorded with actor {string}",
  async function (editorLogin) {
    if (!this.lastEdit) {
      throw new Error("No lastEdit stored — did the When step run?");
    }
    const { id, recordType } = this.lastEdit;
    const logs = await fetchAuditLogs(this);

    const idNum = Number(id);
    let entry;
    if (recordType === "maintainer") {
      entry = logs.find(
        (l) =>
          l.maintainerId === idNum &&
          l.staffLogin &&
          l.staffLogin.toLowerCase() === editorLogin.toLowerCase()
      );
    } else if (recordType === "project") {
      entry = logs.find(
        (l) =>
          l.projectId === idNum &&
          l.staffLogin &&
          l.staffLogin.toLowerCase() === editorLogin.toLowerCase()
      );
    } else {
      throw new Error(`Unsupported record type: "${recordType}"`);
    }

    if (!entry) {
      throw new Error(
        `No audit log entry found for ${recordType} id=${id} with staffLogin="${editorLogin}"`
      );
    }
    // Store for subsequent Then steps
    this.lastAuditEntry = entry;
  }
);

Then("the audit log entry action is {string}", async function (expectedAction) {
  if (!this.lastAuditEntry) {
    // Fall back to fetching and finding by lastEdit
    if (!this.lastEdit) {
      throw new Error("No lastEdit or lastAuditEntry stored");
    }
    const { id, recordType } = this.lastEdit;
    const logs = await fetchAuditLogs(this);
    const idNum = Number(id);
    if (recordType === "maintainer") {
      this.lastAuditEntry = logs.find((l) => l.maintainerId === idNum);
    } else {
      this.lastAuditEntry = logs.find((l) => l.projectId === idNum);
    }
  }

  if (!this.lastAuditEntry) {
    throw new Error("No audit entry found to check action against");
  }

  const actual = this.lastAuditEntry.action;
  if (actual !== expectedAction) {
    throw new Error(
      `Expected audit action "${expectedAction}" but got "${actual}"`
    );
  }
});

Then(
  "the audit log entry target is {string} {string}",
  async function (recordType, _recordName) {
    if (!this.lastEdit) {
      throw new Error("No lastEdit stored — did the When step run?");
    }
    const { id } = this.lastEdit;
    const idNum = Number(id);

    if (!this.lastAuditEntry) {
      const logs = await fetchAuditLogs(this);
      if (recordType === "maintainer") {
        this.lastAuditEntry = logs.find((l) => l.maintainerId === idNum);
      } else if (recordType === "project") {
        this.lastAuditEntry = logs.find((l) => l.projectId === idNum);
      }
    }

    if (!this.lastAuditEntry) {
      throw new Error(
        `No audit log entry found targeting ${recordType} id=${id}`
      );
    }

    if (recordType === "maintainer") {
      if (this.lastAuditEntry.maintainerId !== idNum) {
        throw new Error(
          `Expected audit entry maintainerId=${idNum} but got ${this.lastAuditEntry.maintainerId}`
        );
      }
    } else if (recordType === "project") {
      if (this.lastAuditEntry.projectId !== idNum) {
        throw new Error(
          `Expected audit entry projectId=${idNum} but got ${this.lastAuditEntry.projectId}`
        );
      }
    } else {
      throw new Error(`Unsupported record type: "${recordType}"`);
    }
  }
);
