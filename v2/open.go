package main

import (
	"coinest/okex-go-sdk-api"
	"coinest/v2/config"
	"fmt"
	"github.com/golang/glog"
	"github.com/shopspring/decimal"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

type Player struct {
	debug        bool
	status       int // 0 等待开仓 1等待平仓
	client       *Client
	instrumentId string
	stats        map[string]interface{}
}

func NewPlayer(instrumentId string) *Player {
	p := &Player{
		debug:        false,
		client:       NewClient(),
		instrumentId: instrumentId,
		stats:        make(map[string]interface{}),
	}
	return p
}

func (p *Player) Worker(trend Trend) {
	if p.debug {
		return
	}
	price, err := p.client.MarkPrice(p.instrumentId)
	if err != nil {
		glog.Errorf("client.MarkPrice failed! err:%v\n", err)
		return
	}
	glog.Infoln("合约当前标记价格:", price.StringFixed(2))
	if p.status == 0 {
		if trend == TREND_UNKNOWN {
			return
		}
		sheets, err := p.client.Sheets(p.instrumentId, price)
		if err != nil {
			glog.Errorln("client.Sheets failed!", err)
			return
		}
		orderid, err := p.client.Post(p.instrumentId, p.status, trend, price, sheets)
		if err != nil {
			glog.Errorf("开仓 client.Post failed! sheets:%d err:%v\n", sheets, err)
			return
		}
		glog.Infof("开仓成功! price:%s sheets:%d trend:%s orderid:%s\n", price.StringFixed(2), sheets, trend.String(), orderid)
		p.stats["sheets"] = sheets
		p.stats["orderid"] = orderid
		p.stats["price"] = price
		p.stats["trend"] = trend
		p.status = 1
		return
	}
	if p.status == 1 {
		oldtrend := p.stats["trend"].(Trend)
		oldprice := p.stats["price"].(decimal.Decimal)
		if oldtrend == trend {
			return
		}
		inc := price.Sub(oldprice)
		profile := decimal.NewFromInt(0).Add(inc)
		closed := false
		if oldtrend == TREND_MANY {
			switch trend {
			case TREND_EMPTY:
				if inc.IsPositive() && inc.GreaterThan(decimal.NewFromInt(10)) {
					closed = true
				}
			case TREND_UNKNOWN:
				// 止损 止盈
				if inc.IsNegative() {
					if inc.Neg().GreaterThan(decimal.NewFromInt(disloss)) {
						closed = true
					}
					break
				}
				if inc.GreaterThan(decimal.NewFromInt(disgain)) {
					closed = true
				}
			}
		} else if oldtrend == TREND_EMPTY {
			profile = profile.Neg()
			switch TREND_MANY {
			case TREND_MANY:
				if inc.IsNegative() && inc.LessThan(decimal.NewFromInt(-10)) {
					closed = true
				}
			case TREND_UNKNOWN:
				// 止损 止盈
				if inc.IsPositive() {
					if inc.GreaterThan(decimal.NewFromInt(disloss)) {
						closed = true
					}
					break
				}
				if inc.Neg().GreaterThan(decimal.NewFromInt(disgain)) {
					closed = true
				}
			}
		}

		if p.giveUp(profile) {
			closed = true
		}

		if v, ok := p.stats["history"]; !ok {
			p.stats["history"] = []decimal.Decimal{profile}
		} else {
			history := v.([]decimal.Decimal)
			history = append(history, profile)
			p.stats["history"] = history
		}

		glog.Infof("new_price:%s profile:%s stats: %s\n", price.StringFixed(2), profile.String(), p.Stats())
		if closed {
			glog.Infoln("触发平仓！ profile:", profile.String())
			sheets := p.stats["sheets"].(int64)
			_, err := p.client.Post(p.instrumentId, p.status, oldtrend, price, sheets)
			if err != nil {
				glog.Errorf("平仓 client.Post failed! sheets:%d err:%v\n", sheets, err)
				return
			}
			p.stats = make(map[string]interface{})
			p.status = 0
		}

	}
}

func (p *Player) giveUp(cur decimal.Decimal) bool {
	if cur.LessThan(decimal.NewFromInt(10)) {
		return false
	}
	if p.stats["history"] == nil {
		return false
	}
	history := p.stats["history"].([]decimal.Decimal)
	if len(history) < 4 {
		return false
	}
	if history[len(history)-1].GreaterThan(cur) {
		return true
	}
	history = history[len(history)-4:]
	flag := true
	for i := range history {
		flag = flag && history[i].IsNegative()
	}
	return flag
}

func (p *Player) Stats() string {
	if len(p.stats) == 0 {
		return ""
	}
	arr := make([]string, 0, len(p.stats))
	for k, v := range p.stats {
		arr = append(arr, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(arr, "   ")
}

func NewClient() *Client {
	var cfg okex.Config
	cfg.Endpoint = "https://www.okex.com/"
	cfg.ApiKey = config.ApiKey
	cfg.SecretKey = config.SecretKey
	cfg.Passphrase = config.Passphrase
	cfg.TimeoutSecond = 30
	cfg.IsPrint = false
	cfg.I18n = okex.ENGLISH

	client := okex.NewClient(cfg)
	return &Client{
		client:   client,
		Leverage: int64(config.Leverage),
	}
}

type Client struct {
	client   *okex.Client
	Leverage int64
}

func (c *Client) Sheets(instrumentId string, price decimal.Decimal) (int64, error) {
	setting, err := c.client.PostSwapAccountsLeverage(instrumentId, fmt.Sprintf("%d", c.Leverage), "3")
	if err != nil {
		return 0, err
	}
	glog.Infoln("setting:", setting)
	account, err := c.client.GetSwapAccount(instrumentId)
	if err != nil {
		return 0, err
	}
	glog.Infoln("account:", account)
	equity, err := decimal.NewFromString(account.Info.Equity)
	if err != nil {
		return 0, err
	}
	sheets := equity.Mul(decimal.New(c.Leverage, -2)).Mul(price).IntPart() - 3
	return sheets, nil
}

func (c *Client) Post(instrumentId string, step int, trend Trend, price decimal.Decimal, sheets int64) (string, error) {
	typ := 0
	switch trend {
	case TREND_MANY:
		typ = 1
		if step == 1 {
			typ = 3
		}
	case TREND_EMPTY:
		typ = 2
		if step == 1 {
			typ = 4
		}
	}

	result, err := c.client.PostSwapOrder(instrumentId, &okex.BasePlaceOrderInfo{
		ClientOid:  genRandClientId(),
		Price:      price.StringFixed(1),
		MatchPrice: "1",
		Type:       fmt.Sprintf("%d", typ),
		Size:       fmt.Sprintf("%d", sheets),
	})
	if err != nil {
		return "", err
	}
	if result.OrderId == "" {
		return "", fmt.Errorf("orderid is empty! result:%+v", result)
	}
	return result.OrderId, nil
}

func (c *Client) MarkPrice(instrumentId string) (decimal.Decimal, error) {
	var markPrice decimal.Decimal
	data, err := c.client.GetSwapMarkPriceByInstrument(instrumentId)
	if err != nil {
		return markPrice, err
	}
	markPrice, err = decimal.NewFromString(data.MarkPrice)
	if err != nil {
		return markPrice, err
	}
	if markPrice.IsZero() {
		return markPrice, fmt.Errorf("markPrice.IsZero! data:%v", data)
	}
	return markPrice, nil
}

func genRandClientId() string {
	return "a" + strconv.FormatInt(int64(rand.Int31()), 10)
}

func init() {
	rand.Seed(time.Now().Unix())
}
