Feature: Filtering the audit log
  As a staff user
  I want to filter the audit log by time range, action, and target
  So I can find specific staff actions quickly
  And DOT-PROJECT sync run events should live on their own page

  Background:
    Given the maintainer-d database contains staff, maintainers, and projects
    And I am signed in as staff

  Scenario: Staff filters the audit log by action
    When I edit the "project" record "Project Beacon" with field "maturity" set to "Graduated"
    And I open the audit log page
    And I filter the audit log by action "PROJECT_MATURITY_UPDATE"
    Then every visible audit log row has action "PROJECT_MATURITY_UPDATE"
    And the audit log table includes a row targeting project "Project Beacon"

  Scenario: Staff filters the audit log by target
    When I edit the "project" record "Project Beacon" with field "maturity" set to "Incubating"
    And I open the audit log page
    And I filter the audit log by target "Project Beacon"
    Then the audit log table includes a row targeting project "Project Beacon"
    And every visible audit log row targets project "Project Beacon"

  Scenario: Staff filters the audit log by time range
    When I edit the "project" record "Project Beacon" with field "maturity" set to "Sandbox"
    And I open the audit log page
    And I filter the audit log to only include events since 1 day ago
    Then the audit log table includes a row targeting project "Project Beacon"
    When I filter the audit log to only include events before 1 day ago
    Then the audit log table does not include a row targeting project "Project Beacon"

  Scenario: Sync run events are separated onto their own page
    When I open the audit log page
    Then the audit log table shows no "DOT_PROJECT_SYNC_RUN_STARTED" rows
    When I open the sync runs page
    Then every visible audit log row has an action starting with "DOT_PROJECT_SYNC_RUN"
