package main

import (
	"coinest/v2/config"
	"coinest/v2/goex"
	"coinest/v2/goex/builder"
	"coinest/v2/goex/okex"
	"context"
	"encoding/json"
	"fmt"
	"github.com/buger/jsonparser"
	"github.com/golang/glog"
	"github.com/shopspring/decimal"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	currencyExchange = make(map[string]decimal.Decimal)
	// 止损点
	disloss float64 = 1000
	// 止盈点
	disgain float64 = 5
)

func init() {
	rand.Seed(20200328 << 3)
}

func Init(client *http.Client) {
	currencies := []string{"KRW"}
	for _, v := range currencies {
		dst := getCurrencyExchange(client, v)
		if dst.IsZero() {
			panic(fmt.Sprintf("Currency convert result is Zero! Currency:%s", v))
		}
		currencyExchange[v] = dst
	}
}

func getCurrencyExchange(client *http.Client, currency string) (dst decimal.Decimal) {
	currencyPair := fmt.Sprintf("%s_%s", "USD", strings.ToUpper(currency))
	s := fmt.Sprintf("https://free.currconv.com/api/v7/convert?q=%s&compact=y&apiKey=1e800a5fdc615d6dcf4c", currencyPair)
	resp, err := client.Get(s)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	data := make(map[string]interface{})
	b, _ := ioutil.ReadAll(resp.Body)
	_ = json.Unmarshal(b, &data)
	val := data[currencyPair].(map[string]interface{})
	dst, _ = decimal.NewFromString(fmt.Sprintf("%v", val["val"]))
	return
}

type Args struct {
	Exchange    string
	Currency    *goex.CurrencyPair
	ConvertFlag bool
}

type Exchange struct {
	Api  goex.API
	Args Args
}

type Task struct {
	// 负亏损
	Disloss decimal.Decimal
	// 正收益
	Disgain        decimal.Decimal
	last           decimal.Decimal
	current        decimal.Decimal
	trend          *Trend
	profile        decimal.Decimal
	serverTime     time.Time
	wsLock         sync.Mutex
	WS             *okex.OKExV3Ws
	client         *Client
	tickerPrice    decimal.Decimal
	tickerPriceMtx sync.RWMutex

	fundingRate decimal.Decimal

	depthStream  chan []byte
	tickerStream chan []byte

	apiBuilder *builder.APIBuilder
	proxyUrl   string
	futureModel
}

func (t *Task) websocketSub() {
	t.wsLock.Lock()
	defer t.wsLock.Unlock()
	ws := okex.NewOKExV3Ws(t.apiBuilder.Build(goex.OKEX_V3).(*okex.OKEx), t.WSRegister(map[string]chan<- []byte{
		"depth5": t.depthStream,
		"ticker": t.tickerStream,
	}))
	ws.ProxyUrl(t.proxyUrl)
	if err := ws.Subscribe(map[string]interface{}{
		"op":   "subscribe",
		"args": []string{"swap/depth5:BTC-USD-SWAP"},
	}); err != nil {
		panic(err)
	}
	if err := ws.Subscribe(map[string]interface{}{
		"op":   "subscribe",
		"args": []string{"swap/ticker:BTC-USD-SWAP"},
	}); err != nil {
		panic(err)
	}
	t.WS = ws
}

func (t *Task) Init() {
	t.client = NewClient()
	for i := 0; i < 3; i++ {
		if now, err := t.client.ServerTime(); err == nil {
			t.serverTime = now
			break
		} else {
			glog.Errorln(err)
		}
	}
	if t.serverTime.IsZero() {
		panic("init server time failed!")
	}
	t.depthStream = make(chan []byte)
	t.tickerStream = make(chan []byte)
}

func (t *Task) Now() time.Time {
	t.serverTime = t.serverTime.Add(time.Since(t.serverTime))
	return t.serverTime
}

func (t *Task) TickerPrice() decimal.Decimal {
	t.tickerPriceMtx.RLock()
	defer t.tickerPriceMtx.RUnlock()
	return t.tickerPrice
}

func (t *Task) SetTickerPrice(price decimal.Decimal) {
	t.tickerPriceMtx.Lock()
	t.tickerPrice = price
	t.tickerPriceMtx.Unlock()
}

