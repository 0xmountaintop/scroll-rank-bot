import TelegramBot from 'node-telegram-bot-api';
import axios from 'axios';
import dotenv from 'dotenv';

dotenv.config();

const COINS = {
  starknet: { name: 'Starknet', id: 'starknet' },
  zksync: { name: 'ZkSync', id: 'zksync' },
  taiko: { name: 'Taiko', id: 'taiko' },
  scroll: { name: 'Scroll', id: 'scroll' }
};

const UPDATE_INTERVAL = 5 * 60 * 1000; // 5 minutes in milliseconds

// Create a bot instance
const bot = new TelegramBot(process.env.TELEGRAM_BOT_TOKEN, { polling: true });

let cachedData = null;
let lastFetchTime = null;

const formatValue = (value) => {
    if (!value) return 'N/A';
    if (value >= 1e9) return `${(value / 1e9).toFixed(2)} B`;
    if (value >= 1e6) return `${(value / 1e6).toFixed(2)} M`;
    return value.toLocaleString();
};

const formatPrice = (price) => {
    if (!price) return 'N/A';
    return `$${price.toFixed(4)}`;
};

const fetchCoinData = async (coinId) => {
    try {
        const response = await axios.get(`https://api.coingecko.com/api/v3/coins/${coinId}`);
        const { market_data } = response.data;
        
        return {
            price: market_data.current_price.usd,
            marketCap: market_data.market_cap.usd,
            fullyDilutedValuation: market_data.fully_diluted_valuation?.usd,
            price_change_percentage_24h: market_data.price_change_percentage_24h,
            volume_24h: market_data.total_volume.usd
        };
    } catch (error) {
        console.error(`Error fetching data for ${coinId}:`, error.message);
        return null;
    }
};

const formatCoinMessage = (coinName, data) => {
    if (!data) return `${coinName}:\nData unavailable`;
    
    return `${coinName}:
- Price: ${formatPrice(data.price)}
- 24h Change: ${data.price_change_percentage_24h}%
- 24h Volume (USD): ${formatValue(data.volume_24h)}
- Market Cap: ${formatValue(data.marketCap)}
- FDV: ${formatValue(data.fullyDilutedValuation)}`;
};

async function updateData() {
    const currentTime = new Date();
    const results = {};

    // Fetch data for all coins with rate limiting
    for (const coin of Object.values(COINS)) {
        results[coin.id] = await fetchCoinData(coin.id);
        // await new Promise(resolve => setTimeout(resolve, 1000)); // Rate limiting
    }

    const messages = Object.values(COINS)
        .map(coin => formatCoinMessage(coin.name, results[coin.id]))
        .join('\n\n');

    cachedData = `Date: ${currentTime.toLocaleString()} (UTC)\n\n${messages}`;
    lastFetchTime = currentTime;
    
    console.log('Data updated successfully:', currentTime);
}

// Initialize data and start update interval
updateData();
setInterval(updateData, UPDATE_INTERVAL);

// Bot command handler
bot.onText(/\/check_scroll_ranking/, async (msg) => {
    const chatId = msg.chat.id;
    
    if (cachedData) {
        await bot.sendMessage(chatId, cachedData);
    } else {
        await bot.sendMessage(chatId, 'Sorry, there was an error fetching the data. Please try again later.');
    }
});

// Error handler for the bot
bot.on('error', (error) => {
    console.error('Bot error:', error.message);
});