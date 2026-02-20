# GMP Price History API Documentation

## Overview

The GMP Price History API provides endpoints to retrieve historical Grey Market Premium data for IPOs. This data is essential for building price trend charts and analyzing market sentiment over time.

**Base URL**: `/api/gmp/history`

**Authentication**: Uses existing authentication mechanisms (if configured)

**Rate Limiting**: Standard API rate limits apply

## Endpoints

### 1. Get IPO Price History

Retrieves complete price history for a specific IPO (typically 10-15 days of data).

**Endpoint**: `GET /api/gmp/history/{identifier}`

**Parameters**:
- `identifier` (path, required): Either IPO UUID or Stock ID
  - Example UUID: `d9d0343d-d727-49cf-aa9d-1189c0ecbb3a`
  - Example Stock ID: `2462`

**Example Requests**:
```bash
# Using IPO UUID
GET /api/v1/gmp/history/d9d0343d-d727-49cf-aa9d-1189c0ecbb3a

# Using Stock ID
GET /api/v1/gmp/history/2462
```

**Success Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "ipo_id": "d9d0343d-d727-49cf-aa9d-1189c0ecbb3a",
    "ipo_name": "Bharat Coking Coal Ltd.",
    "company_code": "bharat-coking-coal",
    "total_records": 10,
    "entries": [
      {
        "id": "entry-uuid-1",
        "ipo_id": "d9d0343d-d727-49cf-aa9d-1189c0ecbb3a",
        "company_code": "bharat-coking-coal",
        "record_date": "2026-01-18T00:00:00Z",
        "ipo_price": 175.00,
        "gmp_value": 25.00,
        "estimated_listing": 200.00,
        "listing_percent": 14.29,
        "estimated_profit": 50000.00,
        "subscription_status": "2.5x subscribed",
        "sub2_sauda": 10000.00,
        "last_updated": "18-Jan-2026 13:02",
        "data_source": "investorgain.com",
        "created_at": "2026-01-18T13:05:00Z",
        "updated_at": "2026-01-18T13:05:00Z"
      }
    ],
    "metadata": {
      "data_source": "investorgain.com",
      "last_updated": "2026-01-18T14:00:00Z",
      "last_scraped": "2026-01-18T13:05:00Z",
      "scraping_success": true,
      "processing_time": "5.2s"
    }
  }
}
```

**Error Responses**:

404 Not Found - IPO not found:
```json
{
  "success": false,
  "error": "No price history found for this IPO",
  "message": "No historical GMP data exists for IPO ID: d9d0343d-d727-49cf-aa9d-1189c0ecbb3a"
}
```

400 Bad Request - Invalid identifier:
```json
{
  "success": false,
  "error": "Invalid identifier",
  "message": "The provided identifier must be either a valid IPO UUID or stock ID"
}
```

---

### 2. Get Chart Data

Retrieves price history optimized for frontend chart libraries (typically 10-15 days of data).

**Endpoint**: `GET /api/gmp/history/{identifier}/chart`

**Parameters**:
- `identifier` (path, required): Either IPO UUID or Stock ID
  - Example UUID: `d9d0343d-d727-49cf-aa9d-1189c0ecbb3a`
  - Example Stock ID: `2462`

**Example Requests**:
```bash
# Using IPO UUID
GET /api/v1/gmp/history/d9d0343d-d727-49cf-aa9d-1189c0ecbb3a/chart

# Using Stock ID
GET /api/v1/gmp/history/2462/chart
```

**Success Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "ipo_info": {
      "ipo_id": "d9d0343d-d727-49cf-aa9d-1189c0ecbb3a",
      "ipo_name": "Bharat Coking Coal Ltd.",
      "company_code": "bharat-coking-coal",
      "ipo_price": 175.00,
      "status": "Open"
    },
    "chart_data": [
      {
        "date": "2026-01-09",
        "gmp_value": 15.00,
        "estimated_listing": 190.00,
        "listing_percent": 8.57
      },
      {
        "date": "2026-01-10",
        "gmp_value": 18.00,
        "estimated_listing": 193.00,
        "listing_percent": 10.29
      },
      {
        "date": "2026-01-11",
        "gmp_value": 20.00,
        "estimated_listing": 195.00,
        "listing_percent": 11.43
      },
      {
        "date": "2026-01-12",
        "gmp_value": 22.00,
        "estimated_listing": 197.00,
        "listing_percent": 12.57
      },
      {
        "date": "2026-01-13",
        "gmp_value": 25.00,
        "estimated_listing": 200.00,
        "listing_percent": 14.29
      },
      {
        "date": "2026-01-14",
        "gmp_value": 25.00,
        "estimated_listing": 200.00,
        "listing_percent": 14.29
      },
      {
        "date": "2026-01-15",
        "gmp_value": 25.00,
        "estimated_listing": 200.00,
        "listing_percent": 14.29
      },
      {
        "date": "2026-01-16",
        "gmp_value": 25.00,
        "estimated_listing": 200.00,
        "listing_percent": 14.29
      },
      {
        "date": "2026-01-17",
        "gmp_value": 25.00,
        "estimated_listing": 200.00,
        "listing_percent": 14.29
      },
      {
        "date": "2026-01-18",
        "gmp_value": 25.00,
        "estimated_listing": 200.00,
        "listing_percent": 14.29
      }
    ],
    "statistics": {
      "max_gmp": 25.00,
      "min_gmp": 15.00,
      "average_gmp": 22.50,
      "latest_gmp": 25.00,
      "trend_direction": "up",
      "volatility": "low"
    },
    "metadata": {
      "total_records": 10,
      "data_source": "investorgain.com",
      "last_updated": "2026-01-18T13:05:00Z"
    }
  }
}
```

