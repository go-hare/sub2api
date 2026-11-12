# Reputation Score Routing

Reputation score routing sends an authenticated user to accounts in the same
already-selected group that are bound to one score tier. It does not change
groups, platforms, pricing, or risk-control state. Content Moderation, Prompt
Audit, and client-disconnect enforcement keep their own decisions; a block
still blocks. Scoring is an additional, best-effort side effect.

The feature is off by default. Settings live in the `reputation_config`
setting. Each process caches that document for one second. The console page
is `/admin/reputation`.

## Score and Tiers

A user score is an integer from 0 through 100. Zero is a real score, not an
empty value. The default initial score is 60.

Administrators define named tiers. Each tier has a stable ID, a unique name,
and a unique score line. Accounts bind to the tier ID, not to the display
name. Enabling the feature requires at least one tier. A tier cannot be
deleted while any non-deleted account still references its ID.

Routing uses a floor match: the highest tier whose line is less than or equal
to the user score. A score below every line still uses the lowest tier. There
is no fallback to a higher tier, another group, or accounts with an empty tier
ID. If that tier has no schedulable account left, the request has no available
account.

Default tiers:

| ID | Name | Line |
| --- | --- | --- |
| `tier-premium` | 优质 | 100 |
| `tier-pass` | 及格 | 60 |
| `tier-watch` | 观察 | 50 |
| `tier-risk` | 风险 | 40 |

Examples against those defaults: 100 stays on 优质, 80 and 60 use 及格, 58
uses 观察, and 41, 40, and 0 use 风险.

## Scheduling

Filtering runs after the group and platform schedulable list is built, and
before later platform filters such as Grok free-quota exclusion. It applies
to the Anthropic gateway, the OpenAI gateway, and Gemini Messages
compatibility.

On the scheduler-snapshot path and on every direct database path, including
the Anthropic/Gemini mixed Antigravity fallback used when no scheduler
snapshot is available, the candidate set is reduced to accounts whose
`reputation_tier_id` equals the user's floor tier. This covers the Anthropic
gateway, the OpenAI gateway, and Gemini Messages compatibility.

While the feature is disabled, the candidate list is unchanged. Saved scores
and tier bindings remain.

## Accounts

While the feature is enabled, creating or updating an account must name an
existing tier. An empty tier on create is stored as the floor tier of the
configured initial score. While the feature is disabled, account writes do
not change reputation columns. An update that omits `reputation_tier_id` or
`reputation_score_enabled` leaves that stored column unchanged, so a
credential-only save cannot reset an explicit scoring flag to NULL.

`reputation_score_enabled` controls only security-event deductions for that
account. `oauth` and `setup-token` accounts deduct by default. Every other
account type does not. An explicit true or false overrides the default.
Content-audit deductions do not read this flag.

## Events

Automatic changes run only while the feature is enabled. They never fail the
request that caused them. A zero delta is not stored, except for registration
and an administrator adjustment. The resulting score is always clamped to
0–100. Score writes are one atomic update per event, so concurrent changes on
the same user do not overwrite each other. Each log row records the actual
score before and after that change.

| Event | Default | Effect |
| --- | --- | --- |
| `register` | on, writes the initial score | Self-registration and administrator user creation set the score to the current initial score. This is not added to an existing score. |
| `content_audit_block` | on, deduct 5 | A content-audit decision that blocks the request. Multiple stages of the same HTTP request score once. Each WebSocket turn can score once. Extraction failures, pass-through, and other non-block decisions do not score. |
| `own_account_security` | on, deduct 20 | OpenAI `cyber_policy` on an `oauth` or `setup-token` account whose score flag resolves to enabled. |
| `upstream_security` | on, deduct 5 | The same `cyber_policy` mark on any other account type whose score flag resolves to enabled. |
| `recharge` | on, 5 points per 100 | Positive balance credited by payment fulfillment. |
| `redeem_balance` | on, 5 points per 100 | Positive balance credited by a user or administrator balance redeem. |
| `admin_adjust` | not a configurable rule | An administrator sets an absolute score. This still applies while the feature is disabled. |

Amount rules ignore a non-positive amount and any remainder below the
threshold. `floor(amount / threshold) * points` is the whole delta. A
threshold must be greater than zero when that rule is enabled, and the points
value cannot be negative. Penalty rules store a positive deduction; the
applied delta is negative. The registration rule always follows the initial
score, even if a different registration delta is submitted.

Subscription, concurrency, and non-balance redeems do not change the score.
A `cyber_policy` mark is counted once for an HTTP request. It is ignored when
the request has no account ID or that account cannot be loaded.

Each applied change appends `reputation_score_logs` with the event, delta,
scores before and after, and any request ID, account ID, amount,
administrator ID, or reason that the event has. Log insertion failure does
not roll back the score update.

## Visibility

Login, profile, and other user-facing user payloads omit `reputation_score`
and `reputation_tier`. Administrator user list and user detail include both
while the feature is enabled, including a stored score of 0. While it is
disabled, those administrator payloads omit both fields. The score is still
stored.

## HTTP API

All routes require an administrator session.

| Method | Path | Body |
| --- | --- | --- |
| `GET` | `/api/v1/admin/reputation/settings` | — |
| `PUT` | `/api/v1/admin/reputation/settings` | `enabled`, `initial_score`, `tiers`, `events` |
| `PUT` | `/api/v1/admin/reputation/users/:id` | `score`, optional `reason` |

Settings validation returns `400` for an empty tier name, a score outside
0–100, a duplicate tier ID, name, or score line, a missing tier list while
enabled, an invalid amount threshold, or a negative point value. Deleting an
in-use tier returns `REPUTATION_TIER_IN_USE`.

The user-score response is `{ "reputation_score": <clamped>, "reputation_tier": { "id", "name", "score" } }`.
The tier field is omitted when no tier can be resolved.

## Migration

`251_reputation_score.sql` adds `users.reputation_score` with default 60,
nullable `accounts.reputation_tier_id`, nullable
`accounts.reputation_score_enabled`, and `reputation_score_logs`. Existing
users therefore start at 60. Existing accounts stay unbound and keep the
type default for security deductions until an administrator saves them while
the feature is enabled. Applying the migration does not enable routing.
