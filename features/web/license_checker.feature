Feature: License checker project section
  As a CNCF staff member 
  I want to sign up project maintainers to use a license checker so that the project can comply with the CNCF's Third Party License Policy defined here https://github.com/cncf/foundation/blob/main/policies-guidance/allowed-third-party-license-policy.md#cncf-allowlist-license-policy
  I want to view FOSSA or SNYK team members and invite missing maintainers
  So that the chosen license scanner tool access stays aligned with the project's maintainer roster

  Background:
    Given I am signed in as a staff member

  Rule: Automated compliance with the CNCF's 3rd Party License Policy
    - A Project MUST sign up to either CNCF FOSSA or CNCF Snyk instances
    - Projects are signed up to a CNCF License Checker service during sandbox onboarding or onboarding at a higher maturity level
    - Projects MUST scan all repositories in their GitHub Org for license policy compliance 

  Rule: Source of truth and reporting
    - The maintainer roster in maintainerd is the source of truth that determines who is given user accounts on our license checkers
    - A ServiceTeam record indicates the project has chosen that service in the past. 
    - The UI reports the ServiceTeam data on file.
    - FOSSA team membership is the source of truth for who is already onboarded.
    - Maintainers with pending invites are excluded from the ACTIVE MAINTAINERS ELIGABLE FOR INVITATION list and shown in PENDING INVITATIONS.

  Rule: For projects that have chosen FOSSA
    - FOSSA invitations expire after 72 hours so we must record invitations sent in the database and for each invitation we record
      - time sent
      - maintainer id
      - project id
      - service id
      - service_team id
      - invite status (pending, accepted, expired, error)
      - last checked time
      - last error message (if any)
    - FOSSA invitations are not tied to a team. After an invite is accepted we must add the user to the FOSSA team.
    - When a FOSSA invite expires, we must re-issue the invite.
    - For projects that selected FOSSA, we report known states from our database namely:
      - We show a reference to the team on FOSSA
      - We list the maintainers that have been added to the team
      - We list the number of repositories that have been imported into the team on FOSSA as "FOSSA Projects"
      - We list the number of repositories that exist in the project's GitHub Organization
      - Project maintainers are granted the "Team Admin" role by default; "FOSSA user" and "Team Admin" are equivalent for display.
    - The UI reports invitation status per maintainer (pending, accepted, expired, error) along with the last checked time and last error when present.
    - If a FOSSA user exists but is not in maintainerd, they are still shown in the team list.

  Scenario: Show existing FOSSA team members when a project has selected FOSSA
    Given the project has selected FOSSA
    And a FOSSA team is assigned to the project
    And maintainers are registered on the project
    And those maintainers are already Team Admins on the FOSSA team
    When I open the project page
    And I open the SERVICES / LICENSE CHECKER section
    Then I see the FOSSA team members list with status "Team Admin"

  Scenario: Hide the invite action when every registered maintainer is already on the FOSSA team
    Given the project has selected FOSSA
    And a FOSSA team is assigned to the project
    And all registered maintainers are recorded as Team Admins on the FOSSA team
    When I open the project page
    And I open the SERVICES / LICENSE CHECKER section
    Then I do not see the "Send CNCF FOSSA Invites" button

  Scenario: Show the invite action when at least one registered maintainer is not on FOSSA
    Given the project has selected FOSSA
    And a FOSSA team is assigned to the project
    And at least one registered maintainer is not recorded as a Team Admin on the FOSSA team
    When I open the project page
    And I open the SERVICES / LICENSE CHECKER section
    Then I see the "Send CNCF FOSSA Invites to" button followed a list of checkbox selectable maintainers that are not on FOSSA according to maintainerd database records

    
  Scenario: Poll FOSSA invitations and reconcile team membership
    Given the project has selected FOSSA
    And a FOSSA team is assigned to the project
    And an invitation is recorded as pending
    When the system checks FOSSA invitation status
    Then if the invitation is accepted, the maintainer is added to the FOSSA team and the invitation is marked as accepted
    And if the invitation is expired, the invitation is re-issued and the status remains pending with an updated time sent
    And if the invitation check fails, the invitation is marked as error with the last error message

  Scenario: Display pending invitations in the UI
    Given the project has selected FOSSA
    And a FOSSA team is assigned to the project
    And invitations are recorded with statuses pending, accepted, expired, and error
    When I open the project page
    And I open the SERVICES / LICENSE CHECKER section
    Then I see the PENDING INVITATIONS table
    And I see the invite sent time and estimated expiry time for pending invitations
    
  Scenario: Report missing maintainer email addresses
    Given the project has selected FOSSA
    And a FOSSA team is assigned to the project
    And a registered maintainer is missing an email address
    When I open the project page
    And I open the SERVICES / LICENSE CHECKER section
    Then I see a note that the maintainer email is missing
    And the maintainer is not eligible for FOSSA invites

  Scenario: Report when the project has not selected FOSSA
    Given the project has selected a non-FOSSA license checker
    When I open the project page
    And I open the SERVICES / LICENSE CHECKER section
    Then I see a message that the project has not selected FOSSA
    And I see a note that the project may have an organization on Snyk

  Scenario: Staff can choose FOSSA when no license checker is selected
    Given the project has not selected a license checker
    When I open the project page
    And I open the SERVICES / LICENSE CHECKER section
    Then I see a "Choose FOSSA" button

  Scenario: Choosing FOSSA provisions or reuses the team
    Given the project has not selected a license checker
    When I choose FOSSA
    Then maintainer-d checks for an existing FOSSA team for the project
    And if no team exists, maintainer-d creates a FOSSA team using the project name
    And if a team exists, the audit log records "FOSSA_TEAM_REUSED"
    And if team creation fails, the UI shows the FOSSA error and the error time
    And the FOSSA team is checked for existing team members
    And the FOSSA team is checked for imported repos
    And the ACTIVE MAINTAINERS ELIGABLE FOR INVITATION table is shown

  Scenario: Invite action is disabled when no maintainers are selected
    Given the project has selected FOSSA
    And a FOSSA team is assigned to the project
    And the ACTIVE MAINTAINERS ELIGABLE FOR INVITATION table is visible
    When I select maintainers in the table
    And I clear the maintainer selection
    Then the invite action is disabled when 0 is 0

  Scenario: Staff can select maintainers and send FOSSA invites
    Given the project has selected FOSSA
    And a FOSSA team is assigned to the project
    And the ACTIVE MAINTAINERS ELIGABLE FOR INVITATION table is visible
    When I select maintainers in the table
    Then the invite action shows Send CNCF FOSSA Invites to 1 Selected Maintainers
    When I send invites
    Then invitations are sent to the selected maintainers
    And the PENDING INVITATIONS table is shown with headings
      | Maintainer Email | Team | Status | Invite sent on | Estimated time of expiry | Last Checked | Error |

  Scenario: Pending invitations are reconciled by the poller
    Given invitations are recorded as pending
    When the invitation poller runs
    Then accepted invitations add the maintainer to the FOSSA team
    And the audit log records a FOSSA add-user event
    And expired invitations are re-issued
    And pending invitations can be deleted to remove the invitation from FOSSA and maintainer-d
