# Moesif Integration Service

A Go HTTP service that retrieves and normalizes Asgardeo product activity from Moesif, for use in the PLG (Product-Led Growth) outreach tool. Given a company or user identifier, it returns a compact summary of that account's product activity — suitable for a CS engineer deciding whether an account is worth reaching out to.

**Status:** functional and tested against real Moesif data. Built as an isolated sandbox (see "Why this is a separate repo" below) before being folded back into the main `cs-tools` monorepo.

---

## Quick start

```bash
go mod tidy
cp .env.example .env   # then fill in your real Moesif API key
go run ./cmd/server/main.go
```

Server starts on port 8081 by default (configurable via `.env`).

Test it:
```bash
curl "http://localhost:8081/events?company_id=<a-real-company-id>"
```

Run the tests:
```bash
go test ./... -v
```

---

## API Reference

### `GET /health`

Returns `{"status":"ok"}`. No parameters.

### `GET /events`

Returns a normalized activity summary for a company and/or user.

**Query parameters:**

| Param | Required? | Default | Notes |
|---|---|---|---|
| `company_id` | One of `company_id`/`user_id` required | — | Moesif company identifier |
| `user_id` | One of `company_id`/`user_id` required | — | Moesif user identifier. **Not always a UUID** — confirmed some real values are anonymous/session-style IDs (e.g. `1a07f690def2bd-01adf07d5c33fb-1d525630-1d73c0`). Do not add strict UUID validation. |
| `from` | No | `-30d` | Moesif relative/absolute time string |
| `to` | No | `now` | Moesif relative/absolute time string |

Providing both `company_id` and `user_id` combines them with AND (more precise lookup). All params are capped at 200 characters (sanity bound, not a format check).

**Example response:**
```json
{
  "applicationCreated": true,
  "authenticationAttempts": 0,
  "authenticationSuccessful": false,
  "apiUsageDetected": false,
  "lastActivity": "2026-09-07",
  "firstSeen": "2026-09-07",
  "onboardingSkippedCount": 1,
  "lastSkippedStepNumber": 1,
  "lastSkippedStepName": "app_name_entered",
  "unavailableSignals": [
    "authenticationAttempts",
    "authenticationSuccessful",
    "apiUsageDetected"
  ],
  "eventsFound": 5
}
```

**Response fields:**

| Field | Meaning |
|---|---|
| `applicationCreated` | `true` if the account completed at least one onboarding step |
| `authenticationAttempts` / `authenticationSuccessful` / `apiUsageDetected` | **Currently always `false`/`0` — see "Known limitation: no authentication/login data" below. Do not treat these as real signals yet.** |
| `lastActivity` | Date (`YYYY-MM-DD`) of the most recent event found, across *all* event types — not just recognized ones |
| `firstSeen` | Date (`YYYY-MM-DD`) of the earliest event found. Combined with `lastActivity`, shows overall tenure — how long this account has existed and whether it's still active |
| `onboardingSkippedCount` | How many times this company/user triggered an `Onboarding-Skipped` event |
| `lastSkippedStepNumber` | The onboarding step number at the time of the **most recent** skip (chronologically, not just last in the response order). `null` if never skipped. **This is a pointer/nullable on purpose** — step `0` (`welcome_option_selected`) is a real, valid step, so `null` (never skipped) must be distinguishable from `0` (skipped at the very first step) |
| `lastSkippedStepName` | The human-readable step name matching `lastSkippedStepNumber` (e.g. `"app_name_entered"`). Omitted entirely from the JSON (not just empty string) when no skip has occurred |
| `unavailableSignals` | Lists which fields above are placeholders, not real data. Always the same 3 fields today. A consumer should treat these fields' zero-values as "unknown", not "confirmed zero". |
| `eventsFound` | Moesif's true total matching event count. **`0` means no data exists for this identifier at all** — every company/user record in this data model only comes into existence via an event, so this is the reliable way to distinguish "unknown account" from "known account, quiet." |

**Error responses:** `400` for missing/invalid params, `502` if the Moesif API call itself fails.

---

## Known limitation: no authentication/login data

**Confirmed 2026-09-08, across all four Asgardeo Moesif environments (Prod, Dev, Staging, Test) accessible to this team:** Moesif does not track login, authentication, session, or token-related activity for Asgardeo's own product analytics. Every recorded event is registration/onboarding-related.

Per WSO2's Moesif admin: login/token tracking exists only as a custom, per-customer opt-in feature for specific *end-customer* orgs' own instrumentation — it is not enabled for Asgardeo's own analytics today.

**Complete list of real `action_name` values seen (Prod, via dashboard, all-time, sorted by count):**
`organization_created`, `user_created`, `organization_subscribed`, `Onboarding-Step-Completed`, `Onboarding-Started`, `Onboarding-Skipped`, `user_marketing_data_added`, `organization_marketing_data_added`, `user_association_added`, `Onboarding-Completed`, `organization_ownership_added`, `Onboarding-Step-Back`

Also seen in broader/newer samples (not yet exhaustively confirmed against the full dashboard list, but real): `organization_marketing_data_added`, `user_marketing_data_added`, `Onboarding-Skipped` (multiple real examples, each carrying `step_number`/`step_name` metadata).

**Implication:** the PLG requirements doc's "authentication attempts," "API usage," and similar signals are not achievable from this data source today. This is flagged explicitly in every API response via `unavailableSignals`, rather than silently returning misleading zeros. If this changes (e.g. Asgardeo starts tracking logins, or a different Moesif app is found to contain this data), update the constants and classification logic in `internal/moesif/normalize.go`.

