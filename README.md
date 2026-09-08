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
  "unavailableSignals": [
    "authenticationAttempts",
    "authenticationSuccessful",
    "apiUsageDetected"
  ],
  "eventsFound": 9
}
```

**Response fields:**

| Field | Meaning |
|---|---|
| `applicationCreated` | `true` if the account completed at least one onboarding step |
| `authenticationAttempts` / `authenticationSuccessful` / `apiUsageDetected` | **Currently always `false`/`0` — see "Known limitation" below. Do not treat these as real signals yet.** |
| `lastActivity` | Date (`YYYY-MM-DD`) of the most recent event found, across *all* event types — not just recognized ones |
| `unavailableSignals` | Lists which fields above are placeholders, not real data. Always the same 3 fields today. A consumer should treat these fields' zero-values as "unknown", not "confirmed zero". |
| `eventsFound` | Moesif's true total matching event count. **`0` means no data exists for this identifier at all** — every company/user record in this data model only comes into existence via an event, so this is the reliable way to distinguish "unknown account" from "known account, quiet." |

**Error responses:** `400` for missing/invalid params, `502` if the Moesif API call itself fails.

---

## Known limitation: no authentication/login data

**Confirmed 2026-09-08, across all four Asgardeo Moesif environments (Prod, Dev, Staging, Test) accessible to this team:** Moesif does not track login, authentication, session, or token-related activity for Asgardeo's own product analytics. Every recorded event is registration/onboarding-related.

Per WSO2's Moesif admin: login/token tracking exists only as a custom, per-customer opt-in feature for specific *end-customer* orgs' own instrumentation — it is not enabled for Asgardeo's own analytics today.

**Complete list of real `action_name` values seen (Prod, via dashboard, all-time, sorted by count):**
`organization_created`, `user_created`, `organization_subscribed`, `Onboarding-Step-Completed`, `Onboarding-Started`, `Onboarding-Skipped`, `user_marketing_data_added`, `organization_marketing_data_added`, `user_association_added`, `Onboarding-Completed`, `organization_ownership_added`, `Onboarding-Step-Back`

**Implication:** the PLG requirements doc's "authentication attempts," "API usage," and similar signals are not achievable from this data source today. This is flagged explicitly in every API response via `unavailableSignals`, rather than silently returning misleading zeros. If this changes (e.g. Asgardeo starts tracking logins, or a different Moesif app is found to contain this data), update the constants and classification logic in `internal/moesif/normalize.go`.

---

## Confirmed Moesif API details

These were determined empirically against real data — not assumed from generic docs — after some early false starts (see git history / project chat log for the full investigation). Recorded here so this knowledge isn't lost.

- **Endpoint:** `POST https://api.moesif.com/search/~/search/events?from=<time>&to=<time>`
- **Auth:** `Authorization: Bearer <management-api-key>` (not the `X-Moesif-Application-Id` header — that's for the separate Collector API)
- **Response shape:** hits are nested under `hits.hits`, with total count at `hits.total` (Elasticsearch-style envelope) — **not** flat top-level `hits`/`total` as you might assume from a generic search API
- **The distinguishing action field is `action_name`, not `event_type`.** Every event we've seen has `event_type: "user_action"` — this field alone cannot distinguish activity types; `action_name` is what actually varies (e.g. `"organization_created"`).
- **Pagination is keyset/seek-based**, not offset-based: request `size`, `sort` (we sort by `request.time` descending), and `search_after` (the `sort` value from the last hit of the previous page, omitted on the first page). This service loops automatically up to a safety cap of 1000 events total (10 pages × 100) — Moesif's own docs recommend their separate bulk-export API for larger pulls, since Search is meant for interactive workflows.
- **Required Moesif scopes:** `events: Read` and `customer_actions: Read`. (`companies`/`users` scopes are not needed for this service's current functionality — it never calls a separate Companies/Users lookup endpoint.)

---

## Architecture
cmd/server/main.go — HTTP server, route registration, .env loading
internal/moesif/
client.go — Config/Client/NewClient/do() pattern; Search() handles pagination
filter.go — FilterCriteria + BuildPostFilter (query construction)
types.go — Raw Moesif response shapes (RawHit, RawSource, etc.)
normalize.go — Normalize(): raw events → Summary
client_test.go — Tests Search()'s pagination logic against a fake local HTTP server
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
- `TestNormalize` and the handler tests use hand-built sample data matching confirmed real shapes, not guesses.

---

## Environment variables (`.env`)

| Var | Purpose |
|---|---|
| `MOESIF_API_KEY` | Management API key (Bearer token). Get from Moesif admin — see "Required Moesif scopes" above. |
| `MOESIF_BASE_URL` | `https://api.moesif.com` |
| `PORT` | Local server port (default `8081` if unset) |

**Never commit `.env`** — it's git-ignored at the repo root.