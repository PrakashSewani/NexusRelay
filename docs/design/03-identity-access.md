# Identity and Access Design

## Scope

This design covers local user identity, organization membership, active organization selection, roles, atomic permissions, browser sessions, login throttling, session revocation, and initial owner bootstrap.

Authentication answers who the user is. Membership answers which organizations the user can access. Authorization answers whether the active membership has a required permission.

## Domain Entities

### Users

`users` is a global identity table:

```text
id                    uuid primary key
email                 text not null
normalized_email      text not null unique
display_name          text not null
password_hash         text not null
status                text not null  -- active, disabled
auth_version          bigint not null default 1
must_change_password  boolean not null default false
password_changed_at   timestamptz not null
last_login_at         timestamptz null
created_at            timestamptz not null
updated_at            timestamptz not null
```

Email normalization is deterministic and conservative: trim surrounding whitespace and lowercase the domain and local part for uniqueness. NexusRelay does not attempt provider-specific transformations such as removing dots. The original display email is retained.

`auth_version` increments for password changes, account disablement, and administrative global session revocation. Sessions record the version observed at creation.

### Organizations

```text
id                    uuid primary key
name                  text not null
slug                  text not null unique
status                text not null  -- active, suspended
default_timezone      text not null
version               bigint not null default 1
created_at            timestamptz not null
updated_at            timestamptz not null
```

Slug is deployment-global because it may appear in setup or organization-selection URLs. A suspended organization cannot issue new administrative mutations or inference requests.

### Memberships

```text
id                    uuid primary key
organization_id       uuid not null
user_id               uuid not null
role_id               uuid not null
status                text not null  -- active, suspended
membership_version    bigint not null default 1
created_at            timestamptz not null
created_by            uuid null
updated_at            timestamptz not null
updated_by            uuid null
unique (organization_id, user_id)
```

A user has at most one membership and one role in an organization in V1. Membership suspension preserves history and immediately removes organization access.

### Roles

```text
id                    uuid primary key
organization_id       uuid not null
name                  text not null
description           text not null
kind                  text not null  -- system_owner, system_default, custom
is_mutable            boolean not null
version               bigint not null default 1
created_at            timestamptz not null
created_by            uuid null
updated_at            timestamptz not null
updated_by            uuid null
unique (organization_id, name)
```

Default roles are created per organization so custom role behavior remains organization-scoped. The Owner role is a system invariant and cannot be deleted or stripped of required owner permissions. Other default role permissions may be initialized from a versioned seed catalog and subsequently managed according to product policy.

### Permission Catalog

Permissions are global immutable identifiers stored in `permissions`:

```text
key                   text primary key
resource              text not null
action                text not null
description           text not null
introduced_version    text not null
```

`role_permissions` contains `(organization_id, role_id, permission_key)` and is protected by RLS. Code references permission constants, never role names. Role `kind` is server-assigned and immutable. A unique partial constraint permits exactly one `system_owner` role per organization; custom-role APIs always create `kind=custom` and cannot delete or alter the Owner role or its required permission set.

Initial permission keys:

```text
organization.read
organization.update
members.read
members.manage
roles.read
roles.manage
providers.read
providers.manage
providers.test
providers.rotate_secret
models.read
models.manage
routing.read
routing.manage
api_keys.read_own
api_keys.read_all
api_keys.create_own
api_keys.create_all
api_keys.update_own
api_keys.update_all
api_keys.revoke_own
api_keys.revoke_all
budgets.read
budgets.manage
pricing.read
pricing.manage
usage.read_own
usage.read_all
analytics.read_own
analytics.read_all
audit.read
```

Permission additions are backward-compatible; removals require a migration and role impact review. Implementation must not merge the separate test, secret-rotation, or pricing permissions into broad provider management without updating this design.

## Default Roles

### Owner

Receives all organization permissions and satisfies the final-owner invariant. Authorization still resolves permissions rather than checking the role name, except the explicit invariant that identifies owner memberships by role `kind = system_owner`.