---

### 3. Get History Summary

Retrieves summary statistics for an IPO's price history.

**Endpoint**: `GET /api/gmp/history/{identifier}/summary`

**Parameters**:
- `identifier` (path, required): Either IPO UUID or Stock ID
  - Example UUID: `d9d0343d-d727-49cf-aa9d-1189c0ecbb3a`
  - Example Stock ID: `2462`

**Example Requests**:
```bash
# Using IPO UUID
GET /api/v1/gmp/history/d9d0343d-d727-49cf-aa9d-1189c0ecbb3a/summary

# Using Stock ID
GET /api/v1/gmp/history/2462/summary
```

**Success Response** (200 OK):
```json
{
  "success": true,
  "data": {
    "ipo_id": "d9d0343d-d727-49cf-aa9d-1189c0ecbb3a",
    "ipo_name": "Bharat Coking Coal Ltd.",
    "company_code": "bharat-coking-coal",
    "total_records": 10,
    "statistics": {
      "max_gmp": 25.00,
      "min_gmp": 15.00,
      "average_gmp": 22.50,
      "latest_gmp": 25.00,
      "trend_direction": "up"
    },
    "recent_entries": [
      {
        "date": "2026-01-18",
        "gmp_value": 25.00,
        "estimated_listing": 200.00,
        "listing_percent": 14.29
      },
      {
        "date": "2026-01-17",
        "gmp_value": 25.00,
        "estimated_listing": 200.00,
        "listing_percent": 14.29
      },
      {
        "date": "2026-01-16",
        "gmp_value": 25.00,
        "estimated_listing": 200.00,
        "listing_percent": 14.29
      },
      {
        "date": "2026-01-15",
        "gmp_value": 25.00,
        "estimated_listing": 200.00,
        "listing_percent": 14.29
      },
      {
        "date": "2026-01-14",
        "gmp_value": 25.00,
        "estimated_listing": 200.00,
        "listing_percent": 14.29
      }
    ]
  },
  "metadata": {
    "data_source": "investorgain.com",
    "last_updated": "2026-01-18T14:00:00Z",
    "total_records": 10,
    "last_scraped": "2026-01-18T13:05:00Z",
    "scraping_success": true,
    "processing_time": "5.2s"
  }
}
```

---

## Frontend Integration Examples

### React with Chart.js

```javascript
import React, { useEffect, useState } from 'react';
import { Line } from 'react-chartjs-2';

function GMPPriceChart({ ipoId }) {
  const [chartData, setChartData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    async function fetchData() {
      try {
        const response = await fetch(`/api/gmp/history/${ipoId}/chart`);
        const result = await response.json();
        
        if (!result.success) {
          throw new Error(result.error.message);
        }

        const data = result.data;
        
        setChartData({
          labels: data.chart_data.map(point => point.date),
          datasets: [
            {
              label: 'GMP Value (₹)',
              data: data.chart_data.map(point => point.gmp_value),
              borderColor: 'rgb(75, 192, 192)',
              backgroundColor: 'rgba(75, 192, 192, 0.2)',
              tension: 0.1
            },
            {
              label: 'Estimated Listing (₹)',
              data: data.chart_data.map(point => point.estimated_listing),
              borderColor: 'rgb(255, 99, 132)',
              backgroundColor: 'rgba(255, 99, 132, 0.2)',
              tension: 0.1
            }
          ]
        });
        
        setLoading(false);
      } catch (err) {
        setError(err.message);
        setLoading(false);
      }
    }

    fetchData();
  }, [ipoId]);

  if (loading) return <div>Loading chart...</div>;
  if (error) return <div>Error: {error}</div>;

  return (
    <div>
      <h3>{chartData.ipo_info.ipo_name}</h3>
      <Line data={chartData} options={{
        responsive: true,
        plugins: {
          legend: {
            position: 'top',
          },
          title: {
            display: true,
            text: 'GMP Price History'
          }
        },
        scales: {
          y: {
            beginAtZero: true,
            title: {
              display: true,
              text: 'Price (₹)'
            }
          },
          x: {
            title: {
              display: true,
              text: 'Date'
            }
          }
        }
      }} />
      
      <div className="statistics">
        <p>Latest GMP: ₹{chartData.statistics.latest_gmp}</p>
        <p>Average GMP: ₹{chartData.statistics.average_gmp.toFixed(2)}</p>
        <p>Trend: {chartData.statistics.trend_direction}</p>
      </div>
    </div>
  );
}

export default GMPPriceChart;
```

