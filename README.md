# Gosift

Telegram-бот: пользователь задаёт критерии поиска, бот периодически парсит
магазины и присылает новые позиции и изменения цен. Первый магазин —
[komissionki.ru](https://komissionki.ru) (JSON API).

- `PLAN.md` — архитектура, хранение, этапы реализации, открытые вопросы.
- `docs/ops.md` — деплой и обслуживание на сервере.
- `docs/third-party-libs.md` — обоснование выбора библиотек.

## Стек

Go, telego (Telegram Bot API), SQLite (`modernc.org/sqlite`, без CGO),
драйверы магазинов через общий интерфейс `Parser`.

## Быстрый старт (dev)

```bash
go mod download
cp .env.example .env   # вписать TELEGRAM_BOT_TOKEN
go run ./cmd/app
```

## Деплой

Образ собирается в GitHub Actions, сканируется trivy и публикуется в GHCR
(`ghcr.io/rawizhere/gosift`, теги `latest` / `sha-<commit>` / `vX.Y.Z`).
На сервере — docker compose, см. `docs/ops.md`.
