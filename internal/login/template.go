package login

import _ "embed"

//go:embed template.html
var loginTemplateHTML string

//go:embed loader.html
var loaderTemplateHTML string

//go:embed sw.js
var serviceWorkerJS string
