// Code generated by "stringer -type=Trend -linecomment=true -output=trend_string.go"; DO NOT EDIT.

package main

import "strconv"

const _Trend_name = "未知看多看空"

var _Trend_index = [...]uint8{0, 6, 12, 18}

func (i Trend) String() string {
	i -= -1
	if i < 0 || i >= Trend(len(_Trend_index)-1) {
		return "Trend(" + strconv.FormatInt(int64(i+-1), 10) + ")"
	}
	return _Trend_name[_Trend_index[i]:_Trend_index[i+1]]
}
