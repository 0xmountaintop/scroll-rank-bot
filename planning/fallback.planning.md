我现在这种 query coingecko 的方式会导致 rate limit

我想改成
1. 如果能拿到 coingecko 数据，缓存
1.1 circulated_supply = market_cap / current_price
1.2 full_supply = fully_diluted_valuation / current_price
1.3 total_volume

2. 如果拿不到，用 bianace spot api 获取
current_price 和 price_change_percentage_24h
然后 用缓存的 circulated_supply 计算 market_cap， 用缓存的 full_supply 计算 fully_diluted_valuation
total_volume 就直接使用缓存的 total_volume

3. 如果  bianace spot api 获取不到就 fallback 到 okx，再 fallback 到 bybit ，再 fallback 到 bitget， 


you will read and understand the codebase first, then give me a detailed plan.

no need to start implementing.

---


> Here’s the plan to add caching and multi-exchange fallback without changing your bot’s external behavior.

What You Have

- updateCoinData concurrently fetches each coin from coingecko.Client.FetchCoinData and builds a ranked message by FDV.
- models.CoinData carries everything the bot needs: Price, PriceChangePercentage24h, MarketCap, FullyDilutedValuation, Volume24h.
- No caching beyond the preformatted message; no alternative data sources.

Target Behavior

- Primary: Use CoinGecko. On success, cache:
    - circulated_supply = market_cap / current_price
    - full_supply = fully_diluted_valuation / current_price
    - total_volume = coingecko total_volume (USD)
- If CoinGecko fails: fetch current_price and price_change_percentage_24h from exchanges in order:
    - Binance → OKX → Bybit → Bitget
    - Then reconstruct market_cap and fdv from cached circulated_supply and full_supply; carry cached total_volume.
- If all sources fail: keep prior displayed message; coin becomes “Data unavailable” for the current cycle.

Architecture Changes

- Add a “MarketDataAggregator” that encapsulates:
    - A supply/volume cache per coin ID
    - A chain of exchange providers to fetch price + 24h change
    - Primary CoinGecko fetch and cache update
- Keep internal/bot/bot.go logic mostly intact, but replace direct CoinGecko calls with aggregator calls returning *models.CoinData.

Data Model & Caching

- Add internal/models/supply.go:
    - SupplyCache:
        - coinID string
        - Circulating float64
        - Full float64
        - TotalVolumeUSD float64
        - UpdatedAt time.Time
    - TTLs:
        - supply: 24h (safe default; tune as needed)
        - total_volume: 30m (SAFETY: keeps volume reasonably fresh)
- Add internal/market/aggregator.go:
    - Fields: coingecko *coingecko.Client, providers []Provider, supplies map[string]SupplyCache, mu sync.RWMutex, supplyTTL, volumeTTL.
    - Method: FetchCoinData(coin models.Coin) (*models.CoinData, error)
        - Try CoinGecko:
            - On success: compute and cache circulating/full supply and total_volume; return CoinData direct from CoinGecko.
        - On fail: iterate providers in order to fetch price + 24h change.
            - Compose CoinData:
                - Price.USD = price
                - PriceChangePercentage24h = change
                - MarketCap.USD = price * cachedCirculating (if cached, else 0)
                - FullyDilutedValuation.USD = price * cachedFull (if cached, else 0)
                - Volume24h.USD = cachedVolume (if cached + not expired, else 0)
- Optional: Cache last-good CoinData to smooth out short outages. The UI already uses last formatted message, so this is optional.

Exchange Providers

- Define common interface in internal/exchanges/provider.go:
    - type Provider interface { Name() string; GetPriceAndChange(symbol string) (price float64, changePct24h float64, err error) }
- Implement providers:
    - internal/exchanges/binance.go
        - Endpoint: /api/v3/ticker/24hr?symbol=SYMBOL → fields: lastPrice, priceChangePercent
    - internal/exchanges/okx.go
        - Endpoint: /api/v5/market/ticker?instId=BASE-USDT
        - Use last and open24h to compute changePct24h = (last/open24h - 1) * 100
    - internal/exchanges/bybit.go
        - Endpoint: /v5/market/tickers?category=spot&symbol=SYMBOL
        - Use lastPrice, price24hPcnt (multiply by 100)
    - internal/exchanges/bitget.go
        - Endpoint: /api/spot/v1/market/ticker?symbol=SYMBOL (or tickers list; choose single-symbol endpoint)
        - Use close (last) and open to compute changePct24h
- All use the same shared http.Client with a 5–10s timeout; add User-Agent.

Symbol Mapping

- Add internal/models/symbols.go:
    - Per coin ID, define symbols per exchange, e.g.:
        - Binance: STRKUSDT
        - OKX: STRK-USDT
        - Bybit: STRKUSDT
        - Bitget: STRKUSDT
    - Not all coins are on all exchanges. If missing, provider returns a “not supported” error and the aggregator advances to the next provider.
- Make the map overrideable via .env or a JSON config (e.g., SYMBOLS_JSON) so you can adjust without redeploy.

Orchestration & Rate-Limit Safety

- Aggregator call path per coin:
    - Try CoinGecko (single request). On 429/timeout → fallback chain.
    - Fallback chain tries providers in order; stop on first success.
- Concurrency:
    - Keep existing per-coin concurrency, but add per-provider request pacing if needed:
        - Simple option: small per-provider semaphore (limit 3–5 concurrent) to keep within exchange limits.
- Optional future improvement (Coingecko batching):
    - Add FetchMarkets(ids []string) via /coins/markets?vs_currency=usd&ids=...&price_change_percentage=24h to fetch all coins in one request, reducing CoinGecko load dramatically. Not required for this change, but highly recommended.

Bot Wiring

- In internal/bot/bot.go:
    - Add aggregator *market.Aggregator to Bot.
    - Initialize in New(...) with provider chain [Binance, OKX, Bybit, Bitget] and TTLs.
    - Replace b.coingecko.FetchCoinData(coin.ID) with b.aggregator.FetchCoinData(coin).
    - Sorting by FDV remains unchanged; if FDV=0 due to missing supply cache, coin naturally sinks in ranking.
    - Continue to build cachedCoinDataRespMsg and serve /rank from cache.

Error Handling & Edge Cases

- Division safety for supply computations: if price is 0, skip cache update for supply.
- If supply cache missing or expired and CoinGecko fails:
    - Return coin with price + change only; MC/FDV/Volume become 0 so UI shows “N/A”.
- Log source of data per coin for debugging: source=coingecko|binance|okx|bybit|bitget.
- All HTTP errors and JSON decode errors should be wrapped with provider names for quick diagnosis.

Validation Plan

- Unit tests for:
    - Supply calculations and cache TTL expiry.
    - Provider parsers with sample JSON payloads (especially OKX, Bitget calculation of 24h%).
    - Aggregator flow: CG success, CG fail→Binance success, full chain miss.
- Manual run:
    - Fill symbol mappings for the coins you track.
    - Verify formatted output remains stable; confirm MC/FDV recompute on fallback paths.
    - Temporarily block CoinGecko to exercise fallback; then block Binance to see OKX/Bybit/Bitget steps.