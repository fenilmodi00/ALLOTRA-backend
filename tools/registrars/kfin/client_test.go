package kfin

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func httpResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestGetActiveIPOsWithClient_UsesAPIEndpoint(t *testing.T) {
	t.Parallel()

	kc := NewClient()
	apiCalls := 0
	webCalls := 0

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case kfinAPIBaseURL + "/ipolist":
			apiCalls++
			if req.Header.Get("Origin") != kfinWebBaseURL {
				t.Fatalf("missing Origin header: got %q", req.Header.Get("Origin"))
			}
			if req.Header.Get("Referer") != kfinWebBaseURL+"/" {
				t.Fatalf("missing Referer header: got %q", req.Header.Get("Referer"))
			}
			return httpResponse(http.StatusOK, `[{"clientId":"51058840660","name":"ACCORD TRANSFORMER AND SWITCHGEAR LIMITED"}]`), nil
		case kfinWebBaseURL:
			webCalls++
			return httpResponse(http.StatusInternalServerError, "unexpected fallback"), nil
		default:
			return httpResponse(http.StatusNotFound, "not found"), nil
		}
	})}

	options, err := kc.getActiveIPOsWithClient(context.Background(), client)
	if err != nil {
		t.Fatalf("getActiveIPOsWithClient returned error: %v", err)
	}
	if len(options) != 1 {
		t.Fatalf("expected 1 option, got %d", len(options))
	}
	if options[0].ID != "51058840660" || options[0].Name != "ACCORD TRANSFORMER AND SWITCHGEAR LIMITED" {
		t.Fatalf("unexpected option: %+v", options[0])
	}
	if apiCalls != 1 {
		t.Fatalf("expected 1 API call, got %d", apiCalls)
	}
	if webCalls != 0 {
		t.Fatalf("expected 0 web fallback calls, got %d", webCalls)
	}
}

func TestGetActiveIPOsWithClient_FallsBackToJSBundle(t *testing.T) {
	t.Parallel()

	kc := NewClient()

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case kfinAPIBaseURL + "/ipolist", kfinAPIBaseURL + "/ipo", kfinAPIBaseURL + "/list":
			return httpResponse(http.StatusForbidden, `{"message":"Missing Authentication Token"}`), nil
		case kfinWebBaseURL:
			return httpResponse(http.StatusOK, `<html><head><script src="/static/js/main.test.js"></script></head></html>`), nil
		case kfinWebBaseURL + "/static/js/main.test.js":
			return httpResponse(http.StatusOK, `const rf=JSON.parse('[{"clientId":"51058840660","name":"ACCORD TRANSFORMER AND SWITCHGEAR LIMITED"}]');`), nil
		default:
			return httpResponse(http.StatusNotFound, "not found"), nil
		}
	})}

	options, err := kc.getActiveIPOsWithClient(context.Background(), client)
	if err != nil {
		t.Fatalf("getActiveIPOsWithClient returned error: %v", err)
	}
	if len(options) != 1 {
		t.Fatalf("expected 1 option, got %d", len(options))
	}
	if options[0].ID == "" || options[0].Name == "" {
		t.Fatalf("expected populated option, got %+v", options[0])
	}
}
