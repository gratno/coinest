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
