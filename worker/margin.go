package worker

import (
	"coinest/api"
	"coinest/config"
	"errors"
	"github.com/golang/glog"
	"github.com/shopspring/decimal"
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().Unix())
}

func marginWorker(lastClosedInfo *ClosedInfo) *ClosedInfo {
	var (
		exchange *OpenExchange
		borrow   *Borrow
		err      error
	)
	for {
		exchange, err = openMargin(borrow == nil, func(trade config.TradeType) bool {
			if lastClosedInfo == nil {
				return true
			}
			if price, err := api.MarginMarkPrice("BTC-USDT"); err == nil {
				p, _ := decimal.NewFromString(price.MarkPrice)
				glog.Infof("预开方向:%s 新市价:%s 上次平价:%s \n", trade, p, lastClosedInfo.MarkPrice)
				switch trade {
				case config.OPEN_MANY:
					return p.LessThan(lastClosedInfo.MarkPrice)
				case config.OPEN_EMPTY:
					return p.GreaterThan(lastClosedInfo.MarkPrice)
				}
			}
			return false
		})

		if exchange != nil && borrow == nil {
			borrow = exchange.Borrow
		}

		if err != nil {
			glog.Errorln("开仓失败！", err)
			time.Sleep(10 * time.Second)
			continue
		}
		glog.Infof("等待开仓订单完成! mark_price:%s\n", exchange.MarkPrice)
		if fill := queryOrder(exchange.InstrumentId, exchange.OrderId); fill == "" {
			glog.Warningln("重新开仓，订单已撤销！")
			time.Sleep(5 * time.Second)
			continue
		}
		break
	}

	defer func() {
		if borrow != nil {
			if _, err := api.MarginRepay(map[string]string{
				"borrow_id":     borrow.ID,
				"instrument_id": borrow.InstrumentId,
				"currency":      borrow.Currency,
				"amount":        borrow.Amount,
			}); err != nil {
				glog.Infof("还币失败! borrow:%+v err:%s\n", exchange.Borrow, err)
			}
		}
	}()

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	total := exchange.MarkPrice.Mul(exchange.Amount)
	expect := total.Mul(decimal.NewFromFloat(point / 100))
	glog.Infof("余额:%s 任务需直到收益超过 %s 时结束！\n", total.Truncate(4), expect.Truncate(4))
	var lastIncome decimal.Decimal

	once := true

	stop := func(income decimal.Decimal) bool {
		if income.GreaterThan(expect) {
			if (income.GreaterThan(lastIncome) && lastIncome.GreaterThan(expect)) || once {
				once = false
				glog.Infof("last_income:%s income:%s 还在涨,继续...\n", lastIncome, income)
				return false
			}
			return true
		}
		return false
	}
	for {
		closedInfo, err := closeMargin(exchange, stop)
		if err != nil {
			glog.Errorln("关仓失败! 1s后继续。。。", err)
			time.Sleep(3 * time.Second)
			if errors.Is(err, ErrZeroSize) {
				return nil
			}
			continue
		}

		if stop(closedInfo.Income) {
			glog.Infof("一次任务完成!!! 收益:$ %s trade_type:%s expect:%s \n", closedInfo.Income, exchange.TradeType, expect)
			return closedInfo
		}

		lastIncome = closedInfo.Income

		select {
		case <-ticker.C:
			glog.Infof("继续努力， 当前收益:$ %s  expect:%s\n", closedInfo.Income, expect)
		}
	}

}

func queryOrder(instrumentId, id string) string {
	t0 := time.Now()
	for {
		if time.Since(t0) > 30*time.Second {
			if err := api.MarginCancelOrder(id, instrumentId); err != nil {
				glog.Errorln("取消订单失败!", err)
				time.Sleep(time.Second)
				continue
			}
			break
		}
		detail, err := api.MarginOrderDetail(instrumentId, id)
		if err != nil {
			glog.Warningf("获取币币杠杆订单详情失败! err:%s\n", err.Error())
			time.Sleep(time.Second)
			continue
		}
		if detail.State != "2" {
			time.Sleep(time.Second)
			continue
		}

		return detail.FilledNotional
	}

	return ""
}
