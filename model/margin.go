package model

import "time"

type MarginAvailability struct {
	CurrencyBTC struct {
		Available     string `json:"available"`
		Leverage      string `json:"leverage"`
		LeverageRatio string `json:"leverage_ratio"`
		Rate          string `json:"rate"`
	} `json:"currency:BTC"`
	CurrencyUSDT struct {
		Available     string `json:"available"`
		Leverage      string `json:"leverage"`
		LeverageRatio string `json:"leverage_ratio"`
		Rate          string `json:"rate"`
	} `json:"currency:USDT"`
	InstrumentID string `json:"instrument_id"`
	ProductID    string `json:"product_id"`
}

type MarginLeverage struct {
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
	InstrumentID string `json:"instrument_id"`
	Leverage     string `json:"leverage"`
	Result       bool   `json:"result"`
}

type MarginOrder struct {
	ClientOid    string `json:"client_oid"`
	ErrorMessage string `json:"error_message"`
	ErrorCode    string `json:"error_code"`
	OrderID      string `json:"order_id"`
	Result       bool   `json:"result"`
}

type MarginBorrow struct {
	BorrowID  string `json:"borrow_id"`
	ClientOid string `json:"client_oid"`
	Result    bool   `json:"result"`
}

type MarginBRepay struct {
	ClientOid   string `json:"client_oid"`
	RepaymentID string `json:"repayment_id"`
	Result      bool   `json:"result"`
}

type MarginMarkPrice struct {
	MarkPrice    string    `json:"mark_price"`
	InstrumentID string    `json:"instrument_id"`
	Timestamp    time.Time `json:"timestamp"`
}

type MarginAccount struct {
	CurrencyBTC struct {
		Available   string `json:"available"`
		Balance     string `json:"balance"`
		Borrowed    string `json:"borrowed"`
		CanWithdraw string `json:"can_withdraw"`
		Frozen      string `json:"frozen"`
		Hold        string `json:"hold"`
		Holds       string `json:"holds"`
		LendingFee  string `json:"lending_fee"`
	} `json:"currency:BTC"`
	CurrencyUSDT struct {
		Available   string `json:"available"`
		Balance     string `json:"balance"`
		Borrowed    string `json:"borrowed"`
		CanWithdraw string `json:"can_withdraw"`
		Frozen      string `json:"frozen"`
		Hold        string `json:"hold"`
		Holds       string `json:"holds"`
		LendingFee  string `json:"lending_fee"`
	} `json:"currency:USDT"`
	LiquidationPrice string `json:"liquidation_price"`
	MaintMarginRatio string `json:"maint_margin_ratio"`
	MarginRatio      string `json:"margin_ratio"`
	RiskRate         string `json:"risk_rate"`
	Tiers            string `json:"tiers"`
}

type MarginOrderDetail struct {
	ClientOid      string    `json:"client_oid"`
	CreatedAt      time.Time `json:"created_at"`
	FilledNotional string    `json:"filled_notional"`
	FilledSize     string    `json:"filled_size"`
	Funds          string    `json:"funds"`
	InstrumentID   string    `json:"instrument_id"`
	Notional       string    `json:"notional"`
	OrderID        string    `json:"order_id"`
	OrderType      string    `json:"order_type"`
	Price          string    `json:"price"`
	PriceAvg       string    `json:"price_avg"`
	ProductID      string    `json:"product_id"`
	Side           string    `json:"side"`
	Size           string    `json:"size"`
	Status         string    `json:"status"`
	State          string    `json:"state"`
	Timestamp      time.Time `json:"timestamp"`
	Type           string    `json:"type"`
}
