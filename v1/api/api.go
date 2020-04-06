package api

import (
	"github.com/golang/glog"
	"net/http"
	"time"
)

var (
	ssc       *swapWebsocket
	apiDomain = "https://www.okex.com"
)

func Init() {
	ssc = NewSwapWebsocket(500)
	err := ssc.Login()
	if err != nil {
		glog.Fatal("登录失败!", err)
	}
	glog.Infoln("登录websocket服务器成功!")
	err = ssc.Sub(map[string]string{
		"swap/price_range:BTC-USDT-SWAP": "swap/price_range",
		"swap/depth:BTC-USDT-SWAP":       "swap/depth",
	})
	if err != nil {
		glog.Fatal("订阅失败!", err)
	}
	go ssc.Run()
	httpClient = &http.Client{}
	httpClient.Timeout = 5 * time.Second
	time.Sleep(time.Second)
}
