# IPO Backend System

## Product Overview

The IPO Backend is a comprehensive API service that provides real-time IPO (Initial Public Offering) data and Grey Market Premium (GMP) information for the Indian stock market. The system serves as a data aggregation and processing platform for IPO-related information.

## Core Features

- **IPO Data Management**: Complete IPO lifecycle tracking from announcement to listing
- **Grey Market Premium (GMP) Integration**: Real-time GMP data from multiple sources
- **Allotment Status Checking**: Automated allotment result verification
- **Market Data**: Current market indices and performance metrics
- **Caching Layer**: Intelligent caching for performance optimization
- **Background Jobs**: Automated data scraping and updates

## Data Sources

- **Chittorgarh.com**: Primary source for IPO details and static information
- **InvestorGain.com**: Grey Market Premium data and dynamic pricing
- **Market APIs**: Real-time market indices (planned integration)

## Key Business Logic

- **Dynamic Status Calculation**: IPO status computed in real-time based on dates
  - UPCOMING: Before open_date
  - LIVE: Between open_date and close_date
  - CLOSED: After close_date (before listing_date)
  - LISTED: After listing_date

- **Data Normalization**: Company codes used for cross-referencing between IPO and GMP data
- **Cache Management**: Results cached with configurable TTL and automatic cleanup
- **Performance Monitoring**: Real-time metrics and load testing capabilities

## Target Users

- Frontend applications requiring IPO data
- Investment platforms and financial services
- Individual investors checking allotment status
- Market analysis and research tools