package main

//go:generate stringer -type=Trend -linecomment=true -output=trend_string.go
type Trend int

const (
	TREND_UNKNOWN Trend = -1 // 未知
	TREND_MANY    Trend = 0  // 看多
	TREND_EMPTY   Trend = 1  // 看空
)
