-- +goose Up
CREATE TABLE sent_offers (
    chat_id INTEGER NOT NULL,
    offer_key TEXT NOT NULL,
    last_price TEXT NOT NULL,
    first_seen_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (chat_id, offer_key)
);

-- +goose Down
DROP TABLE sent_offers;
