# PRLab invitation registration and approval

This deployment branch is based on upstream New API v0.13.2. Upstream main, licenses,
credits, and branding remain intact. The site's configurable display name remains PRLab api.

## Flow

1. A root administrator opens **User management → Invitations and registration approval**.
2. Create a single-use invitation for a specific email address (1–30 days; default 7).
   Select the initial account quota. The random invitation code is displayed once.
3. Privately send the generated registration link. Its code is in the URL fragment so
   it is not included in ordinary HTTP access logs or Referer headers.
4. The applicant submits a username, the bound email, and an 8–20 character password.
5. The request remains **pending**. No login-capable User or API token is created.
6. The root administrator approves or rejects it in User management. Approval atomically
   creates an enabled ordinary account. The applicant can then sign in and create tokens.

The pending password is stored only as a bcrypt hash, never returned by the API or emailed.
Approval and rejection erase that hash from the request record. Rejected applicants need
an entirely new invitation; old invitations remain consumed.

Email notification is optional while SMTP is unavailable. Pending applications remain
visible in the console, with `unconfigured`/`failed` notification status. Once SMTP is
configured, root can resend the notification. An email links to the authenticated console;
opening a link or a mail scanner GET request never approves an account.

## Configuration

Keep private environment values on the server, outside Git:

```dotenv
INVITATION_REGISTRATION_ENABLED=true
PRLAB_APPROVAL_EMAIL=administrator@example.org
SMTP_FROM_NAME=example.org
```

The PRLab branch enables the invitation policy by default; explicitly set the variable above in production as well.
Setting it to false restores the upstream flow and should only be done by a trusted operator.

Enable New API's master registration and password registration switches. The invitation
policy then replaces public password registration; these switches can still close all
new applications. Existing accounts and administrator-created users remain unchanged.
New OAuth/WeChat account creation is blocked while the invitation policy is active;
existing bound accounts can continue to sign in.

The real approval recipient belongs in the private environment file, not source code.
`SMTP_FROM_NAME` controls the sender display name for system mail and approval notices,
independently of the platform name. Use a general domain identity when sharing this sender
with other applications. If unset, the existing platform-name behavior is preserved.
Configure SMTP via the existing root settings page, and set the platform server address
to its actual HTTPS public origin. Notifications require verified implicit TLS or STARTTLS;
plaintext SMTP and invalid server certificates are rejected, and delivery has a 15s deadline.
No SMTP server, password, or live mail recipient is embedded in this fork.

Operators must configure SMTP egress according to their own network policy. Preserve
existing inbound isolation when permitting a mail provider. SMTP setup is separate
from the invitation and approval workflow.

## API

Root-authenticated endpoints (session plus New-Api-User, or management access token):

- `GET /api/registration/?status=pending|approved|rejected`: latest 100 requests and invitations.
- `POST /api/registration/invitations`: `{email, valid_days, initial_quota}`; returns code once.
- `POST /api/registration/invitations/:id/revoke`: only unconsumed invitations.
- `POST /api/registration/requests/:id/review`: `{action: "approve" | "reject"}`.
- `POST /api/registration/requests/:id/notify`: retry SMTP notification for a pending request.

Public submission stays at `POST /api/user/register`, with `invite_code`, `username`,
`email`, and `password`; existing email verification and Turnstile checks apply if enabled.
A successful application returns HTTP 202 and `data.status=pending`, not an authenticated
session. Root endpoints return no invitation digest or password hash. Mutations require POST.

## Database and rollout

Two additive GORM migrations: RegistrationInvitation and RegistrationRequest. No existing
user, token, model channel, quota, Slurm, CUDA, or GPU service schema/configuration is changed.
All queries use GORM and portable expressions for SQLite, MySQL, and PostgreSQL.
Invitation consumption and request review use conditional writes inside transactions.
Unique pending username/email indexes prevent simultaneous duplicate applications. Terminal
requests release these reservations, allowing rejection followed by a new invitation.

Before rollout: back up the live SQLite database and private configuration, build the web
assets and Go binary, test against a copy of the database, and stage the binary separately.
Use an isolated build environment.
Enable the policy in the new binary's environment before opening the registration switch.
If rolling back to an upstream binary, first close registration again; the older binary does
not understand the invitation policy. Keep the additive tables or restore the matching backup.

Tests cover invitation expiry/revocation/email binding, concurrent consumption, pending
login denial, repeat approval, rollback on account conflict, rejection/re-invitation, root
access control, overposting role/status fields, missing SMTP, and notification retry.
No test sends live email or uses the GPU.
