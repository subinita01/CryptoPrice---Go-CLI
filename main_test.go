package main

import (
	"bytes"
	"context"
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

// TestBuildMarketsURL checks that the markets URL is built with the correct params.
func TestBuildMarketsURL(t *testing.T) {
	raw := buildMarketsURL(10, "eur")
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("buildMarketsURL returned unparseable URL: %v", err)
	}
	q := u.Query()
	if n := q.Get("per_page"); n != "10" {
		t.Errorf("per_page = %q, want %q", n, "10")
	}
	if curr := q.Get("vs_currency"); curr != "eur" {
		t.Errorf("vs_currency = %q, want %q", curr, "eur")
	}
	if ord := q.Get("order"); ord != "market_cap_desc" {
		t.Errorf("order = %q, want %q", ord, "market_cap_desc")
	}
	if !strings.HasPrefix(raw, marketsURL) {
		t.Errorf("URL does not start with marketsURL: %q", raw)
	}
}

// TestFetchTopCoins exercises the markets HTTP layer using a local httptest server.
func TestFetchTopCoins(t *testing.T) {
	const body = `[
		{"id":"bitcoin","symbol":"btc","name":"Bitcoin","current_price":105000.0,
		 "market_cap":2089000000000,"market_cap_rank":1,"price_change_percentage_24h":1.23},
		{"id":"ethereum","symbol":"eth","name":"Ethereum","current_price":2541.0,
		 "market_cap":305000000000,"market_cap_rank":2,"price_change_percentage_24h":-0.45}
	]`

	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, body)
		}))
		defer srv.Close()

		coins, err := fetchTopCoins(&http.Client{Timeout: 5 * time.Second}, srv.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(coins) != 2 {
			t.Fatalf("got %d coins, want 2", len(coins))
		}
		if coins[0].ID != "bitcoin" {
			t.Errorf("coins[0].ID = %q, want %q", coins[0].ID, "bitcoin")
		}
		if coins[0].MarketCapRank != 1 {
			t.Errorf("coins[0].MarketCapRank = %d, want 1", coins[0].MarketCapRank)
		}
	})

	t.Run("non-200 status returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer srv.Close()

		_, err := fetchTopCoins(&http.Client{Timeout: 5 * time.Second}, srv.URL)
		if err == nil {
			t.Error("expected error for 429 response, got nil")
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `not json`)
		}))
		defer srv.Close()

		_, err := fetchTopCoins(&http.Client{Timeout: 5 * time.Second}, srv.URL)
		if err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}
	})

	t.Run("network error returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		addr := srv.URL
		srv.Close()

		_, err := fetchTopCoins(&http.Client{Timeout: time.Second}, addr)
		if err == nil {
			t.Error("expected error for closed server, got nil")
		}
	})
}

// TestFormatLarge verifies T/B/M suffix formatting for market cap values.
func TestFormatLarge(t *testing.T) {
	tests := []struct {
		input float64
		want  string
	}{
		{1_500_000_000_000, "1.50T"},
		{305_678_000_000, "305.68B"},
		{1_234_567, "1.23M"},
		{999, "999"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := formatLarge(tt.input); got != tt.want {
				t.Errorf("formatLarge(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestPrintMarketsTable verifies the markets table output format.
func TestPrintMarketsTable(t *testing.T) {
	coins := []MarketCoin{
		{ID: "bitcoin", Symbol: "btc", Name: "Bitcoin", CurrentPrice: 105000.50,
			MarketCap: 2_089_000_000_000, MarketCapRank: 1, PriceChangePercent24h: 1.23},
		{ID: "ethereum", Symbol: "eth", Name: "Ethereum", CurrentPrice: 2541.07,
			MarketCap: 305_000_000_000, MarketCapRank: 2, PriceChangePercent24h: -0.45},
	}

	t.Run("contains expected values", func(t *testing.T) {
		var buf bytes.Buffer
		printMarketsTable(&buf, coins, "usd")
		out := buf.String()

		for _, want := range []string{"Bitcoin", "BTC", "105000.50", "2.09T", "+1.23%", "Ethereum", "ETH", "-0.45%", "305.00B", "USD"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("negative change has no extra plus sign", func(t *testing.T) {
		var buf bytes.Buffer
		printMarketsTable(&buf, coins, "usd")
		out := buf.String()
		if strings.Contains(out, "+-") {
			t.Errorf("negative change should not have + prefix: %q", out)
		}
	})

	t.Run("symbol is uppercased", func(t *testing.T) {
		var buf bytes.Buffer
		printMarketsTable(&buf, coins, "usd")
		out := buf.String()
		if strings.Contains(out, "btc") || strings.Contains(out, "eth") {
			t.Errorf("expected uppercase symbols, got lowercase in:\n%s", out)
		}
	})
}

// TestRunWatch verifies watch mode behaviour without touching a real terminal.
func TestRunWatch(t *testing.T) {
	t.Run("fetches and prints prices", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"bitcoin":{"usd":50000}}`)
		}))
		defer srv.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		var buf bytes.Buffer
		runWatch(ctx, &buf, &http.Client{Timeout: 5 * time.Second}, srv.URL, []string{"bitcoin"}, "usd", 10*time.Millisecond)

		out := buf.String()
		if !strings.Contains(out, "bitcoin") {
			t.Errorf("expected bitcoin in output, got: %q", out)
		}
		if !strings.Contains(out, "50000.00") {
			t.Errorf("expected price in output, got: %q", out)
		}
	})

	t.Run("prints error on fetch failure and keeps running", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		var buf bytes.Buffer
		runWatch(ctx, &buf, &http.Client{Timeout: 5 * time.Second}, srv.URL, []string{"bitcoin"}, "usd", 10*time.Millisecond)

		if !strings.Contains(buf.String(), "error") {
			t.Errorf("expected error message in output, got: %q", buf.String())
		}
	})

	t.Run("stops when context is cancelled", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"bitcoin":{"usd":1}}`)
		}))
		defer srv.Close()

		// A very long interval ensures the loop only exits via context, not the ticker.
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		done := make(chan struct{})
		go func() {
			var buf bytes.Buffer
			runWatch(ctx, &buf, &http.Client{Timeout: 5 * time.Second}, srv.URL, []string{"bitcoin"}, "usd", time.Hour)
			close(done)
		}()

		select {
		case <-done:
			// passed — returned after context expired
		case <-time.After(2 * time.Second):
			t.Error("runWatch did not stop after context cancellation")
		}
	})

	t.Run("prints header with interval", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"bitcoin":{"usd":1}}`)
		}))
		defer srv.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		var buf bytes.Buffer
		runWatch(ctx, &buf, &http.Client{Timeout: 5 * time.Second}, srv.URL, []string{"bitcoin"}, "usd", 30*time.Millisecond)

		if !strings.Contains(buf.String(), "30ms") {
			t.Errorf("expected interval in header, got: %q", buf.String())
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
