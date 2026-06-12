# cryptoprice

A command-line tool that fetches live cryptocurrency prices from the
[CoinGecko API](https://www.coingecko.com/en/api) and prints them in a
formatted table. No API key required.

## Requirements

- Go 1.18 or later
- Internet access (CoinGecko free tier, no authentication needed)

## Build

```sh
git clone <repo-url>
cd cryptoprice
go build -o cryptoprice .
```

## Usage

```
cryptoprice [flags] coin [coin ...]
```

Coin IDs are the lowercase slugs used by CoinGecko (e.g. `bitcoin`,
`ethereum`, `solana`). You can look them up at
https://www.coingecko.com/en/coins/all.

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-currency` | `usd` | vs-currency to quote prices in (e.g. `usd`, `eur`, `btc`) |
| `-timeout` | `10s` | HTTP request timeout (e.g. `5s`, `30s`) |
| `-watch` | `0` (off) | Auto-refresh interval; any non-zero duration enables watch mode (e.g. `5s`, `1m`) |
| `-top` | `0` (off) | Fetch top N coins by market cap instead of specific coins (e.g. `-top 10`) |

## Examples

Fetch the Bitcoin price in USD:

```sh
$ ./cryptoprice bitcoin
COIN      PRICE (USD)
----      ----------
bitcoin   105432.18
```

Fetch multiple coins at once:

```sh
$ ./cryptoprice bitcoin ethereum solana
COIN       PRICE (USD)
----       ----------
bitcoin    105432.18
ethereum   2541.07
solana     172.34
```

Quote prices in a different currency:

```sh
$ ./cryptoprice bitcoin ethereum -currency eur
COIN       PRICE (EUR)
----       ----------
bitcoin    97348.55
ethereum   2345.91
```

Use a shorter timeout for slow connections:

```sh
$ ./cryptoprice bitcoin -timeout 5s
```

Auto-refresh every 10 seconds (Ctrl-C to quit):

```sh
$ ./cryptoprice bitcoin ethereum -watch 10s
cryptoprice  —  refreshing every 10s  (Ctrl-C to quit)

COIN       PRICE (USD)
----       ----------
bitcoin    105432.18
ethereum   2541.07
```

Fetch the top 10 coins by market cap:

```sh
$ ./cryptoprice -top 10
RANK   NAME       SYMBOL   PRICE (USD)    MARKET CAP   24H %
----   ----       ------   ----------     ----------   ------
1      Bitcoin    BTC      105432.18      2.09T        +1.23%
2      Ethereum   ETH      2541.07        305.00B      -0.45%
...
```

`-top` and `-currency` can be combined; `-top` and `-watch` cannot be used together.

If a coin ID is not recognised by CoinGecko the row shows `n/a` instead
of a price, and the other coins still print normally.

## Error handling

- Unknown or misspelled coin IDs print `n/a` rather than crashing.
- Network errors and non-200 API responses print a specific error message
  to stderr and exit with code 1.
- Running with no coin arguments prints usage and exits with code 1.

## Running tests

```sh
go test ./...
```

Tests use `net/http/httptest` — no live network calls are made.

## Project structure

```
.
├── main.go       # entry point: flag parsing, URL building, API fetch, table output
├── main_test.go  # unit tests for buildURL, fetchPrices, and printTable
├── go.mod        # module definition (module name: cryptoprice)
└── README.md
```