func (t *Task) IsValidServerTime(timestamp string) bool {
	if svr, err := time.Parse(time.RFC3339, timestamp); err == nil {
		diff := time.Now().Sub(svr)
		if diff < 0 {
			diff = -diff
		}
		if diff < time.Second {
			return true
		}
		//glog.Infoln("now far from svc...", diff)
	}
	return false
}

func (t *Task) Worker() {
	parseTickerPrice := func(data []byte) (price decimal.Decimal) {
		val, err := jsonparser.GetString(data, "last")
		if err != nil {
			glog.Errorln("jsonparser.GetString:mark_price", err)
			return
		}
		timestamp, _ := jsonparser.GetString(data, "timestamp")
		if !t.IsValidServerTime(timestamp) {
			return
		}
		price, _ = decimal.NewFromString(val)
		return
	}
	parseDepth := func(data []byte, depths []depth) []depth {
		jsonparser.ArrayEach(data, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
			switch dataType {
			case jsonparser.Array:
				var depth depth
				arr := make([]string, 0)
				jsonparser.ArrayEach(value, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
					switch dataType {
					case jsonparser.String:
						arr = append(arr, string(value))
					}
				})
				depth.CopyFrom(arr)
				depths = append(depths, depth)
			}
		})
		return depths
	}

	stop := make(chan struct{})
	dumpTicker := time.NewTicker(30 * time.Second)
	fundingRateTicker := time.NewTicker(time.Hour)
	defer fundingRateTicker.Stop()
	defer dumpTicker.Stop()
	player := NewPlayer("BTC-USD-SWAP", t.client)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for {
			select {
			case <-fundingRateTicker.C:
				if t.fundingRate.IsZero() {
					if ft, err := t.client.FundingRate("BTC-USD-SWAP"); err == nil {
						t.fundingRate = ft
					}
				}
			case <-dumpTicker.C:
				t.Dump()
				player.Dump()
			case data := <-t.tickerStream:
				if d := parseTickerPrice(data); !d.IsZero() {
					t.SetTickerPrice(d)
				}
			}
		}
	}()
	go func() {
		for {
			select {
			case data := <-t.depthStream:
				timestamp, _ := jsonparser.GetString(data, "timestamp")
				if !t.IsValidServerTime(timestamp) {
					continue
				}
				var (
					asks, bids []depth
				)
				asksb, _, _, _ := jsonparser.Get(data, "asks")
				bidsb, _, _, _ := jsonparser.Get(data, "bids")
				asks = parseDepth(asksb, asks)
				bids = parseDepth(bidsb, bids)
				t.current = t.TickerPrice()
				model := &OkexReal{
					Asks:        fmt.Sprintf("%v", asks),
					Bids:        fmt.Sprintf("%v", bids),
					Ticker:      t.current.String(),
					FundingRate: t.fundingRate.String(),
				}
				if err := db.New().Create(model).Error; err != nil {
					glog.Errorln(err)
				}
				return
				if t.current.IsZero() {
					continue
				}
				future, _ := t.Future(t.current, asks, bids)
				//t.current = nextPrice
				if t.trend != nil {
					if ok, fee := t.Settlement(); ok {
						glog.Infof("结算: trend:%s last:%s current:%s fee:%s asks:%v bids:%v\n", t.trend, t.last, t.current, fee, asks, bids)
						err := player.Close(t.current, 60, fee.IsNegative())
						if err != nil {
							glog.Errorln("player.Close failed!", err)
							continue
						}
						// 直到可平仓剩余张数为0
						if player.Status() != STATUS_OK {
							continue
						}
						// 撤销止损单
						cancel()
						t.profile = t.profile.Add(fee)
						t.trend = nil
						ctx, cancel = context.WithCancel(context.Background())
					}
					continue
				}
				if future == TREND_UNKNOWN {
					continue
				}
				glog.Infof("开始: trend:%s last:%s current:%s asks:%v bids:%v \n", t.trend, t.last, t.current, asks, bids)
				err := player.Open(future, t.current, 60)
				if err != nil {
					glog.Errorln("player.Open failed!", err)
					continue
				}
				t.trend = &future
				t.last = t.current
				go func() {
					if disloss < 1000 {
						if err := player.Algo(ctx, future, t.current); err != nil {
							glog.Errorln("player.Algo", err)
						}
					}
				}()
			case <-time.After(30 * time.Second):
				t.websocketSub()
			}
		}
	}()
	<-stop
}

