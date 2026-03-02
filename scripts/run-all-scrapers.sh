#!/bin/bash
set -e

echo "========================================="
echo "Starting IPO Data Scraper Orchestration"
echo "========================================="

# Note: Automatic job scheduling is disabled in local development mode.
# Set USE_SUPABASE_CRON=false in .env for local dev (no job_dispatch table required).
# For manual scraping, use the application's HTTP endpoints below.

echo ""
echo "Step 1/2: Daily IPO Update (Chittorgarh + Groww)"
echo "Use: curl -X POST -H 'Authorization: Bearer \$ADMIN_TOKEN' http://localhost:8080/api/v1/admin/ipo/daily-update"

echo ""
echo "Step 2/2: GMP Data Update"
echo "Use: curl -X POST -H 'Authorization: Bearer \$ADMIN_TOKEN' http://localhost:8080/api/v1/admin/gmp/update"

echo ""
echo "Optional: GMP History Update"
echo "Use: curl -X POST -H 'Authorization: Bearer \$ADMIN_TOKEN' http://localhost:8080/api/v1/admin/gmp-history/update"

echo ""
echo "========================================="
echo "Scraper orchestration guide complete!"
echo "========================================="

echo ""
echo "Available Admin Endpoints:"
echo "  - POST /api/v1/admin/ipo/daily-update (v2: /api/v2/admin/ipo/daily-update)"
echo "  - POST /api/v1/admin/gmp/update (v2: /api/v2/admin/gmp/update)"
echo "  - POST /api/v1/admin/gmp-history/update (v2: /api/v2/admin/gmp-history/update)"
echo "  - POST /api/v2/admin/registrar/resolve"
echo ""
echo "Environment: USE_SUPABASE_CRON=false (local dev mode)"
echo "For Supabase deployment, set USE_SUPABASE_CRON=true and ensure job_dispatch table exists."

echo ""
echo "========================================"
echo " Step 3: Registrar Code Resolution"
echo "========================================"
echo ""

# Only resolve if admin endpoint is available
ADMIN_TOKEN=${ADMIN_TOKEN:-""}
if [ -n "$ADMIN_TOKEN" ]; then
  echo "Triggering registrar code resolution..."
  RESPONSE=$(curl -s -w "\n%{http_code}" -X POST \
    -H "X-Admin-Token: $ADMIN_TOKEN" \
    "${API_BASE_URL:-http://localhost:8080}/api/v2/admin/registrar/resolve")
  HTTP_CODE=$(echo "$RESPONSE" | tail -1)
  BODY=$(echo "$RESPONSE" | head -1)
  if [ "$HTTP_CODE" = "200" ]; then
    echo "✓ Registrar codes resolved: $BODY"
  else
    echo "✗ Registrar code resolution failed (HTTP $HTTP_CODE): $BODY"
  fi
else
  echo "⚠ Skipping registrar code resolution (ADMIN_TOKEN not set)"
fi

# Count total IPOs in database
echo ""
echo "Total IPOs in database:"
docker exec ipo-backend-db-1 psql -U postgres -d ipo_db -t -c "SELECT COUNT(*) FROM ipo_list;"

# Show recent IPOs
echo ""
echo "Recent IPOs (last 10):"
docker exec ipo-backend-db-1 psql -U postgres -d ipo_db -c "SELECT stock_id, name, status, open_date, close_date FROM ipo_list ORDER BY created_at DESC LIMIT 10;"
