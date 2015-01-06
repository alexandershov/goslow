package main

import (
	"text/template"
	"time"
)

type TemplateData struct {
	Site              string // e.g k38skjdf
	Path              string
	Method            string
	Delay             time.Duration
	TruncatedResponse string
	CreateDomain      string // e.g create.goslow.link
	Domain            string // e.g k38skjdf.goslow.link
	AdminDomain       string // e.g admin-k38skjdf.goslow.link
	AdminPathPrefix   string
}

func makeTemplate(name, text string) *template.Template {
	return template.Must(template.New(name).Parse(text))
}

var (
	BANNER_TEMPLATE = makeTemplate("banner",
		`===================== goslow ====================
`)

	EXAMPLE_ADD_ENDPOINT_TEMPLATE = makeTemplate("example add endpoint",
		`Example:
Let's say you want to add an endpoint {{ .Path }}
and you want it to respond to GET requests with "{{ .TruncatedResponse }}" and 2.5 seconds delay.

Just make a POST request ...
curl -d "{{ .TruncatedResponse }}" "{{ .AdminDomain }}{{ .AdminPathPrefix }}{{ .Path }}?delay=2.5&method=GET"

... and you're done!
`)

	EXAMPLE_CREATE_SITE_TEMPLATE = makeTemplate("example create site",
		`Example:
To create a new site make a POST request ...
curl -d "{{ .TruncatedResponse }}" "{{ .CreateDomain }}{{ .AdminPathPrefix }}{{ .Path }}?delay=2.5&method=GET"

... and you're done!
`)

	SITE_CREATED_TEMPLATE = makeTemplate("site created",
		`Your personal goslow domain is {{ .Domain }}
You can configure it with the POST requests to {{ .AdminDomain }}
`)

	ENDPOINT_ADDED_TEMPLATE = makeTemplate("endpoint added",
		"Hooray!\n"+
			"Endpoint http://{{ .Domain }}{{ .Path }} responds to {{ or .Method \"any HTTP Method\"}} "+
			"{{ if .Delay }}with {{ .Delay }} delay{{ else }}without any delay{{end}}.\n"+
			"Response is: {{ or .TruncatedResponse \"<EMPTY>\"}}\n")

	UNKNOWN_ENDPOINT_TEMPLATE = makeTemplate("unknown endpoint",
		`Oopsie daisy!
Endpoint http://{{ .Domain }}{{ .Path }} isn't configured yet.
`)

	// TODO: rename, name looks like it helps to create sites, but actually it
	// handles bad requests to create.goslow.link
	HELP_CREATE_SITE_TEMPLATE = makeTemplate("help create site",
		`Oopsie daisy!
Make a POST request to http://{{ .CreateDomain }} to create new endpoints.
`)

	// TODO: rename
	UNKNOWN_ERROR_TEMPLATE = makeTemplate("unknown error",
		`Oopsie daisy!
Server is probably misconfigured. It's not your fault.

Please contact codumentary.com@gmail.com for help.
`)

	UNKNOWN_SITE_TEMPLATE = makeTemplate("unknown site",
		`Oopsie daisy!
Site {{ .Site }} doesn't exist.
`)
)
