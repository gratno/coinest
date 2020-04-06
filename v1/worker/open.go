package worker

import (
	"coinest/v1/config"
	"fmt"
	"github.com/golang/glog"
	"github.com/shopspring/decimal"
	"golang.org/x/sync/errgroup"
)

func openHedge(needBorrow bool) (*OpenedExchangeInfo, error) {
	info := &OpenedExchangeInfo{}
	marginExchange, err := preOpenMargin("BTC-USDT", false, false, nil)
	if err != nil {
		return info, fmt.Errorf("preOpenMargin failed! %w", err)
	}
	info.Margin = marginExchange
	swapExchange, err := preOpenSwap("BTC-USDT-SWAP", marginExchange)
	if err != nil {
		return info, fmt.Errorf("preOpenSwap failed! %w", err)
	}
	info.Swap = swapExchange
	if info.Swap.Amount.LessThanOrEqual(info.Margin.Amount) {
		info.Margin.Amount = info.Swap.Amount
		marginExchange.Params["size"] = info.Swap.Amount.Truncate(2).String()
	}
	group := errgroup.Group{}
	group.Go(func() error {
		marginExchange.OrderId = mustMarginOrder(marginExchange.Params)
		return nil
	})

	group.Go(func() error {
		glog.Infof("合约预下订单 params:%+v\n", swapExchange.Params)
		swapExchange.OrderId = mustSwapOrder(swapExchange.Params)
		return nil
	})

	_ = group.Wait()
	_ = openPool.Wait()
	return info, nil
}

func openMargin(needBorrow bool, hook func(trade config.TradeType) bool) (*OpenExchange, error) {
	exchange, err := preOpenMargin("BTC-USDT", needBorrow, false, hook)
	if err != nil {
		return exchange, err
	}
	glog.Infof("币币杠杆下单 params:%+v\n", exchange.Params)

	orderId := mustMarginOrder(exchange.Params)

	exchange.OrderId = orderId
	exchange.Amount, _ = decimal.NewFromString(exchange.Params["size"])
	return exchange, nil
}
