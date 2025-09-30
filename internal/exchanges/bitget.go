package exchanges

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type BitgetProvider struct {
	client  *http.Client
	baseURL string
}

func NewBitgetProvider(timeout time.Duration) *BitgetProvider {
	return &BitgetProvider{
		client:  SharedHTTPClient(timeout),
		baseURL: "https://api.bitget.com",
	}
}

func (b *BitgetProvider) Name() string {
	return "bitget"
}

type bitgetTickerResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Close string `json:"close"` // Last price
		Open  string `json:"open"`  // 24h open price
	} `json:"data"`
}

func (b *BitgetProvider) GetPriceAndChange(symbol string) (price float64, changePct24h float64, err error) {
	if symbol == "" {
		return 0, 0, NewProviderError(b.Name(), symbol, ErrSymbolNotSupported)
	}

	url := fmt.Sprintf("%s/api/spot/v1/market/ticker?symbol=%s", b.baseURL, symbol)
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

	var tickerResp bitgetTickerResponse
	if err := json.NewDecoder(resp.Body).Decode(&tickerResp); err != nil {
		return 0, 0, NewProviderError(b.Name(), symbol, err)
	}

	// Bitget returns code "00000" for success
	if tickerResp.Code != "00000" {
		return 0, 0, NewProviderError(b.Name(), symbol, fmt.Errorf("Bitget API error: %s (code %s)", tickerResp.Msg, tickerResp.Code))
	}

	close, err := strconv.ParseFloat(tickerResp.Data.Close, 64)
	if err != nil {
		return 0, 0, NewProviderError(b.Name(), symbol, fmt.Errorf("parse close: %w", err))
	}

	open, err := strconv.ParseFloat(tickerResp.Data.Open, 64)
	if err != nil {
		return 0, 0, NewProviderError(b.Name(), symbol, fmt.Errorf("parse open: %w", err))
	}

	if open == 0 {
		return 0, 0, NewProviderError(b.Name(), symbol, fmt.Errorf("open price is zero, cannot calculate change"))
	}

	// Calculate 24h change percentage: (close/open - 1) * 100
	changePct24h = (close/open - 1) * 100

	return close, changePct24h, nil
}
