package main

import (
	"text/template"
	"time"
)

type TemplateData struct {
	Site              string
	Path              string
	Method            string
	Delay             time.Duration
	TruncatedResponse string
	CreateDomain      string
	Domain            string
	AdminDomain       string
	AdminPathPrefix   string
}

// TODO: add helper to create templates easily

var (
	BANNER_TEMPLATE = template.Must(template.New("banner").Parse(
		`===================== goslow ====================
`))

	ADD_ENDPOINT_EXAMPLE_TEMPLATE = template.Must(template.New("add endpoint example").Parse(
		`Example:
Let's say you want to add an endpoint {{ .Path }}
and you want it to respond to GET requests with "{{ .TruncatedResponse }}" and 2.5 seconds delay.

Just make a POST request to your admin domain ...
curl -d "{{ .TruncatedResponse }}" "{{ .AdminDomain }}{{ .AdminPathPrefix }}{{ .Path }}?delay=2.5&method=GET"

... and you're done!
`))

	// TODO: remove duplication with ADD_ENDPOINT_EXAMPLE_TEMPLATE
	CREATE_SITE_EXAMPLE_TEMPLATE = template.Must(template.New("create site example").Parse(
		`Example:
To create a new site make a POST request ...
curl -d "{{ .TruncatedResponse }}" "{{ .CreateDomain }}{{ .AdminPathPrefix }}{{ .Path }}?delay=2.5&method=GET"
... and you're done!
`))

	SITE_CREATED_TEMPLATE = template.Must(template.New("site created").Parse(
		`Your personal goslow domain is {{ .Domain }}
You can configure it with POST requests to {{ .AdminDomain }}
`))

	ENDPOINT_ADDED_TEMPLATE = template.Must(template.New("endpoint added").Parse(
		`Hooray!
Endpoint http://{{ .Domain }}{{ .Path }} responds to {{if .Method }}{{ .Method }}{{else}}any HTTP method{{ end }} {{ if .Delay }}with {{ .Delay }} delay{{ else }}without any delay{{end}}.
Response is: {{ if .TruncatedResponse }}{{ .TruncatedResponse }}{{ else }}<EMPTY>{{ end }}
`))

	UNKNOWN_ENDPOINT_TEMPLATE = template.Must(template.New("unknown endpoint").Parse(
		`Oopsie daisy! Endpoint http://{{ .Domain }}{{ .Path }} isn't configured yet.
`))

	// TODO: rename, too similary to SITE_CREATED_TEMPLATE
	// TODO: remove duplication with SITE_CREATED_TEMPLATE
	CREATE_SITE_HELP_TEMPLATE = template.Must(template.New("create site help").Parse(
		`Oopsie daisy!
Make a POST request to http://{{ .CreateDomain }} to create new endpoints.
`))

	// TODO: rename
	UNKNOWN_ERROR_TEMPLATE = template.Must(template.New("unknown error").Parse(
		`Oopsie daisy! Server is probably misconfigured. It's not your fault.

Please contact codumentary.com@gmail.com for help.
`))

	// TODO: create.link should depend on config.deployedOn
	UNKNOWN_SITE_TEMPLATE = template.Must(template.New("unknown site").Parse(
		`Oopsie daisy! Site {{ .Site }} doesn't exist.
`))

	CONTACT_TEMPLATE = template.Must(template.New("contact").Parse(
		`If you have any questions, don't hesitate to ask: codumentary.com@gmail.com`))
)
