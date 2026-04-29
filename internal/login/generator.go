package login

import (
	"bytes"
	"strings"
	"text/template"
)

// GenerateLoginPage returns the HTML for the login page in the given locale.
// Falls back to English if the locale is unknown.
func GenerateLoginPage(locale string) string {
	s := LocaleStrings(locale)
	tmpl, err := template.New("login").Parse(loginTemplateHTML)
	if err != nil {
		return loginTemplateHTML
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, s); err != nil {
		return loginTemplateHTML
	}
	return buf.String()
}

// GenerateLoader returns a loader HTML page that fetches and decrypts an encrypted file.
// The encURL is the relative path to the .enc file (e.g., "about.html.enc").
func GenerateLoader(encURL string) string {
	return strings.Replace(loaderTemplateHTML, "{{ENC_URL}}", encURL, 1)
}

// GenerateServiceWorker returns the service worker JavaScript source.
func GenerateServiceWorker() string {
	return serviceWorkerJS
}
