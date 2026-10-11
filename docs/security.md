# Security model

## Identity

Production requires `AUTH_MODE=oidc` and `OIDC_ISSUER`. The API rejects production startup when the mode is `dev`, when `TOKEN_SECRET` or `OIDC_CLIENT_SECRET` is empty or `replace-me`, or when `SECURE_COOKIES` is not true.

Development runs an embedded authorization server at `/oidc`. It issues an authorization code after a password check and requires PKCE S256. The browser completes the code at `POST /api/v1/auth/session`. The token endpoint also checks the client secret, which stays on the server. Development password login at `POST /api/v1/auth/login` is disabled when `AUTH_MODE=oidc`.

The session cookie is `sdn_session`, HttpOnly, path `/`. `COOKIE_SAMESITE=lax` is the same-site default. A Vercel origin that differs from the API origin needs `none`, and `none` requires `SECURE_COOKIES=true`.

## Authorization

Roles: viewer, operator, engineer, approver, administrator, recovery_operator.

| Action | Roles |
| --- | --- |
| Read | every role |
| Discover, backup, drift scan | operator, engineer, administrator |
| Author policy and execute | engineer, administrator |
| Approve or reject | approver, administrator |
| Enroll connectors and members | administrator |
| Restore | recovery_operator, administrator |

The requester cannot approve their own change. Execution requires an approved (or unknown, for a retry) status, `confirm: true`, and the current plan hash. A preview token is an HMAC over the policy version, plan hash, and expiry (15 minutes). Creating or approving a change fails with `stale_plan` when the version is no longer the latest or the recomputed hash differs.

## Isolation

Handlers resolve the organization from the session. Store methods set `app.current_org` for the transaction and also filter `organization_id`. Tables use `FORCE ROW LEVEL SECURITY`, so the table owner does not skip the policies. `app.bypass` is transaction-local and is limited to seed, enrollment-token lookup, and the worker claim. Request handlers do not set it.

A user in another organization receives not-found for a foreign device id. The integration test covers that case.

## Secrets and data

Connector create rejects a body field named `credential`. The stored value is a reference such as `vault://{org}/{connector}/management`. Enrollment tokens are returned once and stored as a hash. Logs for enrollment record the word `redacted`. Seed data contains emails, not passwords. The demonstration password exists only in `DEV_ADMIN_PASSWORD` at seed time.

Backups store a manifest and a checksum of the JSON PostgreSQL returned. They do not store secret values. Restore checks that checksum and records `pending_reconciliation`. It does not push configuration to devices.

## Browser

Mutations from a browser origin that is not in `ALLOWED_ORIGINS` are rejected. The console is a static Vite app. `VITE_API_BASE_URL` is a public origin, never a database URL or client secret.

## What this release does not claim

An external identity provider, a Vault server, TLS termination, and a verified OVN lab are deployment tasks. The epoch row rejects a stale writer inside this database. It does not fence a session that a device already accepted.
