Feature: Filter project list by dot-project repo
  As an authenticated staff member
  I want to filter the project list by dot-project repo adoption
  So I can quickly find projects that have moved to the .project repo workflow

  Background:
    Given I am signed in as staff
    And I am on the home page

  Scenario: Filter projects with dot-project repos
    When I filter the project list by dot-project repo "adopted!"
    Then the project list shows projects with dot-project repos

  Scenario: Filter projects without dot-project repos
    When I filter the project list by dot-project repo "legacy file"
    Then the project list shows projects without dot-project repos
