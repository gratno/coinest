package worker

import (
	"flag"
	"fmt"
	"github.com/golang/glog"
	"math"
	"time"
)

var point float64

func init() {
	flag.Float64Var(&point, "point", 1, "设置收益点结束")
}

func StartHedge(count ...int) {
	num := math.MaxInt64
	if len(count) > 0 {
		num = count[0]
	}
	var lastClosedInfo *ClosedInfo
	for i := 0; i < num; i++ {
		fmt.Printf("\n\n")
		t0 := time.Now()
		if closedInfo := hedgeWorker(lastClosedInfo); closedInfo != nil {
			lastClosedInfo = closedInfo
		}
		d := time.Since(t0)
		if d > 10*time.Second {
			glog.Infoln("投资成功！ costing:", d)
			fmt.Printf("\n\n")
			time.Sleep(30 * time.Second)
		} else {
			time.Sleep(3 * time.Second)
		}
	}
}

func StartMargin(count ...int) {
	num := math.MaxInt64
	if len(count) > 0 {
		num = count[0]
	}
	var lastClosedInfo *ClosedInfo
	for i := 0; i < num; i++ {
		fmt.Printf("\n\n")
		t0 := time.Now()
		if closedInfo := marginWorker(lastClosedInfo); closedInfo != nil {
			lastClosedInfo = closedInfo
		}
		d := time.Since(t0)
		if d > 10*time.Second {
			glog.Infoln("投资成功！ costing:", d)
			fmt.Printf("\n\n")
			time.Sleep(30 * time.Second)
		} else {
			time.Sleep(3 * time.Second)
		}
	}
}
