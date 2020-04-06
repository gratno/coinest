package api

import (
	"bytes"
	"coinest/v1/model"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
)

// 某个杠杆配置信息
func MarginAvailability(instrumentId string) (*model.MarginAvailability, error) {
	URL := fmt.Sprintf("/api/margin/v3/accounts/%s/availability", instrumentId)
	req := NewRequest("GET", URL, nil)
	response, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpClient.Do failed! err: %w", err)
	}
	defer response.Body.Close()
	b, _ := ioutil.ReadAll(response.Body)
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("GET %s failed! bad status code: %d %s", URL, response.StatusCode, string(b))
	}

	var abilities []*model.MarginAvailability
	err = json.Unmarshal(b, &abilities)
	if err != nil {
		return nil, fmt.Errorf("json.Decode failed! err: %w body:%s", err, string(b))
	}
	ability := abilities[0]
	if ability.InstrumentID == "" {
		return nil, fmt.Errorf("bad body %s", string(b))
	}
	return ability, nil
}

func SetMarginLeverage(instrumentId string, leverage int) error {
	const maxLeverage = 10
	if leverage > maxLeverage {
		leverage = maxLeverage
	}
	params := map[string]string{
		"leverage": fmt.Sprintf("%d", leverage),
	}
	b, _ := json.Marshal(params)
	URL := fmt.Sprintf("/api/margin/v3/accounts/%s/leverage", instrumentId)
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

	var body model.MarginLeverage
	err = json.Unmarshal(b, &body)
	if err != nil {
		return fmt.Errorf("json.Decode failed! err: %w body:%s", err, string(b))
	}
	if body.InstrumentID == "" {
		return fmt.Errorf("bad body %s", string(b))
	}
	glog.Infof("设置币币杠杆倍数成功! instrument_id: %s leverage:%s \n", body.InstrumentID, body.Leverage)
	return nil
}

func MarginOrder(params map[string]string) (*model.MarginOrder, error) {
	b, _ := json.Marshal(params)
	req := NewRequest("POST", "/api/margin/v3/orders", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	response, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpClient.Do failed! err: %w", err)
	}
	defer response.Body.Close()
	text := string(b)
	b, _ = ioutil.ReadAll(response.Body)
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("GET /api/margin/v3/orders failed! bad status code: %d body:%s params:%s", response.StatusCode, string(b), text)
	}
	var order model.MarginOrder
	err = json.Unmarshal(b, &order)
	if err != nil {
		return nil, fmt.Errorf("json.Decode failed! err: %w body:%s", err, string(b))
	}
	if order.ErrorCode != "0" {
		return nil, fmt.Errorf("bad body %s", string(b))
	}
	return &order, nil
}

func MarginCancelOrder(orderId string, instrumentId string) error {
	URL := fmt.Sprintf("/api/margin/v3/cancel_orders/%s", orderId)
	params := map[string]string{
		"instrument_id": instrumentId,
	}
	b, _ := json.Marshal(params)
	req := NewRequest("POST", URL, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	response, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("httpClient.Do failed! err: %w", err)
	}
	defer response.Body.Close()
	text := string(b)
	b, _ = ioutil.ReadAll(response.Body)
	if response.StatusCode != 200 {
		return fmt.Errorf("GET %s failed! bad status code: %d body:%s params:%s", URL, response.StatusCode, string(b), text)
	}
	var order model.MarginOrder
	err = json.Unmarshal(b, &order)
	if err != nil {
		return fmt.Errorf("json.Decode failed! err: %w body:%s", err, string(b))
	}
	if order.ErrorCode != "0" {
		return fmt.Errorf("bad body %s", string(b))
	}
	return nil
}

func MarginBorrow(params map[string]string) (*model.MarginBorrow, error) {
	b, _ := json.Marshal(params)
	req := NewRequest("POST", "/api/margin/v3/accounts/borrow", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	response, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpClient.Do failed! err: %w", err)
	}
	defer response.Body.Close()
	b, _ = ioutil.ReadAll(response.Body)
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("GET /api/margin/v3/accounts/borrow failed! bad status code: %d body:%s", response.StatusCode, string(b))
	}
	var borrow model.MarginBorrow
	err = json.Unmarshal(b, &borrow)
	if err != nil {
		return nil, fmt.Errorf("json.Decode failed! err: %w body:%s", err, string(b))
	}
	if borrow.BorrowID == "" {
		return nil, fmt.Errorf("bad body %s", string(b))
	}
	return &borrow, nil
}

func MarginRepay(params map[string]string) (*model.MarginBRepay, error) {
	b, _ := json.Marshal(params)
	req := NewRequest("POST", "/api/margin/v3/accounts/repayment", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	response, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpClient.Do failed! err: %w", err)
	}
	defer response.Body.Close()
	b, _ = ioutil.ReadAll(response.Body)
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("GET /api/margin/v3/accounts/repayment failed! bad status code: %d body:%s", response.StatusCode, string(b))
	}
	var repay model.MarginBRepay
	err = json.Unmarshal(b, &repay)
	if err != nil {
		return nil, fmt.Errorf("json.Decode failed! err: %w body:%s", err, string(b))
	}
	if repay.RepaymentID == "" {
		return nil, fmt.Errorf("bad body %s", string(b))
	}
	return &repay, nil
}

func MarginMarkPrice(instrumentId string) (*model.MarginMarkPrice, error) {
	URL := fmt.Sprintf("/api/margin/v3/instruments/%s/mark_price", instrumentId)
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
	var price model.MarginMarkPrice
	err = json.Unmarshal(b, &price)
	if err != nil {
		return nil, fmt.Errorf("json.Decode failed! err: %w body:%s", err, string(b))
	}
	if price.InstrumentID == "" {
		return nil, fmt.Errorf("bad body %s", string(b))
	}
	return &price, nil
}

func MarginAccount(instrumentId string) (*model.MarginAccount, error) {
	URL := fmt.Sprintf("/api/margin/v3/accounts/%s", instrumentId)
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

	var account model.MarginAccount
	err = json.Unmarshal(b, &account)
	if err != nil {
		var accounts []model.MarginAccount
		if err := json.Unmarshal(b, &accounts); err != nil {
			return nil, fmt.Errorf("json.Decode failed! err: %w", err)
		}
		account = accounts[0]
	}
	if account.LiquidationPrice == "" {
		return nil, fmt.Errorf("bad body %s", string(b))
	}
	return &account, nil
}

func MarginBorrowed(instrumentId string) ([]*model.MarginBorrowed, error) {
	URL := fmt.Sprintf("/api/margin/v3/accounts/%s/borrowed?limit=10&status=0", instrumentId)
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

	var borrowed []*model.MarginBorrowed
	err = json.Unmarshal(b, &borrowed)
	if err != nil {
		return nil, fmt.Errorf("json.Decode failed! err: %w", err)
	}
	return borrowed, nil
}

func MarginOrderDetail(instrumentId string, orderId string) (*model.MarginOrderDetail, error) {
	URL := fmt.Sprintf("/api/margin/v3/orders/%s?instrument_id=%s", orderId, instrumentId)
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

	var detail model.MarginOrderDetail
	err = json.Unmarshal(b, &detail)
	if err != nil {
		return nil, fmt.Errorf("json.Decode failed! err: %w body:%s", err, string(b))
	}

	if detail.InstrumentID == "" {
		return nil, fmt.Errorf("bad body %s", string(b))
	}
	return &detail, nil
}
