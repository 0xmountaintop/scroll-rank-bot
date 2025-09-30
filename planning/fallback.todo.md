# Fallback 方案 TODO（CoinGecko 优先，失败依次回退 Binance → OKX → Bybit → Bitget）

本文件为实施计划与可勾选清单，覆盖架构、数据模型、接口、回退链路、日志与测试。完成所有勾选项后再提交变更。

---

## 目标与约束

- [ ] 目标：减少 CoinGecko 触发 rate limit 的概率，保证数据可用性与一致性。
- [ ] 行为：
  - [ ] 首选从 CoinGecko 获取完整数据并缓存：
    - [ ] circulating_supply = market_cap / current_price（若 current_price 为 0 则跳过该缓存项）
    - [ ] full_supply = fully_diluted_valuation / current_price（若 current_price 为 0 则跳过该缓存项）
    - [ ] total_volume（USD）直接缓存
  - [ ] 若 CoinGecko 不可用：从交易所按顺序（Binance → OKX → Bybit → Bitget）获取 current_price 和 price_change_percentage_24h；
    - [ ] 用缓存的 circulating_supply 计算 market_cap
    - [ ] 用缓存的 full_supply 计算 fully_diluted_valuation
    - [ ] total_volume 使用缓存的 total_volume（过期或不存在则置 0）
  - [ ] 若所有交易所均失败：该币本轮显示为 Data unavailable（保留上一轮已生成的总体消息文本，不中断 bot）。
- [ ] 约束：不改变命令与输出格式，不扩大外部依赖范围；默认只用公共 REST 接口，无需 API Key。

---

## 架构与数据流

- [ ] 新增 MarketDataAggregator 作为单一入口，负责：
  - [ ] 调用 CoinGecko（主路径）并更新供应/成交量缓存
  - [ ] 失败时按顺序调用各交易所 Provider 获取价格 + 24h 涨跌
  - [ ] 组装 `models.CoinData` 返回给 bot
- [ ] 并发策略：保持现有每币并发获取，但对每个外部来源加入轻量并发上限与超时。
- [ ] 失败策略：
  - [ ] 对 CoinGecko 出现 429/超时等错误时，立刻走回退链路；
  - [ ] 可选：简单熔断（例如 5 分钟内多次失败则临时跳过 CG）。

---

## 代码结构与文件清单

- [ ] `internal/market/aggregator.go`
  - [ ] 聚合器结构体、构造器与 `FetchCoinData(coin models.Coin)` 主逻辑
- [ ] `internal/exchanges/provider.go`
  - [ ] Provider 接口、通用错误与 HTTP 工具（含 UA、超时）
- [ ] `internal/exchanges/binance.go`
- [ ] `internal/exchanges/okx.go`
- [ ] `internal/exchanges/bybit.go`
- [ ] `internal/exchanges/bitget.go`
- [ ] `internal/models/supply.go`
  - [ ] 供应与成交量快照结构体、TTL 常量与校验函数
- [ ] `internal/models/symbols.go`
  - [ ] 交易所交易对映射与加载逻辑（默认内置，可被 .env/JSON 覆盖）
- [ ] 变更 `internal/bot/bot.go`
  - [ ] 使用 Aggregator 替换直接调用 CoinGecko 的位置

---

## 数据模型与缓存设计

- [ ] 在 `internal/models/supply.go` 定义：
  - [ ] `type SupplySnapshot struct { Circulating float64; Full float64; TotalVolumeUSD float64; UpdatedAt time.Time }`
  - [ ] TTL 策略：
    - [ ] SUPPLY_TTL = 24h（Circulating/Full）
    - [ ] VOLUME_TTL = 30m（TotalVolumeUSD）
  - [ ] 校验方法：`ValidSupply(now time.Time) bool`、`ValidVolume(now time.Time) bool`
- [ ] 在 `internal/market/aggregator.go` 中维护：
  - [ ] `supplies map[string]SupplySnapshot`
  - [ ] `mu sync.RWMutex` 保护并发访问
  - [ ] `supplyTTL time.Duration`、`volumeTTL time.Duration`
- [ ] 缓存更新：
  - [ ] 来自 CG 的数据成功时计算并更新 `Circulating`、`Full`、`TotalVolumeUSD`
  - [ ] `current_price == 0` 时跳过供应计算避免除零
- [ ] 缓存读取（回退路径）：
  - [ ] 读取并校验 TTL，过期则返回 0（对应 UI 将显示 N/A）

---

## Provider 接口与错误

