package main

import (
	"coinest/okex-go-sdk-api"
	"coinest/v2/config"
	"context"
	"encoding/json"
	"fmt"
	"github.com/buger/jsonparser"
	"github.com/golang/glog"
	"github.com/shopspring/decimal"
	"math/rand"
	"strconv"
	"time"
)

type Player struct {
	instrumentId string
	status       int // 0 等待开仓 1等待平仓
	stats        map[string]interface{}
	Fee          decimal.Decimal
	client       *Client
}

func NewPlayer(instrumentId string, client *Client) *Player {
	p := &Player{
		client:       client,
		instrumentId: instrumentId,
		stats:        make(map[string]interface{}),
	}
	return p
}

func (p *Player) SetStatus(i int) {
	p.status = i
}

func (p *Player) Status() int {
	return p.status
}

func (p *Player) Algo(ctx context.Context, trend Trend, price decimal.Decimal) error {
	size, err := p.client.Position(p.instrumentId)
	if err != nil {
		return err
	}
	var triggerPrice decimal.Decimal
	switch trend {
	case TREND_EMPTY:
		triggerPrice = price.Add(decimal.NewFromFloat(2 * disloss))
	case TREND_MANY:
		triggerPrice = price.Sub(decimal.NewFromFloat(2 * disloss))
	}
	algoId, err := p.client.AlgoOrder(p.instrumentId, &okex.BaseAlgoOrderInfo{
		Type:         fmt.Sprintf("%d", int(trend+3)),
		Size:         fmt.Sprintf("%d", size),
		OrderType:    "1",
		TriggerPrice: triggerPrice.String(),
		AlgoType:     "2",
	})
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		mustExecFunc(func() error {
			return p.client.CancelAlgoOrder(p.instrumentId, &okex.BaseCancelAlgoOrderInfo{
				AlgoIds:   []string{algoId},
				OrderType: "1",
			})
		}, time.Second)
	}
	glog.Infoln("止损撤单成功!")
	return nil
}

func (p *Player) Open(trend Trend, price decimal.Decimal) error {
	sheets, err := p.client.Sheets(p.instrumentId, price)
	if err != nil {
		return err
	}
	size, err := p.post("open", trend, price, sheets, nil)
	if err != nil {
		return err
	}
	p.stats["size"] = size
	p.stats["sheets"] = size
	p.stats["trend"] = trend
	p.stats["open_price"] = price
	p.SetStatus(1)
	glog.Infof("trend:%s sheets:%d open_price:%s 开仓成功!\n", &trend, sheets, &price)
	return nil
}

// return nil 进行下一跳
func (p *Player) Close(price decimal.Decimal, neg bool) error {
	sheets, err := p.client.Position(p.instrumentId)
	if err != nil {
		return err
	}
	if sheets == 0 {
		p.SetStatus(0)
		glog.Warningln("Player.Close sheets is zero!")
		return nil
	}
	trend := p.stats["trend"].(Trend)
	openPrice := p.stats["open_price"].(decimal.Decimal)
	size, err := p.post("close", trend, price, sheets, nil)
	if err != nil {
		return err
	}
	before := decimal.NewFromInt(size).Div(openPrice).Div(decimal.New(p.client.Leverage, -2))
	after := decimal.NewFromInt(size).Div(price).Div(decimal.New(p.client.Leverage, -2))
	diff := before.Sub(after).Abs()
	if neg {
		diff = diff.Neg()
	}
	glog.Infof("trend:%s sheets:%d close_price:%s 收益:%s 平仓成功!\n", &trend, size, &price, &diff)
	p.Fee = p.Fee.Add(diff)
	return nil
}

func (p *Player) Dump() {
	glog.Infof("Dump: player profile:%s\n", &p.Fee)
}

