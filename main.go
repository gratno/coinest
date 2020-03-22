package main

import (
	"coinest/worker"
	"flag"
	"github.com/golang/glog"
)

func init() {
	flag.Parse()
	glog.CopyStandardLogTo("INFO")
	defer glog.Flush()
}

func main() {
	worker.StartHedge(1)
	glog.Errorln("exit.")
}
