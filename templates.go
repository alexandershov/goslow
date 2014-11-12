package main

import (
	"text/template"
)

var (
	CREATE_SITE_TEMPLATE = template.Must(template.New("create site").Parse(
		`Site {{ .Domain }} was created successfully.

Use admin-{{ .Domain }} for configuration.
`))

	ADD_RULE_TEMPLATE = template.Must(template.New("add rule").Parse(
		`{{ .Domain }}{{ .Path }} now responds with {{ .ResponseBody }}.
`))
)
