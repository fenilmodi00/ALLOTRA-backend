# Architectural Decisions

## Key Decisions from Metis Review
- **Reuse existing `job_dispatch` table** - not create new job table
- **Match threshold: 80.0 score** to mark resolved
- **Registrar short codes**: "KFIN", "BIGSHARE", "MUFG"
- **IST timezone handling**: Use `time.LoadLocation("Asia/Kolkata")`
- **New table instead of populating existing column**: Cleaner separation of concerns
- **30-minute job intervals**: Starting at 1 PM IST on result_date

## Integration Points
- Use `JobPoller` infrastructure for job execution
- Upgrade existing `v2_allotment_handler.go` (not create new)
- Registrar clients in `tools/registrars/` handle UI + API patterns

---
