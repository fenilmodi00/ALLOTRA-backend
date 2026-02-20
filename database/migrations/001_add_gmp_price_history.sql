-- Migration: Add GMP Price History Tables
-- Description: Creates tables for storing historical GMP price data and job execution logs
-- Date: 2024
-- Requirements: 2.1, 2.4, 2.5

-- GMP Price History table for storing historical GMP data over time
CREATE TABLE IF NOT EXISTS gmp_price_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ipo_id UUID NOT NULL,
    company_code VARCHAR(50) NOT NULL,
    record_date DATE NOT NULL,
    ipo_price DECIMAL(10, 2) NOT NULL,
    gmp_value DECIMAL(10, 2) NOT NULL,
    estimated_listing DECIMAL(10, 2) NOT NULL,
    listing_percent DECIMAL(10, 2) NOT NULL,
    estimated_profit DECIMAL(10, 2) DEFAULT 0,
    subscription_status VARCHAR(100),
    sub2_sauda DECIMAL(10, 2) DEFAULT 0,
    last_updated VARCHAR(200),
    data_source VARCHAR(100) DEFAULT 'investorgain.com',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Foreign key constraints
    CONSTRAINT fk_gmp_history_ipo_id FOREIGN KEY (ipo_id) REFERENCES ipo_list(id) ON DELETE CASCADE,
    
    -- Unique constraint to prevent duplicates
    CONSTRAINT uk_gmp_history_ipo_date UNIQUE (ipo_id, record_date),
    
    -- Check constraints for data validation
    CONSTRAINT chk_gmp_history_prices_positive CHECK (
        ipo_price >= 0 AND gmp_value >= 0 AND estimated_listing >= 0
    ),
    CONSTRAINT chk_gmp_history_listing_calculation CHECK (
        ABS(estimated_listing - (ipo_price + gmp_value)) < 0.01
    )
);

-- Add constraints for GMP price history table
DO $$ 
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE constraint_name = 'gmp_price_history_company_code_not_empty'
    ) THEN
        ALTER TABLE gmp_price_history ADD CONSTRAINT gmp_price_history_company_code_not_empty CHECK (company_code != '');
    END IF;
    
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE constraint_name = 'gmp_price_history_data_source_not_empty'
    ) THEN
        ALTER TABLE gmp_price_history ADD CONSTRAINT gmp_price_history_data_source_not_empty CHECK (data_source != '');
    END IF;
END $$;

-- Performance indexes for GMP price history
CREATE INDEX IF NOT EXISTS idx_gmp_history_ipo_id ON gmp_price_history(ipo_id);
CREATE INDEX IF NOT EXISTS idx_gmp_history_company_code ON gmp_price_history(company_code);
CREATE INDEX IF NOT EXISTS idx_gmp_history_record_date ON gmp_price_history(record_date DESC);
CREATE INDEX IF NOT EXISTS idx_gmp_history_ipo_date_range ON gmp_price_history(ipo_id, record_date DESC);
CREATE INDEX IF NOT EXISTS idx_gmp_history_created_at ON gmp_price_history(created_at DESC);

-- Job execution tracking table for GMP history updates
CREATE TABLE IF NOT EXISTS gmp_history_job_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_start_time TIMESTAMP NOT NULL,
    job_end_time TIMESTAMP,
    ipos_processed INTEGER DEFAULT 0,
    successful_scrapes INTEGER DEFAULT 0,
    failed_scrapes INTEGER DEFAULT 0,
    total_records_added INTEGER DEFAULT 0,
    execution_status VARCHAR(50) DEFAULT 'running',
    error_summary TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Add constraints for job log table
DO $$ 
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE constraint_name = 'gmp_history_job_log_execution_status_not_empty'
    ) THEN
        ALTER TABLE gmp_history_job_log ADD CONSTRAINT gmp_history_job_log_execution_status_not_empty CHECK (execution_status != '');
    END IF;
    
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE constraint_name = 'gmp_history_job_log_ipos_processed_non_negative'
    ) THEN
        ALTER TABLE gmp_history_job_log ADD CONSTRAINT gmp_history_job_log_ipos_processed_non_negative CHECK (ipos_processed >= 0);
    END IF;
    
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE constraint_name = 'gmp_history_job_log_successful_scrapes_non_negative'
    ) THEN
        ALTER TABLE gmp_history_job_log ADD CONSTRAINT gmp_history_job_log_successful_scrapes_non_negative CHECK (successful_scrapes >= 0);
    END IF;
    
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE constraint_name = 'gmp_history_job_log_failed_scrapes_non_negative'
    ) THEN
        ALTER TABLE gmp_history_job_log ADD CONSTRAINT gmp_history_job_log_failed_scrapes_non_negative CHECK (failed_scrapes >= 0);
    END IF;
    
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE constraint_name = 'gmp_history_job_log_total_records_non_negative'
    ) THEN
        ALTER TABLE gmp_history_job_log ADD CONSTRAINT gmp_history_job_log_total_records_non_negative CHECK (total_records_added >= 0);
    END IF;
END $$;

-- Indexes for job log table
CREATE INDEX IF NOT EXISTS idx_gmp_history_job_log_job_start_time ON gmp_history_job_log(job_start_time DESC);
CREATE INDEX IF NOT EXISTS idx_gmp_history_job_log_execution_status ON gmp_history_job_log(execution_status);
CREATE INDEX IF NOT EXISTS idx_gmp_history_job_log_created_at ON gmp_history_job_log(created_at DESC);

-- Create function to automatically update updated_at timestamp
CREATE OR REPLACE FUNCTION update_gmp_history_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger for automatic timestamp updates
DROP TRIGGER IF EXISTS trigger_update_gmp_history_updated_at ON gmp_price_history;
CREATE TRIGGER trigger_update_gmp_history_updated_at
    BEFORE UPDATE ON gmp_price_history
    FOR EACH ROW
    EXECUTE FUNCTION update_gmp_history_updated_at();

-- Add comments for documentation
COMMENT ON TABLE gmp_price_history IS 'Stores historical GMP (Grey Market Premium) price data for IPOs over time';
COMMENT ON TABLE gmp_history_job_log IS 'Tracks execution of GMP history scraping jobs for monitoring and debugging';

COMMENT ON COLUMN gmp_price_history.ipo_id IS 'Foreign key reference to ipo_list table';
COMMENT ON COLUMN gmp_price_history.company_code IS 'Company code for cross-referencing with IPO data';
COMMENT ON COLUMN gmp_price_history.record_date IS 'Date when this GMP price was recorded';
COMMENT ON COLUMN gmp_price_history.gmp_value IS 'Grey Market Premium value on this date';
COMMENT ON COLUMN gmp_price_history.estimated_listing IS 'Estimated listing price (IPO price + GMP)';
COMMENT ON COLUMN gmp_price_history.listing_percent IS 'Percentage gain/loss from IPO price';
COMMENT ON COLUMN gmp_price_history.sub2_sauda IS 'Subject to Sauda value (secondary market indicator)';

COMMENT ON COLUMN gmp_history_job_log.job_start_time IS 'Timestamp when the job started execution';
COMMENT ON COLUMN gmp_history_job_log.job_end_time IS 'Timestamp when the job completed (NULL if still running)';
COMMENT ON COLUMN gmp_history_job_log.execution_status IS 'Status: running, completed, failed, partial';
COMMENT ON COLUMN gmp_history_job_log.error_summary IS 'Summary of errors encountered during execution';
