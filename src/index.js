import TelegramBot from 'node-telegram-bot-api';
import axios from 'axios';
import dotenv from 'dotenv';

dotenv.config();

// Create a new bot instance
const token = process.env.TELEGRAM_BOT_TOKEN;
const bot = new TelegramBot(token, { polling: true });

let cachedData = null; // Variable to store cached data
let lastFetchTime = null; // Variable to store the last fetch time

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
        // Fetch data for each coin
        const starknetResponse = await axios.get('https://api.coingecko.com/api/v3/coins/starknet');
        const zksyncResponse = await axios.get('https://api.coingecko.com/api/v3/coins/zksync');
        const taikoResponse = await axios.get('https://api.coingecko.com/api/v3/coins/taiko');
        const scrollResponse = await axios.get('https://api.coingecko.com/api/v3/coins/scroll');

        // Extract MC and FDV
        const formatValue = (value) => {
            if (value >= 1e9) return `${(value / 1e9).toFixed(2)} B`;
            if (value >= 1e6) return `${(value / 1e6).toFixed(2)} M`;
            return value.toString();
        };

        const starknetMC = formatValue(starknetResponse.data.market_data.market_cap.usd);
        const starknetFDV = formatValue(starknetResponse.data.market_data.fully_diluted_valuation.usd);

        const zksyncMC = formatValue(zksyncResponse.data.market_data.market_cap.usd);
        const zksyncFDV = formatValue(zksyncResponse.data.market_data.fully_diluted_valuation.usd);

        const taikoMC = formatValue(taikoResponse.data.market_data.market_cap.usd);
        const taikoFDV = formatValue(taikoResponse.data.market_data.fully_diluted_valuation.usd);

        const scrollMC = formatValue(scrollResponse.data.market_data.market_cap.usd);
        const scrollFDV = formatValue(scrollResponse.data.market_data.fully_diluted_valuation.usd);

        // Send the response back to the user
        const responseMessage = `
        Starknet:
- Market Cap: ${starknetMC}
- Fully Diluted Valuation: ${starknetFDV}

Zksync:
- Market Cap: ${zksyncMC}
- Fully Diluted Valuation: ${zksyncFDV}

Taiko:
- Market Cap: ${taikoMC}
- Fully Diluted Valuation: ${taikoFDV}

Scroll:
- Market Cap: ${scrollMC}
- Fully Diluted Valuation: ${scrollFDV}
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