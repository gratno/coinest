package worker

import (
	"coinest/api"
	"coinest/config"
	"fmt"
	"github.com/golang/glog"
	"github.com/shopspring/decimal"
	"strconv"
	"sync"
	"time"
)

func closeHedge(openInfo *OpenedExchangeInfo, stop func(income decimal.Decimal) bool) (*ClosedExchangeInfo, error) {
	closedInfo := &ClosedExchangeInfo{
		Swap: &CloseExchange{
			Name:      openInfo.Swap.Name,
			TradeType: openInfo.Swap.TradeType,
			Amount:    openInfo.Swap.Amount,
		},
		Margin: &CloseExchange{
			Name:        openInfo.Margin.Name,
			TradeType:   openInfo.Margin.TradeType,
			Liquidation: openInfo.Margin.Liquidation,
			Amount:      openInfo.Margin.Amount,
		},
	}
	position, err := api.SwapInstrumentPosition(openInfo.Swap.InstrumentId)
	if err != nil {
		return nil, fmt.Errorf("获取持仓信息失败！ err:%w", err)
	}
	if len(position.Holding) == 0 {
		return nil, fmt.Errorf("未检测到仓位")
	}
	if openInfo.Margin.Liquidation.IsZero() {
		account, err := api.MarginAccount(openInfo.Margin.InstrumentId)
		if err != nil {
			return nil, fmt.Errorf("获取账户信息失败! err:%w", err)
		}
		openInfo.Margin.Liquidation, _ = decimal.NewFromString(account.LiquidationPrice)
		glog.Infoln("初始化币币杠杆归零值:", openInfo.Margin.Liquidation)
	}

	holding := position.Holding[0]
	closedInfo.Swap.Liquidation, _ = decimal.NewFromString(holding.LiquidationPrice)
	if openInfo.Swap.Liquidation.IsZero() {
		openInfo.Swap.Liquidation = closedInfo.Swap.Liquidation
		glog.Infoln("初始化永续合约归零值:", openInfo.Swap.Liquidation)
	}
	price, err := api.SwapMarkPrice(openInfo.Swap.InstrumentId)
	if err != nil {
		return nil, fmt.Errorf("获取合约标记价格失败! err:%w", err)
	}
	swapIncome, _ := decimal.NewFromString(holding.UnrealizedPnl)
	SwapMark, _ := decimal.NewFromString(price.MarkPrice)
	swapIncome = swapIncome.Sub(decimal.NewFromFloat(0.002)).Mul(SwapMark)
	closedInfo.Swap.MarkPrice = SwapMark
	closedInfo.Swap.Income = swapIncome
	detail, err := api.MarginOrderDetail(openInfo.Margin.InstrumentId, openInfo.Margin.OrderId)
	if err != nil {
		return nil, fmt.Errorf("获取币币杠杆订单详情失败! err:%w", err)
	}
	if detail.State != "2" {
		return nil, fmt.Errorf("币币杠杆订单未完全成交! ")
	}
	marginIncome, _ := decimal.NewFromString(detail.FilledNotional)
	mark, err := api.MarginMarkPrice(openInfo.Margin.InstrumentId)
	if err != nil {
		return nil, fmt.Errorf("获取币币杠杆标记价格失败! err:%w", err)
	}
	markPrice, _ := decimal.NewFromString(mark.MarkPrice)
	closedInfo.Margin.MarkPrice = markPrice

	glog.Infof("币币杠杆标记价格 old:%s new:%s action:%s\n", openInfo.Margin.MarkPrice, mark.MarkPrice, openInfo.Margin.TradeType)
	marginIncome = marginIncome.Sub(markPrice.Mul(openInfo.Margin.Amount))

	var (
		side string
	)

	switch openInfo.Margin.TradeType {
	case config.OPEN_MANY:
		side = "sell"
		marginIncome = marginIncome.Neg()
	case config.OPEN_EMPTY:
		side = "buy"
	}

	closedInfo.Margin.Income = marginIncome
	glog.Infof("income swap:%s margin:%s \n", swapIncome, marginIncome)
	d := swapIncome.Add(marginIncome).Sub(SwapMark.Mul(decimal.NewFromFloat(0.001)))
	closedInfo.Income = d
	closedInfo.Stop = stop(d)

	boom := func(exchange CloseExchange) bool {
		if boomBunker(exchange) {
			glog.Infof("%s 达到爆仓点，快收手! mark_price:%s liquidation:%s\n", exchange.Name, exchange.MarkPrice, exchange.Liquidation)
			return true
		}
		return false
	}
	if boom(*closedInfo.Swap) || boom(*closedInfo.Margin) {
		goto exit
	}
	if !closedInfo.Stop {
		return closedInfo, nil
	}
exit:
	glog.Infof("可以收手了!!! 预计收益:$ %s \n", d.Truncate(2))
	var swapCloseTrade config.TradeType
	switch openInfo.Swap.TradeType {
	case config.OPEN_MANY:
		swapCloseTrade = config.CLOSE_MANY
	case config.OPEN_EMPTY:
		swapCloseTrade = config.CLOSE_EMPTY
	}

	swapOrder := mustSwapOrder(map[string]string{
		"client_oid":    genRandClientId(),
		"size":          holding.AvailPosition,
		"type":          strconv.Itoa(int(swapCloseTrade)),
		"order_type":    "0",
		"match_price":   "1",
		"instrument_id": openInfo.Swap.InstrumentId,
		"price":         SwapMark.String(),
	})

	marginOrder := mustMarginOrder(map[string]string{
		"client_oid":     genRandClientId(),
		"type":           "limit",
		"instrument_id":  openInfo.Margin.InstrumentId,
		"margin_trading": "2",
		"side":           side,
		"order_type":     "0",
		"size":           openInfo.Margin.Amount.Truncate(4).String(),
		"price":          markPrice.String(),
	})

	full := waitFinished(map[string]string{"instrument_id": openInfo.Swap.InstrumentId, "order_id": swapOrder},
		map[string]string{"instrument_id": openInfo.Margin.InstrumentId, "order_id": marginOrder})

	if details, err := api.SwapFills(map[string]string{
		"instrument_id": openInfo.Swap.InstrumentId,
		"limit":         "10",
	}); err == nil {
		for _, v := range details {
			if v.OrderID == swapOrder {
				p, _ := decimal.NewFromString(v.Price)
				q, _ := decimal.NewFromString(v.OrderQty)
				f, _ := decimal.NewFromString(v.Fee)
				fu, _ := decimal.NewFromString(full)
				s := p.Mul(q).Add(f).Sub(fu)
				closedInfo.Income = s
				return closedInfo, nil
			}
		}
	}

	return closedInfo, nil
}

