package exchanges

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Provider defines the interface for exchange data providers
type Provider interface {
	Name() string
	GetPriceAndChange(symbol string) (price float64, changePct24h float64, err error)
}

// Common errors
var (
	ErrSymbolNotSupported = errors.New("symbol not supported by this exchange")
	ErrRateLimited        = errors.New("rate limited by exchange")
)

// ProviderError wraps errors with context about the provider and symbol
type ProviderError struct {
	Provider string
	Symbol   string
	Err      error
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("provider=%s symbol=%s: %v", e.Provider, e.Symbol, e.Err)
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

// NewProviderError creates a new provider error with context
func NewProviderError(provider, symbol string, err error) error {
	return &ProviderError{
		Provider: provider,
		Symbol:   symbol,
		Err:      err,
	}
}

// SharedHTTPClient returns a shared HTTP client with appropriate settings
func SharedHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}

// DefaultUserAgent returns a user agent string for the bot
func DefaultUserAgent() string {
	return "scroll-rank-bot/1.0"
}
