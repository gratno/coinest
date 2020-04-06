package api

import (
	"bytes"
	"coinest/v1/config"
	"coinest/v1/model"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/shopspring/decimal"
	"github.com/smallnest/weighted"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

var (
	httpClient *http.Client
)

func SwapTradeType(instrumentId string) config.TradeType {
	depths, ok := ssc.Channel("swap/depth")
	if !ok {
		glog.Fatal("no swap/depth ", ssc.tbms)
	}

	mark, err := SwapMarkPrice(instrumentId)
	if err != nil {
		glog.Errorln("获取标记价格失败!", err)
		return config.OPEN_PAUSE
	}

	markPrice, _ := decimal.NewFromString(mark.MarkPrice)
	fakers := make([][]FakerPrice, 0)
	for {
		if len(fakers) > 10 {
			break
		}
		select {
		case depth := <-depths:
			var sd model.SwapDepth
			if err := json.Unmarshal(depth, &sd); err != nil {
				glog.Errorln("json.Unmarshal to SwapDepth failed!", err)
				continue
			}

			arrs := make([]FakerPrice, 0)
			// 只看卖方
			for _, v := range sd.Data {
				if time.Now().In(time.UTC).Before(v.Timestamp.In(time.UTC).Add(2 * time.Second)) {
					if len(v.Asks) == 0 {
						continue
					}
					//glog.Infof("depth: %s now:%s ask: %s\n", v.Timestamp, time.Now(), v.Asks[0])

					for _, ask := range v.Asks {
						size, _ := strconv.Atoi(ask[1])
						arrs = append(arrs, FakerPrice{
							Price:    ask[0],
							Weighted: size,
						})
					}
				}
			}
			if len(fakers) > 0 && EqualsFakerPrice(arrs, fakers[len(fakers)-1]) {
				continue
			}
			fakers = append(fakers, arrs)
		}
	}

	prices := make([]FakerPrice, 0)

	markf, _ := markPrice.Float64()

	for _, faker := range fakers {
	lf:
		for _, v := range faker {
			f1, _ := strconv.ParseFloat(v.Price, 64)
			if f1-markf > 40 || markf-f1 > 40 {
				continue
			}
			for i, p := range prices {
				if v.Price == p.Price {
					prices[i].Weighted += v.Weighted
					continue lf
				}
			}
			prices = append(prices, v)

		}
	}

	dw := weighted.NewRandW()
	for _, v := range prices {
		dw.Add(v.Price, v.Weighted)
	}
	glog.Infof("标记价格:%s 预测趋势: %+v\n", markPrice.String(), prices)
	item := dw.Next().(string)
	expect, _ := decimal.NewFromString(item)
	if expect.GreaterThan(markPrice.Add(decimal.NewFromInt(5))) {
		return config.OPEN_MANY
	}

	if expect.LessThan(markPrice.Sub(decimal.NewFromInt(5))) {
		return config.OPEN_EMPTY
	}
	return config.OPEN_PAUSE
}

func SwapInstrumentPosition(instrumentId string) (*model.SwapInstrumentPosition, error) {
	URL := fmt.Sprintf("/api/swap/v3/%s/position", instrumentId)
	req := NewRequest("GET", URL, nil)
	response, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpClient.Do failed! err: %w", err)
	}
	defer response.Body.Close()
	b, _ := ioutil.ReadAll(response.Body)
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("GET %s failed! bad status code: %d body:%s", URL, response.StatusCode, string(b))
	}

	var position model.SwapInstrumentPosition
	err = json.Unmarshal(b, &position)
	if err != nil {
		return nil, fmt.Errorf("json.Decode failed! err: %w body:%s", err, string(b))
	}
	return &position, nil
}

func SwapOrder(params map[string]string) (*model.Order, error) {
	b, _ := json.Marshal(params)
	req := NewRequest("POST", "/api/swap/v3/order", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	response, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpClient.Do failed! err: %w", err)
	}
	defer response.Body.Close()
	b, _ = ioutil.ReadAll(response.Body)
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("GET /api/swap/v3/order failed! bad status code: %d body:%s", response.StatusCode, string(b))
	}

	var order model.Order
	err = json.Unmarshal(b, &order)
	if err != nil {
		return nil, fmt.Errorf("json.Decode failed! err: %w body:%s", err, string(b))
	}

	if order.OrderID == "" {
		return nil, fmt.Errorf("bad body %s", string(b))
	}
	return &order, nil
}

