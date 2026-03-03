Feature: Global maintainer search
  As a staff member
  I want to search across all projects for maintainers using email addresses
  So that I can quickly find and manage maintainer records

  Background:
    Given I am signed in as a staff member

  Scenario: Search by exact email match
    Given the maintainer-d database contains staff, maintainers, and projects
    When I search globally for maintainer email "alex@example.dev"
    Then I see maintainer "Alex Example" in the results
    And the result shows the maintainer email "alex@example.dev"

  Scenario: Search by email is case-insensitive
    Given the maintainer-d database contains staff, maintainers, and projects
    When I search globally for maintainer email "ALEX@EXAMPLE.DEV"
    Then I see maintainer "Alex Example" in the results

  Scenario: Search by partial email
    Given the maintainer-d database contains staff, maintainers, and projects
    When I search globally for maintainer email "example.dev"
    Then I see maintainer "Alex Example" in the results
    And I see maintainer "Renee Sample" in the results

  Scenario: No results for unknown email
    Given the maintainer-d database contains staff, maintainers, and projects
    When I search globally for maintainer email "missing@example.dev"
    Then I see an empty state indicating no maintainers were found

  Scenario: Search results include project associations
    Given the maintainer-d database contains staff, maintainers, and projects
    When I search globally for maintainer email "alex@example.dev"
    Then the results include the projects associated with "Alex Example"

  Scenario: Non-staff users cannot access global email search
    Given I am signed in as maintainer "self"
    When I try to access global maintainer search by email
    Then I am denied access to global maintainer search
