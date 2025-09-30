package models

import (
	"encoding/json"
	"log"
	"os"
)

// ExchangeSymbols holds trading pair symbols for a coin across different exchanges
type ExchangeSymbols struct {
	Binance string `json:"binance"`
	OKX     string `json:"okx"`
	Bybit   string `json:"bybit"`
	Bitget  string `json:"bitget"`
}

// Symbols maps coin IDs to their exchange-specific trading symbols
// Key is models.Coin.ID
var Symbols = map[string]ExchangeSymbols{
	"starknet": {
		Binance: "STRKUSDT",
		OKX:     "STRK-USDT",
		Bybit:   "STRKUSDT",
		Bitget:  "STRKUSDT",
	},
	"zksync": {
		Binance: "ZKUSDT",
		OKX:     "ZK-USDT",
		Bybit:   "ZKUSDT",
		Bitget:  "ZKUSDT",
	},
	"taiko": {
		Binance: "TAIKOUSDT",
		OKX:     "TAIKO-USDT",
		Bybit:   "TAIKOUSDT",
		Bitget:  "TAIKOUSDT",
	},
	"scroll": {
		Binance: "SCRUSDT",
		OKX:     "SCR-USDT",
		Bybit:   "SCRUSDT",
		Bitget:  "SCRUSDT",
	},
	"movement": {
		Binance: "MOVEUSDT",
		OKX:     "MOVE-USDT",
		Bybit:   "MOVEUSDT",
		Bitget:  "MOVEUSDT",
	},
	"polyhedra-network": {
		Binance: "ZKJUSDT",
		OKX:     "ZKJ-USDT",
		Bybit:   "ZKJUSDT",
		Bitget:  "ZKJUSDT",
	},
	"linea": {
		Binance: "LINEAUSDT",
		OKX:     "LINEA-USDT",
		Bybit:   "LINEAUSDT",
		Bitget:  "LINEAUSDT",
	},
}

// LoadSymbolsFromJSON loads symbol mappings from a JSON file and merges with defaults
// If the file doesn't exist or is empty, uses default mappings
func LoadSymbolsFromJSON(filePath string) error {
	if filePath == "" {
		log.Println("No custom symbols file specified, using defaults")
		return nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Symbols file %s not found, using defaults", filePath)
			return nil
		}
		return err
	}

	var customSymbols map[string]ExchangeSymbols
	if err := json.Unmarshal(data, &customSymbols); err != nil {
		return err
	}

	// Merge custom symbols with defaults
	for coinID, exchangeSymbols := range customSymbols {
		Symbols[coinID] = exchangeSymbols
	}

	log.Printf("Loaded custom symbols from %s", filePath)
	return nil
}
