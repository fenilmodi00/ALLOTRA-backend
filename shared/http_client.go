package shared

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// HTTPClientFactory creates optimized HTTP clients with standardized configuration
type HTTPClientFactory struct {
	defaultTimeout time.Duration
	mutex          sync.RWMutex
	clients        map[string]*http.Client
}

// NewHTTPClientFactory creates a new HTTP client factory
func NewHTTPClientFactory(defaultTimeout time.Duration) *HTTPClientFactory {
	return &HTTPClientFactory{
		defaultTimeout: defaultTimeout,
		clients:        make(map[string]*http.Client),
	}
}

// CreateOptimizedHTTPClient creates an HTTP client with connection pooling and optimized settings
func (f *HTTPClientFactory) CreateOptimizedHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = f.defaultTimeout
	}

	// Create client key for caching
	clientKey := fmt.Sprintf("timeout_%d", timeout.Milliseconds())

	f.mutex.RLock()
	if client, exists := f.clients[clientKey]; exists {
		f.mutex.RUnlock()
		return client
	}
	f.mutex.RUnlock()

	// Create new optimized client
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			// Connection pool configuration for efficient resource utilization
			MaxIdleConns:        100,              // Maximum idle connections across all hosts
			MaxIdleConnsPerHost: 10,               // Maximum idle connections per host
			IdleConnTimeout:     90 * time.Second, // Duration to keep idle connections alive

			// Enable connection reuse for better performance
			DisableKeepAlives: false,

			// Timeout configurations for robust error handling
			TLSHandshakeTimeout:   10 * time.Second, // Maximum time for TLS handshake
			ResponseHeaderTimeout: 10 * time.Second, // Maximum time to wait for response headers
			ExpectContinueTimeout: 1 * time.Second,  // Maximum time to wait for 100-continue response

			// Enable compression to reduce bandwidth usage
			DisableCompression: false,
		},
	}

	// Cache the client
	f.mutex.Lock()
	f.clients[clientKey] = client
	f.mutex.Unlock()

	logrus.WithFields(logrus.Fields{
		"component":  "HTTPClientFactory",
		"timeout":    timeout,
		"client_key": clientKey,
	}).Debug("Created new optimized HTTP client")

	return client
}

// SetBrowserLikeHeaders configures HTTP request headers to mimic browser behavior
func SetBrowserLikeHeaders(request *http.Request, acceptHeader string) {
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	request.Header.Set("Accept", acceptHeader)
	request.Header.Set("Accept-Language", "en-US,en;q=0.9")
	request.Header.Set("Cache-Control", "no-cache")
	request.Header.Set("Connection", "keep-alive")
}

// ExecuteHTTPRequestWithRetry executes HTTP requests with exponential backoff retry logic
func ExecuteHTTPRequestWithRetry(client *http.Client, request *http.Request, maxRetryAttempts int) (*http.Response, error) {
	logger := logrus.WithFields(logrus.Fields{
		"component": "HTTPClientFactory",
		"method":    "ExecuteHTTPRequestWithRetry",
		"url":       request.URL.String(),
	})

	var httpResponse *http.Response
	var lastExecutionError error

	for attemptNumber := 0; attemptNumber <= maxRetryAttempts; attemptNumber++ {
		if attemptNumber > 0 {
			// Calculate exponential backoff duration with jitter to prevent thundering herd
			baseBackoffDuration := time.Duration(1<<uint(attemptNumber-1)) * time.Second
			jitterDuration := time.Duration(float64(baseBackoffDuration) * 0.1 * (0.5 + 0.5*float64(attemptNumber%3)/2))
			totalBackoffDuration := baseBackoffDuration + jitterDuration

			logger.WithFields(logrus.Fields{
				"attempt":          attemptNumber + 1,
				"backoff_duration": totalBackoffDuration,
			}).Debug("Retrying HTTP request after backoff")

			time.Sleep(totalBackoffDuration)
		}

		httpResponse, lastExecutionError = client.Do(request)
		if lastExecutionError == nil && httpResponse.StatusCode == http.StatusOK {
			logger.WithFields(logrus.Fields{
				"attempt":     attemptNumber + 1,
				"status_code": httpResponse.StatusCode,
			}).Debug("HTTP request successful")
			return httpResponse, nil // Successful execution
		}

		// Store detailed error information for potential return
		if lastExecutionError != nil {
			lastExecutionError = fmt.Errorf("attempt %d failed with network error: %w", attemptNumber+1, lastExecutionError)
			logger.WithError(lastExecutionError).Debug("HTTP request failed with network error")
		} else {
			lastExecutionError = fmt.Errorf("attempt %d failed with HTTP %d: %s", attemptNumber+1, httpResponse.StatusCode, http.StatusText(httpResponse.StatusCode))
			logger.WithFields(logrus.Fields{
				"attempt":     attemptNumber + 1,
				"status_code": httpResponse.StatusCode,
			}).Debug("HTTP request failed with non-200 status")
			httpResponse.Body.Close() // Clean up response body before retrying
		}
	}

	// All retry attempts exhausted
	totalAttempts := maxRetryAttempts + 1
	logger.WithFields(logrus.Fields{
		"total_attempts": totalAttempts,
		"final_error":    lastExecutionError,
	}).Error("HTTP request failed after all retry attempts")

	return nil, fmt.Errorf("HTTP request failed after %d attempts: %w", totalAttempts, lastExecutionError)
}

// CleanupHTTPClient properly closes and cleans up HTTP client resources
func (f *HTTPClientFactory) CleanupHTTPClient(client *http.Client) {
	if client != nil && client.Transport != nil {
		if transport, ok := client.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}
}

// CleanupAllClients cleans up all cached HTTP clients
func (f *HTTPClientFactory) CleanupAllClients() {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	for key, client := range f.clients {
		f.CleanupHTTPClient(client)
		delete(f.clients, key)
	}

	logrus.WithField("component", "HTTPClientFactory").Debug("Cleaned up all cached HTTP clients")
}
