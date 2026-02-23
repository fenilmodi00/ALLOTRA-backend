# ALLOTRA-backend

## Groww Scraper Behavior

- Groww slug discovery scans these tabs: `/ipo`, `/ipo/open`, `/ipo/upcoming`, `/ipo/closed`, `/ipo/gmp`, `/ipo/allotment`.
- Discovery returns a deduplicated, sorted slug list to keep runs deterministic.
- Groww Details API is the primary record source for IPO content.
- Groww CMS API is supplemental; CMS `404` is treated as non-fatal when Details succeeds.
- Daily job logs explicitly differentiate:
  - Details success + CMS miss: `Groww details fetched; CMS missing (non-fatal)`
  - Details failure: fallback to Chittorgarh path.
