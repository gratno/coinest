package worker

import (
	"coinest/api"
	"errors"
	"github.com/golang/glog"
	"github.com/shopspring/decimal"
	"time"
)

func hedgeWorker(lastClosedInfo *ClosedInfo) *ClosedInfo {
	openInfo, err := openHedge(true)
	if err != nil {
		glog.Errorln("开仓失败！", err)
		return nil
	}

	defer func() {
		exchange := openInfo.Margin
		if exchange.Borrow != nil {
			if _, err := api.MarginRepay(map[string]string{
				"borrow_id":     exchange.Borrow.ID,
				"instrument_id": exchange.InstrumentId,
				"currency":      exchange.Borrow.Currency,
				"amount":        exchange.Borrow.Amount,
			}); err != nil {
				glog.Infof("还币失败! borrow:%+v err:%s\n", exchange.Borrow, err)
			}
		}
	}()

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	total := openInfo.Swap.MarkPrice.Mul(openInfo.Swap.Amount).
		Add(openInfo.Margin.MarkPrice.Mul(openInfo.Margin.Amount))
	expect := total.Mul(decimal.NewFromFloat(point / 100))
	glog.Infof("余额:%s 任务需直到收益超过 %s 时结束！\n", total.Truncate(4), expect.Truncate(4))

	var (
		lastIncome decimal.Decimal
		once       = true
	)

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
		closedInfo, err := closeHedge(openInfo, stop)
		if err != nil {
			glog.Errorln("关仓失败! 1s后继续。。。", err)
			time.Sleep(3 * time.Second)
			if errors.Is(err, ErrZeroSize) {
				return nil
			}
			continue
		}

		if stop(closedInfo.Income) {
			glog.Infof("一次任务完成!!! 收益:$ %s trade_type:%s expect:%s \n", closedInfo.Income, expect)
			return closedInfo
		}

		lastIncome = closedInfo.Income

		select {
		case <-ticker.C:
			glog.Infof("继续努力， 当前收益:$ %s  expect:%s\n", closedInfo.Income, expect)
		}
	}
}
