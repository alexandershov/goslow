package main

import (
	"text/template"
)

var (
	BANNER_TEMPLATE = template.Must(template.New("banner").Parse(
		`=========================================
`))

	CREATE_SITE_TEMPLATE = template.Must(template.New("create site").Parse(
		`Your goslow domain is {{ .Domain }}

Use admin-{{ .Domain }} for configuration.

Example.
If you want an endpoint {{ .Domain }}/xmas to respond to GET requests with "hohoho" and 3 seconds delay,
then make a POST request:
curl -X POST -d "hohoho" admin-{{ .Domain }}/xmas?delay=3&method=GET
`))

	ADD_RULE_TEMPLATE = template.Must(template.New("add rule").Parse(
		`Endpoint {{ .Domain }}{{ .Path }} now responds to {{if .Method }}{{ .Method }}{{else}}any HTTP method{{ end }} {{ if .Delay }}with the delay {{ .Delay }}{{ else }}without delay{{end}}. Response is:
{{ .StringBody }}
`))
)