- [ ] 在 `internal/exchanges/provider.go` 定义：
  - [ ] `type Provider interface { Name() string; GetPriceAndChange(symbol string) (price float64, changePct24h float64, err error) }`
  - [ ] 错误：
    - [ ] `ErrSymbolNotSupported`（该交易所无此交易对）
    - [ ] `ErrRateLimited`（HTTP 429 或明确的限频返回）
    - [ ] 其它错误统一包装并包含 `provider` 与 `symbol` 便于日志定位
  - [ ] 公共 HTTP 客户端：
    - [ ] `http.Client` 带 5–10s 超时
    - [ ] 统一 `User-Agent`
    - [ ] JSON 解析工具与错误封装

---

## 各交易所 Provider 细节（现均支持公开 REST，无需鉴权）

- [ ] Binance（优先）
  - [ ] 接口：`GET /api/v3/ticker/24hr?symbol=SYMBOL`
  - [ ] 字段：`lastPrice`（价格），`priceChangePercent`（24h 涨跌百分比，字符串，需解析为 float）
  - [ ] 注意：429 映射为 `ErrRateLimited`；HTTP 非 2xx 统一为错误

- [ ] OKX（第二）
  - [ ] 接口：`GET /api/v5/market/ticker?instId=BASE-USDT`（现货）
  - [ ] 字段：`data[0].last`（价格），`data[0].open24h`（24h 开盘价）
  - [ ] 计算：`changePct24h = (last/open24h - 1) * 100`

- [ ] Bybit（第三）
  - [ ] 接口：`GET /v5/market/tickers?category=spot&symbol=SYMBOL`
  - [ ] 字段：`result.list[0].lastPrice`（价格），`result.list[0].price24hPcnt`（小数比例，乘 100 转百分比）

- [ ] Bitget（第四）
  - [ ] 接口（其一）：`GET /api/spot/v1/market/ticker?symbol=SYMBOL`
  - [ ] 字段：`data.close`（价格），`data.open`（开盘）
  - [ ] 计算：`changePct24h = (close/open - 1) * 100`

---

## 交易对映射（Symbol Mapping）

- [ ] 在 `internal/models/symbols.go`：
  - [ ] `type ExchangeSymbols struct { Binance string; OKX string; Bybit string; Bitget string }`
  - [ ] `var Symbols map[string]ExchangeSymbols` // key 为 `models.Coin.ID`
  - [ ] 提供加载覆盖：
    - [ ] 支持从 `.env` 指定的 JSON 文件路径（如 `SYMBOLS_JSON`）加载映射并覆盖默认值
    - [ ] 若缺失或为空则使用内置默认映射
  - [ ] TODO：为当前币种补齐真实交易对（需人工确认）：
    - [ ] starknet（ID: starknet）
    - [ ] zksync（ID: zksync）
    - [ ] taiko（ID: taiko）
    - [ ] scroll（ID: scroll）
    - [ ] movement（ID: movement）
    - [ ] polyhedra-network（ID: polyhedra-network）
    - [ ] linea（ID: linea）
  - [ ] 若某交易所无该交易对：对应字段置空，Provider 返回 `ErrSymbolNotSupported`，Aggregator 继续尝试下一个

---

## Aggregator 流程（核心）

- [ ] `FetchCoinData(coin models.Coin) (*models.CoinData, error)`：
  - [ ] 1) CoinGecko 主路径：
    - [ ] 请求 CG（保留现实现：逐币 `/api/v3/coins/{id}`）；
    - [ ] 可选优化（后续）：使用 `/api/v3/coins/markets?vs_currency=usd&ids=...&price_change_percentage=24h` 批量获取，降低 CG 压力；
    - [ ] 解析成功：计算 `circulating_supply`、`full_supply`、缓存 `total_volume`，并立即返回 CG 数据；
    - [ ] 解析失败或 429/超时：跳转回退链路
  - [ ] 2) 回退链路：
    - [ ] 基于 `Symbols` 为该币依次尝试 Provider；
    - [ ] 某 Provider 返回价格与 24h 涨跌成功：
      - [ ] 读取缓存（若 TTL 过期，返回 0）；
      - [ ] 组装 `models.CoinData`：
        - [ ] `Price.USD = price`
        - [ ] `PriceChangePercentage24h = changePct24h`
        - [ ] `MarketCap.USD = price * Circulating (若>0，否则 0)`
        - [ ] `FullyDilutedValuation.USD = price * Full (若>0，否则 0)`
        - [ ] `Volume24h.USD = TotalVolumeUSD (若有效，否则 0)`
      - [ ] 返回该数据
    - [ ] 若所有 Provider 均失败：返回错误（调用方记录为 Data unavailable）
