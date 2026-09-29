-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN address TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN address;
-- +goose StatementEnd