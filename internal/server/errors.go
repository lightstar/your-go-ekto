package server

import "errors"

var (
	ErrAddrRequired   = errors.New("address required")
	ErrLoggerRequired = errors.New("logger required")
)