func SwapFills(params map[string]string) ([]*model.FillDetail, error) {
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	req := NewRequest("GET", "/api/swap/v3/fills", nil)
	response, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpClient.Do failed! err: %w", err)
	}
	defer response.Body.Close()
	b, _ := ioutil.ReadAll(response.Body)
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("GET /api/swap/v3/fills failed! bad status code: %d body:%s", response.StatusCode, string(b))
	}
	var details []*model.FillDetail
	err = json.Unmarshal(b, &details)
	if err != nil {
		return nil, fmt.Errorf("json.Decode failed! err: %w body:%s", err, string(b))
	}
	return details, nil

}

func SwapOrderInfo(instrumentId string, orderId string) (*model.OrderInfo, error) {
	URL := fmt.Sprintf("api/swap/v3/orders/%s/%s", instrumentId, orderId)
	req := NewRequest("GET", URL, nil)
	response, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpClient.Do failed! err: %w", err)
	}
	defer response.Body.Close()
	b, _ := ioutil.ReadAll(response.Body)
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("GET %s failed! bad status code: %d body:%s", URL, response.StatusCode, string(b))
	}

	var account model.OrderInfo
	err = json.Unmarshal(b, &account)
	if err != nil {
		return nil, fmt.Errorf("json.Decode failed! err: %w body:%s", err, string(b))
	}
	if account.InstrumentID == "" {
		return nil, fmt.Errorf("bad body %s", string(b))
	}
	return &account, nil
}

func SwapMarkPrice(instrumentId string) (*model.SwapMarkPrice, error) {
	URL := fmt.Sprintf("/api/swap/v3/instruments/%s/mark_price", instrumentId)
	req := NewRequest("GET", URL, nil)
	response, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpClient.Do failed! err: %w", err)
	}
	defer response.Body.Close()
	b, _ := ioutil.ReadAll(response.Body)
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("GET %s failed! bad status code: %d body:%s", URL, response.StatusCode, string(b))
	}

	var price model.SwapMarkPrice
	err = json.Unmarshal(b, &price)
	if err != nil {
		return nil, fmt.Errorf("json.Decode failed! err: %w body:%s", err, string(b))
	}

	if price.InstrumentID == "" {
		return nil, fmt.Errorf("bad body %s", string(b))
	}
	return &price, nil
}

func SetSwapLeverage(instrumentId string, leverage int) error {
	const maxLeverage = 20
	if leverage > maxLeverage {
		leverage = maxLeverage
	}
	params := map[string]string{
		"leverage": fmt.Sprintf("%d", leverage),
		"side":     "3",
	}
	b, _ := json.Marshal(params)
	URL := fmt.Sprintf("/api/swap/v3/accounts/%s/leverage", instrumentId)
	req := NewRequest("POST", URL, bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	response, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("httpClient.Do failed! err: %w", err)
	}
	defer response.Body.Close()
	b, _ = ioutil.ReadAll(response.Body)
	if response.StatusCode != 200 {
		return fmt.Errorf("GET %s failed! bad status code: %d body:%s", URL, response.StatusCode, string(b))
	}

	var body model.SwapLeverage
	err = json.Unmarshal(b, &body)
	if err != nil {
		return fmt.Errorf("json.Decode failed! err: %w body:%s", err, string(b))
	}
	if body.InstrumentID == "" {
		return fmt.Errorf("bad body %s", string(b))
	}
	glog.Infof("设置币币杠杆倍数成功! instrument_id: %s leverage:%s\n", body.InstrumentID, body.LongLeverage)
	return nil

}

func SwapAccount(instrumentId string) (*model.SwapAccount, error) {
	URL := fmt.Sprintf("/api/swap/v3/%s/accounts", instrumentId)
	req := NewRequest("GET", URL, nil)
	response, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpClient.Do failed! err: %w", err)
	}
	defer response.Body.Close()
	b, _ := ioutil.ReadAll(response.Body)
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("GET %s failed! bad status code: %d body:%s", URL, response.StatusCode, string(b))
	}

	var account model.SwapAccount
	err = json.Unmarshal(b, &account)
	if err != nil {
		return nil, fmt.Errorf("json.Decode failed! err: %w body:%s", err, string(b))
	}

	return &account, nil
}
