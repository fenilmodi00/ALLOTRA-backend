# GMP History API - Frontend Integration Guide

## Quick Start

### Fetching Chart Data

```javascript
// Using IPO UUID or Stock ID
const ipoId = "d9d0343d-d727-49cf-aa9d-1189c0ecbb3a"; // or "2462"
const response = await fetch(`/api/v1/gmp/history/${ipoId}/chart`);
const result = await response.json();

if (result.success) {
  const { ipo_info, chart_data, statistics, metadata } = result.data;
  console.log(`IPO: ${ipo_info.ipo_name}`);
  console.log(`Latest GMP: ₹${statistics.latest_gmp}`);
  console.log(`Trend: ${statistics.trend_direction}`);
}
```

## API Response Structure

### Chart Endpoint: `GET /api/v1/gmp/history/:identifier/chart`

| Field | Type | Description |
|-------|------|-------------|
| `ipo_info.ipo_id` | string | IPO UUID |
| `ipo_info.ipo_name` | string | Company name |
| `ipo_info.company_code` | string | URL-friendly code |
| `chart_data` | array | Array of daily GMP points |
| `chart_data[].date` | string | Date (YYYY-MM-DD) |
| `chart_data[].gmp_value` | number | GMP in rupees |
| `chart_data[].estimated_listing` | number | Expected listing price |
| `chart_data[].listing_percent` | number | Gain/loss percentage |
| `statistics.max_gmp` | number | Highest GMP |
| `statistics.min_gmp` | number | Lowest GMP |
| `statistics.average_gmp` | number | Average GMP |
| `statistics.latest_gmp` | number | Most recent GMP |
| `statistics.trend_direction` | string | "up", "down", or "stable" |
| `metadata.total_records` | number | Days of history |
| `metadata.data_source` | string | "investorgain.com" |
| `metadata.last_updated` | string | RFC3339 timestamp |

### Other Endpoints

| Endpoint | Purpose |
|----------|---------|
| `GET /api/v1/gmp/history/:id` | Full history with all fields |
| `GET /api/v1/gmp/history/:id/summary` | Stats + recent 5 days |
| `GET /api/v1/gmp/history/health` | Service health status |

## Data Flow

```
User opens IPO detail page
         │
         ▼
Frontend calls /chart endpoint
         │
         ▼
Backend fetches from database (gmp_price_history table)
         │
         ▼
Response: chart_data array + statistics + metadata
         │
         ▼
Frontend renders line chart with GMP values
```

## Example: React + Chart.js

```jsx
import { Line } from 'react-chartjs-2';

function GMPChart({ ipoId }) {
  const [data, setData] = useState(null);

  useEffect(() => {
    fetch(`/api/v1/gmp/history/${ipoId}/chart`)
      .then(r => r.json())
      .then(r => {
        if (r.success) setData(r.data);
      });
  }, [ipoId]);

  if (!data) return <p>Loading...</p>;

  const chartConfig = {
    labels: data.chart_data.map(d => d.date),
    datasets: [{
      label: 'GMP (₹)',
      data: data.chart_data.map(d => d.gmp_value),
      borderColor: '#10b981',
      backgroundColor: 'rgba(16,185,129,0.1)',
    }]
  };

  return (
    <div>
      <h3>{data.ipo_info.ipo_name}</h3>
      <p>Trend: {data.statistics.trend_direction}</p>
      <Line data={chartConfig} />
    </div>
  );
}
```

## Error Handling

| Status | Meaning | Frontend Action |
|--------|---------|-----------------|
| 200 | Success | Render chart |
| 400 | Invalid ID | Show validation error |
| 404 | No history | Show "No data available" |
| 503 | Service down | Show retry message |

## Notes

- Both UUID and numeric stock ID are accepted as `:identifier`
- Data is refreshed daily via background job
- Response is cached for 10 minutes
- All prices are in INR (₹)
