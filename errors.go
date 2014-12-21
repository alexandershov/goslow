package main

import (
	"fmt"
	"net/http"
	"time"
)

const CANT_CREATE_SITE_ERROR = `Oopsie daisy!

Can't create a new site. It's not your fault.

Please try again in a few seconds or contact codumentary.com@gmail.com for help
`

type ApiError struct {
	Message    string
	StatusCode int
}

// NewApiError returns an ApiError with the the message made from the format string.
func NewApiError(statusCode int, format string, a ...interface{}) error {
	return &ApiError{
		Message:    fmt.Sprintf(format, a...),
		StatusCode: statusCode,
	}
}

func (error *ApiError) Error() string {
	return error.Message
}

func InvalidDelayError(rawDelay string) error {
	return NewApiError(http.StatusBadRequest,
		"Oopsie daisy! Could not convert delay <%s> to float.", rawDelay)
}

func DelayIsTooBigError(delay time.Duration) error {
	return NewApiError(http.StatusBadRequest,
		"Oopsie daisy! Delay can't be greater than %s, got delay %s",
		MAX_DELAY, delay)
}

func ChangeBuiltinSiteError() error {
	return NewApiError(http.StatusForbidden, "Oopsie daisy! You can't change builtin sites.")
}

func UnknownSiteError(site string) error {
	return NewApiError(http.StatusNotFound, "Oopsie daisy! Site <%s> doesn't exist.", site)
}

// TODO: rename to CantGenerateUniqueSiteNameError? (It is used in server.generateUniqueSiteName)
func CantCreateSiteError() error {
	return NewApiError(http.StatusInternalServerError, CANT_CREATE_SITE_ERROR)
}
