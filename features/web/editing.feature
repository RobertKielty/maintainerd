Feature: Editing permissions and persistence
  As a staff member
  I want to edit maintainer and project data
  So that updates are saved in the database

  Background:
    Given the maintainer-d database contains staff, maintainers, and projects

  Scenario Outline: Staff can edit any project or maintainer record
    Given I am signed in as staff
    When I edit the "<record_type>" record "<record_name>" with field "<field_name>" set to "<new_value>"
    Then the "<record_type>" record "<record_name>" is updated in the database
    And an audit log entry is recorded for "<record_type>" "<record_name>"

    Examples:
      | record_type | record_name      | field_name  | new_value                    |
      | maintainer  | antonio-example  | githubEmail | antonio.commits@example.dev  |
      | maintainer  | renee-sample     | location    | Berlin, Germany              |
      | maintainer  | alex-example     | email       | alex.updated@example.dev     |
      | project     | Project Beacon   | maturity    | Graduated                    |

  Scenario Outline: Audit log captures editor identity and action
    Given I am signed in as "<role>" "<editor_login>"
    When I edit the "<record_type>" record "<record_name>" with field "<field_name>" set to "<new_value>"
    Then an audit log entry is recorded with actor "<editor_login>"
    And the audit log entry action is "<action>"
    And the audit log entry target is "<record_type>" "<record_name>"

    Examples:
      | role  | editor_login | record_type | record_name     | field_name  | new_value                   | action                  |
      | staff | staff-tester | maintainer  | antonio-example | githubEmail | antonio.commits@example.dev | MAINTAINER_UPDATE       |
      | staff | staff-tester | project     | Project Beacon  | maturity    | Graduated                   | PROJECT_MATURITY_UPDATE |