func (t *Task) Dump() {
	glog.Infof("Dump: trend:%v last:%s current:%s profile:%s\n", t.trend, t.last, t.current, t.profile)
}

func (t *Task) Settlement() (ok bool, fee decimal.Decimal) {
	if t.trend != nil {
		diff := t.current.Sub(t.last)
		switch *t.trend {
		case TREND_MANY:
		case TREND_EMPTY:
			diff = diff.Neg()
		}
		if diff.GreaterThanOrEqual(t.Disgain) || diff.LessThanOrEqual(t.Disloss) {
			return true, diff
		}
	}

	return false, decimal.Decimal{}
}

func (t *Task) AsksDepth(api goex.API, args Args) ([]decimal.Decimal, error) {
	if args.Currency == nil {
		args.Currency = &goex.BTC_USD
	}
	depth, err := api.GetDepth(10, *args.Currency)
	if err != nil {
		return nil, err
	}
	var list []decimal.Decimal
	for _, v := range depth.AskList {
		price := decimal.NewFromFloat(v.Price)
		if d := currencyExchange[args.Currency.CurrencyB.String()]; args.ConvertFlag {
			price = price.Div(d)
		}
		price.Truncate(2)
		list = append(list, price)
	}
	return list, nil
}

func (t *Task) WSRegister(chs map[string]chan<- []byte) func(channel string, data json.RawMessage) error {
	return func(channel string, data json.RawMessage) error {
		b, err := data.MarshalJSON()
		if err != nil {
			glog.Errorln("[WS]", err)
			return nil
		}
		result := make([]map[string]interface{}, 0)
		if err = json.Unmarshal(b, &result); err == nil {
			res := result[0]
			b, _ := json.Marshal(res)
			switch channel {
			case "ticker":
				chs["ticker"] <- b
			case "depth5":
				chs["depth5"] <- b
			}
		} else {
			glog.Errorln("json.Unmarshal failed!", err, string(b))
		}
		return nil
	}

}

func main() {
	defer glog.Flush()
	os.Setenv("GOEX_LOG_LEVEL", "ERROR")
	config.Parse()
	//proxyUrl := "socks5://127.0.0.1:1080"
	proxyUrl := ""
	apiBuilder := builder.NewAPIBuilder().HttpTimeout(5 * time.Second).HttpProxy(proxyUrl)

	//Init(apiBuilder.GetHttpClient())
	exchanges := make([]Exchange, 0)

	for _, v := range []Args{
		{Exchange: goex.BIGONE, Currency: &goex.BTC_USDT},
		{Exchange: goex.BINANCE, Currency: &goex.BTC_USDT},
		{Exchange: goex.BITFINEX},
		//{Exchange: goex.BITHUMB, Currency: &goex.BTC_KRW, ConvertFlag: true},
		{Exchange: goex.BITSTAMP},
		{Exchange: goex.BITTREX},
		{Exchange: goex.COINEX, Currency: &goex.BTC_USDT},
		{Exchange: goex.GATEIO, Currency: &goex.BTC_USDT},
		{Exchange: goex.HITBTC},
		{Exchange: goex.HUOBI_PRO},
		{Exchange: goex.KRAKEN},
		{Exchange: goex.KUCOIN, Currency: &goex.BTC_USDT},
		{Exchange: goex.OKEX_V3},
		{Exchange: goex.POLONIEX},
		{Exchange: goex.ZB},
	} {
		exchanges = append(exchanges, Exchange{
			Api:  apiBuilder.Build(v.Exchange),
			Args: v,
		})
	}

	loss := decimal.NewFromFloat(disloss).Neg()
	gain := decimal.NewFromFloat(disgain)
	t := Task{Disloss: loss, Disgain: gain, apiBuilder: apiBuilder, proxyUrl: proxyUrl}
	t.Init()
	t.websocketSub()
	t.Worker()
}
