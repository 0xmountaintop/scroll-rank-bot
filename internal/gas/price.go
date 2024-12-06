package gas

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"telegrambot/internal/models"
)

type PriceService struct {
	httpClient *http.Client
	endpoints  map[string]string
}

func NewPriceService() *PriceService {
	return &PriceService{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		endpoints: map[string]string{
			"ethereum": "https://rpc.mevblocker.io",
			"zksync":   "https://mainnet.era.zksync.io",
			"taiko":    "https://rpc.mainnet.taiko.xyz",
			"scroll":   "https://rpc.scroll.io",
		},
	}
}

func (s *PriceService) FetchAllPrices() map[string]float64 {
	results := make(chan struct {
		network string
		price   float64
	}, len(s.endpoints))

	var wg sync.WaitGroup
	for network, endpoint := range s.endpoints {
		wg.Add(1)
		go s.fetchSinglePrice(network, endpoint, results, &wg)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	prices := make(map[string]float64)
	for result := range results {
		prices[result.network] = result.price
	}

	return prices
}

func (s *PriceService) fetchSinglePrice(network, endpoint string, results chan<- struct {
	network string
	price   float64
}, wg *sync.WaitGroup) {
	defer wg.Done()

	reqBody := models.RPCRequest{
		JsonRPC: "2.0",
		Method:  "eth_gasPrice",
		Params:  []string{},
		ID:      1,
	}

	jsonData, _ := json.Marshal(reqBody)
	resp, err := s.httpClient.Post(endpoint, "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		results <- struct {
			network string
			price   float64
		}{network: network, price: 0}
		return
	}
	defer resp.Body.Close()

	var result struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		results <- struct {
			network string
			price   float64
		}{network: network, price: 0}
		return
	}

	hexValue := strings.TrimPrefix(result.Result, "0x")
	intValue, _ := strconv.ParseInt(hexValue, 16, 64)
	gweiPrice := float64(intValue) / 1e9

	results <- struct {
		network string
		price   float64
	}{network: network, price: gweiPrice}
}
