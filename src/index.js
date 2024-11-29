import TelegramBot from 'node-telegram-bot-api';
import axios from 'axios';
import dotenv from 'dotenv';

dotenv.config();

// Create a new bot instance
const token = process.env.TELEGRAM_BOT_TOKEN;
const bot = new TelegramBot(token, { polling: true });

let cachedData = null; // Variable to store cached data
let lastFetchTime = null; // Variable to store the last fetch time

const formatValue = (value) => {
    if (value >= 1e9) return `${(value / 1e9).toFixed(2)} B`;
    if (value >= 1e6) return `${(value / 1e6).toFixed(2)} M`;
    return value.toString();
};

// Function to fetch coin data
const fetchCoinData = async (coin) => {
    const response = await axios.get(`https://api.coingecko.com/api/v3/coins/${coin}`);
    return {
        marketCap: response.data.market_data.market_cap.usd,
        fullyDilutedValuation: response.data.market_data.fully_diluted_valuation.usd,
    };
};

// Function to get formatted values
const getFormattedValues = async (coins) => {
    const results = {};
    for (const coin of coins) {
        const data = await fetchCoinData(coin);
        results[coin] = {
            marketCap: formatValue(data.marketCap),
            fullyDilutedValuation: formatValue(data.fullyDilutedValuation),
        };
    }
    return results;
};

// Listen for the command
bot.onText(/\/check_scroll_ranking/, async (msg) => {
    const chatId = msg.chat.id;

    // Check if cached data is available and if it's within the last hour
    const currentTime = new Date();
    if (cachedData && lastFetchTime && (currentTime - lastFetchTime) < 3600000) {
        // Send the cached response back to the user
        bot.sendMessage(chatId, cachedData);
        return;
    }

    try {
        const coins = ['starknet', 'zksync', 'taiko', 'scroll'];
        const coinData = await getFormattedValues(coins);

        // Send the response back to the user
        const responseMessage = `
        Starknet:
- Market Cap: ${coinData.starknet.marketCap}
- Fully Diluted Valuation: ${coinData.starknet.fullyDilutedValuation}

Zksync:
- Market Cap: ${coinData.zksync.marketCap}
- Fully Diluted Valuation: ${coinData.zksync.fullyDilutedValuation}

Taiko:
- Market Cap: ${coinData.taiko.marketCap}
- Fully Diluted Valuation: ${coinData.taiko.fullyDilutedValuation}

Scroll:
- Market Cap: ${coinData.scroll.marketCap}
- Fully Diluted Valuation: ${coinData.scroll.fullyDilutedValuation}
        `;

        // Cache the result and update the last fetch time
        cachedData = responseMessage;
        lastFetchTime = currentTime;

        bot.sendMessage(chatId, responseMessage);
    } catch (error) {
        console.error('Error fetching data:', error);
        bot.sendMessage(chatId, 'Sorry, there was an error fetching the data.');
    }
});