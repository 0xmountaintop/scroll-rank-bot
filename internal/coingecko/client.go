package coingecko

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"telegrambot/internal/models"
)

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) FetchCoinData(coinID string) (*models.CoinData, error) {
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/coins/%s", coinID)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch coin data: %w", err)
	}
	defer resp.Body.Close()

	var geckoResp models.CoinGeckoResponse
	if err := json.NewDecoder(resp.Body).Decode(&geckoResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &geckoResp.MarketData, nil
}
