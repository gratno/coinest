package worker

import (
	"coinest/api"
	"coinest/config"
	"coinest/model"
	"fmt"
	"github.com/golang/glog"
	"github.com/shopspring/decimal"
	"golang.org/x/sync/errgroup"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

var openPool errgroup.Group

func preOpenSwap(instrumentId string, exchange *OpenExchange) (*OpenExchange, error) {
	swapExchange := &OpenExchange{
		Name:         "永续合约",
		InstrumentId: instrumentId,
		Leverage:     exchange.Leverage,
	}

	account, err := api.SwapAccount(instrumentId)
	if err != nil {
		return nil, fmt.Errorf("合约获取账户信息失败! err:%w", err)
	}

	equity, _ := decimal.NewFromString(account.Info.Equity)
	glog.Infof("合约账户信息:%+v 可用权益:%s\n", account.Info, equity)

	switch exchange.TradeType {
	case config.OPEN_MANY:
		swapExchange.TradeType = config.OPEN_EMPTY
	case config.OPEN_EMPTY:
		swapExchange.TradeType = config.OPEN_MANY
	}

	openPool.Go(func() error {
		if err := api.SetSwapLeverage(instrumentId, exchange.Leverage); err != nil {
			glog.Errorln("设置合约杠杆倍数失败!", err)
		}
		return nil
	})

	price, err := api.SwapMarkPrice(instrumentId)
	if err != nil {
		return nil, fmt.Errorf("获取合约标记价格失败! %w", err)
	}

	if markPrice, err := decimal.NewFromString(price.MarkPrice); err == nil {
		swapExchange.MarkPrice = markPrice
	}

	// 最大对冲btc数
	maxAmount := exchange.Amount
	equity = equity.Mul(decimal.NewFromFloat(0.9))
	if maxAmount.GreaterThan(equity) {
		maxAmount = equity
	}

	// 最大可开合约张数
	sheets := maxAmount.Mul(decimal.New(int64(exchange.Leverage), -2)).Mul(swapExchange.MarkPrice).IntPart()

	// 重算双方最大对冲btc数
	maxAmount = decimal.NewFromInt(sheets).Div(swapExchange.MarkPrice).Div(decimal.New(1, -2))

	swapExchange.Params = map[string]string{
		"client_oid":    genRandClientId(),
		"size":          fmt.Sprintf("%d", sheets),
		"type":          strconv.Itoa(int(swapExchange.TradeType)),
		"order_type":    "0",
		"match_price":   "1",
		"instrument_id": instrumentId,
		"price":         swapExchange.MarkPrice.String(),
	}
	swapExchange.Amount = maxAmount
	swapExchange.CreateAt = time.Now()

	return swapExchange, nil
}

func preOpenMargin(instrumentId string, needBorrow bool, reverse bool, hook func(trade config.TradeType) bool) (*OpenExchange, error) {
	exchange := &OpenExchange{
		Name:         "币币杠杆",
		InstrumentId: instrumentId,
	}
	exchange.TradeType = api.SwapTradeType("BTC-USDT-SWAP")

	glog.Infof("币币杠杆预测 %s \n", exchange.TradeType)

	switch exchange.TradeType {
	case config.OPEN_MANY:
		if reverse {
			exchange.TradeType = config.OPEN_EMPTY
		}
	case config.OPEN_EMPTY:
		if reverse {
			exchange.TradeType = config.OPEN_MANY
		}
	case config.OPEN_PAUSE:
		return nil, ErrOpenSuspend
	}
	if hook != nil && !hook(exchange.TradeType) {
		return nil, fmt.Errorf("hook failed! ")
	}
	availability, err := api.MarginAvailability(instrumentId)
	if err != nil {
		return nil, fmt.Errorf("获取币币杠杆配置信息失败! err:%w", err)
	}

	leverage, _ := strconv.Atoi(availability.CurrencyUSDT.Leverage)
	exchange.Leverage = leverage

	openPool.Go(func() error {
		if err := api.SetMarginLeverage(instrumentId, leverage); err != nil {
			glog.Errorln("设置币币杠杆倍数失败!", err)
		}
		return nil
	})

	if needBorrow {
		if borroweds, err := api.MarginBorrowed(instrumentId); err == nil {
			btc, _ := decimal.NewFromString(availability.CurrencyBTC.Available)
			usdt, _ := decimal.NewFromString(availability.CurrencyUSDT.Available)
			for _, v := range borroweds {
				brr, _ := decimal.NewFromString(v.Amount)
				switch strings.ToLower(v.Currency) {
				case "btc":
					btc = btc.Sub(brr)
				case "usdt":
					usdt = usdt.Sub(brr)
				}
			}
			availability.CurrencyUSDT.Available = usdt.String()
			availability.CurrencyBTC.Available = btc.String()
		}
		if borrow := marginBorrow(exchange, availability); borrow != nil {
			exchange.Borrow = borrow
		} else {
			os.Exit(1)
		}
	}

	account, err := api.MarginAccount(instrumentId)
	if err != nil {
		return exchange, fmt.Errorf("获取账户信息失败! err:%w", err)
	}

	glog.Infof("当前币币杠杆账户:%+v\n", account)

	exchange.Liquidation, _ = decimal.NewFromString(account.LiquidationPrice)

	switch exchange.TradeType {
	case config.OPEN_MANY:
		if d, _ := decimal.NewFromString(account.CurrencyUSDT.Available); d.LessThan(decimal.NewFromInt(1)) {
			exchange.TradeType = config.OPEN_EMPTY
		}
	case config.OPEN_EMPTY:
		if d, _ := decimal.NewFromString(account.CurrencyBTC.Available); d.LessThan(decimal.NewFromFloat(0.005)) {
			exchange.TradeType = config.OPEN_MANY
		}
	}

	var side string

	price, err := api.MarginMarkPrice(instrumentId)
	if err != nil {
		return exchange, fmt.Errorf("获取币币杠杆市场价失败! %w", err)
	}
	markPrice, _ := decimal.NewFromString(price.MarkPrice)
	exchange.MarkPrice = markPrice

	var amount decimal.Decimal
	switch exchange.TradeType {
	case config.OPEN_MANY:
		usdt, _ := decimal.NewFromString(account.CurrencyUSDT.Available)
		amount = usdt.Div(markPrice)
		side = "buy"
		glog.Infof("币币杠杆预开 %s usdt:%s mark_price:%s\n", side, usdt.Truncate(4), markPrice.Truncate(2))
	case config.OPEN_EMPTY:
		amount, _ = decimal.NewFromString(account.CurrencyBTC.Available)
		side = "sell"
		glog.Infof("币币杠杆预开 %s btc:%s \n", side, amount.Truncate(4))
	}

	if amount.Truncate(4).LessThan(decimal.New(5, -3)) {
		return exchange, ErrZeroSize
	}

	exchange.Amount = amount
	exchange.CreateAt = time.Now()
	exchange.Params = map[string]string{
		"client_oid":     genRandClientId(),
		"type":           "limit",
		"instrument_id":  instrumentId,
		"margin_trading": "2",
		"side":           side,
		"order_type":     "0",
		"size":           amount.Truncate(4).String(),
		"price":          exchange.MarkPrice.Truncate(2).String(),
	}

	if side == "buy" {
		sd, _ := decimal.NewFromString(exchange.Params["size"])
		balance, _ := decimal.NewFromString(account.CurrencyUSDT.Available)
		d := balance.Div(exchange.MarkPrice).Sub(sd)
		if d.LessThanOrEqual(decimal.Zero) {
			fixprice := exchange.MarkPrice.Add(d.Mul(exchange.MarkPrice))
			exchange.Params["price"] = fixprice.Truncate(2).String()
		}
	}

	return exchange, nil

}

func marginBorrow(exchange *OpenExchange, availability *model.MarginAvailability) *Borrow {
	// 借币 ==================
	borrowParam := map[string]string{
		"instrument_id": exchange.InstrumentId,
		"client_oid":    genRandClientId(),
	}

	if v, _ := decimal.NewFromString(availability.CurrencyBTC.Available); v.GreaterThan(decimal.Zero) && exchange.TradeType == config.OPEN_EMPTY {
		borrowParam["currency"] = "btc"
		available, _ := decimal.NewFromString(availability.CurrencyBTC.Available)
		borrowParam["amount"] = available.Truncate(4).String()
	} else if v, _ := decimal.NewFromString(availability.CurrencyUSDT.Available); v.GreaterThan(decimal.Zero) && exchange.TradeType == config.OPEN_MANY {
		borrowParam["currency"] = "usdt"
		available, _ := decimal.NewFromString(availability.CurrencyUSDT.Available)
		borrowParam["amount"] = available.Truncate(4).String()
	}

	borrow, err := api.MarginBorrow(borrowParam)
	if err != nil {
		glog.Infof("借币失败! currency:%s amount:%s err:%s\n", borrowParam["currency"], borrowParam["amount"], err)
		return nil
	}

	glog.Infof("借币 currency:%s amount:%s\n", borrowParam["currency"], borrowParam["amount"])
	return &Borrow{
		ID:           borrow.BorrowID,
		Currency:     borrowParam["currency"],
		Amount:       borrowParam["amount"],
		InstrumentId: exchange.InstrumentId,
	}
}

func genRandClientId() string {
	return "a" + strconv.FormatInt(int64(rand.Int31()), 10)
}

func mustSwapOrder(params map[string]string) string {
	delta := float64(0)
	if params["type"] == strconv.Itoa(int(config.OPEN_MANY)) || params["type"] == strconv.Itoa(int(config.CLOSE_MANY)) {
		delta = -1
	}
	for i := 0; i < 50; i++ {
		orderId, err := api.SwapOrder(params)
		if err != nil {
			glog.Errorln("合约下单失败! ", err)
			size, _ := strconv.ParseFloat(params["size"], 64)
			if t := size + delta*0.01; t > 0 {
				size = t
			}
			params["size"] = strconv.FormatFloat(size, 'g', -1, 64)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		return orderId.OrderID
	}
	panic("mustSwapOrder exceed max retry")
}

func mustMarginOrder(params map[string]string) string {
	delta := float64(0)
	if params["side"] == "buy" {
		delta = -1
	}
	for i := 0; i < 50; i++ {
		orderId, err := api.MarginOrder(params)
		if err != nil {
			glog.Errorln("杠杆下单失败! ", err)
			size, _ := strconv.ParseFloat(params["size"], 64)
			if t := size + delta*0.002; t > 0 {
				size = t
			}
			params["size"] = strconv.FormatFloat(size, 'g', -1, 64)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		return orderId.OrderID
	}
	panic("mustMarginOrder exceed max retry")
}
