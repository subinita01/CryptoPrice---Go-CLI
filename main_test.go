package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestBuildURL checks that the returned URL contains the correct query parameters.
func TestBuildURL(t *testing.T) {
	tests := []struct {
		name     string
		coins    []string
		currency string
		wantIDs  string
		wantCurr string
	}{
		{"single coin", []string{"bitcoin"}, "usd", "bitcoin", "usd"},
		{"multiple coins", []string{"bitcoin", "ethereum", "solana"}, "eur", "bitcoin,ethereum,solana", "eur"},
		{"non-default currency", []string{"dogecoin"}, "btc", "dogecoin", "btc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := buildURL(tt.coins, tt.currency)
			u, err := url.Parse(raw)
			if err != nil {
				t.Fatalf("buildURL returned unparseable URL: %v", err)
			}
			q := u.Query()
			if ids := q.Get("ids"); ids != tt.wantIDs {
				t.Errorf("ids = %q, want %q", ids, tt.wantIDs)
			}
			if curr := q.Get("vs_currencies"); curr != tt.wantCurr {
				t.Errorf("vs_currencies = %q, want %q", curr, tt.wantCurr)
			}
			if !strings.HasPrefix(raw, baseURL) {
				t.Errorf("URL does not start with baseURL: %q", raw)
			}
		})
	}
}

// TestFetchPrices exercises the HTTP layer using a local httptest server so no
// real network calls are made.
func TestFetchPrices(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"bitcoin":{"usd":50000.50},"ethereum":{"usd":3000.25}}`)
		}))
		defer srv.Close()

		result, err := fetchPrices(&http.Client{Timeout: 5 * time.Second}, srv.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p := result["bitcoin"]["usd"]; p != 50000.50 {
			t.Errorf("bitcoin/usd = %v, want 50000.50", p)
		}
		if p := result["ethereum"]["usd"]; p != 3000.25 {
			t.Errorf("ethereum/usd = %v, want 3000.25", p)
		}
	})

	t.Run("non-200 status returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer srv.Close()

		_, err := fetchPrices(&http.Client{Timeout: 5 * time.Second}, srv.URL)
		if err == nil {
			t.Error("expected error for 429 response, got nil")
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `not valid json {{{`)
		}))
		defer srv.Close()

		_, err := fetchPrices(&http.Client{Timeout: 5 * time.Second}, srv.URL)
		if err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}
	})

	t.Run("network error returns error", func(t *testing.T) {
		// Close the server immediately to force a connection-refused error.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		addr := srv.URL
		srv.Close()

		_, err := fetchPrices(&http.Client{Timeout: time.Second}, addr)
		if err == nil {
			t.Error("expected error for closed server, got nil")
		}
	})

	t.Run("empty response body returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Write 200 but no body — json.Unmarshal will fail.
		}))
		defer srv.Close()

		_, err := fetchPrices(&http.Client{Timeout: 5 * time.Second}, srv.URL)
		if err == nil {
			t.Error("expected error for empty body, got nil")
		}
	})
}

// TestPrintTable verifies formatting, n/a handling, and output ordering.
func TestPrintTable(t *testing.T) {
	t.Run("formats prices with two decimal places", func(t *testing.T) {
		result := PriceResult{
			"bitcoin":  {"usd": 50000.5},
			"ethereum": {"usd": 3000.25},
		}
		var buf bytes.Buffer
		printTable(&buf, result, []string{"bitcoin", "ethereum"}, "usd")
		out := buf.String()

		for _, want := range []string{"bitcoin", "ethereum", "50000.50", "3000.25", "USD"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("unknown coin shows n/a", func(t *testing.T) {
		var buf bytes.Buffer
		printTable(&buf, PriceResult{}, []string{"notacoin"}, "usd")
		out := buf.String()
		if !strings.Contains(out, "n/a") {
			t.Errorf("expected n/a for unknown coin, got:\n%s", out)
		}
	})

	t.Run("known coin with unknown currency shows n/a", func(t *testing.T) {
		result := PriceResult{"bitcoin": {"usd": 50000}}
		var buf bytes.Buffer
		printTable(&buf, result, []string{"bitcoin"}, "gbp")
		out := buf.String()
		if !strings.Contains(out, "n/a") {
			t.Errorf("expected n/a for unknown currency, got:\n%s", out)
		}
	})

	t.Run("preserves input order", func(t *testing.T) {
		result := PriceResult{
			"bitcoin":  {"usd": 1},
			"ethereum": {"usd": 2},
		}
		var buf bytes.Buffer
		printTable(&buf, result, []string{"ethereum", "bitcoin"}, "usd")
		out := buf.String()
		if strings.Index(out, "ethereum") > strings.Index(out, "bitcoin") {
			t.Errorf("ethereum should appear before bitcoin:\n%s", out)
		}
	})

	t.Run("currency header is uppercased", func(t *testing.T) {
		result := PriceResult{"bitcoin": {"eur": 45000}}
		var buf bytes.Buffer
		printTable(&buf, result, []string{"bitcoin"}, "eur")
		out := buf.String()
		if !strings.Contains(out, "EUR") {
			t.Errorf("expected uppercase EUR in header, got:\n%s", out)
		}
	})

	t.Run("mixed known and unknown coins", func(t *testing.T) {
		result := PriceResult{"bitcoin": {"usd": 50000}}
		var buf bytes.Buffer
		printTable(&buf, result, []string{"bitcoin", "fakecoin"}, "usd")
		out := buf.String()
		if !strings.Contains(out, "50000.00") {
			t.Errorf("expected bitcoin price in output:\n%s", out)
		}
		if !strings.Contains(out, "n/a") {
			t.Errorf("expected n/a for fakecoin:\n%s", out)
		}
	})
}
