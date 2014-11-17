package main

import (
	"text/template"
)

var (
	BANNER_TEMPLATE = template.Must(template.New("banner").Parse(
		`===================== goslow ====================
`))

	CREATE_SITE_TEMPLATE = template.Must(template.New("create site").Parse(
		`Your goslow domain is {{ .Domain }}

Use admin-{{ .Domain }} for configuration.

Example:
Let's say you want to add an endpoint /christmas
and you want it to respond to GET requests with "hohoho" and 3 seconds delay.
Just make a POST request to your admin domain...
curl -d "hohoho" "admin-{{ .Domain }}/christmas?delay=3&method=GET"

... and you're done!

If you have any questions, don't hesitate to ask: codumentary.com@gmail.com`))

	ADD_RULE_TEMPLATE = template.Must(template.New("add rule").Parse(
		`Endpoint http://{{ .Domain }}{{ .Path }} responds to {{if .Method }}{{ .Method }}{{else}}any HTTP method{{ end }} {{ if .Delay }}with {{ .Delay }} delay{{ else }}without any delay{{end}}.
Response is: {{ .StringBody }}
`))
)
