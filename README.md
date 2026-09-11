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
  "organizationName": "wayfinderenterprise",
  "firstSeen": "2026-09-07T08:30:21Z",
  "lastActivity": "2026-09-07T08:30:22Z",
  "timezone": "Asia/Colombo",
  "countryName": "Sri Lanka",
  "averageTimePerActiveDayMinutes": 0.01285,
  "productActivity": {
    "applicationCreated": false,
    "hasSkippedOnboarding": true,
    "skippedStepNumber": 0,
    "skippedStepName": "welcome_option_selected",
    "onboardingSetupType": "full_setup"
  },
  "eventsFound": 2
}
```

### Response shape: parent vs. `productActivity`

**Per team decision (2026-09-09):** this API will eventually cover 5 SaaS products, not just Asgardeo. To keep that sane, the response splits into:

- **Parent-level fields** — must stay **consistent across every product** this API ever covers (organization name, tenure, location, activity cadence, event count).
- **`productActivity`** — deliberately **product-specific**. Asgardeo's shape (onboarding steps, setup type) is *not* a template other products must follow — a different product's `productActivity` may look completely different, because different products have different concepts.

This is also why there's no `authenticationAttempts`/`apiUsageDetected`/`unavailableSignals` in the response anymore (an earlier design had these, showing `false`/`0` with a disclaimer list). Since Asgardeo genuinely has no auth/login data (see limitation below), those fields simply don't exist in Asgardeo's `productActivity` — not hidden, just not part of this product's schema. A future product that *does* have that data would include it for real in its own `productActivity`.

**Parent-level response fields:**

| Field | Meaning |
|---|---|
| `organizationName` | Best-effort — taken from whichever event in the result set happens to carry `company.metadata.account_name`. Omitted if never found. |
| `firstSeen` / `lastActivity` | Full ISO-8601 UTC timestamps (`2026-09-07T08:30:21Z`) of the earliest/latest event found, across **all** event types — not just recognized ones. Together these show overall tenure. |
| `timezone` / `countryName` | Taken from `request.geo_ip` on the **most recent** event (same one that sets `lastActivity`) — a best-effort snapshot of where the account was *last* active, not necessarily where it originally signed up. Omitted if that event had no geo data. |
| `averageTimePerActiveDayMinutes` | Averages (last-event-time − first-event-time) across every calendar day that had **2 or more** events. Days with only 1 event are excluded on purpose — a single event can't establish a real duration, and counting it as 0 would understate activity dishonestly. Omitted if no day qualifies. See "Known limitation: no reliable session/visit duration" below for why this is a *daily* average and not a per-session number. |
| `eventsFound` | Moesif's true total matching event count. **`0` means no data exists for this identifier at all** — every company/user record in this data model only comes into existence via an event, so this is the reliable way to distinguish "unknown account" from "known account, quiet." |

**`productActivity` fields (Asgardeo-specific):**

| Field | Meaning |
|---|---|
| `applicationCreated` | `true` if the account completed at least one onboarding step |
| `hasSkippedOnboarding` | Boolean — `true` if at least one `Onboarding-Skipped` event occurred. Deliberately a boolean, not a count (team decision 2026-09-09) — a count wasn't considered meaningful enough to track. |
| `skippedStepNumber` | The step number at which onboarding was skipped (from the **chronologically most recent** skip, if somehow more than one exists). **`null` means never skipped** — this is a pointer/nullable on purpose, since step `0` (`welcome_option_selected`) is a real, valid step and must not be confused with "no skip happened." |
| `skippedStepName` | Human-readable name matching `skippedStepNumber` (e.g. `"app_name_entered"`). Omitted entirely from the JSON (not just empty string) when no skip has occurred. |
| `onboardingSetupType` | From Moesif's `metadata.wizard_path` (confirmed real values: `"full_setup"`, `"preview"`). Uses the first non-empty value found across the event set. Omitted if never present. |

**Error responses:** `400` for missing/invalid params, `502` if the Moesif API call itself fails.

---

## Known limitation: no authentication/login data

**Confirmed 2026-09-08, across all four Asgardeo Moesif environments (Prod, Dev, Staging, Test) accessible to this team:** Moesif does not track login, authentication, session, or token-related activity for Asgardeo's own product analytics. Every recorded event is registration/onboarding-related.

Per WSO2's Moesif admin: login/token tracking exists only as a custom, per-customer opt-in feature for specific *end-customer* orgs' own instrumentation — it is not enabled for Asgardeo's own analytics today.

**Complete list of real `action_name` values seen (Prod, via dashboard, all-time, sorted by count):**
`organization_created`, `user_created`, `organization_subscribed`, `Onboarding-Step-Completed`, `Onboarding-Started`, `Onboarding-Skipped`, `user_marketing_data_added`, `organization_marketing_data_added`, `user_association_added`, `Onboarding-Completed`, `organization_ownership_added`, `Onboarding-Step-Back`

**Implication:** the PLG requirements doc's "authentication attempts" and "API usage" signals are not achievable from this data source today. Rather than fake them with `false`/`0` placeholders, they're simply absent from Asgardeo's `productActivity` object (see "Response shape" above). If this changes (e.g. Asgardeo starts tracking logins, or a different Moesif app is found to contain this data), add the real fields to `ProductActivity` and wire up classification logic in `internal/moesif/normalize.go` — the `ActionNameAuthenticationAttempt`/`ActionNameAPICall` constants are already there, kept as reference for exactly this.

---

## Known limitation: no reliable per-session duration

**Investigated and confirmed 2026-09-08:** Moesif's `session_token` field for Asgardeo events is **just the request's IP address**, not a genuine session identifier. Confirmed by tracing one real user's continuous, unbroken 66-second signup-to-onboarding-completion sequence: it showed **two different `session_token` values** partway through, simply because early events came from a backend API call (different client) while later ones came from the browser — same visit, different "session."

**What this means:** there's no reliable way to detect session *boundaries* (where one visit ends and the next begins) from this data. `averageTimePerActiveDayMinutes` (see above) sidesteps this problem entirely — it doesn't try to detect sessions at all. It just measures the gap between the first and last event on a given **calendar day**, averaged across days with 2+ events. This is an honest, defensible number (a rough "how long were they engaged on days they showed up"), whereas a `session_token`-based per-visit duration would have been actively misleading, since visits would get incorrectly split or merged depending on which client happened to make each request.

---

## Confirmed Moesif API details

These were determined empirically against real data — not assumed from generic docs — after some early false starts (see git history / project chat log for the full investigation). Recorded here so this knowledge isn't lost.

- **Endpoint:** `POST https://api.moesif.com/search/~/search/events?from=<time>&to=<time>`
- **Auth:** `Authorization: Bearer <management-api-key>` (not the `X-Moesif-Application-Id` header — that's for the separate Collector API)
- **Response shape:** hits are nested under `hits.hits`, with total count at `hits.total` (Elasticsearch-style envelope) — **not** flat top-level `hits`/`total` as you might assume from a generic search API
- **The distinguishing action field is `action_name`, not `event_type`.** Every event we've seen has `event_type: "user_action"` — this field alone cannot distinguish activity types; `action_name` is what actually varies (e.g. `"organization_created"`).
- **Event `metadata` carries onboarding details.** Both `Onboarding-Step-Completed` and `Onboarding-Skipped` events include `metadata.step_number` (int, 0-indexed — 0 is a real, valid step), `metadata.step_name` (e.g. `"welcome_option_selected"`, `"app_name_entered"`), and `metadata.wizard_path` (e.g. `"full_setup"`, `"preview"`).
- **Event `request.geo_ip` carries location details.** Confirmed real fields: `timezone` (e.g. `"America/New_York"`) and `country_name` (e.g. `"United States"`).
- **Event `company.metadata.account_name` carries the organization's display name** (e.g. `"roadsidecoderytt"`) — not present on every event, best-effort only.
- **`session_token` is just the request's IP address** — not a genuine session ID. See "Known limitation: no reliable per-session duration" above.
- **Pagination is keyset/seek-based**, not offset-based: request `size`, `sort` (we sort by `request.time` descending), and `search_after` (the `sort` value from the last hit of the previous page, omitted on the first page). This service loops automatically up to a safety cap of 1000 events total (10 pages × 100) — Moesif's own docs recommend their separate bulk-export API for larger pulls, since Search is meant for interactive workflows.
- **Required Moesif scopes:** `events: Read` and `customer_actions: Read`. (`companies`/`users` scopes are not needed for this service's current functionality — it never calls a separate Companies/Users lookup endpoint.)

---

## Architecture

cmd/server/main.go — HTTP server, route registration, .env loading
internal/moesif/
client.go — Config/Client/NewClient/do() pattern; Search() handles pagination
filter.go — FilterCriteria + BuildPostFilter (query construction)
types.go — Raw Moesif response shapes (RawHit, RawSource, RawMetadata, RawGeoIP, RawCompany, etc.)
normalize.go — Normalize(): raw events → Summary (parent fields + productActivity)
client_test.go — Tests Search()'s pagination logic against a fake local HTTP server
filter_test.go — Tests BuildPostFilter()'s query construction for every param combination
normalize_test.go — Tests Normalize()'s classification/aggregation logic
internal/handler/
events.go — GET /events handler: param validation → Search → Normalize → JSON
events_test.go — Full handler test suite, using a mock Moesif client
helpers_test.go — mockMoesifClient test double

---

**Data flow:** `FilterCriteria` → `BuildPostFilter()` (query DSL) → `Client.Search()` (paginated fetch + parse) → `Normalize()` (raw hits → `Summary`, with nested `ProductActivity`) → JSON response.

---

## Why this is a separate repo

Built in an isolated sandbox repo (`uvini-wso2/moesif-integration-service`) rather than directly in the `cs-tools` monorepo fork, per team decision — to freely iterate against unconfirmed Moesif response shapes without polluting shared history. The `cs-tools` fork was reverted to clean `main` with no trace of this work. **This code has not yet been ported into `cs-tools`** — that's a separate next step, pending team decision on timing/approach.

---

## Testing notes

- All tests run without any real Moesif API key or network access — `internal/handler` uses a mock satisfying the `eventsClient` interface; `internal/moesif`'s pagination tests use a local `httptest.Server` simulating Moesif's response shape.
- Test data (`normalize_test.go`, `client_test.go`, `filter_test.go`) is hand-built to match confirmed real shapes, not guesses — including edge cases like step `0` being a valid onboarding step (must not be confused with "no skip occurred"), and geo/wizard-path data being taken from the correct (most-recent, or first-found) event.
- 23 tests total, covering filter construction, pagination, classification/normalization, and the full HTTP handler.

---

## Environment variables (`.env`)

| Var | Purpose |
|---|---|
| `MOESIF_API_KEY` | Management API key (Bearer token). Get from Moesif admin — see "Required Moesif scopes" above. |
| `MOESIF_BASE_URL` | `https://api.moesif.com` |
| `PORT` | Local server port (default `8081` if unset) |

**Never commit `.env`** — it's git-ignored at the repo root.