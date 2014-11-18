package main

import (
	"fmt"
	"net/http"
)

type ApiError struct {
	Message    string
	StatusCode int
}

func NewApiError(statusCode int, format string, a ...interface{}) error {
	return &ApiError{
		Message:    fmt.Sprintf(format, a...),
		StatusCode: statusCode,
	}
}

func (error *ApiError) Error() string {
	return error.Message
}

func InvalidDelayError(delay string) error {
	return NewApiError(http.StatusBadRequest,
		"Oopsie daisy! Could not convert delay <%s> to float", delay)
}

func DelayIsTooBigError(delayInSeconds float64) error {
	return NewApiError(http.StatusBadRequest,
		"Oopsie daisy! Delay can't be greater then %d seconds, got: %.0f", MAX_DELAY, delayInSeconds)
}

func ChangeBuiltinSiteError() error {
	return NewApiError(http.StatusForbidden, "Oopsie daisy! You can't change builtin sites")
}

func UnknownSiteError(site string) error {
	return NewApiError(http.StatusNotFound, "Oopsie daisy! Site <%s> doesn't exist", site)
}
