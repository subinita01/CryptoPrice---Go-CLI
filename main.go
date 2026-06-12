package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"text/tabwriter"
	"time"
)

const baseURL = "https://api.coingecko.com/api/v3/simple/price"

// PriceResult maps each coin ID to a map of currency → price.
type PriceResult map[string]map[string]float64

// buildURL constructs the CoinGecko simple/price request URL from the given
// coin IDs and vs-currency so the URL can be built and tested independently.
func buildURL(coins []string, currency string) string {
	params := url.Values{}
	params.Set("ids", strings.Join(coins, ","))
	params.Set("vs_currencies", currency)
	return baseURL + "?" + params.Encode()
}

// fetchPrices performs a GET request to rawURL using client and decodes the
// JSON body into a PriceResult. It returns an error for non-200 responses or
// any network/decode failure.
func fetchPrices(client *http.Client, rawURL string) (PriceResult, error) {
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from API", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var result PriceResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding JSON: %w", err)
	}

	return result, nil
}

// printTable writes a tab-aligned price table to w, iterating coins in the
// order supplied so the output is deterministic and matches user input order.
func printTable(w io.Writer, result PriceResult, coins []string, currency string) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintf(tw, "COIN\tPRICE (%s)\n", strings.ToUpper(currency))
	fmt.Fprintf(tw, "----\t----------\n")
	for _, coin := range coins {
		data, ok := result[coin]
		if !ok {
			fmt.Fprintf(tw, "%s\tn/a\n", coin)
			continue
		}
		price, ok := data[strings.ToLower(currency)]
		if !ok {
			fmt.Fprintf(tw, "%s\tn/a\n", coin)
			continue
		}
		fmt.Fprintf(tw, "%s\t%.2f\n", coin, price)
	}
	tw.Flush()
}

// runWatch repeatedly fetches and prints prices at interval, clearing the
// terminal before each refresh. It returns as soon as ctx is cancelled,
// making Ctrl-C (or any signal wired to the context) a clean exit path.
func runWatch(ctx context.Context, w io.Writer, client *http.Client, rawURL string, coins []string, currency string, interval time.Duration) {
	for {
		fmt.Fprint(w, "\033[2J\033[H")
		fmt.Fprintf(w, "cryptoprice  —  refreshing every %s  (Ctrl-C to quit)\n\n", interval)

		result, err := fetchPrices(client, rawURL)
		if err != nil {
			fmt.Fprintf(w, "error: %v\n", err)
		} else {
			printTable(w, result, coins, currency)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

func main() {
	currency := flag.String("currency", "usd", "vs-currency for prices (e.g. usd, eur, btc)")
	timeout := flag.Duration("timeout", 10*time.Second, "HTTP request timeout")
	watch := flag.Duration("watch", 0, "auto-refresh interval (e.g. 5s, 1m); 0 disables watch mode")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: cryptoprice [flags] coin [coin ...]\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n  cryptoprice bitcoin\n  cryptoprice bitcoin ethereum solana -currency eur\n  cryptoprice bitcoin -watch 10s\n")
	}
	flag.Parse()

	coins := flag.Args()
	if len(coins) == 0 {
		fmt.Fprintln(os.Stderr, "error: at least one coin ID required (e.g. bitcoin, ethereum)")
		flag.Usage()
		os.Exit(1)
	}

	client := &http.Client{Timeout: *timeout}
	rawURL := buildURL(coins, *currency)

	if *watch > 0 {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		runWatch(ctx, os.Stdout, client, rawURL, coins, *currency, *watch)
		return
	}

	result, err := fetchPrices(client, rawURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	printTable(os.Stdout, result, coins, *currency)
}