func closeMargin(exchange *OpenExchange, stop func(income decimal.Decimal) bool) (*CloseExchange, error) {
	account, err := api.MarginAccount(exchange.InstrumentId)
	if err != nil {
		return nil, fmt.Errorf("获取账户信息失败! err:%w", err)
	}
	if exchange.Liquidation.IsZero() {
		exchange.Liquidation, _ = decimal.NewFromString(account.LiquidationPrice)
	}
	detail, err := api.MarginOrderDetail(exchange.InstrumentId, exchange.OrderId)
	if err != nil {
		return nil, fmt.Errorf("获取币币杠杆订单详情失败! err:%w", err)
	}
	if detail.State != "2" {
		return nil, fmt.Errorf("币币杠杆订单未完全成交! ")
	}
	marginIncome, _ := decimal.NewFromString(detail.FilledNotional)
	mark, err := api.MarginMarkPrice(exchange.InstrumentId)
	if err != nil {
		return nil, fmt.Errorf("获取币币杠杆标记价格失败! err:%w", err)
	}
	markPrice, _ := decimal.NewFromString(mark.MarkPrice)

	glog.Infof("币币杠杆标记价格 old:%s new:%s action:%s\n", exchange.MarkPrice, mark.MarkPrice, exchange.TradeType)
	marginIncome = marginIncome.Sub(markPrice.Mul(exchange.Amount))

	markPriceInc := markPrice.Sub(exchange.MarkPrice)

	var (
		side       string
		amount     decimal.Decimal
		size       string
		closedInfo = &CloseExchange{
			TradeType: exchange.TradeType,
			MarkPrice: markPrice,
		}
	)
	switch exchange.TradeType {
	case config.OPEN_MANY:
		marginIncome = marginIncome.Neg()
		side = "sell"
		amount, _ = decimal.NewFromString(account.CurrencyBTC.Available)
	case config.OPEN_EMPTY:
		markPriceInc = markPriceInc.Neg()
		usdt, _ := decimal.NewFromString(account.CurrencyUSDT.Available)
		amount = usdt.Div(markPrice)
		side = "buy"
	}

	closedInfo.Amount = amount
	size = amount.Truncate(4).String()

	closedInfo.Income = marginIncome
	closedInfo.Stop = stop(marginIncome)
	if boomBunker(*closedInfo) {
		glog.Infof("%s 达到爆仓点，快收手! mark_price:%s liquidation:%s\n", exchange.Name, exchange.MarkPrice, exchange.Liquidation)
		goto exit
	}
	if !closedInfo.Stop {
		return closedInfo, nil
	}
exit:
	glog.Infof("可以收手了!!! 预计收益:$ %s 标记价格趋势:%s \n", marginIncome.Truncate(2), markPriceInc)

	params := map[string]string{
		"client_oid":     genRandClientId(),
		"type":           "limit",
		"instrument_id":  exchange.InstrumentId,
		"margin_trading": "2",
		"side":           side,
		"order_type":     "0",
		"size":           size,
		"price":          markPrice.String(),
	}

	if side == "buy" {
		sd, _ := decimal.NewFromString(params["size"])
		balance, _ := decimal.NewFromString(account.CurrencyUSDT.Available)
		d := balance.Div(exchange.MarkPrice).Sub(sd)
		if d.LessThanOrEqual(decimal.Zero) {
			fixprice := exchange.MarkPrice.Add(d.Mul(exchange.MarkPrice))
			params["price"] = fixprice.Truncate(2).String()
		}
	}

	marginOrder := mustMarginOrder(params)

	glog.Infof("等待平仓订单完成... price:%s\n", markPrice)

	full := waitFinished(nil, map[string]string{
		"instrument_id": exchange.InstrumentId,
		"order_id":      marginOrder,
	})

	d, _ := decimal.NewFromString(detail.FilledNotional)
	d2, _ := decimal.NewFromString(full)
	d = d.Sub(d2)
	glog.Infoln("币币杠杆真实收益:", d.String())

	return closedInfo, nil

}