---

## Known limitation: no reliable session/visit duration

**Investigated and confirmed 2026-09-08:** Moesif's `session_token` field for Asgardeo events is **just the request's IP address**, not a genuine session identifier. Confirmed by tracing one real user's continuous, unbroken 66-second signup-to-onboarding-completion sequence: it showed **two different `session_token` values** partway through, simply because early events came from a backend API call (different client) while later ones came from the browser — same visit, different "session."

**Implication:** there's no reliable way to detect session boundaries or measure time-spent-per-visit from this data. Grouping events by `session_token` would produce misleading duration numbers. This signal was requested (see PLG requirements/mentor follow-up) but is deliberately **not implemented** — an approximate number here would look precise while being untrustworthy, which is worse than not having it at all.

If this becomes necessary, a genuine session-tracking mechanism would need to be added to Asgardeo's own event instrumentation (e.g. a real per-visit session ID), which isn't something this API can construct after the fact.

---

## Confirmed Moesif API details

These were determined empirically against real data — not assumed from generic docs — after some early false starts (see git history / project chat log for the full investigation). Recorded here so this knowledge isn't lost.

- **Endpoint:** `POST https://api.moesif.com/search/~/search/events?from=<time>&to=<time>`
- **Auth:** `Authorization: Bearer <management-api-key>` (not the `X-Moesif-Application-Id` header — that's for the separate Collector API)
- **Response shape:** hits are nested under `hits.hits`, with total count at `hits.total` (Elasticsearch-style envelope) — **not** flat top-level `hits`/`total` as you might assume from a generic search API
- **The distinguishing action field is `action_name`, not `event_type`.** Every event we've seen has `event_type: "user_action"` — this field alone cannot distinguish activity types; `action_name` is what actually varies (e.g. `"organization_created"`).
- **Event `metadata` carries onboarding step details.** Both `Onboarding-Step-Completed` and `Onboarding-Skipped` events include `metadata.step_number` (int, 0-indexed — 0 is a real, valid step) and `metadata.step_name` (e.g. `"welcome_option_selected"`, `"app_name_entered"`, `"redirect_url_configured"`, `"signin_options_configured"`, `"design_login_configured"`).
- **`session_token` is just the request's IP address** — not a genuine session ID. See "Known limitation: no reliable session/visit duration" above.
- **Pagination is keyset/seek-based**, not offset-based: request `size`, `sort` (we sort by `request.time` descending), and `search_after` (the `sort` value from the last hit of the previous page, omitted on the first page). This service loops automatically up to a safety cap of 1000 events total (10 pages × 100) — Moesif's own docs recommend their separate bulk-export API for larger pulls, since Search is meant for interactive workflows.
- **Required Moesif scopes:** `events: Read` and `customer_actions: Read`. (`companies`/`users` scopes are not needed for this service's current functionality — it never calls a separate Companies/Users lookup endpoint.)

---

## Architecture

cmd/server/main.go — HTTP server, route registration, .env loading
internal/moesif/
client.go — Config/Client/NewClient/do() pattern; Search() handles pagination
filter.go — FilterCriteria + BuildPostFilter (query construction)
types.go — Raw Moesif response shapes (RawHit, RawSource, RawMetadata, etc.)
normalize.go — Normalize(): raw events → Summary
client_test.go — Tests Search()'s pagination logic against a fake local HTTP server
filter_test.go — Tests BuildPostFilter()'s query construction for every param combination
normalize_test.go — Tests Normalize()'s classification/aggregation logic
internal/handler/
events.go — GET /events handler: param validation → Search → Normalize → JSON
events_test.go — Full handler test suite, using a mock Moesif client
helpers_test.go — mockMoesifClient test double

---


**Data flow:** `FilterCriteria` → `BuildPostFilter()` (query DSL) → `Client.Search()` (paginated fetch + parse) → `Normalize()` (raw hits → `Summary`) → JSON response.

---

## Why this is a separate repo

Built in an isolated sandbox repo (`uvini-wso2/moesif-integration-service`) rather than directly in the `cs-tools` monorepo fork, per team decision — to freely iterate against unconfirmed Moesif response shapes without polluting shared history. The `cs-tools` fork was reverted to clean `main` with no trace of this work. **This code has not yet been ported into `cs-tools`** — that's a separate next step, pending team decision on timing/approach.

---

## Testing notes

- All tests run without any real Moesif API key or network access — `internal/handler` uses a mock satisfying the `eventsClient` interface; `internal/moesif`'s pagination tests use a local `httptest.Server` simulating Moesif's response shape.
- Test data (`normalize_test.go`, `client_test.go`, `filter_test.go`) is hand-built to match confirmed real shapes, not guesses — including edge cases like step `0` being a valid onboarding step (must not be confused with "no skip occurred").
- 21 tests total, covering filter construction, pagination, classification/normalization, and the full HTTP handler.

---

## Environment variables (`.env`)

| Var | Purpose |
|---|---|
| `MOESIF_API_KEY` | Management API key (Bearer token). Get from Moesif admin — see "Required Moesif scopes" above. |
| `MOESIF_BASE_URL` | `https://api.moesif.com` |
| `PORT` | Local server port (default `8081` if unset) |

**Never commit `.env`** — it's git-ignored at the repo root.