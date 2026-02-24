# Groww Scraper Coverage and Failure Semantics

This backend uses Groww as the primary IPO data source and applies explicit partial-failure semantics.

## Discovery coverage

Slug discovery scans all relevant Groww IPO tabs:

- `https://groww.in/ipo`
- `https://groww.in/ipo/open`
- `https://groww.in/ipo/upcoming`
- `https://groww.in/ipo/closed`
- `https://groww.in/ipo/gmp`
- `https://groww.in/ipo/allotment`

Only valid detail slugs are kept (section pages are skipped), duplicates are removed, and the result is sorted.

## Details vs CMS semantics

- **Details API** is authoritative for IPO ingestion.
- **CMS API** enriches content (about/objectives style data).
- If Details succeeds and CMS fails (including `404`), ingestion continues with a non-fatal CMS error.
- If Details fails, the pipeline falls back to Chittorgarh scraping for continuity.

## Operational logs

Daily job logs intentionally distinguish the two cases:

- `Groww details fetched; CMS missing (non-fatal)`
- `Groww failed or 404ed, falling back to Chittorgarh`

This avoids false positives where partial data looked like full rich-data success.
