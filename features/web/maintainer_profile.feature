Feature: Maintainer profile page
  As an authenticated user
  I want to open a maintainer profile
  So I can see full maintainer details

  Scenario: Navigate from project card to maintainer card
    Given I am signed in as staff
    When I search for "Antonio"
    And I click on maintainer "Antonio Example"
    Then I see the maintainer card for "Antonio Example"

  Scenario: Maintainer card shows GitHub profile location
    Given I am signed in as staff
    When I search for "Antonio"
    And I click on maintainer "Antonio Example"
    Then I see the maintainer card for "Antonio Example"
    And I see the maintainer location "Madrid, Spain"

  Scenario: Maintainer card shows location for an Armenian maintainer
    Given I am signed in as staff
    When I search for "Sam"
    And I click on maintainer "Sam NoEmail"
    Then I see the maintainer card for "Sam NoEmail"
    And I see the maintainer location "Yerevan, Armenia"

  Rule: Staff can inspect and repair maintainer service associations
    - The maintainer page shows what maintainer-d knows about a maintainer's association with remote services
    - The maintainer page shows which project service assignments imply that the maintainer should have remote service access
    - Remote service checks use the maintainer email address and may also use the maintainer GitHub email address
    - From the maintainer page staff can reconcile missing remote service access
    - If a maintainer is not registered with CNCF FOSSA, staff can send a CNCF FOSSA invite from the maintainer page

  Scenario: Staff can view service associations on a maintainer page
    Given I am signed in as a staff member
    And a maintainer exists in maintainer-d
    And the maintainer belongs to one or more projects
    When I open the maintainer page
    Then I see a service associations section
    And I see which remote services the maintainer is associated with
    And I see which project service assignments imply that the maintainer should be associated with those services

  Scenario: Staff can recheck remote service associations after updating a maintainer email
    Given I am signed in as a staff member
    And a maintainer exists in maintainer-d
    And the maintainer is associated with a project that uses FOSSA
    When I update the maintainer email address
    And I save the maintainer record
    And I refresh the maintainer's remote service associations
    Then maintainer-d checks whether the maintainer exists on the remote service using the updated maintainer email address
    And maintainer-d may also check using the maintainer GitHub email address
    And the maintainer page shows the updated remote service association status

  Scenario: Staff can see when a maintainer is known to FOSSA but missing from a required project team
    Given I am signed in as a staff member
    And a maintainer exists in maintainer-d
    And the maintainer belongs to a project that uses FOSSA
    And the maintainer exists in CNCF FOSSA
    And the maintainer is not a member of that project's FOSSA team
    When I open the maintainer page
    Then I see that the maintainer is associated with CNCF FOSSA
    And I see that the maintainer is missing from the FOSSA team required by the project

  Scenario: Staff can reconcile a known FOSSA user to all missing required project teams
    Given I am signed in as a staff member
    And a maintainer exists in maintainer-d
    And the maintainer belongs to multiple active projects that use FOSSA
    And the maintainer exists in CNCF FOSSA
    And the maintainer is missing from one or more required FOSSA teams
    When I reconcile the maintainer's FOSSA access from the maintainer page
    Then maintainer-d adds the maintainer to every missing required FOSSA team using the FOSSA REST API
    And the maintainer page shows the full set of required FOSSA teams for the maintainer
    And the maintainer page shows that the maintainer is now associated with those FOSSA teams

  Scenario: Staff can invite a maintainer to CNCF FOSSA from the maintainer page
    Given I am signed in as a staff member
    And a maintainer exists in maintainer-d
    And the maintainer belongs to a project that uses FOSSA
    And the maintainer does not exist in CNCF FOSSA
    When I send a CNCF FOSSA invite from the maintainer page
    Then maintainer-d sends a CNCF FOSSA invitation to the maintainer
    And the maintainer page shows that FOSSA onboarding is pending

  Scenario: Staff can reconcile FOSSA access after a maintainer accepts an invitation
    Given I am signed in as a staff member
    And a maintainer exists in maintainer-d
    And the maintainer belongs to one or more projects that use FOSSA
    And a CNCF FOSSA invitation is pending for the maintainer
    When the FOSSA invitation is accepted
    And I refresh the maintainer's remote service associations
    Then maintainer-d reconciles the maintainer to each required FOSSA team
    And the maintainer page shows that the maintainer is associated with those FOSSA teams

  Scenario: Staff can see which identifier matched a remote account
    Given I am signed in as a staff member
    And a maintainer exists in maintainer-d
    And the maintainer exists in CNCF FOSSA
    When I open the maintainer page
    Then I see whether the maintainer was matched by maintainer email address or GitHub email address
