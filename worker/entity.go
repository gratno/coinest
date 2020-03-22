package worker

import (
	"coinest/config"
	"fmt"
	"github.com/shopspring/decimal"
	"time"
)

type Borrow struct {
	ID           string
	Currency     string
	Amount       string
	InstrumentId string
}

type ClosedInfo struct {
	TradeType config.TradeType
	MarkPrice decimal.Decimal
	Income    decimal.Decimal
	Amount    decimal.Decimal
	Stop      bool
}

type OpenedExchangeInfo struct {
	Swap   *OpenExchange
	Margin *OpenExchange
}

type OpenExchange struct {
	TradeType config.TradeType
	// 杠杆倍数
	Leverage int
	// btc数量
	Amount decimal.Decimal

	MarkPrice    decimal.Decimal
	OrderId      string
	Borrow       *Borrow
	Name         string
	CreateAt     time.Time
	InstrumentId string
	Params       map[string]string
}

func (e OpenExchange) String() string {
	return fmt.Sprintf("name:%s  instrument_id:%s trade_type:%s leverage:%d amount:%s mark_price:%s order_id:%s create_at:%s",
		e.Name, e.InstrumentId, e.TradeType.String(), e.Leverage, e.Amount.String(), e.MarkPrice.String(), e.OrderId, e.CreateAt.String(),
	)
}
