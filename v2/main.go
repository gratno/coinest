package main

import (
	"coinest/v2/goex"
	"coinest/v2/goex/builder"
	"coinest/v2/goex/okex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/golang/glog"
	"github.com/shopspring/decimal"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	currencyExchange = make(map[string]decimal.Decimal)
)

const (
	DOWN    = "down"
	UP      = "up"
	UNKNOWN = "unknown"
)

func init() {
	rand.Seed(20200328 << 3)
}

func Avg(src []decimal.Decimal) decimal.Decimal {
	dst := make([]decimal.Decimal, len(src))
	copy(dst, src)
	sort.SliceStable(dst, func(i, j int) bool {
		return dst[i].LessThan(dst[j])
	})
	dst = dst[1 : len(dst)-1]
	avg := decimal.Avg(dst[0], dst[1:]...)
	avg = avg.Truncate(2)
	return avg
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
	fmt.Println(string(b))
	_ = json.Unmarshal(b, &data)
	val := data[currencyPair].(map[string]interface{})
	dst, _ = decimal.NewFromString(fmt.Sprintf("%v", val["val"]))
	return
}

type K []decimal.Decimal

func (k K) Len() int {
	return len(k)
}

func (k K) Time(i int) string {
	n := k.Len()
	if n < i {
		return UNKNOWN
	}
	dst := make([]decimal.Decimal, i)
	copy(dst, k[n-i:n])
	var direct *bool
	for i := 1; i < len(dst); i++ {
		b := dst[i].GreaterThan(dst[i-1])
		if direct == nil {
			direct = &b
		} else if b != *direct {
			goto _UNKNOWN
		}
	}

	if !*direct {
		return DOWN
	} else {
		return UP
	}

_UNKNOWN:
	first, end := dst[0], dst[len(dst)-1]
	max, min, avg := decimal.Max(dst[0], dst[1:]...), decimal.Min(dst[0], dst[1:]...), decimal.Avg(dst[0], dst[1:]...)
	if avg.LessThan(first) && min.Equal(end) {
		return DOWN
	}
	if avg.GreaterThan(first) && max.Equal(end) {
		return UP
	}

	return UNKNOWN

}

func (k K) Shadow(dst, delta decimal.Decimal) string {
	if dst.IsZero() {
		return UNKNOWN
	}
	avg := Avg(k)
	gtc := 0
	for _, v := range k {
		if v.GreaterThan(avg) {
			gtc++
		}
	}
	ltc := k.Len() - gtc
	d := dst.Sub(avg)
	neg := false
	if d.IsNegative() {
		d = d.Neg()
		neg = true
	}
	if d.GreaterThan(delta) {
		if neg {
			if ltc >= k.Len()/2 {
				return DOWN
			}
		} else if gtc >= k.Len()/2 {
			return UP
		}
	}
	return UNKNOWN
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
	history         K
	current         map[string]decimal.Decimal
	exchanges       []Exchange
	size            int
	ws              *okex.OKExV3Ws
	real            decimal.Decimal
	lastTradedCount int64
	tradedCount     int64
}

func (t *Task) Worker() {
	dumpt := time.NewTicker(10 * time.Second)
	decidet := time.NewTicker(time.Minute)
	go func() {
		for {
			select {
			case <-dumpt.C:
				t.Dump()
				t.history = append(t.history, t.current[goex.OKEX_V3])
			case <-decidet.C:
				recent := atomic.LoadInt64(&t.tradedCount)
				trend := UNKNOWN
				quotes := make(K, 0)
				for _, v := range t.current {
					quotes = append(quotes, v)
				}
				source := ""
				if r := quotes.Shadow(t.real, decimal.New(30, 0)); r != UNKNOWN {
					trend = r
					source = "shadow"
				} else if r18 := t.history.Time(18); r18 != UNKNOWN && r18 == t.history.Time(6) {
					trend = r18
					source = "ktime"
				}
				glog.Infof("last:%d recent:%d real:%s source:%s 预判趋势: 【%s】 \n", t.lastTradedCount, recent, t.real, source, trend)
				t.lastTradedCount = recent
			}
		}
	}()

}

func (t *Task) Dump() {
	ch := make(chan struct {
		Name string
		Avg  decimal.Decimal
	}, len(t.exchanges))
	wg := sync.WaitGroup{}
	for _, exchange := range t.exchanges {
		wg.Add(1)
		go func(exchange Exchange) {
			defer wg.Done()
			depths, err := t.AsksDepth(exchange.Api, exchange.Args)
			if err != nil {
				glog.Errorln(exchange.Api.GetExchangeName(), err)
				return
			}
			ch <- struct {
				Name string
				Avg  decimal.Decimal
			}{Name: exchange.Args.Exchange, Avg: Avg(depths)}
		}(exchange)

	}
	wg.Wait()
	close(ch)
	t.current = make(map[string]decimal.Decimal)
	for v := range ch {
		t.current[v.Name] = v.Avg
	}
}

func (t *Task) AsksDepth(api goex.API, args Args) ([]decimal.Decimal, error) {
	if args.Currency == nil {
		args.Currency = &goex.BTC_USD
	}
	depth, err := api.GetDepth(t.size, *args.Currency)
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

func (t *Task) WSRegister(channel string, data json.RawMessage) error {
	atomic.AddInt64(&t.tradedCount, 1)
	b, err := data.MarshalJSON()
	if err != nil {
		glog.Errorln("[WS]", err)
		return nil
	}
	result := make([]map[string]string, 0)
	_ = json.Unmarshal(b, &result)
	if len(result) > 0 {
		if d, err := decimal.NewFromString(result[0]["price"]); err == nil {
			d = d.Truncate(2)
			t.real = d
		}
	}
	return nil
}

func main() {
	flag.Parse()
	defer glog.Flush()
	os.Setenv("GOEX_LOG_LEVEL", "ERROR")
	//proxyUrl := "socks5://127.0.0.1:1080"
	proxyUrl := ""
	apiBuilder := builder.NewAPIBuilder().HttpTimeout(5 * time.Second).HttpProxy(proxyUrl)

	Init(apiBuilder.GetHttpClient())
	exchanges := make([]Exchange, 0)

	for _, v := range []Args{
		{Exchange: goex.BIGONE, Currency: &goex.BTC_USDT},
		{Exchange: goex.BINANCE, Currency: &goex.BTC_USDT},
		{Exchange: goex.BITFINEX},
		{Exchange: goex.BITHUMB, Currency: &goex.BTC_KRW, ConvertFlag: true},
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

	t := Task{
		exchanges: exchanges,
		size:      10,
	}

	ws := okex.NewOKExV3Ws(apiBuilder.Build(goex.OKEX_V3).(*okex.OKEx), t.WSRegister)
	ws.ProxyUrl(proxyUrl)
	err := ws.Subscribe(map[string]interface{}{
		"op":   "subscribe",
		"args": []string{"spot/trade:BTC-USDT"},
	})
	if err != nil {
		panic(err)
	}
	t.ws = ws

	t.Worker()

	select {}

}