- [ ] 并发与限速：
  - [ ] 为各 Provider 设置一个小并发上限（如 3–5）以避免并发拥挤
  - [ ] 对 CG 出现连续失败时设置简单熔断窗口（如 5–10 分钟）

---

## Bot 接入与排序

- [ ] 新增 `aggregator *market.Aggregator` 到 `internal/bot/bot.go` 的 `Bot` 结构体
- [ ] 在 `New(...)` 中构造 Aggregator，注入 Provider 链与 TTL 配置
- [ ] 在 `updateCoinData()` 中将 `b.coingecko.FetchCoinData(coin.ID)` 替换为 `b.aggregator.FetchCoinData(coin)`
- [ ] 排序仍按 FDV 降序；当 FDV=0 时自然靠后
- [ ] 输出格式与现有 `formatSingleCoin` 一致（价格为 `$` 格式，`N/A` 规则不变）

---

## 日志与可观测性

- [ ] 统一日志：包含 `coin_id`、`source`（coingecko/binance/okx/bybit/bitget）、`latency_ms`、`status`
- [ ] 对 429/5xx 记录告警级别日志，便于追踪限频与稳定性
- [ ] 记录缓存命中与失效（supply/volume）

---

## 配置与环境变量

- [ ] `.env.example` 新增：
  - [ ] `SYMBOLS_JSON`（可选）：交易对映射 JSON 文件路径
  - [ ] `HTTP_TIMEOUT_SECONDS`（可选）：默认 10
  - [ ] `SUPPLY_TTL_HOURS`（可选）：默认 24
  - [ ] `VOLUME_TTL_MINUTES`（可选）：默认 30
- [ ] 为 HTTP 客户端设置 `User-Agent: scroll-rank-bot/<version>`

---

## 测试计划（优先单元测试，其次手动）

- [ ] 解析测试：
  - [ ] Binance 24hr JSON 解析到 price、changePct
  - [ ] OKX ticker JSON 解析与涨跌计算
  - [ ] Bybit tickers JSON 解析与涨跌计算
  - [ ] Bitget ticker JSON 解析与涨跌计算
- [ ] 缓存测试：
  - [ ] CG 成功后正确计算/缓存 supply 与 volume
  - [ ] TTL 到期后失效策略正确（返回 0）
  - [ ] price==0 时不更新 supply（避免除零）
- [ ] 聚合器流程：
  - [ ] CG 成功直返路径
  - [ ] CG 失败 → Binance 成功路径
  - [ ] CG 失败 → Binance 失败 → OKX 成功路径
  - [ ] 全部失败路径（返回错误，bot 标记 Data unavailable）
- [ ] 并发与稳定性：
  - [ ] 多币种并发下各 Provider 并发限制生效
  - [ ] CG 连续失败时熔断策略生效（如启用）
- [ ] Bot 集成：
  - [ ] `/rank` 输出字段与格式未回归；FDV 排序正确

---

## 手动验证清单

- [ ] 准备真实交易对映射（先本地覆盖 JSON，不提交）
- [ ] 正常网络下运行：观察 CG 命中、缓存更新、输出正确
- [ ] 人为阻断 CG（hosts 或防火墙）：观察 Binance/OKX/Bybit/Bitget 回退是否成功
- [ ] 人为阻断 Binance：验证继续回退 OKX/Bybit/Bitget
- [ ] 清空/过期缓存：验证 MC/FDV/Volume 取值为 0 → UI 显示 N/A

---

## 发布与回滚

- [ ] 分阶段开关：
  - [ ] 阶段 1：引入 Aggregator 但仍默认走 CG（功能等价）
  - [ ] 阶段 2：启用回退链路（feature flag，可通过环境变量控制）
  - [ ] 阶段 3（可选）：CG 改为批量接口 `/coins/markets` 进一步减负
- [ ] 回滚策略：
  - [ ] 一键关闭回退链路，退回 CG 单一路径

---

## 备注与风险

- [ ] 交易对映射需要人工确认，不要猜测（尤其是新币/新上所）
- [ ] 各交易所字段语义略有差异，注意百分比单位（绝对百分比 vs 小数）
- [ ] 当 CG 获取失败 + 缓存缺失时，MC/FDV/Volume 会为 0（N/A），排序可能变化（符合预期）

---

## 最终完成判定（Definition of Done）

- [ ] 在 CG 可用时：输出与当前版本等价或更好，且建立了缓存
- [ ] 在 CG 不可用时：至少一个交易所可回退且 MC/FDV 基于缓存计算
- [ ] 所有单元测试通过；手动验证通过
- [ ] `.env.example`、README（如需）已更新

