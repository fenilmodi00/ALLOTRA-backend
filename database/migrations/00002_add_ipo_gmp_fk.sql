-- +goose Up
ALTER TABLE ipo_gmp
    ADD COLUMN IF NOT EXISTS ipo_id UUID;

WITH prioritized_matches AS (
    SELECT
        g.id AS gmp_id,
        i.id AS ipo_id,
        ROW_NUMBER() OVER (
            PARTITION BY g.id
            ORDER BY
                CASE
                    WHEN i.stock_id IS NOT NULL AND g.stock_id IS NOT NULL AND i.stock_id = g.stock_id THEN 1
                    WHEN i.company_code = g.company_code THEN 2
                    ELSE 3
                END,
                i.created_at DESC
        ) AS rn
    FROM ipo_gmp g
    JOIN ipo_list i
      ON (i.stock_id IS NOT NULL AND g.stock_id IS NOT NULL AND i.stock_id = g.stock_id)
      OR i.company_code = g.company_code
)
UPDATE ipo_gmp g
SET ipo_id = pm.ipo_id
FROM prioritized_matches pm
WHERE g.id = pm.gmp_id
  AND pm.rn = 1
  AND g.ipo_id IS NULL;

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.table_constraints
        WHERE table_schema = 'public'
          AND table_name = 'ipo_gmp'
          AND constraint_name = 'fk_ipo_gmp_ipo_id'
    ) THEN
        ALTER TABLE ipo_gmp
            ADD CONSTRAINT fk_ipo_gmp_ipo_id
            FOREIGN KEY (ipo_id)
            REFERENCES ipo_list(id)
            ON DELETE SET NULL;
    END IF;
END $$;
-- +goose StatementEnd

CREATE INDEX IF NOT EXISTS idx_ipo_gmp_ipo_id
    ON ipo_gmp(ipo_id)
    WHERE ipo_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_ipo_gmp_ipo_id;

ALTER TABLE ipo_gmp
    DROP CONSTRAINT IF EXISTS fk_ipo_gmp_ipo_id;

ALTER TABLE ipo_gmp
    DROP COLUMN IF EXISTS ipo_id;
