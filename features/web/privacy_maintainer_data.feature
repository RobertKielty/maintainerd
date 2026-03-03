Feature: Maintainer email privacy
  As an authenticated user
  I want maintainer email addresses protected
  So that only staff or the maintainer can see them

  Background:
    Given the maintainer-d database contains staff, maintainers, and projects

  Scenario: Maintainer can view their own email
    Given I am signed in as maintainer "self"
    When I open the maintainer profile for "self"
    Then the maintainer email is visible

  Scenario: Maintainer can view all projects
    Given I am signed in as maintainer "self"
    When I view the projects list
    Then project "Project Atlas" is visible

  Scenario: Maintainer cannot view another maintainer email
    Given I am signed in as maintainer "self"
    When I open the maintainer profile for "other"
    Then the maintainer email is hidden

  Scenario: Maintainer can view another maintainer profile without email
    Given I am signed in as maintainer "self"
    When I open the maintainer profile for "other"
    Then the maintainer email is hidden

  Scenario: Maintainer can update their own email and company affiliation
    Given I am signed in as maintainer "self"
    When I open the maintainer profile for "self"
    And I edit my maintainer profile with field "email" set to "antonio.updated@test.dev"
    And I edit my maintainer profile with field "company" set to "No company"
    And I save my maintainer profile changes
    Then my maintainer profile shows email "antonio.updated@test.dev"
    And my maintainer profile shows no company

  Scenario: Staff can view any maintainer email
    Given I am signed in as staff
    When I open the maintainer profile for "other"
    Then the maintainer email is visible
