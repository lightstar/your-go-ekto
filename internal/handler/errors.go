package handler

import (
	"errors"
)

var (
	ErrServiceRequired   = errors.New("service required")
	ErrLoggerRequired    = errors.New("logger required")
	ErrAPIPrefixRequired = errors.New("API prefix required")
)
