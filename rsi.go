// Copyright 2021-2026
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	_ "embed"
	"fmt"
	"math"
	"time"

	"github.com/penny-vault/pvbt/asset"
	"github.com/penny-vault/pvbt/data"
	"github.com/penny-vault/pvbt/engine"
	"github.com/penny-vault/pvbt/portfolio"
	"github.com/penny-vault/pvbt/tradecron"
	"github.com/penny-vault/pvbt/universe"
)

//go:embed README.md
var description string

// RSIMeanReversion trades mean reversion signals using a short-term RSI.
// It buys when RSI drops below the buy threshold and sells when RSI rises
// above the sell threshold.
type RSIMeanReversion struct {
	TargetUniverse universe.Universe `pvbt:"target" desc:"ETF or stock to trade" default:"SPY" suggest:"SP500=SPY|QQQ=QQQ"`
	CashTicker     string            `pvbt:"cash-ticker" desc:"Safe-haven asset when not in position" default:"BIL" suggest:"SP500=BIL|QQQ=BIL"`
	RSIPeriod      int               `pvbt:"rsi-period" desc:"RSI lookback period in days" default:"2" suggest:"SP500=2|QQQ=2"`
	BuyThreshold   float64           `pvbt:"buy-threshold" desc:"RSI level below which to buy" default:"10" suggest:"SP500=10|QQQ=10"`
	SellThreshold  float64           `pvbt:"sell-threshold" desc:"RSI level above which to sell" default:"90" suggest:"SP500=90|QQQ=90"`
}

func (s *RSIMeanReversion) Name() string {
	return "RSI Mean Reversion"
}

func (s *RSIMeanReversion) Setup(eng *engine.Engine) {
	tc, err := tradecron.New("@daily", tradecron.MarketHours{Open: 930, Close: 1600})
	if err != nil {
		panic(err)
	}

	eng.Schedule(tc)
	eng.SetBenchmark(eng.Asset("VFINX"))
}

func (s *RSIMeanReversion) Describe() engine.StrategyDescription {
	return engine.StrategyDescription{
		ShortCode:   "rsi",
		Description: description,
		Source:      "",
		Version:     "1.0.0",
		VersionDate: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
	}
}

func (s *RSIMeanReversion) Compute(ctx context.Context, eng *engine.Engine, strategyPortfolio portfolio.Portfolio) error {
	// 1. Fetch enough daily price history for RSI calculation.
	// RSI(2) needs at least 3 price changes plus some warmup for smoothing.
	// Fetch 30 days to be safe.
	priceDF, err := s.TargetUniverse.Window(ctx, portfolio.Days(30), data.MetricClose)
	if err != nil {
		return fmt.Errorf("failed to fetch target prices: %w", err)
	}

	// No downsampling -- we work with daily data directly.
	if priceDF.Len() < s.RSIPeriod+2 {
		return nil
	}

	// 2. Compute RSI for the target asset.
	targetAsset := priceDF.AssetList()[0]
	prices := priceDF.Column(targetAsset, data.MetricClose)

	rsiValue := computeRSI(prices, s.RSIPeriod)

	if math.IsNaN(rsiValue) {
		return nil
	}

	ts := eng.CurrentDate().Unix()

	strategyPortfolio.Annotate(ts, "rsi", fmt.Sprintf("%.2f", rsiValue))

	// 3. Decision logic: buy when oversold, sell when overbought, hold otherwise.
	cashAsset := eng.Asset(s.CashTicker)
	members := make(map[asset.Asset]float64)

	var justification string

	if rsiValue < s.BuyThreshold {
		// Oversold -- buy the target.
		members[targetAsset] = 1.0
		justification = fmt.Sprintf("buy: RSI(%.0f)=%.1f < %.0f", float64(s.RSIPeriod), rsiValue, s.BuyThreshold)
	} else if rsiValue > s.SellThreshold {
		// Overbought -- move to cash.
		members[cashAsset] = 1.0
		justification = fmt.Sprintf("sell: RSI(%.0f)=%.1f > %.0f", float64(s.RSIPeriod), rsiValue, s.SellThreshold)
	} else {
		// Hold current position -- return without rebalancing.
		strategyPortfolio.Annotate(ts, "justification", fmt.Sprintf("hold: RSI(%.0f)=%.1f", float64(s.RSIPeriod), rsiValue))

		return nil
	}

	strategyPortfolio.Annotate(ts, "justification", justification)

	allocation := portfolio.Allocation{
		Date:          eng.CurrentDate(),
		Members:       members,
		Justification: justification,
	}

	if err := strategyPortfolio.RebalanceTo(ctx, allocation); err != nil {
		return fmt.Errorf("rebalance failed: %w", err)
	}

	return nil
}

// computeRSI calculates the Relative Strength Index using Wilder's smoothing.
// It returns the RSI value for the most recent price in the series.
func computeRSI(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return math.NaN()
	}

	// Compute daily price changes.
	changes := make([]float64, len(prices)-1)
	for idx := 1; idx < len(prices); idx++ {
		changes[idx-1] = prices[idx] - prices[idx-1]
	}

	if len(changes) < period {
		return math.NaN()
	}

	// Seed the average gain/loss with the first 'period' changes.
	avgGain := 0.0
	avgLoss := 0.0

	for _, change := range changes[:period] {
		if change > 0 {
			avgGain += change
		} else {
			avgLoss -= change
		}
	}

	avgGain /= float64(period)
	avgLoss /= float64(period)

	// Apply Wilder's smoothing for remaining changes.
	for _, change := range changes[period:] {
		gain := 0.0
		loss := 0.0

		if change > 0 {
			gain = change
		} else {
			loss = -change
		}

		avgGain = (avgGain*float64(period-1) + gain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + loss) / float64(period)
	}

	if avgLoss == 0 {
		return 100.0
	}

	relativeStrength := avgGain / avgLoss

	return 100.0 - 100.0/(1.0+relativeStrength)
}
