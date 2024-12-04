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

const formatCoinMessage = (coinName, data, fdvRatio) => {
    if (!data) return `${coinName}:\nData unavailable`;
    
    return `${coinName}:
- Price: ${formatPrice(data.price)}
- 24h Change: ${data.price_change_percentage_24h}%
- 24h Volume (USD): ${formatValue(data.volume_24h)}
- Market Cap: ${formatValue(data.marketCap)}
- FDV: ${formatValue(data.fullyDilutedValuation)}
- FDV Ratio: ${(fdvRatio * 100).toFixed(2)}%`;
};

async function updateData() {
    const currentTime = new Date();
    const results = {};

    // Fetch data for all coins in parallel
    const fetchPromises = Object.values(COINS).map(coin => fetchCoinData(coin.id));
    const resultsArray = await Promise.all(fetchPromises);
    resultsArray.forEach((data, index) => {
        results[Object.values(COINS)[index].id] = data;
    });

    // Calculate total FDV
    const totalFDV = Object.values(results)
        .reduce((sum, data) => sum + (data?.fullyDilutedValuation || 0), 0);

    // Create messages with FDV ratios
    const messages = Object.values(COINS)
        .map(coin => {
            const data = results[coin.id];
            const fdvRatio = data?.fullyDilutedValuation ? data.fullyDilutedValuation / totalFDV : 0;
            return formatCoinMessage(coin.name, data, fdvRatio);
        })
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