### Administrator

Receives all organization-scoped permissions except no deployment-global organization-creation or identity-administration authority. Organization ownership invariants remain protected. Administrators may manage members and custom/default non-owner roles but cannot remove/demote the final owner or mutate the system Owner role.

### Developer

Receives `providers.read`, `models.read`, `routing.read`, `api_keys.read_own`, `api_keys.create_own`, `api_keys.update_own`, `api_keys.revoke_own`, `usage.read_own`, and `analytics.read_own`. Developers cannot test providers, rotate provider secrets, manage pricing/budgets/users/roles, or read audit events.

### Viewer

Receives `organization.read`, `providers.read`, `models.read`, `routing.read`, `budgets.read`, `usage.read_all`, and `analytics.read_all`. Viewer does not receive API key listing, user/role administration, provider tests, pricing mutation, or audit access by default.

Owner receives every permission. Administrator receives every permission except no special bypass of owner invariants. Developer and Viewer receive the explicit sets above. Migration seed data must match this matrix exactly and is tested as a contract.

## Browser Sessions

### Session Record

```text
id                    uuid primary key
user_id               uuid not null
  token_hash            bytea not null unique
  auth_version          bigint not null
  session_version       bigint not null default 1
  active_organization_id uuid null
created_at            timestamptz not null
last_seen_at          timestamptz not null
idle_expires_at       timestamptz not null
absolute_expires_at   timestamptz not null
revoked_at            timestamptz null
revoked_by            uuid null
revoke_reason         text null
ip_created            inet null
user_agent_hash       bytea null
```

The browser receives an opaque random token with at least 256 bits of entropy. PostgreSQL and Redis identify it only by a full fixed-length HMAC using `SESSION_SECRET_FILE`; prefixes are never used for session identity. V1 secret replacement revokes all sessions. The cookie contains no user, organization, permission, or role data.

### Cookie Policy

- `HttpOnly`, `Secure` in production, and `SameSite=Lax` by default.
- Host-only cookie where proxy topology permits it.
- Path `/`.
- No JavaScript access.
- Cookie name is configuration-driven. Production HTTPS validation requires a `__Host-` prefix when host/path conditions permit; local HTTP development uses a non-prefixed name.

### Session Validation

Session validation has two levels. Identity-only endpoints (`GET /session`, active-organization selection, own-session listing/revocation, logout, and forced password change) validate the session, expiry, user status, and `auth_version` without requiring an active organization. Tenant endpoints additionally require active organization resolution.

For tenant administrative requests:

1. Hash the presented token and load an unrevoked session.
2. Verify idle and absolute expiry.
3. Verify user is active and session `auth_version` matches the user.
4. Resolve active organization membership and verify membership and organization are active.
5. Load effective permissions for the membership role.
6. After a configured minimum write interval, synchronously and monotonically set `last_seen_at = now` and `idle_expires_at = min(now + idle_timeout, absolute_expires_at)`. Session validity never depends on a lossy asynchronous update.

Session and permission data may be cached briefly in Redis, keyed by the full session HMAC plus session, user, organization, membership, role, and role-permission-set versions. Individual revocation synchronously marks the session revoked in PostgreSQL and writes a session deny marker/version before returning; cached descriptors cannot authorize after committed revocation. Every organization suspension, membership suspension/removal, role-permission reduction, and user disablement follows the shared deny-marker protocol. Tenant requests check session, organization, user, membership, and role marker/version state. Identity-only endpoints check session and user state. A successfully verified absence of a resource marker means not denied; unavailable or incomplete critical Redis namespace/version state fails closed as defined in `02-persistence-tenancy.md`. Security-sensitive endpoints force authoritative validation.

Fan-out revocation never enumerates descendant sessions or keys. The exact parent checks are:

