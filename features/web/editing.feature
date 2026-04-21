@wip
Feature: Editing permissions and persistence
  As a staff member or project maintainer
  I want to edit data I am allowed to change
  So that updates are saved in the database

  Background:
    Given the maintainer-d database contains staff, maintainers, and projects

  Scenario Outline: Staff can edit any project or maintainer record
    Given I am signed in as staff
    When I edit the "<record_type>" record "<record_name>" with field "<field_name>" set to "<new_value>"
    Then the "<record_type>" record "<record_name>" is updated in the database
    And an audit log entry is recorded for "<record_type>" "<record_name>"

    Examples:
      | record_type | record_name | field_name  | new_value        |
      | project     | Alpha       | description | Core services    |
      | maintainer  | alice       | email       | alice@example.dev  |

  Scenario Outline: Maintainer can edit their own data
    Given I am signed in as a maintainer for project "<project_name>"
    When I edit my maintainer profile with field "<field_name>" set to "<new_value>"
    Then my maintainer profile is updated in the database
    And an audit log entry is recorded for maintainer "<maintainer_login>"

    Examples:
      | project_name | field_name | new_value       | maintainer_login |
      | Alpha        | email      | alice@example.dev | alice            |

  Scenario Outline: Maintainer can edit team data
    Given I am signed in as a maintainer for project "<project_name>"
    When I edit maintainer "<maintainer_login>" for project "<project_name>" with field "<field_name>" set to "<new_value>"
    Then the maintainer record is updated in the database
    And an audit log entry is recorded for maintainer "<maintainer_login>"

    Examples:
      | project_name | maintainer_login | field_name | new_value        |
      | Alpha        | bob              | email      | bob@example.dev    |

  Scenario: Maintainer cannot edit other project data
    Given I am signed in as a maintainer for project "Alpha"
    When I attempt to edit project "Beta"
    Then the change is rejected
    And the project "Beta" record is not updated in the database
    And no audit log entry is recorded for project "Beta"

  Scenario Outline: Audit log captures editor identity and action
    Given I am signed in as "<role>" "<editor_login>"
    When I edit the "<record_type>" record "<record_name>" with field "<field_name>" set to "<new_value>"
    Then an audit log entry is recorded with actor "<editor_login>"
    And the audit log entry action is "<action>"
    And the audit log entry target is "<record_type>" "<record_name>"

    Examples:
      | role      | editor_login | record_type | record_name | field_name  | new_value       | action |
      | staff     | staff-alice  | project     | Alpha       | description | Core services   | update |
      | maintainer| alice        | maintainer  | alice       | email       | alice@example.dev | update |
