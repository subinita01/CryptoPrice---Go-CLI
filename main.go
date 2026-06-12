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
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

const (
	baseURL    = "https://api.coingecko.com/api/v3/simple/price"
	marketsURL = "https://api.coingecko.com/api/v3/coins/markets"
)

// PriceResult maps each coin ID to a map of currency → price.
type PriceResult map[string]map[string]float64

// MarketCoin holds the subset of fields returned by the /coins/markets endpoint.
type MarketCoin struct {
	ID                    string  `json:"id"`
	Symbol                string  `json:"symbol"`
	Name                  string  `json:"name"`
	CurrentPrice          float64 `json:"current_price"`
	MarketCap             float64 `json:"market_cap"`
	MarketCapRank         int     `json:"market_cap_rank"`
	PriceChangePercent24h float64 `json:"price_change_percentage_24h"`
}

// buildURL constructs the CoinGecko simple/price request URL from the given
// coin IDs and vs-currency so the URL can be built and tested independently.
func buildURL(coins []string, currency string) string {
	params := url.Values{}
	params.Set("ids", strings.Join(coins, ","))
	params.Set("vs_currencies", currency)
	return baseURL + "?" + params.Encode()
}

// buildMarketsURL constructs the CoinGecko /coins/markets URL for the top n
// coins ordered by market cap descending in the given currency.
func buildMarketsURL(n int, currency string) string {
	params := url.Values{}
	params.Set("vs_currency", currency)
	params.Set("order", "market_cap_desc")
	params.Set("per_page", strconv.Itoa(n))
	params.Set("page", "1")
	return marketsURL + "?" + params.Encode()
}

// fetchTopCoins performs a GET request to rawURL and decodes the JSON array
// into a slice of MarketCoin. It returns an error for non-200 responses or
// any network/decode failure.
func fetchTopCoins(client *http.Client, rawURL string) ([]MarketCoin, error) {
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

	var coins []MarketCoin
	if err := json.Unmarshal(body, &coins); err != nil {
		return nil, fmt.Errorf("decoding JSON: %w", err)
	}

	return coins, nil
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

// formatLarge formats a large float as a compact human-readable string using
// T/B/M suffixes so market cap values fit neatly in the table column.
func formatLarge(f float64) string {
	switch {
	case f >= 1e12:
		return fmt.Sprintf("%.2fT", f/1e12)
	case f >= 1e9:
		return fmt.Sprintf("%.2fB", f/1e9)
	case f >= 1e6:
		return fmt.Sprintf("%.2fM", f/1e6)
	default:
		return fmt.Sprintf("%.0f", f)
	}
}

// printMarketsTable writes a tab-aligned markets table to w showing rank,
// name, symbol, price, market cap, and 24-hour price change for each coin.
func printMarketsTable(w io.Writer, coins []MarketCoin, currency string) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintf(tw, "RANK\tNAME\tSYMBOL\tPRICE (%s)\tMARKET CAP\t24H %%\n", strings.ToUpper(currency))
	fmt.Fprintf(tw, "----\t----\t------\t----------\t----------\t------\n")
	for _, c := range coins {
		sign := "+"
		if c.PriceChangePercent24h < 0 {
			sign = ""
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%.2f\t%s\t%s%.2f%%\n",
			c.MarketCapRank,
			c.Name,
			strings.ToUpper(c.Symbol),
			c.CurrentPrice,
			formatLarge(c.MarketCap),
			sign,
			c.PriceChangePercent24h,
		)
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
	top := flag.Int("top", 0, "fetch top N coins by market cap (e.g. -top 10); mutually exclusive with coin arguments")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: cryptoprice [flags] coin [coin ...]\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n  cryptoprice bitcoin\n  cryptoprice bitcoin ethereum solana -currency eur\n  cryptoprice bitcoin -watch 10s\n  cryptoprice -top 10\n  cryptoprice -top 5 -currency eur\n")
	}
	flag.Parse()

	coins := flag.Args()

	if *top > 0 && len(coins) > 0 {
		fmt.Fprintln(os.Stderr, "error: -top and coin arguments are mutually exclusive")
		os.Exit(1)
	}
	if *top > 0 && *watch > 0 {
		fmt.Fprintln(os.Stderr, "error: -top and -watch cannot be used together")
		os.Exit(1)
	}
	if *top == 0 && len(coins) == 0 {
		fmt.Fprintln(os.Stderr, "error: specify at least one coin ID or use -top N (e.g. -top 10)")
		flag.Usage()
		os.Exit(1)
	}

	client := &http.Client{Timeout: *timeout}

	if *top > 0 {
		rawURL := buildMarketsURL(*top, *currency)
		topCoins, err := fetchTopCoins(client, rawURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		printMarketsTable(os.Stdout, topCoins, *currency)
		return
	}

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
