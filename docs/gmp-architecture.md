# GMP Architecture

This repository has two GMP domains:

- **Current GMP snapshot**: latest GMP value per IPO.
- **GMP history**: time-series GMP data and analytics endpoints.

## Active Runtime Paths

- `jobs/gmp_update_job.go`
  - Scheduled job for latest GMP snapshot updates.
  - Uses `services/SimpleGMPService` (`services/simple_gmp_service.go`).

- `jobs/gmp_history_update_job.go`
  - Scheduled job for GMP history updates.
  - Uses `services/GMPHistoryService` (`services/gmp_history_service.go`).

- `handlers/gmp_handler.go`
  - Serves latest GMP endpoint and links to history endpoints.

- `handlers/gmp_history_handler.go`
  - Serves history, chart, summary, health, and metrics endpoints.

## Compatibility Layer

- `services/gmp_service.go` remains for backward compatibility in tests and tooling.
- IPO scraping methods in this file delegate to `services/simplified_ipo_scraper.go`
  (`ChittorgarhIPOScrapingService`) to avoid duplicate logic.

## Cleanup Notes

- Removed dead file: `jobs/simple_gmp_update_job.go`.
- Consolidated duplicated IPO scraping flow by delegating from `gmp_service.go` to
  `simplified_ipo_scraper.go`.
- Added zero-division guard in GMP extraction success-rate logging.
- Removed dead GMP history archival code from `services/gmp_history_service.go`
  (`ArchiveOldHistory`, `ArchiveByVolume`, `GetArchivalHistory`,
  `CheckArchivalNeeded`, and `ArchivalReport`).
- Removed archival-only assets: `services/gmp_history_archival_test.go` and
  `services/ARCHIVAL_IMPLEMENTATION.md`.
