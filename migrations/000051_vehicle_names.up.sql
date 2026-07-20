ALTER TABLE vehicles
    ADD COLUMN name text NOT NULL DEFAULT '';

ALTER TABLE vehicles
    ADD CONSTRAINT vehicles_name_length CHECK (char_length(name) <= 80);
