-- +goose Up
DROP TABLE bot_settings;

-- +goose Down
CREATE TABLE bot_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
