# cryptoprice — project context for Claude Code

## What this is
A Go CLI tool that fetches live cryptocurrency prices from the CoinGecko API
(https://www.coingecko.com/en/api) and prints them in a formatted table.
This is a portfolio project demonstrating Go fundamentals: CLI flag parsing,
HTTP requests with timeouts, JSON handling, and unit testing.

## Files
- `main.go` — entry point, argument parsing, URL building, API fetching
- `main_test.go` — unit tests using net/http/httptest (no live network calls)
- `README.md` — usage docs, kept in sync with actual CLI behavior
- `go.mod` — module definition (module name: cryptoprice)

## Conventions
- Standard library only. Don't add external dependencies (e.g. cobra, viper)
  unless explicitly requested — the simplicity is part of the point.
- Every function gets a short doc comment explaining what it does and why.
- All new logic that can be unit tested should be. Use httptest.NewServer
  for anything that talks to the network; never call the real CoinGecko API
  in tests.
- Error messages should be specific and wrap underlying errors with %w.
- CLI flags use Go's `flag` package, with sensible defaults documented in
  both `--help` output (via flag descriptions) and README.md.

## Before finishing any task
1. `go vet ./...`
2. `go build ./...`
3. `go test ./...`
All three must pass. If README.md usage examples, flags, or behavior
changed, update README.md to match.

## Commit style
Small, focused commits. One logical change per commit. Commit message
format: short imperative summary line (e.g. "Add --watch flag for
auto-refreshing prices"), optionally followed by a blank line and
1-3 bullet points of detail for non-trivial changes.

## Things to avoid
- Don't silently swallow errors — always return or log them.
- Don't hardcode API keys or secrets (CoinGecko's free tier needs none).
- Don't break existing CLI flag behavior without updating README and tests.
