package config

import (
	"encoding/json"
	"flag"
	"io/ioutil"
)

var (
	SecretKey  = ""
	ApiKey     = ""
	Passphrase = ""
	configPath = ""
)

func init() {
	flag.StringVar(&SecretKey, "secret_key", "", "set secret_key")
	flag.StringVar(&ApiKey, "api_key", "", "set api_key")
	flag.StringVar(&Passphrase, "passphrase", "", "set passphrase")
	flag.StringVar(&configPath, "config", "/etc/coinest.json", "set config location")
}

func Parse() {
	flag.Parse()
	if configPath != "" {
		b, _ := ioutil.ReadFile(configPath)
		m := make(map[string]string)
		if err := json.Unmarshal(b, &m); err == nil {
			SecretKey = m["secret_key"]
			ApiKey = m["api_key"]
			Passphrase = m["passphrase"]
		}
	}
}

type TradeType int

const (
	OPEN_PAUSE  TradeType = -1
	OPEN_MANY   TradeType = 1
	OPEN_EMPTY  TradeType = 2
	CLOSE_MANY  TradeType = 3
	CLOSE_EMPTY TradeType = 4
)

var tradesm = map[TradeType]string{
	OPEN_PAUSE:  "不开",
	OPEN_MANY:   "看多",
	OPEN_EMPTY:  "看空",
	CLOSE_MANY:  "平多",
	CLOSE_EMPTY: "平空",
}

func (t TradeType) String() string {
	return tradesm[t]
}
