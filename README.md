# RSI Mean Reversion

The **RSI Mean Reversion** strategy is based on the work of Larry Connors and Cesar Alvarez. It uses a very short-term RSI (2-period) to identify oversold and overbought conditions in broad market indices or large-cap ETFs, then trades the mean reversion.

## Rules

1. Every trading day, compute the 2-period RSI for the target asset (default: SPY).
2. **Buy signal**: If RSI(2) drops below the buy threshold (default: 10), allocate 100% to the target asset.
3. **Sell signal**: If RSI(2) rises above the sell threshold (default: 90), allocate 100% to cash (BIL).
4. **Hold**: If neither threshold is crossed, maintain the current position.
5. Rebalance daily.

RSI(2) is computed using Wilder's smoothing method:
- For each day, compute the price change from the prior day
- Separate into gains (positive changes) and losses (absolute negative changes)
- Compute the average gain and average loss over the lookback period using exponential smoothing
- RSI = 100 - 100 / (1 + avgGain/avgLoss)

The 2-period RSI is extremely sensitive to short-term price movements, making it effective for mean reversion signals on liquid, large-cap assets.

## Parameters

- **Target**: The ETF or stock to trade (default: SPY)
- **Cash Ticker**: Safe-haven asset when not in position (default: BIL)
- **RSI Period**: Lookback period for RSI calculation (default: 2)
- **Buy Threshold**: RSI level below which to buy (default: 10)
- **Sell Threshold**: RSI level above which to sell (default: 90)

## References

- Connors, L. and Alvarez, C. (2009). *Short Term Trading Strategies That Work*. TradingMarkets.
