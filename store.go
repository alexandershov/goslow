package main

import (
	"net/http"
)

type Store interface {
	AddRule(*Rule)
	FindRuleFor(*http.Request) (rule *Rule, found bool)
}
