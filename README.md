# ocpi-mock-hub

A mock OCPI 2.2.1 hub server in Go for end-to-end development and testing of OCPI integrations without a live partner.

## Quick Start

```bash
go run .
```

The server starts on port 4000. OCPI versions URL: `http://localhost:4000/ocpi/versions`

## How It Works

The mock behaves as an OCPI HUB (`DE*HUB`) with 5 fake CPO parties and ~50 charging locations across Europe. It supports the full OCPI 2.2.1 flow:

1. **Handshake** — Token A → versions → credentials → Token B
2. **Pull** — Locations, tariffs, sessions, CDRs, hub client info
3. **Push** — Token registration from eMSP, location/tariff updates to eMSP
4. **Commands** — START_SESSION / STOP_SESSION with async session lifecycle

### Session Lifecycle

When a START_SESSION command is received:
- Session is created with status `PENDING`
- After a configurable delay, callback is sent and session becomes `ACTIVE`
- After configurable duration, session `COMPLETED` and a CDR is generated

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
| `REDIS_URL` | — | Redis connection URL (enables persistent store) |

## Simulation Modes

Set via `MOCK_MODE` env var or `POST /admin/mode`:

- **happy** — Normal operation
- **slow** — Extended command delays
- **reject** — Commands return REJECTED
- **partial** — Some data has missing optional fields
- **pagination-stress** — Very small page sizes

## Connecting your eMSP

Point your eMSP's OCPI client at the mock hub:

```env
OCPI_HUB_VERSIONS_URL=http://localhost:4000/ocpi/versions
OCPI_HUB_INITIAL_TOKEN_A=mock-token-a-secret
OCPI_TARGET_COUNTRY_CODE=DE
OCPI_TARGET_PARTY_ID=HUB
```

Or use the admin UI at `http://localhost:4000/admin/` to initiate a hub-to-eMSP handshake.

## Admin UI

Navigate to `/admin/` for a built-in dashboard with:

- **Connection status** — handshake state, Token B, eMSP callback URL
- **Handshake trigger** — initiate hub → eMSP handshake from the UI
- **Push controls** — trigger location/tariff update pushes with configurable traffic patterns (burst, staggered, realistic)
- **Locations** — browse seed data
- **Sessions & CDRs** — monitor active sessions and completed CDRs
- **Request log** — last 100 OCPI requests

## Deploying to Vercel

The project includes `vercel.json` with rewrites and cron configuration. Connect the repo to Vercel and add a Redis store — the `REDIS_URL` env var will be injected automatically.

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
