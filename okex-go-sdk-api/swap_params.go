package okex

/*
 OKEX swap api parameter's definition
 @author Lingting Fu
 @date 2018-12-27
 @version 1.0.0
*/

type BasePlaceOrderInfo struct {
	ClientOid  string `json:"client_oid"`
	Price      string `json:"price"`
	MatchPrice string `json:"match_price"`
	Type       string `json:"type"`
	Size       string `json:"size"`
	OrderType  string `json:"order_type"`
}

type PlaceOrderInfo struct {
	BasePlaceOrderInfo
	InstrumentId string `json:"instrument_id"`
}

type PlaceOrdersInfo struct {
	InstrumentId string                `json:"instrument_id"`
	OrderData    []*BasePlaceOrderInfo `json:"order_data"`
}

type BaseAlgoOrderInfo struct {
	Type         string `json:"type"`
	Size         string `json:"size"`
	OrderType    string `json:"order_type"`
	TriggerPrice string `json:"trigger_price"`
	AlgoPrice    string `json:"algo_price"`
	AlgoType     string `json:"algo_type"`
}

type AlgoOrderInfo struct {
	BaseAlgoOrderInfo
	InstrumentId string `json:"instrument_id"`
}

type BaseCancelAlgoOrderInfo struct {
	AlgoIds   []string `json:"algo_ids"`
	OrderType string   `json:"order_type"`
}

type CancelAlgoOrderInfo struct {
	BaseCancelAlgoOrderInfo
	InstrumentId string `json:"instrument_id"`
}
