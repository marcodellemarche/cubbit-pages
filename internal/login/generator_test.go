package login

import (
	"strings"
	"testing"
)

func TestGenerateLoginPageContainsForm(t *testing.T) {
	html := GenerateLoginPage()
	if !strings.Contains(html, `id="login-form"`) {
		t.Fatal("login page missing form element")
	}
	if !strings.Contains(html, `type="password"`) {
		t.Fatal("login page missing password input")
	}
}

func TestGenerateLoginPageContainsVerifyReference(t *testing.T) {
	html := GenerateLoginPage()
	if !strings.Contains(html, "_verify.enc") {
		t.Fatal("login page missing reference to _verify.enc")
	}
}

func TestGenerateLoginPageContainsDecryptionJS(t *testing.T) {
	html := GenerateLoginPage()
	if !strings.Contains(html, "crypto.subtle") {
		t.Fatal("login page missing Web Crypto API usage")
	}
	if !strings.Contains(html, "PBKDF2") {
		t.Fatal("login page missing PBKDF2 key derivation")
	}
	if !strings.Contains(html, "AES-GCM") {
		t.Fatal("login page missing AES-GCM decryption")
	}
}

func TestGenerateLoginPageContainsLocalStorageKey(t *testing.T) {
	html := GenerateLoginPage()
	if !strings.Contains(html, "cubbitseal_password") {
		t.Fatal("login page missing localStorage key")
	}
}

func TestGenerateLoaderContainsEncURL(t *testing.T) {
	loader := GenerateLoader("about.html.enc")
	if !strings.Contains(loader, "about.html.enc") {
		t.Fatal("loader missing enc URL")
	}
}

func TestGenerateLoaderContainsRedirect(t *testing.T) {
	loader := GenerateLoader("page.html.enc")
	if !strings.Contains(loader, "index.html") {
		t.Fatal("loader missing redirect to index.html")
	}
}

func TestGenerateLoaderContainsDecryptionLogic(t *testing.T) {
	loader := GenerateLoader("test.html.enc")
	if !strings.Contains(loader, "crypto.subtle") {
		t.Fatal("loader missing Web Crypto API")
	}
	if !strings.Contains(loader, "document.write") {
		t.Fatal("loader missing document.write for rendering")
	}
}

func TestGenerateLoginPageIsValidHTML(t *testing.T) {
	html := GenerateLoginPage()
	if !strings.HasPrefix(html, "<!DOCTYPE html>") {
		t.Fatal("login page missing DOCTYPE")
	}
	if !strings.Contains(html, "</html>") {
		t.Fatal("login page missing closing html tag")
	}
}
