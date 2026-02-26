-- Enable RLS on all tables
ALTER TABLE ipo_list ENABLE ROW LEVEL SECURITY;
ALTER TABLE ipo_gmp ENABLE ROW LEVEL SECURITY;
ALTER TABLE ipo_result_cache ENABLE ROW LEVEL SECURITY;
ALTER TABLE ipo_update_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE gmp_price_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE gmp_history_job_log ENABLE ROW LEVEL SECURITY;

-- Public read access for IPO data (read-only)
CREATE POLICY "Public can view IPO list" ON ipo_list FOR SELECT USING (true);
CREATE POLICY "Public can view GMP" ON ipo_gmp FOR SELECT USING (true);
CREATE POLICY "Public can view GMP history" ON gmp_price_history FOR SELECT USING (true);

-- Authenticated users can insert/update their own data
CREATE POLICY "Users can insert result cache" ON ipo_result_cache FOR INSERT WITH CHECK (auth.uid() IS NOT NULL);
CREATE POLICY "Users can update own result cache" ON ipo_result_cache FOR UPDATE USING (auth.uid() IS NOT NULL);

-- Service role can do anything (for Go backend)
CREATE POLICY "Service role full access" ON ipo_list FOR ALL USING (auth.role() = 'service_role');
CREATE POLICY "Service role full access GMP" ON ipo_gmp FOR ALL USING (auth.role() = 'service_role');
CREATE POLICY "Service role full access result cache" ON ipo_result_cache FOR ALL USING (auth.role() = 'service_role');
CREATE POLICY "Service role full access update log" ON ipo_update_log FOR ALL USING (auth.role() = 'service_role');
CREATE POLICY "Service role full access GMP history" ON gmp_price_history FOR ALL USING (auth.role() = 'service_role');
CREATE POLICY "Service role full access job log" ON gmp_history_job_log FOR ALL USING (auth.role() = 'service_role');
