package config

import (
	"encoding/json"
	"flag"
	"github.com/golang/glog"
	"io/ioutil"
)

var (
	SecretKey  = ""
	ApiKey     = ""
	Passphrase = ""
	configPath = ""
	Leverage   = 10
)

func init() {
	flag.IntVar(&Leverage, "leverage", 10, "set leverage")
	flag.StringVar(&SecretKey, "secret_key", "", "set secret_key")
	flag.StringVar(&ApiKey, "api_key", "", "set api_key")
	flag.StringVar(&Passphrase, "passphrase", "", "set passphrase")
	flag.StringVar(&configPath, "config", "/etc/coinest.json", "set config location")
}

func Parse() {
	flag.Parse()
	if configPath != "" {
		b, _ := ioutil.ReadFile(configPath)
		m := make(map[string]interface{})
		if err := json.Unmarshal(b, &m); err == nil {
			SecretKey = m["secret_key"].(string)
			ApiKey = m["api_key"].(string)
			Passphrase = m["passphrase"].(string)
			f := m["leverage"].(float64)
			Leverage = int(f)
		}
	}
	glog.Infof("config is secret_key=%s api_key=%s passphrase=%s leverage=%d\n", SecretKey, ApiKey, Passphrase, Leverage)
}
