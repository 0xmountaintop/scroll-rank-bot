# Scroll Rank Bot

A Telegram bot that tracks and compares Layer 2 cryptocurrency prices, market data, and gas fees. The bot provides real-time information about various L2 solutions including Scroll, ZkSync, Starknet, Taiko, and Movement.

## Features

- 📊 Real-time cryptocurrency price tracking
- ⛽ Gas price monitoring across different networks
- 📈 Market statistics including:
  - Current price
  - 24h price changes
  - Trading volume
  - Market cap
  - Fully Diluted Valuation (FDV)
- 🔄 Automatic data updates
- 💬 Easy-to-use Telegram commands

## Commands

- `/rank` - Get current market data for $SCR and its competitors
- `/gas_price` - Get current gas prices across scroll and its competitors' networks

## Environment Variables

The following environment variables need to be set:

```bash
TELEGRAM_BOT_TOKEN=your_telegram_bot_token
OPENAI_API_KEY=your_openai_api_key
```

## Setup

1. Clone the repository

```bash
git clone https://github.com/yourusername/scroll-rank-bot.git
cd scroll-rank-bot
```

2. Install dependencies

```bash
go mod download
```

3. Set up environment variables
```bash
export TELEGRAM_BOT_TOKEN=your_telegram_bot_token
export OPENAI_API_KEY=your_openai_api_key
```

4. Run the bot
```bash
go run main.go
```

## Architecture

The bot is built with the following components:

- `Bot`: Main bot structure handling Telegram interactions
- `CoinGecko Client`: Fetches cryptocurrency market data
- `Gas Price Service`: Monitors gas prices across different networks
- `OpenAI Client`: Handles AI-powered interactions (currently configured to use Deepseek API)

## Data Update Intervals

- Cryptocurrency data: Updates every 5 minutes
- Gas prices: Real-time fetching on request

## Dependencies

- [go-telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api)
- [go-openai](https://github.com/sashabaranov/go-openai)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

[MIT License](LICENSE)
