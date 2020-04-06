package model

import "time"

type SwapDepth struct {
	Table  string `json:"table"`
	Action string `json:"action"`
	Data   []struct {
		InstrumentID string     `json:"instrument_id"`
		Asks         [][]string `json:"asks"`
		Bids         [][]string `json:"bids"`
		Timestamp    time.Time  `json:"timestamp"`
		Checksum     int        `json:"checksum"`
	} `json:"data"`
}

type SwapAccount struct {
	Info struct {
		Equity            string    `json:"equity"`
		FixedBalance      string    `json:"fixed_balance"`
		InstrumentID      string    `json:"instrument_id"`
		MaintMarginRatio  string    `json:"maint_margin_ratio"`
		Margin            string    `json:"margin"`
		MarginFrozen      string    `json:"margin_frozen"`
		MarginMode        string    `json:"margin_mode"`
		MarginRatio       string    `json:"margin_ratio"`
		RealizedPnl       string    `json:"realized_pnl"`
		Timestamp         time.Time `json:"timestamp"`
		TotalAvailBalance string    `json:"total_avail_balance"`
		UnrealizedPnl     string    `json:"unrealized_pnl"`
		MaxWithdraw       string    `json:"max_withdraw"`
	} `json:"info"`
}

type SwapMarkPrice struct {
	InstrumentID string    `json:"instrument_id"`
	MarkPrice    string    `json:"mark_price"`
	Timestamp    time.Time `json:"timestamp"`
}

type SwapLeverage struct {
	LongLeverage  string `json:"long_leverage"`
	ShortLeverage string `json:"short_leverage"`
	MarginMode    string `json:"margin_mode"`
	InstrumentID  string `json:"instrument_id"`
}

type OrderInfo struct {
	FilledQty    string    `json:"filled_qty"`
	Fee          string    `json:"fee"`
	ClientOid    string    `json:"client_oid"`
	PriceAvg     string    `json:"price_avg"`
	TriggerPrice string    `json:"trigger_price"`
	Type         string    `json:"type"`
	InstrumentID string    `json:"instrument_id"`
	Size         string    `json:"size"`
	Price        string    `json:"price"`
	ContractVal  string    `json:"contract_val"`
	OrderID      string    `json:"order_id"`
	OrderType    string    `json:"order_type"`
	Status       string    `json:"status"`
	State        string    `json:"state"`
	Timestamp    time.Time `json:"timestamp"`
}

// /api/swap/v3/<instrument_id>/position
type SwapInstrumentPosition struct {
	MarginMode string    `json:"margin_mode"`
	Timestamp  time.Time `json:"timestamp"`
	Holding    []struct {
		AvailPosition    string    `json:"avail_position"`
		AvgCost          string    `json:"avg_cost"`
		InstrumentID     string    `json:"instrument_id"`
		Last             string    `json:"last"`
		Leverage         string    `json:"leverage"`
		LiquidationPrice string    `json:"liquidation_price"`
		MaintMarginRatio string    `json:"maint_margin_ratio"`
		Margin           string    `json:"margin"`
		Position         string    `json:"position"`
		RealizedPnl      string    `json:"realized_pnl"`
		SettledPnl       string    `json:"settled_pnl"`
		SettlementPrice  string    `json:"settlement_price"`
		Side             string    `json:"side"`
		Timestamp        time.Time `json:"timestamp"`
		UnrealizedPnl    string    `json:"unrealized_pnl"`
	} `json:"holding"`
}

// /api/swap/v3/<instrument_id>/accounts

type InstrumentAccount struct {
	Info struct {
		Equity            string    `json:"equity"`
		FixedBalance      string    `json:"fixed_balance"`
		InstrumentID      string    `json:"instrument_id"`
		MaintMarginRatio  string    `json:"maint_margin_ratio"`
		Margin            string    `json:"margin"`
		MarginFrozen      string    `json:"margin_frozen"`
		MarginMode        string    `json:"margin_mode"`
		MarginRatio       string    `json:"margin_ratio"`
		RealizedPnl       string    `json:"realized_pnl"`
		Timestamp         time.Time `json:"timestamp"`
		TotalAvailBalance string    `json:"total_avail_balance"`
		UnrealizedPnl     string    `json:"unrealized_pnl"`
		MaxWithdraw       string    `json:"max_withdraw"`
	} `json:"info"`
}

// /api/swap/v3/accounts/<instrument_id>/settings

type InstrumentSetting struct {
	LongLeverage  string `json:"long_leverage"`
	ShortLeverage string `json:"short_leverage"`
	MarginMode    string `json:"margin_mode"`
	InstrumentID  string `json:"instrument_id"`
}

// /api/swap/v3/trade_fee

type TradeFee struct {
	Maker     string    `json:"maker"`
	Taker     string    `json:"taker"`
	Timestamp time.Time `json:"timestamp"`
}

// /api/swap/v3/order

type Order struct {
	ErrorMessage string `json:"error_message"`
	Result       string `json:"result"`
	ErrorCode    string `json:"error_code"`
	ClientOid    string `json:"client_oid"`
	OrderID      string `json:"order_id"`
}

// /api/swap/v3/fills

type FillDetail struct {
	TradeID      string    `json:"trade_id"`
	InstrumentID string    `json:"instrument_id"`
	OrderID      string    `json:"order_id"`
	Price        string    `json:"price"`
	OrderQty     string    `json:"order_qty"`
	Fee          string    `json:"fee"`
	Timestamp    time.Time `json:"timestamp"`
	ExecType     string    `json:"exec_type"`
	Side         string    `json:"side"`
}

// ==========================  websocket ================================

type PriceRange struct {
	Table string `json:"table"`
	Data  []struct {
		Highest      string    `json:"highest"`
		InstrumentID string    `json:"instrument_id"`
		Lowest       string    `json:"lowest"`
		Timestamp    time.Time `json:"timestamp"`
	} `json:"data"`
}
