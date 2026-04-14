package login

import (
	"strings"
)

// GenerateLoginPage returns the HTML for the login page.
func GenerateLoginPage() string {
	return loginTemplateHTML
}

// GenerateLoader returns a loader HTML page that fetches and decrypts an encrypted file.
// The encURL is the relative path to the .enc file (e.g., "about.html.enc").
func GenerateLoader(encURL string) string {
	return strings.Replace(loaderTemplateHTML, "{{ENC_URL}}", encURL, 1)
}
