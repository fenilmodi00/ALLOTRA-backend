#!/bin/bash

# Script to apply GMP price history migration
# This script creates the new tables for GMP price history tracking

set -e

echo "=== GMP Price History Migration ==="
echo "This will create the following tables:"
echo "  - gmp_price_history"
echo "  - gmp_history_job_log"
echo ""

# Check if docker container is running
if ! docker ps | grep -q ipo_db; then
    echo "Error: Database container 'ipo_db' is not running"
    echo "Please start it with: docker-compose up -d db"
    exit 1
fi

echo "Applying migration..."
docker exec -i ipo_db psql -U user -d ipo_db < database/migrations/001_add_gmp_price_history.sql

if [ $? -eq 0 ]; then
    echo ""
    echo "✓ Migration applied successfully!"
    echo ""
    echo "Verifying tables..."
    docker exec -i ipo_db psql -U user -d ipo_db -c "\dt gmp_price_history"
    docker exec -i ipo_db psql -U user -d ipo_db -c "\dt gmp_history_job_log"
    echo ""
    echo "Migration complete!"
else
    echo ""
    echo "✗ Migration failed!"
    exit 1
fi
