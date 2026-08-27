Feature: Filter project list by maintainer location
  As an authenticated staff member
  I want to filter the project list by maintainer location, country, or timezone
  So I can answer questions like "which projects have maintainers in Armenia?"

  Background:
    Given I am signed in as staff
    And I am on the home page

  Scenario: Filter projects by maintainer country name
    When I filter the maintainer column by "Armenia"
    Then the project list shows projects with maintainers in "Armenia"
    And the project list does not show projects without maintainers in "Armenia"

  Scenario: Filter projects by maintainer city
    When I filter the maintainer column by "Tokyo"
    Then the project list shows projects with maintainers in "Tokyo"

  Scenario: Filter projects by maintainer timezone
    When I filter the maintainer column by "Asia/Yerevan"
    Then the project list shows projects with maintainers in "Asia/Yerevan"

  Scenario: Filter produces no results for unknown location
    When I filter the maintainer column by "Atlantis"
    Then the project list shows no results