func (p *Player) post(action string, trend Trend, price decimal.Decimal, size int64, after func()) (sheet int64, err error) {
	typ := 0
	switch action {
	case "open":
		typ = int(trend + 1)
	case "close":
		typ = int(trend + 3)
	default:
		panic("Unsupport action!")
	}
	f := func(orderid string) (sheet int64, ok bool) {
		// 确认成交数量, 未全部成交需撤销剩余订单
		var orderInfo *okex.BaseOrderInfo
	query:
		mustExecFunc(func() error {
			var err error
			orderInfo, err = p.client.OrderInfo(p.instrumentId, orderid)
			return err
		}, time.Second)
		total, _ := decimal.NewFromString(orderInfo.Size)
		qty, _ := decimal.NewFromString(orderInfo.FilledQty)
		if qty.IsZero() {
			return 0, false
		}
		switch orderInfo.Status {
		case "-1", "2":
		default:
			if qty.LessThan(total) {
				if err := p.client.Cancel(p.instrumentId, orderid); err != nil {
					glog.Errorln("p.client.Cancel", err)
				}
			}
			goto query
		}
		return qty.IntPart(), true
	}
	orderid, err := p.client.Post(p.instrumentId, &okex.BasePlaceOrderInfo{
		ClientOid:  genRandClientId(),
		Price:      price.String(),
		MatchPrice: "0",
		Type:       fmt.Sprintf("%d", typ),
		Size:       fmt.Sprintf("%d", size),
		OrderType:  "1",
	})
	if err != nil {
		return
	}
	var ok bool
	for range time.Tick(time.Second) {
		if sheet, ok = f(orderid); ok {
			break
		}
	}
	if after != nil {
		after()
	}
	return
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

func (c *Client) Post(instrumentId string, info *okex.BasePlaceOrderInfo) (string, error) {
	result, err := c.client.PostSwapOrder(instrumentId, info)
	if err != nil {
		return "", err
	}
	if result.OrderId == "" {
		return "", fmt.Errorf("orderid is empty! result:%+v", result)
	}
	return result.OrderId, nil
}

func (c *Client) OrderInfo(instrumentId string, orderOrClientId string) (*okex.BaseOrderInfo, error) {
	result, err := c.client.GetSwapOrderById(instrumentId, orderOrClientId)
	if err != nil {
		return nil, err
	}
	if result.OrderId == "" {
		return nil, fmt.Errorf("orderid is empty! result:%+v", result)
	}
	return result, nil
}

func (c *Client) Cancel(instrumentId string, orderid string) error {
	result, err := c.client.PostSwapCancelOrder(instrumentId, orderid)
	if err != nil {
		return err
	}
	if result.OrderId == "" {
		return fmt.Errorf("orderid is empty! result:%+v", result)
	}
	return nil
}

func (c *Client) Position(instrumentId string) (int64, error) {
	result, err := c.client.GetSwapPositionByInstrument(instrumentId)
	if err != nil {
		return 0, err
	}
	b, _ := json.Marshal(result)
	if _, ok := result["holding"]; !ok {
		return 0, fmt.Errorf("bad body! result:%+v", string(b))
	}
	val, _, _, err := jsonparser.Get(b, "holding")
	if err != nil {
		return 0, err
	}
	sheet := int64(0)
	errs := make(chan error, 10)
	_, err = jsonparser.ArrayEach(val, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		if sheet == 0 {
			availPosition, _ := jsonparser.GetString(value, "avail_position")
			position, _ := jsonparser.GetString(value, "position")
			ap, _ := decimal.NewFromString(availPosition)
			p, _ := decimal.NewFromString(position)
			if !ap.Equal(p) {
				errs <- fmt.Errorf("position not equal avail_position! avail_position:%s position:%s", availPosition, position)
				return
			}
			sheet = ap.IntPart()
		}
	})
	if err != nil {
		return 0, err
	}
	close(errs)
	for err := range errs {
		if err != nil {
			return 0, err
		}
	}
	return sheet, nil
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

func (c *Client) ServerTime() (time.Time, error) {
	serverTime, err := c.client.GetServerTime()
	if err != nil {
		return time.Time{}, err
	}
	t, err := time.Parse(time.RFC3339, serverTime.Iso)
	if err != nil {
		return time.Time{}, err
	}
	return t.In(time.UTC), nil
}

func (c *Client) AlgoOrder(instrumentId string, info *okex.BaseAlgoOrderInfo) (algoId string, err error) {
	result, err := c.client.PostSwapAlgoOrder(instrumentId, info)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(result)
	if _, ok := result["data"]; !ok {
		return "", fmt.Errorf("bad body! str:%s", string(b))
	}
	algoId, err = jsonparser.GetString(b, "data", "algo_id")
	if err != nil {
		return
	}
	if algoId == "-1" || algoId == "" {
		return "", fmt.Errorf("error algo_id! id:%s", algoId)
	}
	return algoId, nil
}

func (c *Client) CancelAlgoOrder(instrumentId string, info *okex.BaseCancelAlgoOrderInfo) error {
	result, err := c.client.PostSwapCancelAlgoOrder(instrumentId, info)
	if err != nil {
		return err
	}
	b, _ := json.Marshal(result)
	s, err := jsonparser.GetString(b, "data", "result")
	if err != nil {
		return err
	}
	if s != "success" {
		return fmt.Errorf("result is not success! result:%s", result)
	}
	return nil
}

func genRandClientId() string {
	return "a" + strconv.FormatInt(int64(rand.Int31()), 10)
}

func mustExecFunc(f func() error, d time.Duration) {
	for {
		err := f()
		if err != nil {
			glog.Infof("Exec %T failed! err:%v\n", f, err)
			time.Sleep(d)
			continue
		}
		return
	}
}

func init() {
	rand.Seed(time.Now().Unix())
}
