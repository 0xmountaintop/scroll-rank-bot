package market

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"scroll-rank-bot/internal/coingecko"
	"scroll-rank-bot/internal/exchanges"
	"scroll-rank-bot/internal/models"
)

// Aggregator fetches coin data from CoinGecko (primary) or exchanges (fallback)
type Aggregator struct {
	coingecko *coingecko.Client
	providers []exchanges.Provider
	supplies  map[string]models.SupplySnapshot
	mu        sync.RWMutex
	supplyTTL time.Duration
	volumeTTL time.Duration
}

// NewAggregator creates a new market data aggregator
func NewAggregator(cgClient *coingecko.Client, httpTimeout time.Duration, supplyTTL, volumeTTL time.Duration) *Aggregator {
	return &Aggregator{
		coingecko: cgClient,
		providers: []exchanges.Provider{
			exchanges.NewBinanceProvider(httpTimeout),
			exchanges.NewOKXProvider(httpTimeout),
			exchanges.NewBybitProvider(httpTimeout),
			exchanges.NewBitgetProvider(httpTimeout),
		},
		supplies:  make(map[string]models.SupplySnapshot),
		supplyTTL: supplyTTL,
		volumeTTL: volumeTTL,
	}
}

// FetchCoinData fetches data for a coin, trying CoinGecko first, then exchanges
func (a *Aggregator) FetchCoinData(coin models.Coin) (*models.CoinData, error) {
	// Try CoinGecko first
	data, err := a.fetchFromCoinGecko(coin.ID)
	if err == nil {
		// CoinGecko succeeded, cache supply and volume data
		a.updateSupplyCache(coin.ID, data)
		log.Printf("[%s] source=coingecko status=success", coin.ID)
		return data, nil
	}

	// CoinGecko failed, log and try fallback
	log.Printf("[%s] source=coingecko status=failed error=%v, trying exchanges", coin.ID, err)

	// Try exchanges in order
	data, err = a.fetchFromExchanges(coin)
	if err == nil {
		log.Printf("[%s] source=exchange status=success", coin.ID)
		return data, nil
	}

	log.Printf("[%s] source=all status=failed error=%v", coin.ID, err)
	return nil, err
}

// fetchFromCoinGecko fetches data from CoinGecko
func (a *Aggregator) fetchFromCoinGecko(coinID string) (*models.CoinData, error) {
	return a.coingecko.FetchCoinData(coinID)
}

// updateSupplyCache calculates and caches supply and volume data
func (a *Aggregator) updateSupplyCache(coinID string, data *models.CoinData) {
	if data == nil {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	snapshot := models.SupplySnapshot{
		UpdatedAt:      time.Now(),
		TotalVolumeUSD: data.Volume24h.USD,
	}

	// Calculate supply only if price is not zero (avoid division by zero)
	if data.Price.USD > 0 {
		if data.MarketCap.USD > 0 {
			snapshot.Circulating = data.MarketCap.USD / data.Price.USD
		}
		if data.FullyDilutedValuation.USD > 0 {
			snapshot.Full = data.FullyDilutedValuation.USD / data.Price.USD
		}
	}

	a.supplies[coinID] = snapshot
	log.Printf("[%s] cache_update circulating=%.2f full=%.2f volume=%.2f", coinID, snapshot.Circulating, snapshot.Full, snapshot.TotalVolumeUSD)
}

// fetchFromExchanges tries to fetch price and change from exchanges in order
func (a *Aggregator) fetchFromExchanges(coin models.Coin) (*models.CoinData, error) {
	exchangeSymbols, ok := models.Symbols[coin.ID]
	if !ok {
		return nil, fmt.Errorf("no exchange symbols configured for coin %s", coin.ID)
	}

	var lastErr error

	for _, provider := range a.providers {
		var symbol string
		switch provider.Name() {
		case "binance":
			symbol = exchangeSymbols.Binance
		case "okx":
			symbol = exchangeSymbols.OKX
		case "bybit":
			symbol = exchangeSymbols.Bybit
		case "bitget":
			symbol = exchangeSymbols.Bitget
		}

		if symbol == "" {
			// This exchange doesn't support this coin
			continue
		}

		price, changePct, err := provider.GetPriceAndChange(symbol)
		if err != nil {
			// Check if it's a "not supported" error
			if errors.Is(err, exchanges.ErrSymbolNotSupported) {
				log.Printf("[%s] provider=%s symbol=%s status=not_supported", coin.ID, provider.Name(), symbol)
				continue
			}
			// Other errors, log and continue
			log.Printf("[%s] provider=%s symbol=%s status=failed error=%v", coin.ID, provider.Name(), symbol, err)
			lastErr = err
			continue
		}

		// Success! Construct CoinData from cached supply and fetched price
		log.Printf("[%s] provider=%s symbol=%s price=%.4f change=%.2f%%", coin.ID, provider.Name(), symbol, price, changePct)
		return a.composeCoinData(coin.ID, price, changePct), nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all exchanges failed, last error: %w", lastErr)
	}
	return nil, fmt.Errorf("no supported exchange found for coin %s", coin.ID)
}

// composeCoinData creates CoinData from exchange price + cached supply
func (a *Aggregator) composeCoinData(coinID string, price, changePct float64) *models.CoinData {
	a.mu.RLock()
	snapshot, exists := a.supplies[coinID]
	a.mu.RUnlock()

	now := time.Now()
	data := &models.CoinData{
		Price: models.MultiCurrency{
			USD: price,
		},
		PriceChangePercentage24h: changePct,
		MarketCap: models.MultiCurrency{
			USD: 0,
		},
		FullyDilutedValuation: models.MultiCurrency{
			USD: 0,
		},
		Volume24h: models.MultiCurrency{
			USD: 0,
		},
	}

	if !exists {
		log.Printf("[%s] cache_miss: no cached supply data", coinID)
		return data
	}

	// Use cached supply if valid
	if snapshot.ValidSupply(now) {
		if snapshot.Circulating > 0 {
			data.MarketCap.USD = price * snapshot.Circulating
		}
		if snapshot.Full > 0 {
			data.FullyDilutedValuation.USD = price * snapshot.Full
		}
		log.Printf("[%s] cache_hit supply: mc=%.2f fdv=%.2f", coinID, data.MarketCap.USD, data.FullyDilutedValuation.USD)
	} else {
		log.Printf("[%s] cache_expired supply", coinID)
	}

	// Use cached volume if valid
	if snapshot.ValidVolume(now) {
		data.Volume24h.USD = snapshot.TotalVolumeUSD
		log.Printf("[%s] cache_hit volume: %.2f", coinID, data.Volume24h.USD)
	} else {
		log.Printf("[%s] cache_expired volume", coinID)
	}

	return data
}
