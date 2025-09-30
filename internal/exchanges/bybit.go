package exchanges

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type BybitProvider struct {
	client  *http.Client
	baseURL string
}

func NewBybitProvider(timeout time.Duration) *BybitProvider {
	return &BybitProvider{
		client:  SharedHTTPClient(timeout),
		baseURL: "https://api.bybit.com",
	}
}

func (b *BybitProvider) Name() string {
	return "bybit"
}

type bybitTickerResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		List []struct {
			LastPrice    string `json:"lastPrice"`
			Price24hPcnt string `json:"price24hPcnt"` // This is a decimal (e.g., "0.0123" means 1.23%)
		} `json:"list"`
	} `json:"result"`
}

func (b *BybitProvider) GetPriceAndChange(symbol string) (price float64, changePct24h float64, err error) {
	if symbol == "" {
		return 0, 0, NewProviderError(b.Name(), symbol, ErrSymbolNotSupported)
	}

	url := fmt.Sprintf("%s/v5/market/tickers?category=spot&symbol=%s", b.baseURL, symbol)
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

	var tickerResp bybitTickerResponse
	if err := json.NewDecoder(resp.Body).Decode(&tickerResp); err != nil {
		return 0, 0, NewProviderError(b.Name(), symbol, err)
	}

	// Bybit returns retCode 0 for success
	if tickerResp.RetCode != 0 {
		return 0, 0, NewProviderError(b.Name(), symbol, fmt.Errorf("Bybit API error: %s (code %d)", tickerResp.RetMsg, tickerResp.RetCode))
	}

	if len(tickerResp.Result.List) == 0 {
		return 0, 0, NewProviderError(b.Name(), symbol, fmt.Errorf("no data returned"))
	}

	ticker := tickerResp.Result.List[0]

	price, err = strconv.ParseFloat(ticker.LastPrice, 64)
	if err != nil {
		return 0, 0, NewProviderError(b.Name(), symbol, fmt.Errorf("parse lastPrice: %w", err))
	}

	// price24hPcnt is in decimal form (e.g., "0.0123" = 1.23%), multiply by 100 to get percentage
	changePctDecimal, err := strconv.ParseFloat(ticker.Price24hPcnt, 64)
	if err != nil {
		return 0, 0, NewProviderError(b.Name(), symbol, fmt.Errorf("parse price24hPcnt: %w", err))
	}

	changePct24h = changePctDecimal * 100

	return price, changePct24h, nil
}
