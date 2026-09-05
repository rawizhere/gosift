# gosift

Telegram bot for monitoring retail and secondary market listings with price drop alerts.

## Features

- Multi-store parser architecture (pluggable driver interface)
- Custom search rules with price ranges, city filters and excluded words
- Matching against both listing title and description
- Photos from listings attached to Telegram cards (album for multiple photos)
- Price drop and new item notifications via Telegram
- SQLite storage (pure Go via `modernc.org/sqlite`, zero CGO)
- Graceful shutdown and rate-limited HTTP client with retry logic

## Quick Start

### Local Development

```bash
cp .env.example .env
# Set your TELEGRAM_BOT_TOKEN in .env

go run ./cmd/app
```

### Docker

```bash
docker build -t gosift -f ./deployments/Dockerfile .
docker run --env-file .env -v gosift_data:/data gosift
```

## Configuration

All configuration is handled via environment variables (see `.env.example`):

| Variable | Description | Default |
|---|---|---|
| `TELEGRAM_BOT_TOKEN` | Telegram Bot API token (required) | — |
| `TELEGRAM_ALLOWED_USERS` | Comma-separated allowed user IDs (empty = anyone) | — |
| `DB_PATH` | Path to SQLite database file | `/data/gosift.db` |
| `LOG_LEVEL` | Log level (`debug`, `info`, `warn`, `error`) | `info` |
| `LOG_FORMAT` | Log format (`text`, `json`) | `text` |
| `PARSE_INTERVAL` | Scheduler parse interval | `15m` |
| `PARSE_LIMIT` | Max offers parsed per rule per cycle | `20` |
| `PARSE_TIMEOUT` | Per-request timeout | `20s` |
| `PARSE_RETRIES` | Retries on transient HTTP errors | `3` |
| `HTTP_PROXY` | Optional HTTP proxy URL | — |
| `STORE_KOMISSIONKI_ENABLED` | Enable the komissionki store driver | `true` |
| `STORE_KOMISSIONKI_CDN_URL` | Primary photo CDN host | `https://c.komissionki.ru` |
| `STORE_KOMISSIONKI_CDN_FALLBACKS` | Comma-separated fallback photo CDN hosts | `https://cdny.komissionki.ru` |

