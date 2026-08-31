-- name: CreateUser :exec
INSERT OR IGNORE INTO users (user_id, username, first_name, chat_id)
VALUES (?, ?, ?, ?);

-- name: CreateRule :exec
INSERT INTO rules (user_id, chat_id, store, query, city, min_price, max_price)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListRulesByUser :many
SELECT id, user_id, chat_id, store, query, city, min_price, max_price, enabled, created_at, updated_at
FROM rules WHERE user_id = ? ORDER BY id;

-- name: ListEnabledRules :many
SELECT id, user_id, chat_id, store, query, city, min_price, max_price, enabled, created_at, updated_at
FROM rules WHERE enabled = 1 ORDER BY id;

-- name: GetRule :one
SELECT id, user_id, chat_id, store, query, city, min_price, max_price, enabled, created_at, updated_at
FROM rules WHERE id = ? AND user_id = ?;

-- name: UpdateRule :exec
UPDATE rules SET query = ?, city = ?, min_price = ?, max_price = ?, updated_at = datetime('now')
WHERE id = ? AND user_id = ?;

-- name: SetRuleEnabled :exec
UPDATE rules SET enabled = ?, updated_at = datetime('now') WHERE id = ? AND user_id = ?;

-- name: DeleteRule :exec
DELETE FROM rules WHERE id = ? AND user_id = ?;

-- name: SetSetting :exec
INSERT INTO bot_settings (key, value) VALUES (?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value;

-- name: GetSetting :one
SELECT value FROM bot_settings WHERE key = ?;

-- name: UpsertDialogState :exec
INSERT INTO dialog_state (chat_id, state, data, updated_at)
VALUES (?, ?, ?, datetime('now'))
ON CONFLICT(chat_id) DO UPDATE SET state = excluded.state, data = excluded.data, updated_at = datetime('now');

-- name: GetDialogState :one
SELECT chat_id, state, data, updated_at FROM dialog_state WHERE chat_id = ?;

-- name: DeleteDialogState :exec
DELETE FROM dialog_state WHERE chat_id = ?;

-- name: GetSentOffer :one
SELECT last_price FROM sent_offers WHERE chat_id = ? AND offer_key = ?;

-- name: InsertSentOffer :exec
INSERT INTO sent_offers (chat_id, offer_key, last_price)
VALUES (?, ?, ?);

-- name: UpdateSentOfferPrice :exec
UPDATE sent_offers SET last_price = ? WHERE chat_id = ? AND offer_key = ?;
