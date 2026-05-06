/* eslint-disable @typescript-eslint/no-require-imports */
const path = require("node:path");

const reportsDir = process.env.WEB_BDD_REPORT_DIR || path.join("..", "testdata");
const jsonReportPath =
  process.env.WEB_BDD_JSON_PATH || path.join(reportsDir, "web-bdd-report.json");
const junitReportPath =
  process.env.WEB_BDD_JUNIT_PATH || path.join(reportsDir, "web-bdd-results.xml");

const useMicrocks = process.env.WEB_BDD_USE_MICROCKS === "true";
const tags = useMicrocks ? "not @wip" : "not @wip and not @microcks";

module.exports = {
  default: {
    paths: process.env.WEB_BDD_FEATURE
      ? [process.env.WEB_BDD_FEATURE]
      : ["../features/web/**/*.feature"],
    require: ["tests/steps/**/*.js"],
    tags,
    format: [
      "progress",
      `json:${jsonReportPath}`,
      `junit:${junitReportPath}`,
    ],
    publishQuiet: true,
  },
};
