-- +goose Up
CREATE TABLE modules (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token         text NOT NULL UNIQUE DEFAULT public_token(),
    name          text NOT NULL UNIQUE,
    version       text NOT NULL,
    core_range    text NOT NULL,
    api_prefix    text NOT NULL,
    requirements  jsonb NOT NULL DEFAULT '[]',
    enabled       boolean NOT NULL DEFAULT true,
    registered_at timestamptz NOT NULL
);

ALTER TABLE permissions
    ADD COLUMN module_id  bigint REFERENCES modules (id),
    ADD COLUMN retired_at timestamptz;

-- +goose Down
ALTER TABLE permissions DROP COLUMN retired_at, DROP COLUMN module_id;
DROP TABLE modules;
