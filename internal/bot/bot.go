package bot

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"telegrambot/internal/coingecko"
	"telegrambot/internal/gas"
	"telegrambot/internal/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api            *tgbotapi.BotAPI
	coingecko      *coingecko.Client
	gasService     *gas.PriceService
	coins          map[string]models.Coin
	cachedData     string
	cachedGas      string
	lastFetchTime  time.Time
	lastGasTime    time.Time
	mutex          sync.RWMutex
	updateInterval time.Duration
	gasCacheDur    time.Duration
}

func New(token string) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	return &Bot{
		api:            api,
		coingecko:      coingecko.NewClient(),
		gasService:     gas.NewPriceService(),
		updateInterval: 5 * time.Minute,
		gasCacheDur:    1 * time.Minute,
		coins: map[string]models.Coin{
			"starknet": {Name: "Starknet", ID: "starknet"},
			"zksync":   {Name: "ZkSync", ID: "zksync"},
			"taiko":    {Name: "Taiko", ID: "taiko"},
			"scroll":   {Name: "Scroll", ID: "scroll"},
		},
	}, nil
}

func (b *Bot) Start() {
	log.Printf("Authorized on account %s", b.api.Self.UserName)

	b.updateData()
	go b.startUpdateTicker()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)
	b.handleUpdates(updates)
}

func (b *Bot) handleUpdates(updates tgbotapi.UpdatesChannel) {
	for update := range updates {
		if update.Message == nil {
			continue
		}

		switch update.Message.Command() {
		case "check_scroll_ranking":
			b.mutex.RLock()
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, b.cachedData)
			b.mutex.RUnlock()
			b.api.Send(msg)

		case "get_current_gas_price":
			gasPrices := b.gasService.FetchAllPrices()
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, b.formatGasPrices(gasPrices))
			b.api.Send(msg)
		}
	}
}

func (b *Bot) startUpdateTicker() {
	ticker := time.NewTicker(b.updateInterval)
	for range ticker.C {
		b.updateData()
	}
}

func (b *Bot) updateData() {
	var wg sync.WaitGroup
	results := make(chan struct {
		id   string
		data *models.CoinData
	}, len(b.coins))

	for _, coin := range b.coins {
		wg.Add(1)
		go func(coin models.Coin) {
			defer wg.Done()
			data, err := b.coingecko.FetchCoinData(coin.ID)
			if err != nil {
				log.Printf("Error fetching data for %s: %v", coin.ID, err)
				results <- struct {
					id   string
					data *models.CoinData
				}{id: coin.ID, data: nil}
				return
			}
			results <- struct {
				id   string
				data *models.CoinData
			}{id: coin.ID, data: data}
		}(coin)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var coinDataList []struct {
		id   string
		data *models.CoinData
	}
	for result := range results {
		coinDataList = append(coinDataList, result)
	}

	sort.Slice(coinDataList, func(i, j int) bool {
		if coinDataList[i].data == nil || coinDataList[j].data == nil {
			return false
		}
		return coinDataList[i].data.FullyDilutedValuation.USD > coinDataList[j].data.FullyDilutedValuation.USD
	})

	b.mutex.Lock()
	b.cachedData = b.formatCoinData(coinDataList)
	b.lastFetchTime = time.Now()
	b.mutex.Unlock()

	log.Printf("Data updated successfully at %v", time.Now())
}

func (b *Bot) formatCoinData(data []struct {
	id   string
	data *models.CoinData
}) string {
	var messages []string
	for _, item := range data {
		messages = append(messages, b.formatSingleCoin(item.id, item.data))
	}
	return fmt.Sprintf("Date: %s (UTC)\n\n%s",
		time.Now().UTC().Format("2006-01-02 15:04:05"),
		strings.Join(messages, "\n\n"))
}

func (b *Bot) formatSingleCoin(coinID string, data *models.CoinData) string {
	if data == nil {
		return fmt.Sprintf("%s:\nData unavailable", coinID)
	}

	priceChangeArrow := ""
	if data.PriceChangePercentage24h > 0 {
		priceChangeArrow = "⬆️"
	} else if data.PriceChangePercentage24h < 0 {
		priceChangeArrow = "⬇️"
	}

	return fmt.Sprintf(`%s:
- Price: %s
- 24h Price Change: %.2f%% %s
- 24h Volume (USD): %s
- Market Cap: %s
- FDV: %s`,
		coinID,
		formatPrice(data.Price.USD),
		data.PriceChangePercentage24h,
		priceChangeArrow,
		formatValue(data.Volume24h.USD),
		formatValue(data.MarketCap.USD),
		formatValue(data.FullyDilutedValuation.USD))
}

func (b *Bot) formatGasPrices(prices map[string]float64) string {
	return fmt.Sprintf(`🔄 Current Gas Prices (Gwei):

⬙ Ethereum: %.2f
⇆ ZkSync: %.2f
▲ Taiko: %.2f
📜 Scroll: %.2f

Updated: %s UTC`,
		prices["ethereum"],
		prices["zksync"],
		prices["taiko"],
		prices["scroll"],
		time.Now().UTC().Format("2006-01-02 15:04:05"))
}

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
