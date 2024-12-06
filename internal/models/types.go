package models

type Coin struct {
	Name string
	ID   string
}

type MultiCurrency struct {
	USD float64 `json:"usd"`
}

type CoinData struct {
	Price                    MultiCurrency `json:"current_price"`
	PriceChangePercentage24h float64       `json:"price_change_percentage_24h"`
	MarketCap                MultiCurrency `json:"market_cap"`
	FullyDilutedValuation    MultiCurrency `json:"fully_diluted_valuation"`
	Volume24h                MultiCurrency `json:"total_volume"`
}

type CoinGeckoResponse struct {
	MarketData CoinData `json:"market_data"`
}

type RPCRequest struct {
	JsonRPC string   `json:"jsonrpc"`
	Method  string   `json:"method"`
	Params  []string `json:"params"`
	ID      int      `json:"id"`
}