func waitFinished(swap map[string]string, margin map[string]string) string {
	total := ""

	wg := sync.WaitGroup{}
	if len(swap) > 0 {
		go func() {
			wg.Add(1)
			defer wg.Done()
			for {
				info, err := api.SwapOrderInfo(swap["instrument_id"], swap["order_id"])
				if err != nil {
					glog.Errorln("获取swap平仓订单信息失败!", err)
					time.Sleep(3 * time.Second)
					continue
				}
				if info.State != "2" && info.State != "-1" && info.State != "4" {
					glog.Infoln("swap平仓还未完全成交! state:", info.State)
					time.Sleep(3 * time.Second)
					continue
				}
				return
			}
		}()
	}

	if len(margin) > 0 {
		go func() {
			wg.Add(1)
			defer wg.Done()
			for {
				info, err := api.MarginOrderDetail(margin["instrument_id"], margin["order_id"])
				if err != nil {
					glog.Errorln("获取margin平仓订单信息失败!", err)
					time.Sleep(3 * time.Second)
					continue
				}
				if info.State != "2" && info.State != "-1" && info.State != "4" {
					glog.Infoln("margin平仓还未完全成交! state:", info.State)
					time.Sleep(3 * time.Second)
					continue
				}
				total = info.FilledNotional
				return
			}
		}()
	}
	wg.Wait()
	return total
}

func boomBunker(exchange CloseExchange) bool {
	if !exchange.Liquidation.GreaterThan(decimal.Zero) {
		return false
	}
	risk := decimal.NewFromFloat(0.98)
	switch exchange.TradeType {
	case config.OPEN_MANY:
		exchange.Liquidation = exchange.Liquidation.Div(risk)
		return exchange.MarkPrice.LessThan(exchange.Liquidation)
	case config.OPEN_EMPTY:
		exchange.Liquidation = exchange.Liquidation.Mul(risk)
		return exchange.MarkPrice.GreaterThan(exchange.Liquidation)
	}
	return false
}
