# Invitation-registration validation

The feature's regression checks run with the standard project toolchains:

```sh
go test -p 2 ./model ./controller ./router ./service -count=1
# in web/
bun install --frozen-lockfile
bun --bun run build
```

The new feature adds 11 tests covering pending login denial, single-use invitations,
expiry/revocation/email binding, concurrent submission and approval, rejection and
re-invitation, transactional rollback, root-only endpoints, role overposting, OAuth
creation restrictions, missing SMTP/retry, and refusing plaintext SMTP authentication.
Existing tests in the affected packages are included in the command above.

Database runtime tests use SQLite. MySQL/PostgreSQL portability is addressed by GORM
migrations, nullable unique reservation fields, and portable query expressions; these
tests do not provision live MySQL/PostgreSQL services.

Notifications use synthetic example.org addresses, a mock sender, and a local test
server. Tests do not send live email. No deployment-specific address, credential,
internal host configuration, or binary is included in the feature.
