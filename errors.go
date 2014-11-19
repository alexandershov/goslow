package main

import (
	"fmt"
	"net/http"
)

const CANT_CREATE_SITE_ERROR = `Can't create.
Try again in a few seconds or contact codumentary.com@gmail.com for help`


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
		"Oopsie daisy! Could not convert delay <%s> to float.", delay)
}

func DelayIsTooBigError(delayInSeconds float64) error {
	return NewApiError(http.StatusBadRequest,
		"Oopsie daisy! Delay can't be greater then %d seconds, got delay %.0f seconds.",
    MAX_DELAY, delayInSeconds)
}

func ChangeBuiltinSiteError() error {
	return NewApiError(http.StatusForbidden, "Oopsie daisy! You can't change builtin sites.")
}

func UnknownSiteError(site string) error {
	return NewApiError(http.StatusNotFound, "Oopsie daisy! Site <%s> doesn't exist.", site)
}

func CantCreateSiteError() error {
  return NewApiError(http.StatusInternalServerError, CANT_CREATE_SITE_ERROR)
}