| Authorization path | Required marker/version checks |
| --- | --- |
| Identity-only browser request | session, user |
| Tenant browser request | session, user, active organization, active membership, assigned role/permission-set version |
| API-key authentication/admission | API key, owner user, owner membership, organization |
| Model listing/routing | all API-key admission parents plus gateway model; each candidate also checks its route target and provider connection |
| Every provider dispatch/fallback | API key, owner user, owner membership, organization, selected gateway model, selected route target, selected provider connection |

Thus a user marker revokes all of that user's sessions and owned keys, a membership marker revokes that organization's browser access and all keys owned by that membership, a role marker revokes permissions of all assigned memberships, an organization marker revokes all tenant access, a key marker revokes only that key, and provider/model/route-target markers remove those resources from listing/routing/dispatch. Child rows are not rewritten to achieve fan-out. PostgreSQL status/version checks remain authoritative on reload and the critical Redis namespace fails closed when these parent states cannot be verified.

## Active Organization Selection

- A session may have no active organization immediately after login if the user has multiple memberships or none.
- If exactly one active membership exists, it becomes active automatically.
- Organization switching requires an authenticated state-changing request with CSRF protection.
- The server verifies membership before updating `active_organization_id`.
- Clients never establish scope by sending an arbitrary organization header for browser control-plane requests.
- APIs obtain organization scope from validated session state and pass it explicitly to services and repositories.

## Login Flow

1. Validate request shape and normalize email.
2. Apply Redis rate limits by normalized account identifier and network address. Store only keyed hashes of the account identifier in limiter keys.
3. Load user by normalized email through the restricted global authentication query.
4. If absent, verify against a fixed dummy Argon2id hash to reduce timing distinction.
5. Compare password using Argon2id parameters encoded in the stored hash.
6. Return the same public failure message for absent, disabled, or incorrect credentials.
7. In one short transaction, create the server-side session, determine active organization, update last login, and append a redacted login audit event. Every failed login appends a mandatory global security audit event through a separate bounded restricted transaction.
8. After successful commit, set the session cookie and return the derived CSRF token for the committed session version.

Argon2id parameters are startup-validated and versioned through encoded hashes. Successful login may rehash a password when parameters are outdated.

## CSRF Design

V1 uses a derived session CSRF token and stores no CSRF token or digest. `CSRF_SECRET_RING_FILE` contains one active HMAC key plus retained verify-only keys identified by a non-secret key ID. The exact version-1 MAC input is:

```text
u16be(18) || UTF-8("NexusRelay CSRF v1") ||
session_id_uuid_raw_16_bytes ||
u64be(session_version) ||
u64be(session_auth_version)
```

`session_id_uuid_raw_16_bytes` is the RFC 4122/network-order 16-byte UUID representation. Unsigned integers are fixed-width big-endian. No textual UUID, decimal integer, JSON, delimiter-based, or platform-native encoding is permitted. The exact token bytes are `u8(1) || u8(key_id_length) || key_id_ascii || HMAC-SHA-256(mac_input)`, where `key_id_length` is 1 through 64 and the key ID matches the ring identifier grammar. The browser value is unpadded base64url of those bytes. Validation decodes with a strict maximum length, requires exact remaining length after the key ID, selects a retained key by ID, recomputes the MAC, and compares in constant time. The token contains no bearer session secret and is useful only with the `HttpOnly` session cookie. `GET /session` and successful login can derive the token after authoritative session validation without a database write.

Creating/replacing a session or incrementing `session_version` immediately changes the tuple and invalidates the prior token. A CSRF-ring rotation first distributes a ring containing the new active key and old verify key, then switches active key; newly derived tokens use the new ID while old tokens remain valid only for unchanged live sessions during a bounded overlap no longer than the configured CSRF/session cache window. Retirement occurs after that overlap and forces clients presenting an old key ID to refresh with `GET /session`; it does not require session-row migration. Replacing the entire ring without overlap intentionally invalidates all outstanding CSRF tokens but not session cookies.

State-changing browser requests require:

- A valid same-site session cookie.
- The current derived CSRF token in `x-csrf-token`, verified against the current session tuple in constant time.
- A matching custom header on mutation requests.
- Origin/Referer validation against configured administrative origins as defense in depth.

