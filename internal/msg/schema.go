package msg

// TickMsg represents a market data tick message
type TickMsg struct {
	EventID       string  `json:"event_id"`
	Symbol        string  `json:"symbol"`
	Price         float64 `json:"price"`
	TsUnixMillis  int64   `json:"ts_unix_millis"`
}

// OrderCmdMsg represents an order command message
type OrderCmdMsg struct {
	EventID       string `json:"event_id"`
	OrderID       string `json:"order_id"`
	Symbol        string `json:"symbol"`
	Side          string `json:"side"` // "BUY" or "SELL"
	Qty           int64  `json:"qty"`
	Price         float64 `json:"price"`
	TsUnixMillis  int64   `json:"ts_unix_millis"`
}

// OrderEventMsg represents an order event message
type OrderEventMsg struct {
	EventID       string `json:"event_id"`
	OrderID       string `json:"order_id"`
	Status        string `json:"status"` // "ACCEPTED", "REJECTED", "EXECUTED"
	Reason        string `json:"reason"`
	TsUnixMillis  int64   `json:"ts_unix_millis"`
}

