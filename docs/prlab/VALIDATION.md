# Account-application validation

```sh
go test -p 2 ./model ./controller ./router ./service -count=1
# web/
bun install --frozen-lockfile
bun --bun run build
```

The regression suite includes email-only pending requests, duplicate normalization, ordinary credential
creation, concurrent review, failed-delivery recovery and protecting established users from credential
retry resets. Router checks reject malformed email, preserve root-only access, ignore username/password/
role overposting and acknowledge duplicate email consistently. Notification checks verify recipient
separation, omission of the display name, and plaintext password inclusion only in the applicant mail.
The existing verified-TLS and legacy-model regressions remain covered.

An isolated candidate was also tested through real HTTP endpoints using a copy of the current SQLite
database and a loopback certificate-verified TLS SMTP fixture. Captured test mail remained private;
its generated password was used to log in, change password, and prove the initial password stopped
working. Rejection created no account. Synthetic SMTP failure and retry succeeded, and retry after
first login was denied. Existing users, API tokens and channels retained their credentials and permissions.
No production test accounts were created. Frontend production build and Go binary build passed.
Runtime database tests use SQLite; MySQL/PostgreSQL instances were not provisioned. New ORM definitions
and conditional queries use GORM and nullable indexes compatible with all three supported dialects.
No GPU was allocated for builds or acceptance tests.
