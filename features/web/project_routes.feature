Feature: Project route navigation
  As a staff member
  I want project reconciliation workflows to live on dedicated routes
  So I can navigate directly to each route-scoped section

  Background:
    Given the maintainer-d database contains staff, maintainers, and projects
    And I am signed in as a staff member

  Scenario: Staff can navigate the project reconciliation routes
    Given a project exists with reconciliation routes
    When I open the project github route
    Then the project route navigation exposes these routes
      | label                         | path-suffix            | heading                       | body-snippet                                                                     |
      | LEGACY ROLL CALL              | /github                | LEGACY ROLL CALL              | This project has a maintainer file in its .project repo.                         |
      | DOT-PROJECT ROLL CALL         | /dot-project           | DOT-PROJECT ROLL CALL         | Formatted maintainer file                                                       |
      | LICENSE CHECKER - FOSSA       | /fossa                 | LICENSE CHECKER - FOSSA       | This project has not selected a license checker.                                 |
      | MAILING LISTS / MAINTAINERS   | /mailing-maintainers   | MAILING LISTS / MAINTAINERS   | Placeholder for maintainer mailing list references                               |
      | MAILING LISTS / SECURITY      | /mailing-security      | MAILING LISTS / SECURITY      | Placeholder for security mailing list references.                                |
      | DOCUMENTATION                 | /docs                  | DOCUMENTATION                 | Placeholder for documentation hosting details.                                   |
      | COLLABORATION / SLACK         | /slack                 | COLLABORATION / SLACK         | Placeholder for Slack workspace/channel references.                              |
      | COLLABORATION / DISCORD       | /discord               | COLLABORATION / DISCORD       | Placeholder for Discord server/channel references.                               |

  Scenario: Staff can see persisted dot-project migration status on roll call routes
    Given a project exists with reconciliation routes
    When I open the project github route
    Then the legacy roll call shows a dot-project migration note
    And the project route navigation shows a red X on LEGACY ROLL CALL
    And the project route navigation shows a green tick on DOT-PROJECT ROLL CALL

  Scenario: Staff can review the persisted dot-project roll call
    Given a project exists with reconciliation routes
    When I open the project github route
    And I open the DOT-PROJECT ROLL CALL route
    Then the dot-project roll call shows the persisted discovery summary
    And the dot-project roll call lists the tracked .project files
    And the dot-project roll call renders the cached maintainer file as formatted YAML
