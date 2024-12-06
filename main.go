package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

type Coin struct {
	Name string
	ID   string
}

type MultiCurrency struct {
	USD float64 `json:"usd"`
}

type CoinData struct {
	Price                        MultiCurrency `json:"current_price"`
	PriceChangePercentage24h     float64       `json:"price_change_percentage_24h"`
	MarketCap                    MultiCurrency `json:"market_cap"`
	FullyDilutedValuation        MultiCurrency `json:"fully_diluted_valuation"`
	MarketCapChangePercentage24h float64       `json:"market_cap_change_percentage_24h"`
	Volume24h                    MultiCurrency `json:"total_volume"`
}

type CoinGeckoResponse struct {
	MarketData CoinData `json:"market_data"`
}

var (
	coins = map[string]Coin{
		"starknet": {Name: "Starknet", ID: "starknet"},
		"zksync":   {Name: "ZkSync", ID: "zksync"},
		"taiko":    {Name: "Taiko", ID: "taiko"},
		"scroll":   {Name: "Scroll", ID: "scroll"},
	}

	updateInterval        = 5 * time.Minute
	gasPriceCacheDuration = 1 * time.Minute

	cachedData            string
	lastFetchTime         time.Time
	cachedGasPrices       string
	lastGasPriceFetchTime time.Time
	mutex                 sync.RWMutex
)

func formatValue(value float64) string {
	if value == 0 {
		return "N/A"
	}
	if value >= 1e9 {
		return fmt.Sprintf("%.2f B", value/1e9)
	}
	if value >= 1e6 {
		return fmt.Sprintf("%.2f M", value/1e6)
	}
	return fmt.Sprintf("%.2f", value)
}

func formatPrice(price float64) string {
	if price == 0 {
		return "N/A"
	}
	return fmt.Sprintf("$%.4f", price)
}

