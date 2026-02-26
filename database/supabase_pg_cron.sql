-- =============================================================
-- SUPABASE pg_cron SMART SCHEDULING SYSTEM
-- All times are IST (Asia/Kolkata = UTC+5:30)
-- pg_cron uses UTC, so we offset accordingly
-- =============================================================

-- Enable pg_cron extension
CREATE EXTENSION IF NOT EXISTS pg_cron;
GRANT USAGE ON SCHEMA cron TO postgres;

-- Job dispatch table: pg_cron writes triggers here, Go backend polls and executes
CREATE TABLE job_dispatch (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_type VARCHAR(100) NOT NULL,
    target_ipo_id UUID REFERENCES ipo_list(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    priority INTEGER NOT NULL DEFAULT 5,
    payload JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    picked_up_at TIMESTAMP,
    completed_at TIMESTAMP,
    error_message TEXT,

    CONSTRAINT chk_job_dispatch_status CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    CONSTRAINT chk_job_dispatch_priority CHECK (priority BETWEEN 1 AND 10)
);

CREATE INDEX idx_job_dispatch_pending ON job_dispatch(status, priority DESC, created_at ASC) WHERE status = 'pending';
CREATE INDEX idx_job_dispatch_type ON job_dispatch(job_type, status);
CREATE INDEX idx_job_dispatch_ipo ON job_dispatch(target_ipo_id) WHERE target_ipo_id IS NOT NULL;
CREATE INDEX idx_job_dispatch_cleanup ON job_dispatch(created_at) WHERE status IN ('completed', 'failed');

ALTER TABLE job_dispatch ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Service role full access job_dispatch" ON job_dispatch FOR ALL USING (auth.role() = 'service_role');

-- =============================================================
-- FIXED SCHEDULE CRON JOBS
-- =============================================================

-- 1. Daily IPO Fetch — 6:00 AM IST (UTC 00:30)
SELECT cron.schedule('daily-ipo-fetch', '30 0 * * *',
    $$INSERT INTO job_dispatch (job_type, priority, payload)
      VALUES ('daily_ipo_update', 5, '{"trigger": "cron", "schedule": "daily_6am_ist"}')$$);

-- 2. GMP Update — Every hour
SELECT cron.schedule('gmp-update-hourly', '0 * * * *',
    $$INSERT INTO job_dispatch (job_type, priority, payload)
      VALUES ('gmp_update', 6, '{"trigger": "cron", "schedule": "hourly"}')$$);

-- 3. GMP History — 8 AM IST (UTC 02:30)
SELECT cron.schedule('gmp-history-morning', '30 2 * * *',
    $$INSERT INTO job_dispatch (job_type, priority, payload)
      VALUES ('gmp_history_update', 5, '{"trigger": "cron", "schedule": "daily_8am_ist"}')$$);

-- 4. GMP History — 8 PM IST (UTC 14:30)
SELECT cron.schedule('gmp-history-evening', '30 14 * * *',
    $$INSERT INTO job_dispatch (job_type, priority, payload)
      VALUES ('gmp_history_update', 5, '{"trigger": "cron", "schedule": "daily_8pm_ist"}')$$);

-- 5. Cache Cleanup — 2 AM IST (UTC 20:30)
SELECT cron.schedule('cache-cleanup-daily', '30 20 * * *',
    $$INSERT INTO job_dispatch (job_type, priority, payload)
      VALUES ('cache_cleanup', 2, '{"trigger": "cron", "schedule": "daily_2am_ist"}')$$);

-- 6. Stale Job Cleanup — purge completed/failed jobs older than 7 days
SELECT cron.schedule('job-dispatch-cleanup', '0 3 * * *',
    $$DELETE FROM job_dispatch WHERE status IN ('completed', 'failed') AND created_at < NOW() - INTERVAL '7 days'$$);

-- =============================================================
-- SMART FUNCTION: IPO Closing-Day GMP Burst
--
-- Runs daily at 8:30 AM IST. For each IPO closing TODAY:
--   9 AM - 1 PM:  2 runs (9:30 AM, 12:00 PM)
--   1 PM - 4 PM:  6 runs (every 30 min)
--   6 PM:         1 final run
-- Total: 9 targeted jobs per closing IPO
-- =============================================================
CREATE OR REPLACE FUNCTION schedule_closing_day_gmp_burst()
RETURNS void AS $$
DECLARE
    ipo_rec RECORD;
    ist_now TIMESTAMP;
BEGIN
    ist_now := NOW() AT TIME ZONE 'Asia/Kolkata';
    FOR ipo_rec IN
        SELECT id, name, company_code FROM ipo_list
        WHERE close_date::date = ist_now::date
          AND status IN ('Active', 'Open', 'Upcoming', 'Live')
    LOOP
        -- 9:30 AM IST
        INSERT INTO job_dispatch (job_type, target_ipo_id, priority, payload)
        VALUES ('gmp_history_update', ipo_rec.id, 9,
            json_build_object('trigger','closing_day_burst','ipo_name',ipo_rec.name,'company_code',ipo_rec.company_code,'window','9am-1pm','run',1)::jsonb);
        -- 12:00 PM IST
        INSERT INTO job_dispatch (job_type, target_ipo_id, priority, payload)
        VALUES ('gmp_history_update', ipo_rec.id, 9,
            json_build_object('trigger','closing_day_burst','ipo_name',ipo_rec.name,'company_code',ipo_rec.company_code,'window','9am-1pm','run',2)::jsonb);
        -- 1 PM - 4 PM: 6 runs
        INSERT INTO job_dispatch (job_type, target_ipo_id, priority, payload)
        SELECT 'gmp_history_update', ipo_rec.id, 10,
            json_build_object('trigger','closing_day_burst','ipo_name',ipo_rec.name,'company_code',ipo_rec.company_code,'window','1pm-4pm','run',gs.n)::jsonb
        FROM generate_series(1, 6) AS gs(n);
        -- 6 PM IST final
        INSERT INTO job_dispatch (job_type, target_ipo_id, priority, payload)
        VALUES ('gmp_history_update', ipo_rec.id, 8,
            json_build_object('trigger','closing_day_final','ipo_name',ipo_rec.name,'company_code',ipo_rec.company_code,'window','6pm_final')::jsonb);
    END LOOP;
END;
$$ LANGUAGE plpgsql;

SELECT cron.schedule('closing-day-gmp-burst', '0 3 * * *',
    $$SELECT schedule_closing_day_gmp_burst()$$);

-- =============================================================
-- SMART FUNCTION: Listing-Day Result Checks
--
-- Runs daily at 9:45 AM IST. For each IPO with listing_date = TODAY:
--   3 checks at 10:15 AM, 1:00 PM, 3:45 PM IST
-- =============================================================
CREATE OR REPLACE FUNCTION schedule_listing_day_result_checks()
RETURNS void AS $$
DECLARE
    ipo_rec RECORD;
    ist_now TIMESTAMP;
BEGIN
    ist_now := NOW() AT TIME ZONE 'Asia/Kolkata';
    FOR ipo_rec IN
        SELECT id, name, company_code FROM ipo_list
        WHERE listing_date::date = ist_now::date
          AND status IN ('Active', 'Open', 'Upcoming', 'Live', 'Listed', 'Allotted')
    LOOP
        INSERT INTO job_dispatch (job_type, target_ipo_id, priority, payload) VALUES
        ('result_release_check', ipo_rec.id, 10,
            json_build_object('trigger','listing_day_check','ipo_name',ipo_rec.name,'scheduled_time','10:15 AM IST','run',1)::jsonb),
        ('result_release_check', ipo_rec.id, 10,
            json_build_object('trigger','listing_day_check','ipo_name',ipo_rec.name,'scheduled_time','1:00 PM IST','run',2)::jsonb),
        ('result_release_check', ipo_rec.id, 10,
            json_build_object('trigger','listing_day_check','ipo_name',ipo_rec.name,'scheduled_time','3:45 PM IST','run',3)::jsonb);
    END LOOP;
END;
$$ LANGUAGE plpgsql;

SELECT cron.schedule('listing-day-result-checks', '15 4 * * *',
    $$SELECT schedule_listing_day_result_checks()$$);