Login itself validates origin and uses request rate limiting. Public bearer-key inference endpoints do not use browser cookie authentication and are not subject to session CSRF handling.

## Authorization Flow

Control-plane handlers declare exactly one or more atomic permission requirements. Request middleware authenticates and resolves the active organization but does not make resource-specific authorization decisions implicitly.

Application service sequence:

1. Require the permission relevant to the requested operation.
2. Validate resource identifiers and input.
3. Start tenant transaction and set RLS context.
4. Load/mutate only organization-scoped rows.
5. Recheck race-sensitive invariants inside the transaction.
6. Commit resource, audit, and outbox changes atomically.

Permission denial returns a stable 403 response without revealing whether an inaccessible cross-tenant resource exists. Tenant-scoped resource lookup generally maps both absent and cross-tenant to 404 after permission to access the resource class is established.

## Final Owner Invariant

Operations that can remove an owner include membership suspension, membership role change, and deployment-operator user disablement. The system Owner role itself cannot be deleted. For an organization-scoped mutation, inside the transaction:

1. Lock active owner memberships for the organization or acquire an organization-scoped advisory lock.
2. Count active memberships assigned to a `system_owner` role.
3. Reject the mutation if it would leave zero active owners.
4. Apply mutation, audit event, and invalidation event.

Concurrent demotions therefore cannot both observe themselves as leaving another owner.

Global user disablement is a deployment-operator operation implemented through the narrowly scoped identity-administration database function/role defined in `02-persistence-tenancy.md`. It loads only owner memberships, sorts organization IDs, acquires each organization advisory lock in deterministic order, verifies that every affected organization retains another active owner, disables the user, increments `auth_version`, and inserts one organization-scoped audit/outbox event per affected organization in the same transaction. If any organization would become ownerless, the entire mutation rolls back.

## Membership Lifecycle

V1 does not require outbound email invitations. Authorized administrators can:

- Add an existing user by normalized email.
- Create a user with a temporary operator-provided password only if the deployment policy permits it.
- Change a membership role.
- Suspend or reactivate a membership.
- Suspend or reactivate the user's membership in the active organization.

Adding an existing identity and creating a new identity share one explicit transaction contract. The tenant service first authorizes `members.manage` and establishes organization RLS context. It then calls a narrow mixed-scope database function owned/granted under the rules in `02-persistence-tenancy.md`: for an existing normalized email it returns only the user ID/status needed to add membership and never returns the password hash or unrelated memberships; for an absent email it may create the global user only when the request supplies the required temporary-password fields and deployment policy permits creation. Concurrent normalized-email creation is resolved by the global unique constraint and a retry/load of the bounded descriptor. The function never changes an existing user's password, status, display name, or global sessions.

The surrounding transaction creates the tenant membership, validates the same-organization role, preserves the final-owner/system-role rules, and writes tenant audit/outbox records atomically. A new global user additionally writes a restricted global identity audit event in that transaction. If membership creation fails, a newly inserted user is rolled back; an existing user is unchanged. Public responses do not reveal unrelated organization membership, and attempts to add an already-member identity return the tenant-scoped membership conflict. Tenant administrators never receive general global-user search/list authority.

Because password reset/email is deferred, the V1 onboarding flow is administrator-created user plus a temporary password delivered out of band and `must_change_password = true`. Until changed, the user may authenticate only to the password-change/session endpoints and cannot enter an organization dashboard. Password change verifies the current/temporary password, validates bounded password policy, establishes the user parent deny marker, then in one transaction replaces the Argon2id hash, clears `must_change_password`, updates `password_changed_at`, increments `auth_version`, marks all prior sessions revoked, creates exactly one replacement session carrying the new `auth_version` and `session_version = 1`, and inserts the audit/outbox records. The response replaces the cookie and derives CSRF from the replacement session; old cookies fail by both revocation and auth-version mismatch. The replacement session remains fail closed while the user marker is promoted and safely removed against the committed new version, and no old session can become valid during that window. Retrying the password-change command cannot create multiple replacement sessions.

