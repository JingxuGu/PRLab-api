# Email account applications and administrator approval

This maintenance branch is based on New API v0.13.2. It implements an application flow,
replacing the previous invitation-only design. Upstream attribution and existing accounts remain intact.

1. A visitor opens `/register` and submits only an email address. No invitation, username or password is required.
2. A pending `AccountApplication` reserves that normalized email. No user or password exists yet.
3. The private administrator mailbox receives a review notification linking to `/console/user`.
4. A root administrator approves or rejects the application. GET requests never approve accounts.
5. Approval atomically creates an ordinary user with a randomly generated username, a 16-character
   cryptographically random initial password, and the quota selected in the console.
6. The username and initial password are emailed to the application address. The password is stored
   only as bcrypt; plaintext exists transiently for that mail attempt and is never returned by an API.
7. The applicant can log in and change the initial password in personal settings.

Rejection creates no account and releases the pending email reservation. Duplicate pending or
existing-account email submissions receive the same public acknowledgement and do not send duplicate
notifications. Existing OAuth and WeChat bindings may log in; automatic new-account creation cannot
bypass administrator approval.

## Private runtime settings

```dotenv
APPROVAL_REGISTRATION_ENABLED=true
PRLAB_APPROVAL_EMAIL=administrator@example.org
SMTP_FROM_NAME=
```

Keep real addresses and SMTP credentials on the server, outside Git. An explicitly empty sender
name shows only the email address. The platform's display name is separate.
The previous `INVITATION_REGISTRATION_ENABLED` variable is retained only as an operator-policy fallback
for safe upgrades; it no longer requires an invitation. The public status flag is now
`approval_registration_enabled`; `invitation_registration_enabled` is false.

The master registration and password-registration switches must be enabled to accept applications.
Turning them off closes new applications. Existing accounts are unaffected.
Set the server address to the real HTTPS origin so email login and review links are correct.
SMTP requires certificate-verified TLS; body/credential details are never logged by this flow.

## Delivery failures

The console reports administrator-notification and applicant-credential delivery separately.
Approval is unavailable until SMTP and the private administrator recipient are configured.
If credential delivery fails, the approved user remains linked to the application and the root
administrator can retry before the first successful login. Retry generates a new initial password,
updates its bcrypt hash, and sends the new credentials. It never exposes that password in an API.
A sent credential email cannot be resent through this action. A stale interrupted send becomes
retryable after two minutes. Once the user has logged in, retry cannot reset their password;
use the normal authenticated password change or password-recovery flow instead.
No plaintext password outbox is persisted.

## API

Public: `POST /api/user/register` with `{ "email": "applicant@example.org" }` returns HTTP 202.
Existing Turnstile and request-rate controls remain applied. Public responses do not reveal whether
an email is already registered or awaiting review.

Root-only endpoints:

- `GET /api/registration/?status=pending|approved|rejected`
- `POST /api/registration/requests/:id/review` with `action` and optional `initial_quota`
- `POST /api/registration/requests/:id/notify` to retry an administrator notification
- `POST /api/registration/requests/:id/credentials` to retry failed initial credential delivery

The former invitation-creation/revocation routes and invitation screens have been removed.
Old invitation tables are retained for migration compatibility, but are not used by this flow.
The new `account_applications` table is an additive GORM migration, using a nullable unique pending-email
reservation and transactional conditional review/delivery transitions.

## Validation and rollout

Run existing and new model/controller/router/service regression tests, the frozen-lockfile frontend
build, and full Go build. Test the candidate against a copy of the current database and a loopback
TLS SMTP fixture: email-only application, pending login denial, approval mail parsing, ordinary login,
password change, duplicate review, rejection, delivery failure/retry, and denial of password reset after
first login. No test account or initial test credential should be created in production.
Back up the live database, private configuration and unit before switching. Verify the new public
policy and console before accepting applications; retain resource/device/network restrictions.
After new applications arrive, do not roll back by overwriting the database with an older backup.
Old invitation binaries do not implement the new application table; close registration before any rollback.
