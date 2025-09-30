package exchanges

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type BinanceProvider struct {
	client  *http.Client
	baseURL string
}

func NewBinanceProvider(timeout time.Duration) *BinanceProvider {
	return &BinanceProvider{
		client:  SharedHTTPClient(timeout),
		baseURL: "https://api.binance.com",
	}
}

func (b *BinanceProvider) Name() string {
	return "binance"
}

type binanceTicker24hr struct {
	LastPrice          string `json:"lastPrice"`
	PriceChangePercent string `json:"priceChangePercent"`
}

func (b *BinanceProvider) GetPriceAndChange(symbol string) (price float64, changePct24h float64, err error) {
	if symbol == "" {
		return 0, 0, NewProviderError(b.Name(), symbol, ErrSymbolNotSupported)
	}

	url := fmt.Sprintf("%s/api/v3/ticker/24hr?symbol=%s", b.baseURL, symbol)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, 0, NewProviderError(b.Name(), symbol, err)
	}

	req.Header.Set("User-Agent", DefaultUserAgent())

	resp, err := b.client.Do(req)
	if err != nil {
		return 0, 0, NewProviderError(b.Name(), symbol, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return 0, 0, NewProviderError(b.Name(), symbol, ErrRateLimited)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, 0, NewProviderError(b.Name(), symbol, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body)))
	}

	var ticker binanceTicker24hr
	if err := json.NewDecoder(resp.Body).Decode(&ticker); err != nil {
		return 0, 0, NewProviderError(b.Name(), symbol, err)
	}

	price, err = strconv.ParseFloat(ticker.LastPrice, 64)
	if err != nil {
		return 0, 0, NewProviderError(b.Name(), symbol, fmt.Errorf("parse lastPrice: %w", err))
	}

	changePct24h, err = strconv.ParseFloat(ticker.PriceChangePercent, 64)
	if err != nil {
		return 0, 0, NewProviderError(b.Name(), symbol, fmt.Errorf("parse priceChangePercent: %w", err))
	}

	return price, changePct24h, nil
}