Additional organization creation is not a tenant permission in V1. An explicit deployment-operator command creates the organization, seeded roles/permissions, and initial owner membership atomically and emits restricted global plus tenant audit events. Organization operational settings use a versioned `organization_settings` row for retention defaults, private-network policy defaults, and other documented organization-level behavior; deployment secrets and process settings remain outside it.

## Initial Owner Bootstrap

The deployment starts uninitialized when no organizations exist. A one-time bootstrap command in the Go image:

1. Requires owner email, display name, organization name, organization slug, and password through protected input or secret files.
2. Acquires a deployment bootstrap advisory lock and refuses to run when a singleton initialization record or any organization already exists unless an explicit reviewed recovery procedure is invoked.
3. Creates user, organization, seeded roles, role permissions, owner membership, and an audit event in one transaction.
4. Does not print the password or password hash.

An optional web setup flow may later wrap the same service, but must be protected by a one-time deployment token and disabled permanently after initialization.

## Session Revocation

- Sign-out revokes the current session and clears the cookie.
- Password changes and user disablement increment `auth_version`, invalidating all sessions.
- Users can list/revoke their own global sessions. Tenant administrators cannot revoke another user's global sessions; they suspend or reactivate the active-organization membership. Deployment-operator identity administration may perform audited global revocation for a security incident.
- Removing/suspending a membership invalidates organization access immediately through membership version invalidation, even if the global session remains valid for other organizations.
- Revocation actions are audited without storing session tokens.

## Audit Events

Identity/access events include:

```text
auth.login.succeeded
auth.login.failed
auth.logout
auth.password.changed
session.revoked
organization.created
organization.updated
membership.created
membership.role_changed
membership.suspended
membership.reactivated
role.created
role.updated
role.deleted
role.permissions_changed
```

Failed login audit records are mandatory global security events when no organization is known. They use a keyed hash of the normalized identifier rather than plaintext email, record source IP according to retention policy, and never reveal whether the identity exists.

## Failure and Abuse Handling

- Redis unavailable during login throttling: fail closed with a controlled authentication-service-unavailable response. V1 has no local or per-replica fail-open allowance.
- Session store unavailable: return controlled authentication service unavailable, not an unauthenticated fallback.
- Repeated failed login: increase retry delay and return generic errors.
- Suspicious session token mismatch or invalid format: do not query by partial raw token; return unauthenticated and increment a low-cardinality metric.
- Disabled user with active sessions: validation rejects due to user status/auth version.
- Membership removed during an active request: the current transaction's authorization snapshot may complete, but new requests fail immediately after committed invalidation.

## Verification

- Correct and incorrect login with indistinguishable public failures.
- Argon2id parameter and rehash tests.
- Session idle, absolute expiry, sign-out, and administrative revocation.
- Multi-organization switching and no client-controlled tenant selection.
- Permission tests for every administrative operation.
- Two-organization negative tests for users, roles, and memberships.
- Concurrent final-owner demotion/removal tests.
- Derived-CSRF canonicalization, key-ID/rotation/retirement, no-digest persistence, cookie attribute, and origin validation tests.
- Bootstrap single-use and transaction rollback tests.
- Explicit own/all permission tests for custom roles, forced password-change flow, concurrent bootstrap, session-HMAC collision resistance, organization-only access revocation, and deployment-operator global user-disable tests.
- Identity-only endpoint tests with no active organization, exact parent-marker matrix tests, immediate cached-session/key revocation tests, mixed global/tenant user-creation races, and replacement-session password-change tests.

## Requirement Coverage

This design satisfies FR-ORG-002 through FR-ORG-006, FR-AUTH-001 through FR-AUTH-011, FR-RBAC-001 through FR-RBAC-008, relevant FR-AUDIT requirements, SEC-004, SEC-011, SEC-014, SEC-020, SEC-022, and the local-auth V1 acceptance criteria.
