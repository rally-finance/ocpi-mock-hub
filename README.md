<a href="https://www.getrally.com/">
  <img src="https://www.getrally.com/apple-touch-icon.png" alt="Rally" height="40">
</a>

### Built by [Rally](https://www.getrally.com/)

[Rally](https://www.getrally.com/) builds modern fleet expense management for European businesses. One Visa-powered card replaces legacy fuel cards and scattered apps, giving fleet operators real-time spend controls, AI-powered analytics, and fraud detection across 21 countries.

---

# ocpi-mock-hub

A mock OCPI 2.2.1 hub server in Go for end-to-end development and testing of OCPI integrations without a live partner.

## Quick Start

```bash
go run .
```

The server starts on port 4000. OCPI versions URL: `http://localhost:4000/ocpi/versions`

## How It Works

The mock behaves as an OCPI HUB (`DE*HUB`) with 5 fake CPO parties and ~50 charging locations across Europe. It supports the full OCPI 2.2.1 flow:

1. **Handshake** — Token A → versions → credentials → Token B (including PUT re-registration and DELETE unregister)
2. **Pull** — Locations (with EVSE and connector sub-resources), tariffs, sessions, CDRs, hub client info — all with get-by-ID, date filtering, and paging
3. **Push** — Token registration from eMSP, location/tariff/EVSE status updates to eMSP
4. **Commands** — START_SESSION, STOP_SESSION, RESERVE_NOW, CANCEL_RESERVATION, UNLOCK_CONNECTOR with async lifecycle
5. **Routing headers** — `OCPI-To-*` party filtering on all sender endpoints; `OCPI-From-*` set on all OCPI responses

### Session & EVSE Lifecycle

When a START_SESSION command is received:
- Session is created with status `PENDING`
- After a configurable delay, callback is sent and session becomes `ACTIVE`
- The corresponding EVSE status is pushed as `CHARGING` to the eMSP
- After configurable duration, session `COMPLETED`, a CDR is generated, and EVSE status is pushed back to `AVAILABLE`
- GET endpoints dynamically overlay EVSE status based on active sessions

When a STOP_SESSION command is received:
- Session transitions to `STOPPING` (the async command result callback fires on the next tick)
- On the next tick, session moves to `COMPLETED`, a CDR is generated, and EVSE status returns to `AVAILABLE`

Full state machine: `PENDING` → `ACTIVE` → `COMPLETED`, or `ACTIVE` → `STOPPING` → `COMPLETED`

In standalone mode, a background ticker handles this. On Vercel, a cron job (`/api/tick`) does it.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `4000` | Listen port |
| `MOCK_TOKEN_A` | `mock-token-a-secret` | Pre-shared Token A |
| `MOCK_HUB_COUNTRY` | `DE` | Hub country code |
| `MOCK_HUB_PARTY` | `HUB` | Hub party ID |
| `MOCK_INITIATE_HANDSHAKE_VERSIONS_URL` | — | Optional URL advertised in outbound `POST /credentials` during admin-initiated handshake (defaults to inferred request host URL) |
| `MOCK_SESSION_DURATION_S` | `60` | Session duration (seconds) |
| `MOCK_COMMAND_DELAY_MS` | `2000` | Command callback delay (ms) |
| `EMSP_CALLBACK_URL` | `http://localhost:3000/api/ocpi` | eMSP callback URL |
| `MOCK_SEED_LOCATIONS` | `50` | Number of fake locations |
| `MOCK_MODE` | `happy` | Simulation mode |
| `FREE_TIER_REDIS_URL` | — | Redis connection URL (enables persistent store) |

## Simulation Modes

Set via `MOCK_MODE` env var or `POST /admin/mode`:

**Normal:**
- **happy** — Normal operation, all requests succeed

**Error modes:**
- **reject** — Commands return REJECTED, authorization returns NOT_ALLOWED
- **rate-limit** — 50% of requests return HTTP 429 with `Retry-After: 2` header
- **random-500** — ~20% of requests return HTTP 500 server error
- **auth-fail** — Token authorization returns random rejections (NOT_ALLOWED, EXPIRED, BLOCKED)

**Stress modes:**
- **slow** — Adds 3–8 second random delay to all OCPI responses
- **partial** — Returns truncated/malformed JSON (tests eMSP error handling)
- **pagination-stress** — Forces 1-item pages on all list endpoints

## Connecting your eMSP

Point your eMSP's OCPI client at the mock hub:

```env
OCPI_HUB_VERSIONS_URL=http://localhost:4000/ocpi/versions
OCPI_HUB_INITIAL_TOKEN_A=mock-token-a-secret
OCPI_TARGET_COUNTRY_CODE=DE
OCPI_TARGET_PARTY_ID=HUB
```

Or use the admin UI at `http://localhost:4000/admin/` to initiate a hub-to-eMSP handshake.

## OCPI Endpoints

### Version Discovery

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/ocpi/versions` | Token A | List supported versions |
| GET | `/ocpi/2.2.1` | Token A | Version 2.2.1 details and module endpoints |

### Credentials

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/ocpi/2.2.1/credentials` | Token A | Register (initial handshake) |
| GET | `/ocpi/2.2.1/credentials` | Token B | Get current credentials |
| PUT | `/ocpi/2.2.1/credentials` | Token B | Re-register (rotates Token B) |
| DELETE | `/ocpi/2.2.1/credentials` | Token B | Unregister (clears all handshake state) |

### Sender Modules (hub → eMSP pull)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/ocpi/2.2.1/sender/locations` | Token B | List locations (paged, date-filtered, OCPI-To filtered) |
| GET | `/ocpi/2.2.1/sender/locations/{locationID}` | Token B | Get single location |
| GET | `/ocpi/2.2.1/sender/locations/{locationID}/{evseUID}` | Token B | Get single EVSE |
| GET | `/ocpi/2.2.1/sender/locations/{locationID}/{evseUID}/{connectorID}` | Token B | Get single connector |
| GET | `/ocpi/2.2.1/sender/tariffs` | Token B | List tariffs |
| GET | `/ocpi/2.2.1/sender/tariffs/{cc}/{pid}/{tariffID}` | Token B | Get single tariff |
| GET | `/ocpi/2.2.1/sender/sessions` | Token B | List sessions |
| GET | `/ocpi/2.2.1/sender/sessions/{cc}/{pid}/{sessionID}` | Token B | Get single session |
| PUT | `/ocpi/2.2.1/sender/sessions/{sessionID}/charging_preferences` | Token B | Submit ChargingPreferences for an ACTIVE session |
| GET | `/ocpi/2.2.1/sender/cdrs` | Token B | List CDRs |
| GET | `/ocpi/2.2.1/sender/cdrs/{cc}/{pid}/{cdrID}` | Token B | Get single CDR |
| GET | `/ocpi/2.2.1/sender/tokens` | Token B | List tokens |
| GET | `/ocpi/2.2.1/sender/tokens/{cc}/{pid}/{uid}` | Token B | Get single token |
| POST | `/ocpi/2.2.1/sender/tokens/{cc}/{pid}/{uid}/authorize` | Token B | Real-time authorization |
| GET | `/ocpi/2.2.1/sender/hubclientinfo` | Token B | List connected parties |

### Receiver Modules (eMSP → hub push)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| PUT | `/ocpi/2.2.1/receiver/tokens/{cc}/{pid}/{uid}` | Token B | Push/replace a token (`?type=` selects TokenType, default `RFID`) |
| PATCH | `/ocpi/2.2.1/receiver/tokens/{cc}/{pid}/{uid}` | Token B | Merge partial token update (requires `last_updated`) |
| GET | `/ocpi/2.2.1/receiver/tokens/{cc}/{pid}/{uid}` | Token B | Read back the stored token (eMSP validation) |
| POST | `/ocpi/2.2.1/receiver/commands/{command}` | Token B | Send a command |
| PUT | `/ocpi/2.2.1/receiver/sessions/{cc}/{pid}/{sessionID}` | Token B | Push/replace a session |
| PATCH | `/ocpi/2.2.1/receiver/sessions/{cc}/{pid}/{sessionID}` | Token B | Merge partial session update; `charging_periods` appends |
| POST | `/ocpi/2.2.1/receiver/cdrs` | Token B | Push a CDR (returns `Location` header) |
| GET | `/ocpi/2.2.1/receiver/cdrs/{cdrID}` | Token B | Get a received CDR |
| PUT | `/ocpi/2.2.1/receiver/chargingprofiles/{sessionID}` | Token B | `SetChargingProfile`; sync `ChargingProfileResponse`, async `ChargingProfileResult` to `response_url` |
| GET | `/ocpi/2.2.1/receiver/chargingprofiles/{sessionID}?duration=&response_url=` | Token B | Sync `ChargingProfileResponse`, async `ActiveChargingProfileResult` to `response_url` |
| DELETE | `/ocpi/2.2.1/receiver/chargingprofiles/{sessionID}?response_url=` | Token B | `ClearChargingProfile`; sync `ChargingProfileResponse`, async `ClearProfileResult` to `response_url` |

All sender list endpoints support `date_from`/`date_to` query parameters, `offset`/`limit` paging, and `OCPI-To-Country-Code`/`OCPI-To-Party-Id` header filtering.

## Multi-Party Support

The hub supports multiple eMSPs connected simultaneously. Each party gets its own Token B, callback URL, and credentials record keyed by `{country_code}/{party_id}`. The auth middleware resolves incoming Token B values to the correct party.

Use the admin-initiated handshake flow or standard `POST /credentials` from different eMSPs to register multiple parties.

## Charging Preferences

OCPI 2.2.1 adds a `ChargingPreferences` endpoint that lets an eMSP pass user
intent (profile type, departure time, energy need, V2G allowance) for an
active session. The mock exposes it at
`PUT /ocpi/2.2.1/sender/sessions/{sessionID}/charging_preferences` and returns
the full `ChargingPreferencesResponse` enum depending on session state and the
request body:

- `ACCEPTED` — stored on the session so the simulation can consume it.
- `DEPARTURE_REQUIRED` — missing `departure_time` for `CHEAP` or `GREEN`.
- `ENERGY_NEED_REQUIRED` — missing `energy_need` for `CHEAP`.
- `PROFILE_TYPE_NOT_SUPPORTED` — unrecognized `profile_type`.
- `NOT_POSSIBLE` — session is not `ACTIVE`, or the EVSE does not advertise
  `CHARGING_PREFERENCES_CAPABLE` (roughly half of seeded EVSEs do).

## Charging Profiles

The `ChargingProfiles` receiver module lets an eMSP set, inspect, and clear
power limits on active sessions. All three verbs follow OCPI 2.2.1's
request/response + async callback pattern:

- The sync HTTP response body is always a `ChargingProfileResponse`
  (`{"result": "<ChargingProfileResponseType>", "timeout": <seconds>}`).
  `result` is one of `ACCEPTED`, `NOT_SUPPORTED`, `REJECTED`, `TOO_OFTEN`, or
  `UNKNOWN_SESSION`. `timeout` tells the eMSP how long to wait before assuming
  the async callback will never arrive (the mock advertises 30s).
- After the sync response the hub POSTs a follow-up to the eMSP's
  `response_url`:
  - **PUT** (`SetChargingProfile`) → `ChargingProfileResult` `{result}`
    (`ACCEPTED` / `REJECTED` / `UNKNOWN`).
  - **GET** → `ActiveChargingProfileResult` `{result, profile}` where `profile`
    is an `ActiveChargingProfile` with `start_date_time` and `charging_profile`
    (either the stored profile or a synthesized empty-profile fallback).
  - **DELETE** (`ClearChargingProfile`) → `ClearProfileResult` `{result}`.
    `ACCEPTED` when a profile existed and was cleared, `UNKNOWN` when there was
    nothing to clear.
- For all three verbs a missing session returns a sync `UNKNOWN_SESSION` result
  with envelope `status_code=1000` (per spec this is a domain outcome, not an
  OCPI error) and skips the async callback. A missing `response_url` returns
  HTTP 400 with OCPI `2003` (`invalid parameters`).
- `POST /admin/push-active-profile` triggers an unsolicited
  `ActiveChargingProfile` PUT to a caller-provided `target_url`, simulating the
  CPO-side behavior when an EVSE notifies the hub that the active profile has
  changed outside of a request/response cycle.
- The simulation lifecycle still respects `min_charging_rate` from stored
  profiles when capping kWh growth.

## Data Model Enrichment

Generated OCPI objects include spec-realistic detail:

- **Locations** — `state`, `related_locations` (additional geo-coordinates), `directions`, `operator`/`suboperator`/`owner` as full `BusinessDetails` objects with optional `website` and `logo`, `facilities`, `opening_times`, `charging_when_closed`, `images`, `energy_mix`. The `publish_allowed_to` list (`PublishTokenType`) is available on the type for private locations.
- **EVSEs** — `status_schedule` (planned maintenance windows), `floor_level`, `physical_reference` (bay numbers), `directions`, `parking_restrictions`, `images`. Coverage is partial by design so clients see both populated and omitted optional fields in one corpus.
- **Connectors** — `terms_and_conditions` URL in addition to the existing power, amperage, voltage, and tariff identifiers.
- **Tariffs** — full OCPI 2.2.1 object: all five `TariffType` enum values (REGULAR, AD_HOC_PAYMENT, PROFILE_CHEAP, PROFILE_FAST, PROFILE_GREEN), all four `PriceComponent` types (ENERGY, FLAT, TIME, PARKING_TIME), `tariff_alt_text` (multi-lingual), `tariff_alt_url`, `min_price`/`max_price`, `start_date_time`/`end_date_time` validity windows, `energy_mix`, and every `TariffRestrictions` field (`start_time`/`end_time`, `start_date`/`end_date`, `min_kwh`/`max_kwh`, `min_current`/`max_current`, `min_power`/`max_power`, `min_duration`/`max_duration`, `day_of_week`, `reservation` including `RESERVATION_EXPIRES`). Seven tariffs per CPO cover the full matrix.
- **Sessions** — `authorization_reference`, `meter_id`, `charging_periods` with dimensions.
- **CDRs** — `total_fixed_cost`, `total_energy_cost`, `total_time_cost`, `total_parking_cost`, `total_parking_time`, `total_reservation_cost`, `remark`, `meter_id` and `authorization_reference` propagated from the originating session, `invoice_reference_id`, `home_charging_compensation`, `tariffs[]` (matching the CPO/currency of the session), and a representative `signed_data` block (OCMF encoding).
- **Tokens** — the receiver PUT/PATCH flow preserves OCPI 2.2.1 optional fields (`visual_number`, `issuer`, `group_id`, `language`, `default_profile_type`, `energy_contract`) on storage; they round-trip untouched on subsequent GETs.

## Admin UI

Navigate to `/admin/` for a built-in dashboard with:

- **Connection status** — handshake state, Token B, eMSP callback URL
- **Handshake trigger** — initiate hub → eMSP handshake from the UI
- **Push controls** — trigger location/tariff update pushes with configurable traffic patterns (burst, staggered, realistic)
- **Locations** — browse seed data
- **Sessions & CDRs** — monitor active sessions and completed CDRs
- **Request log** — last 500 OCPI requests

## Deploying to Vercel

The project includes `vercel.json` with rewrites and cron configuration. Connect the repo to Vercel and add a Redis store — the `FREE_TIER_REDIS_URL` env var will be injected automatically.

## Fake CPO Parties

| Party | Name |
|-------|------|
| `DE*AAA` | FastCharge GmbH |
| `NL*BBB` | GreenPlug BV |
| `FR*CCC` | ChargeRapide SA |
| `AT*DDD` | AlpenStrom |
| `BE*EEE` | PowerBelgium |

## License

[MIT](LICENSE)
