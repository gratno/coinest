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
	worker.StartHedge(1)
	glog.Errorln("exit.")
}
