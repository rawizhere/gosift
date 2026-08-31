-- +goose Up
ALTER TABLE rules ADD COLUMN category TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE rules DROP COLUMN category;
