package main

import (
	"coinest/v1/api"
	"coinest/v1/config"
	"coinest/v1/worker"
	"fmt"
	"github.com/golang/glog"
	"os"
	"os/signal"
	"syscall"
)

func ExampleWebsocket() {
	swap := api.NewSwapWebsocket(128)
	err := swap.Login()
	if err != nil {
		panic(err)
	}
	fmt.Println("登录成功")
	err = swap.Sub(map[string]string{
		"swap/price_range:BTC-USD-SWAP": "swap/price_range",
	})
	if err != nil {
		panic(err)
	}
	go swap.Run()
	ch, _ := swap.Channel("swap/price_range")
	for v := range ch {
		fmt.Println(string(v))
	}
}

func Example() {
	worker.StartMargin()
}

func main() {
	config.Parse()
	api.Init()
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGKILL, syscall.SIGTERM)
	defer glog.Flush()
	go func() {
		select {
		case s := <-sig:
			fmt.Println("接受到信号:", s.String())
			glog.Flush()
			os.Exit(0)
		}
	}()
	glog.CopyStandardLogTo("INFO")
	Example()
	glog.Errorln("exit.")
}
