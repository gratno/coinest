package worker

import "errors"

var (
	ErrOpenSuspend = errors.New("暂时不开仓")
	ErrZeroSize    = errors.New("size is zero")
)