func fetchCoinData(coinID string) (*CoinData, error) {
	resp, err := http.Get(fmt.Sprintf("https://api.coingecko.com/api/v3/coins/%s", coinID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var geckoResp CoinGeckoResponse
	if err := json.NewDecoder(resp.Body).Decode(&geckoResp); err != nil {
		return nil, err
	}

	return &geckoResp.MarketData, nil
}

func formatCoinMessage(coinName string, data *CoinData, fdvRatio float64) string {
	if data == nil {
		return fmt.Sprintf("%s:\nData unavailable", coinName)
	}

	priceChangeArrow := ""
	if data.PriceChangePercentage24h > 0 {
		priceChangeArrow = "⬆️"
	} else if data.PriceChangePercentage24h < 0 {
		priceChangeArrow = "⬇️"
	}

	marketCapChangeArrow := ""
	if data.MarketCapChangePercentage24h > 0 {
		marketCapChangeArrow = "⬆️"
	} else if data.MarketCapChangePercentage24h < 0 {
		marketCapChangeArrow = "⬇️"
	}

	return fmt.Sprintf(`%s:
- Price: %s
- 24h Price Change: %.2f%% %s
- 24h Volume (USD): %s
- Market Cap: %s
- 24h MC Change: %.2f%% %s
- FDV: %s
- FDV Ratio: %.2f%%`,
		coinName,
		formatPrice(data.Price.USD),
		data.PriceChangePercentage24h,
		priceChangeArrow,
		formatValue(data.Volume24h.USD),
		formatValue(data.MarketCap.USD),
		data.MarketCapChangePercentage24h,
		marketCapChangeArrow,
		formatValue(data.FullyDilutedValuation.USD),
		fdvRatio*100)
}

func updateData() {
	results := make(map[string]*CoinData)
	var wg sync.WaitGroup
	resultsChan := make(chan struct {
		id   string
		data *CoinData
	}, len(coins))

	for _, coin := range coins {
		wg.Add(1)
		go func(coin Coin) {
			defer wg.Done()
			data, err := fetchCoinData(coin.ID)
			if err != nil {
				log.Printf("Error fetching data for %s: %v", coin.ID, err)
				resultsChan <- struct {
					id   string
					data *CoinData
				}{coin.ID, nil}
				return
			}
			resultsChan <- struct {
				id   string
				data *CoinData
			}{coin.ID, data}
		}(coin)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	for result := range resultsChan {
		results[result.id] = result.data
	}

	var totalFDV float64
	for _, data := range results {
		if data != nil {
			totalFDV += data.FullyDilutedValuation.USD
		}
	}

	var messages []string
	for _, coin := range coins {
		data := results[coin.ID]
		var fdvRatio float64
		if data != nil && totalFDV > 0 {
			fdvRatio = data.FullyDilutedValuation.USD / totalFDV
		}
		messages = append(messages, formatCoinMessage(coin.Name, data, fdvRatio))
	}

	mutex.Lock()
	cachedData = fmt.Sprintf("Date: %s (UTC)\n\n%s", time.Now().UTC().Format("2006-01-02 15:04:05"), strings.Join(messages, "\n\n"))
	lastFetchTime = time.Now()
	mutex.Unlock()

	log.Printf("Data updated successfully: %v", time.Now())
}

type RPCRequest struct {
	JsonRPC string   `json:"jsonrpc"`
	Method  string   `json:"method"`
	Params  []string `json:"params"`
	ID      int      `json:"id"`
}

func fetchGasPrice() string {
	mutex.RLock()
	if time.Since(lastGasPriceFetchTime) < gasPriceCacheDuration && cachedGasPrices != "" {
		defer mutex.RUnlock()
		return cachedGasPrices
	}
	mutex.RUnlock()

	rpcEndpoints := map[string]string{
		"ethereum": "https://rpc.mevblocker.io",
		"zksync":   "https://mainnet.era.zksync.io",
		"taiko":    "https://rpc.mainnet.taiko.xyz",
		"scroll":   "https://rpc.scroll.io",
	}

	type gasResult struct {
		network string
		price   float64
	}

	results := make(chan gasResult, len(rpcEndpoints))
	var wg sync.WaitGroup

	for network, endpoint := range rpcEndpoints {
		wg.Add(1)
		go func(network, endpoint string) {
			defer wg.Done()

			reqBody := RPCRequest{
				JsonRPC: "2.0",
				Method:  "eth_gasPrice",
				Params:  []string{},
				ID:      1,
			}

			jsonData, _ := json.Marshal(reqBody)
			resp, err := http.Post(endpoint, "application/json", strings.NewReader(string(jsonData)))
			if err != nil {
				log.Printf("Error fetching gas price from %s: %v", endpoint, err)
				results <- gasResult{network: network, price: 0}
				return
			}
			defer resp.Body.Close()

			var result struct {
				Result string `json:"result"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				results <- gasResult{network: network, price: 0}
				return
			}

			hexValue := strings.TrimPrefix(result.Result, "0x")
			intValue, _ := strconv.ParseInt(hexValue, 16, 64)
			gweiPrice := float64(intValue) / 1e9

			results <- gasResult{network: network, price: gweiPrice}
		}(network, endpoint)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	gasPrices := make(map[string]float64)
	for result := range results {
		gasPrices[result.network] = result.price
	}

	message := fmt.Sprintf(`🔄 Current Gas Prices (Gwei):

⬙ Ethereum: %.2f
⇆ ZkSync: %.2f
▲ Taiko: %.2f
📜 Scroll: %.2f

Updated: %s UTC`,
		gasPrices["ethereum"],
		gasPrices["zksync"],
		gasPrices["taiko"],
		gasPrices["scroll"],
		time.Now().UTC().Format("2006-01-02 15:04:05"))

	mutex.Lock()
	cachedGasPrices = message
	lastGasPriceFetchTime = time.Now()
	mutex.Unlock()

	return message
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	updateData()
	go func() {
		ticker := time.NewTicker(updateInterval)
		for range ticker.C {
			updateData()
		}
	}()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		switch update.Message.Command() {
		case "check_scroll_ranking":
			mutex.RLock()
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, cachedData)
			mutex.RUnlock()
			bot.Send(msg)

		case "get_current_gas_price":
			gasPriceMessage := fetchGasPrice()
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, gasPriceMessage)
			bot.Send(msg)
		}
	}
}
