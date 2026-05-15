package strategy

import "time"

// Candle is a normalized market candle for built-in strategies.
type Candle struct {
	InstrumentUID string
	Open          float64
	High          float64
	Low           float64
	Close         float64
	Volume        int64
	Time          time.Time
	IsComplete    bool
}

// Orderbook is a normalized order book snapshot.
type Orderbook struct {
	InstrumentUID string
	Bids          []BookLevel
	Asks          []BookLevel
	Time          time.Time
}

// BookLevel is one bid or ask row.
type BookLevel struct {
	Price    float64
	Quantity int64
}

// Signal is a trading intent produced by a strategy.
type Signal struct {
	InstrumentUID string
	Direction     string // "buy" or "sell"
	Quantity      int64
	Price         float64 // 0 = market (unless order_type says otherwise)
	OrderType     string  // "market" or "limit"
	Reason        string
	// CandleTime is the bar time that produced this signal (for journal / UI).
	// Zero means the journal may fall back to wall-clock time.
	CandleTime time.Time
}

// ExecutionEvent is feedback from the execution layer (fills, cancels, etc.).
type ExecutionEvent struct {
	OrderID       string
	InstrumentUID string
	Status        string // filled, partially_filled, cancelled, rejected, new, ...
	FilledQty     int64
	AvgPrice      float64
	Message       string
}

// StrategyInfo describes a strategy for management APIs.
type StrategyInfo struct {
	ID          string
	Name        string
	Version     string
	Description string
	Author      string
}
