package exchanges

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type OKXProvider struct {
	client  *http.Client
	baseURL string
}

func NewOKXProvider(timeout time.Duration) *OKXProvider {
	return &OKXProvider{
		client:  SharedHTTPClient(timeout),
		baseURL: "https://www.okx.com",
	}
}

func (o *OKXProvider) Name() string {
	return "okx"
}

type okxTickerResponse struct {
	Code string `json:"code"`
	Data []struct {
		Last    string `json:"last"`
		Open24h string `json:"open24h"`
	} `json:"data"`
}

func (o *OKXProvider) GetPriceAndChange(symbol string) (price float64, changePct24h float64, err error) {
	if symbol == "" {
		return 0, 0, NewProviderError(o.Name(), symbol, ErrSymbolNotSupported)
	}

	url := fmt.Sprintf("%s/api/v5/market/ticker?instId=%s", o.baseURL, symbol)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, 0, NewProviderError(o.Name(), symbol, err)
	}

	req.Header.Set("User-Agent", DefaultUserAgent())

	resp, err := o.client.Do(req)
	if err != nil {
		return 0, 0, NewProviderError(o.Name(), symbol, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return 0, 0, NewProviderError(o.Name(), symbol, ErrRateLimited)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, 0, NewProviderError(o.Name(), symbol, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body)))
	}

	var tickerResp okxTickerResponse
	if err := json.NewDecoder(resp.Body).Decode(&tickerResp); err != nil {
		return 0, 0, NewProviderError(o.Name(), symbol, err)
	}

	// OKX returns code "0" for success
	if tickerResp.Code != "0" {
		return 0, 0, NewProviderError(o.Name(), symbol, fmt.Errorf("OKX API error code: %s", tickerResp.Code))
	}

	if len(tickerResp.Data) == 0 {
		return 0, 0, NewProviderError(o.Name(), symbol, fmt.Errorf("no data returned"))
	}

	ticker := tickerResp.Data[0]

	last, err := strconv.ParseFloat(ticker.Last, 64)
	if err != nil {
		return 0, 0, NewProviderError(o.Name(), symbol, fmt.Errorf("parse last: %w", err))
	}

	open24h, err := strconv.ParseFloat(ticker.Open24h, 64)
	if err != nil {
		return 0, 0, NewProviderError(o.Name(), symbol, fmt.Errorf("parse open24h: %w", err))
	}

	if open24h == 0 {
		return 0, 0, NewProviderError(o.Name(), symbol, fmt.Errorf("open24h is zero, cannot calculate change"))
	}

	// Calculate 24h change percentage: (last/open24h - 1) * 100
	changePct24h = (last/open24h - 1) * 100

	return last, changePct24h, nil
}
