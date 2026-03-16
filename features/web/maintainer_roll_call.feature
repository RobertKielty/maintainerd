Feature: Maintainer roll call selection
  As a staff member
  I want to select maintainers from the maintainer roll call
  So I can apply bulk or individual status changes

  Background:
    Given the maintainer-d database contains staff, maintainers, and projects
    And I am signed in as a staff member

  Scenario: Staff can select all maintainers and then select an individual maintainer
    Given a project has multiple active maintainers in the maintainer roll call
    When I open the project page
    Then I see the "ROLL CALL" section
    And I see the "PRESENT IN CNCF DATABASE" section
    When I select all maintainers in the roll call
    Then all maintainer checkboxes in the roll call are selected
    And the "Archive" bulk action is enabled for the roll call
    When I clear the roll call selection
    And I select maintainer "Antonio Example" in the roll call
    Then only maintainer "Antonio Example" is selected in the roll call
    And the "Archive" bulk action is enabled for the roll call
