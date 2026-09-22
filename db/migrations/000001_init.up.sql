-- Shared trigger function: every table's updated_at is kept current by
-- one BEFORE UPDATE trigger per table (Postgres has no built-in
-- equivalent of MySQL's ON UPDATE CURRENT_TIMESTAMP).
CREATE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Lookup tables replace the old native asset_type/operational_status
-- ENUM types: adding a new type/status later is a plain seed-row
-- INSERT, not an ALTER TYPE/schema migration. Natural-key PK (the code
-- itself), matching assets.asset_id — no surrogate id, no translation
-- layer between an int id and the string codes used throughout the
-- API/CSV.
CREATE TABLE asset_types (
    code        TEXT PRIMARY KEY,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_asset_types_set_updated_at
    BEFORE UPDATE ON asset_types
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

INSERT INTO asset_types (code) VALUES
    ('SUBSTATION'),
    ('TRANSFORMER'),
    ('LV_BOARD'),
    ('SWITCHBOARD'),
    ('SWITCHBOARD_PANEL');

CREATE TABLE operational_statuses (
    code        TEXT PRIMARY KEY,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_operational_statuses_set_updated_at
    BEFORE UPDATE ON operational_statuses
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

INSERT INTO operational_statuses (code) VALUES
    ('IN_SERVICE'),
    ('MAINTENANCE'),
    ('OUT_OF_SERVICE');

-- Permitted child->parent type pairs (brief section 3), a table instead
-- of hardcoded Go constants for the same scalability reason as
-- asset_types/operational_statuses. A type with no row here
-- (SUBSTATION) is a root type and must have no parent at all — see
-- ARCHITECTURE.md section 4 pass 2, rule 1.
CREATE TABLE asset_type_parent_rules (
    child_type_code   TEXT NOT NULL REFERENCES asset_types(code) ON UPDATE CASCADE,
    parent_type_code  TEXT NOT NULL REFERENCES asset_types(code) ON UPDATE CASCADE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (child_type_code, parent_type_code)
);

CREATE TRIGGER trg_asset_type_parent_rules_set_updated_at
    BEFORE UPDATE ON asset_type_parent_rules
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

INSERT INTO asset_type_parent_rules (child_type_code, parent_type_code) VALUES
    ('TRANSFORMER', 'SUBSTATION'),
    ('LV_BOARD', 'SUBSTATION'),
    ('SWITCHBOARD', 'SUBSTATION'),
    ('SWITCHBOARD_PANEL', 'SWITCHBOARD');

CREATE TABLE assets (
    asset_id            TEXT PRIMARY KEY,
    parent_asset_id     TEXT REFERENCES assets(asset_id),
    asset_type          TEXT NOT NULL REFERENCES asset_types(code) ON UPDATE CASCADE,
    asset_name          TEXT NOT NULL,
    operational_status  TEXT NOT NULL REFERENCES operational_statuses(code) ON UPDATE CASCADE,
    voltage_kv          NUMERIC,
    rating_kva          NUMERIC CHECK (rating_kva IS NULL OR rating_kva >= 0),
    manufacturer        TEXT,
    model               TEXT,
    serial_number       TEXT,
    commissioned_date   DATE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_assets_set_updated_at
    BEFORE UPDATE ON assets
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_assets_parent       ON assets (parent_asset_id);
CREATE INDEX idx_assets_type         ON assets (asset_type);
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX idx_assets_name_trgm    ON assets USING gin (asset_name gin_trgm_ops);
CREATE INDEX idx_assets_id_trgm      ON assets USING gin (asset_id gin_trgm_ops);

-- Audit trail for each upload, satisfying 4.2's "make it clear whether
-- data was committed."
CREATE TABLE import_runs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    filename        TEXT NOT NULL,
    client_ip       INET NOT NULL,
    total_rows      INT NOT NULL,
    imported_rows   INT NOT NULL,
    rejected_rows   INT NOT NULL,
    committed       BOOLEAN NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_import_runs_set_updated_at
    BEFORE UPDATE ON import_runs
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE import_rejections (
    id              BIGSERIAL PRIMARY KEY,
    import_run_id   UUID NOT NULL REFERENCES import_runs(id) ON DELETE CASCADE,
    csv_row_number  INT NOT NULL,
    asset_id        TEXT,
    reason          TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_import_rejections_set_updated_at
    BEFORE UPDATE ON import_rejections
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
