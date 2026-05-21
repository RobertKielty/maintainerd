Feature: Protected deep-link routes
  As an unauthenticated user
  I want protected deep links to send me through sign-in and preserve my destination
  So I can land back on the route I originally opened

  Background:
    Given the maintainer-d database contains staff, maintainers, and projects
    And I am signed out

  Scenario: Unauthenticated project route visits preserve the deep link through sign-in
    Given a project exists with reconciliation routes
    When I open a protected project route while signed out
    Then I am redirected to sign in with a next parameter for the protected route
    When I complete sign-in for the protected route
    Then I land on the original protected project route

  Scenario: Unauthenticated maintainer page visits preserve the deep link through sign-in
    When I open a protected maintainer page while signed out
    Then I am redirected to sign in with a next parameter for the protected route
    When I complete sign-in for the protected route
    Then I land on the original protected maintainer page

  Scenario: Unauthenticated company page visits preserve the deep link through sign-in
    When I open a protected company page while signed out
    Then I am redirected to sign in with a next parameter for the protected route
    When I complete sign-in for the protected route
    Then I land on the original protected company page

  Scenario: Unauthenticated search deep links preserve the query through sign-in
    When I open a protected search page with a query while signed out
    Then I am redirected to sign in with a next parameter for the protected route
    When I complete sign-in for the protected route
    Then I land on the original protected search page with the query intact
