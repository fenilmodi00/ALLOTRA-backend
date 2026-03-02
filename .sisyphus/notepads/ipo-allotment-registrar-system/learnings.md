## 2026-03-02 - Local bootstrap learnings

- Fresh local DB bootstrap should not rely on Supabase DNS/host availability.
- `job_dispatch` must include `completed_at` because `jobs/job_poller.go` writes it when marking completed/failed.
- Long-running `gmp_history_update` can block strict bootstrap waits; non-strict queueing is better for quick local bring-up.
- Stale `running` rows in `job_dispatch` can stall expected flow visibility; reset stale runs before new bootstrap sequence.
- For registrar-code pipeline, the critical path is:
  1) IPO data present in `ipo_list`
  2) `fetch_registrar_company_code` job inserted
  3) poller executor resolves and writes `ipo_registrar_codes`
