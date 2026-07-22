Feature: Generate a MAINTAINERS.yaml for projects without a .project repo
  As a staff member
  I want to generate a draft MAINTAINERS.yaml from a project's active maintainer-d roster
  So I can bootstrap the project's .project repo without maintainer-d writing to GitHub

  Background:
    Given the maintainer-d database contains staff, maintainers, and projects
    And I am signed in as a staff member

  Scenario: Staff sees a generated MAINTAINERS.yaml for a project with no .project repo
    Given a project exists without a .project repo
    When I open the DOT-PROJECT ROLL CALL route for that project
    Then the dot-project roll call shows a generate MAINTAINERS.yaml panel
    And the generated MAINTAINERS.yaml lists the project's active maintainers

  Scenario: Staff can copy the generated MAINTAINERS.yaml to the clipboard
    Given a project exists without a .project repo
    When I open the DOT-PROJECT ROLL CALL route for that project
    And I click "Copy to clipboard" on the generate MAINTAINERS.yaml panel
    Then the system clipboard contains the generated MAINTAINERS.yaml
    And a "Copied" confirmation is shown

  Scenario: The generate panel does not appear once a .project repo is adopted
    Given a project exists with reconciliation routes
    When I open the DOT-PROJECT ROLL CALL route for that project
    Then the dot-project roll call does not show a generate MAINTAINERS.yaml panel