### Vue.js with ApexCharts

```vue
<template>
  <div class="gmp-chart">
    <h3>{{ ipoName }}</h3>
    <apexchart
      v-if="chartOptions"
      type="line"
      :options="chartOptions"
      :series="series"
      height="350"
    />
    <div v-if="statistics" class="stats">
      <div class="stat-item">
        <span>Latest GMP:</span>
        <strong>₹{{ statistics.latest_gmp }}</strong>
      </div>
      <div class="stat-item">
        <span>Trend:</span>
        <strong :class="trendClass">{{ statistics.trend_direction }}</strong>
      </div>
    </div>
  </div>
</template>

<script>
import VueApexCharts from 'vue3-apexcharts';

export default {
  name: 'GMPPriceChart',
  components: {
    apexchart: VueApexCharts
  },
  props: {
    ipoId: {
      type: String,
      required: true
    }
  },
  data() {
    return {
      ipoName: '',
      series: [],
      chartOptions: null,
      statistics: null
    };
  },
  computed: {
    trendClass() {
      return {
        'trend-up': this.statistics?.trend_direction === 'up',
        'trend-down': this.statistics?.trend_direction === 'down',
        'trend-stable': this.statistics?.trend_direction === 'stable'
      };
    }
  },
  async mounted() {
    await this.fetchChartData();
  },
  methods: {
    async fetchChartData() {
      try {
        const response = await fetch(`/api/gmp/history/${this.ipoId}/chart`);
        const result = await response.json();
        
        if (!result.success) {
          throw new Error(result.error.message);
        }

        const data = result.data;
        this.ipoName = data.ipo_info.ipo_name;
        this.statistics = data.statistics;

        this.series = [
          {
            name: 'GMP Value',
            data: data.chart_data.map(point => ({
              x: point.date,
              y: point.gmp_value
            }))
          },
          {
            name: 'Estimated Listing',
            data: data.chart_data.map(point => ({
              x: point.date,
              y: point.estimated_listing
            }))
          }
        ];

        this.chartOptions = {
          chart: {
            type: 'line',
            zoom: {
              enabled: true
            }
          },
          dataLabels: {
            enabled: false
          },
          stroke: {
            curve: 'smooth',
            width: 2
          },
          title: {
            text: 'GMP Price History',
            align: 'left'
          },
          xaxis: {
            type: 'datetime',
            title: {
              text: 'Date'
            }
          },
          yaxis: {
            title: {
              text: 'Price (₹)'
            }
          },
          tooltip: {
            x: {
              format: 'dd MMM yyyy'
            }
          }
        };
      } catch (error) {
        console.error('Error fetching chart data:', error);
      }
    }
  }
};
</script>

<style scoped>
.gmp-chart {
  padding: 20px;
}

.stats {
  display: flex;
  gap: 20px;
  margin-top: 20px;
}

.stat-item {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.trend-up {
  color: green;
}

.trend-down {
  color: red;
}

.trend-stable {
  color: gray;
}
</style>
```

---

## Error Codes

| Code | Description |
|------|-------------|
| `IPO_NOT_FOUND` | No price history found for the specified IPO |
| `INVALID_PARAMETERS` | Invalid query parameters provided |
| `INVALID_DATE_FORMAT` | Date format is incorrect (use ISO 8601) |
| `INVALID_DATE_RANGE` | Start date is after end date |
| `LIMIT_EXCEEDED` | Requested limit exceeds maximum allowed |
| `INTERNAL_ERROR` | Internal server error occurred |
| `SERVICE_UNAVAILABLE` | External service temporarily unavailable |

---

## Rate Limiting

- **Default Limit**: 100 requests per minute per IP
- **Burst Limit**: 20 requests per second
- **Headers**: Rate limit information included in response headers
  - `X-RateLimit-Limit`: Maximum requests allowed
  - `X-RateLimit-Remaining`: Remaining requests in current window
  - `X-RateLimit-Reset`: Time when the rate limit resets (Unix timestamp)

---

## Caching

- **Cache Duration**: 10 minutes for chart data
- **Cache Key**: Based on IPO ID and query parameters
- **Cache Invalidation**: Automatic when new data is scraped
- **Cache Header**: `X-Cache: HIT` or `X-Cache: MISS` in response

---

## Performance

- **Response Time**: < 500ms for datasets up to 1000 records
- **Pagination**: Recommended for large datasets
- **Optimization**: Use date range filters to reduce response size
- **Best Practice**: Cache responses on frontend for frequently accessed data

---

## Support

For issues or questions:
- Check the implementation summary: `GMP_HISTORY_IMPLEMENTATION_SUMMARY.md`
- Review the spec documents in `.kiro/specs/gmp-price-history/`
- Contact the development team

---

## Changelog

### Version 1.0.0 (2026-01-18)
- Initial release
- Basic price history endpoints
- Chart-optimized data format
- Summary statistics endpoint
