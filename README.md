# gosift

Telegram bot for monitoring retail and secondary market listings with price drop alerts.

## Features

- Multi-store parser architecture (pluggable driver interface)
- Custom search rules with price ranges and city filters
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
| `DB_PATH` | Path to SQLite database file | `gosift.db` |
| `LOG_LEVEL` | Log level (`debug`, `info`, `warn`, `error`) | `info` |
| `POLL_INTERVAL` | Scheduler polling interval | `15m` |
| `HTTP_TIMEOUT` | Parser HTTP request timeout | `15s` |

