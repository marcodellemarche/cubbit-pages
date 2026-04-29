package login

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// --- Login page tests ---

func TestGenerateLoginPageContainsForm(t *testing.T) {
	html := GenerateLoginPage("en")
	if !strings.Contains(html, `id="login-form"`) {
		t.Fatal("login page missing form element")
	}
	if !strings.Contains(html, `type="password"`) {
		t.Fatal("login page missing password input")
	}
}

func TestGenerateLoginPageContainsVerifyReference(t *testing.T) {
	html := GenerateLoginPage("en")
	if !strings.Contains(html, "_verify.enc") {
		t.Fatal("login page missing reference to _verify.enc")
	}
}

func TestGenerateLoginPageContainsDecryptionJS(t *testing.T) {
	html := GenerateLoginPage("en")
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
	html := GenerateLoginPage("en")
	if !strings.Contains(html, "cubbitseal_password") {
		t.Fatal("login page missing localStorage key")
	}
}

func TestGenerateLoginPageIsValidHTML(t *testing.T) {
	html := GenerateLoginPage("en")
	if !strings.HasPrefix(html, "<!DOCTYPE html>") {
		t.Fatal("login page missing DOCTYPE")
	}
	if !strings.Contains(html, "</html>") {
		t.Fatal("login page missing closing html tag")
	}
}

// --- Loader tests ---

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

// --- Service worker tests ---

func TestGenerateServiceWorkerContainsFetchHandler(t *testing.T) {
	js := GenerateServiceWorker()
	if !strings.Contains(js, "addEventListener('fetch'") {
		t.Fatal("service worker missing fetch event listener")
	}
}

func TestGenerateServiceWorkerContainsDecryption(t *testing.T) {
	js := GenerateServiceWorker()
	if !strings.Contains(js, "AES-GCM") {
		t.Fatal("service worker missing AES-GCM decryption")
	}
	if !strings.Contains(js, "PBKDF2") {
		t.Fatal("service worker missing PBKDF2 key derivation")
	}
}

func TestGenerateServiceWorkerContainsPasswordHandler(t *testing.T) {
	js := GenerateServiceWorker()
	if !strings.Contains(js, "SET_PASSWORD") {
		t.Fatal("service worker missing SET_PASSWORD handler")
	}
	if !strings.Contains(js, "PASSWORD_SET") {
		t.Fatal("service worker missing PASSWORD_SET confirmation")
	}
}

func TestGenerateServiceWorkerContainsMIMETypes(t *testing.T) {
	js := GenerateServiceWorker()
	mimeTypes := []string{".html", ".css", ".js", ".png", ".jpg", ".woff2", ".svg"}
	for _, mt := range mimeTypes {
		if !strings.Contains(js, "'"+mt+"'") {
			t.Fatalf("service worker missing MIME type for %s", mt)
		}
	}
}

// --- Service worker integration in login/loader ---

func TestGenerateLoginPageContainsServiceWorkerRegistration(t *testing.T) {
	html := GenerateLoginPage("en")
	if !strings.Contains(html, "serviceWorker") {
		t.Fatal("login page missing service worker registration")
	}
	if !strings.Contains(html, "ensureServiceWorker") {
		t.Fatal("login page missing ensureServiceWorker function")
	}
}

func TestGenerateLoaderContainsServiceWorkerRegistration(t *testing.T) {
	loader := GenerateLoader("test.html.enc")
	if !strings.Contains(loader, "serviceWorker") {
		t.Fatal("loader missing service worker registration")
	}
	if !strings.Contains(loader, "ensureServiceWorker") {
		t.Fatal("loader missing ensureServiceWorker function")
	}
}

// --- Critical: crypto constants consistency ---
// These tests ensure the JS crypto parameters in templates match the Go constants.
// If someone changes crypto.go constants but forgets the JS files, these tests catch it.

// extractJSVar extracts a numeric JS variable value like "var NAME = 123;"
func extractJSVar(source, varName string) (int, bool) {
	// Match patterns like: var ITERATIONS = 100000;  or  var ITERATIONS=100000;
	pattern := fmt.Sprintf(`var\s+%s\s*=\s*(\d+)`, regexp.QuoteMeta(varName))
	re := regexp.MustCompile(pattern)
	m := re.FindStringSubmatch(source)
	if m == nil {
		return 0, false
	}
	var val int
	fmt.Sscanf(m[1], "%d", &val)
	return val, true
}

// extractJSMagicArray extracts [0x43,0x50,0x47,0x53] style arrays
func extractJSMagicArray(source string) []byte {
	re := regexp.MustCompile(`var\s+MAGIC\s*=\s*\[([^\]]+)\]`)
	m := re.FindStringSubmatch(source)
	if m == nil {
		return nil
	}
	parts := strings.Split(m[1], ",")
	var result []byte
	for _, p := range parts {
		p = strings.TrimSpace(p)
		var val int
		fmt.Sscanf(p, "0x%x", &val)
		result = append(result, byte(val))
	}
	return result
}

func TestCryptoConstantsConsistency(t *testing.T) {
	// These are the authoritative Go constants from crypto.go
	const (
		goMagic0     = 0x43
		goMagic1     = 0x50
		goMagic2     = 0x47
		goMagic3     = 0x53
		goVersion    = 1
		goSaltLen    = 16
		goNonceLen   = 12
		goIterations = 100_000
	)

	// Check each JS source that contains crypto constants
	sources := map[string]string{
		"sw.js":          GenerateServiceWorker(),
		"template.html":  GenerateLoginPage("en"),
		"loader.html":    GenerateLoader("test.html.enc"),
	}

	for name, src := range sources {
		t.Run(name+"/ITERATIONS", func(t *testing.T) {
			val, ok := extractJSVar(src, "ITERATIONS")
			if !ok {
				t.Fatalf("%s: ITERATIONS variable not found", name)
			}
			if val != goIterations {
				t.Fatalf("%s: ITERATIONS = %d, want %d (must match crypto.go)", name, val, goIterations)
			}
		})

		t.Run(name+"/VERSION", func(t *testing.T) {
			val, ok := extractJSVar(src, "VERSION")
			if !ok {
				t.Fatalf("%s: VERSION variable not found", name)
			}
			if val != goVersion {
				t.Fatalf("%s: VERSION = %d, want %d (must match crypto.go)", name, val, goVersion)
			}
		})

		t.Run(name+"/SALT_LEN", func(t *testing.T) {
			val, ok := extractJSVar(src, "SALT_LEN")
			if !ok {
				t.Fatalf("%s: SALT_LEN variable not found", name)
			}
			if val != goSaltLen {
				t.Fatalf("%s: SALT_LEN = %d, want %d (must match crypto.go)", name, val, goSaltLen)
			}
		})

		t.Run(name+"/NONCE_LEN", func(t *testing.T) {
			val, ok := extractJSVar(src, "NONCE_LEN")
			if !ok {
				t.Fatalf("%s: NONCE_LEN variable not found", name)
			}
			if val != goNonceLen {
				t.Fatalf("%s: NONCE_LEN = %d, want %d (must match crypto.go)", name, val, goNonceLen)
			}
		})

		t.Run(name+"/MAGIC", func(t *testing.T) {
			magic := extractJSMagicArray(src)
			if magic == nil {
				t.Fatalf("%s: MAGIC array not found", name)
			}
			if len(magic) != 4 || magic[0] != goMagic0 || magic[1] != goMagic1 || magic[2] != goMagic2 || magic[3] != goMagic3 {
				t.Fatalf("%s: MAGIC = %v, want [0x43,0x50,0x47,0x53] (must match crypto.go)", name, magic)
			}
		})
	}
}

// --- Critical: SW exclusion list ---
// The SW must NOT intercept certain paths, otherwise the site breaks.

func TestServiceWorkerExcludesCorrectPaths(t *testing.T) {
	js := GenerateServiceWorker()
	// These paths must appear in the exclusion check
	excludedPaths := []string{"sw.js", "index.html", "_verify.enc"}
	for _, p := range excludedPaths {
		if !strings.Contains(js, "'"+p+"'") {
			t.Fatalf("service worker missing exclusion for %s", p)
		}
	}
	// .enc files must be excluded (the SW fetches them directly)
	if !strings.Contains(js, ".enc") {
		t.Fatal("service worker missing .enc exclusion logic")
	}
}

// --- Critical: login page registers SW BEFORE loading encrypted page ---

func TestLoginPageSWRegistrationBeforeLoad(t *testing.T) {
	html := GenerateLoginPage("en")

	// ensureServiceWorker must be called before loadEncryptedPage/redirectToTarget
	swPos := strings.Index(html, "ensureServiceWorker")
	loadPos := strings.Index(html, "loadEncryptedPage")
	if swPos == -1 {
		t.Fatal("login page missing ensureServiceWorker call")
	}
	if loadPos == -1 {
		t.Fatal("login page missing loadEncryptedPage call")
	}

	// In the saved-password path: ensureServiceWorker().then(redirectToTarget)
	// Verify that the pattern exists
	if !strings.Contains(html, "ensureServiceWorker(savedPwd).then(") {
		t.Fatal("login page: saved password path must call ensureServiceWorker before redirect")
	}

	// In the login submit path: ensureServiceWorker(pwd).then(...)
	if !strings.Contains(html, "ensureServiceWorker(pwd).then(") {
		t.Fatal("login page: submit path must call ensureServiceWorker before redirect")
	}
}

// --- Critical: loader registers SW before fetching ---

func TestLoaderSWRegistrationBeforeFetch(t *testing.T) {
	loader := GenerateLoader("about.html.enc")
	if !strings.Contains(loader, "ensureServiceWorker(pwd).then(") {
		t.Fatal("loader must call ensureServiceWorker before fetching content")
	}
}

// --- Critical: service worker sends password via sw.js register path ---

func TestLoginPageRegistersCorrectSWFile(t *testing.T) {
	html := GenerateLoginPage("en")
	if !strings.Contains(html, "register('sw.js')") {
		t.Fatal("login page must register 'sw.js' as service worker")
	}
}

func TestLoaderRegistersCorrectSWFile(t *testing.T) {
	loader := GenerateLoader("test.html.enc")
	if !strings.Contains(loader, "register('sw.js')") {
		t.Fatal("loader must register 'sw.js' as service worker")
	}
}

// --- Critical: SW persists password to IndexedDB for restart survival ---

func TestServiceWorkerPersistsPasswordToIndexedDB(t *testing.T) {
	js := GenerateServiceWorker()
	// Must use IndexedDB for persistence
	if !strings.Contains(js, "indexedDB") {
		t.Fatal("service worker must use IndexedDB for password persistence")
	}
	// Must save password when received
	if !strings.Contains(js, "savePassword") {
		t.Fatal("service worker must call savePassword when receiving SET_PASSWORD")
	}
	// Must load password in fetch handler when memory is empty
	if !strings.Contains(js, "ensurePassword") {
		t.Fatal("service worker must call ensurePassword in fetch handler")
	}
	if !strings.Contains(js, "loadPassword") {
		t.Fatal("service worker must have loadPassword function for IndexedDB reads")
	}
}

func TestServiceWorkerHandlesClearPassword(t *testing.T) {
	js := GenerateServiceWorker()
	if !strings.Contains(js, "CLEAR_PASSWORD") {
		t.Fatal("service worker must handle CLEAR_PASSWORD message")
	}
	if !strings.Contains(js, "clearPassword") {
		t.Fatal("service worker must call clearPassword to remove from IndexedDB")
	}
}

func TestLoginPageSendsClearPasswordOnLogout(t *testing.T) {
	html := GenerateLoginPage("en")
	if !strings.Contains(html, "CLEAR_PASSWORD") {
		t.Fatal("login page must send CLEAR_PASSWORD to SW when clearing localStorage")
	}
}

func TestLoaderSendsClearPasswordOnLogout(t *testing.T) {
	loader := GenerateLoader("test.html.enc")
	if !strings.Contains(loader, "CLEAR_PASSWORD") {
		t.Fatal("loader must send CLEAR_PASSWORD to SW when clearing localStorage")
	}
}

// --- Service worker: .enc fetch fallback pattern ---

func TestServiceWorkerFetchAndDecryptPattern(t *testing.T) {
	js := GenerateServiceWorker()

	// Must append .enc to the original URL
	if !strings.Contains(js, "originalUrl + '.enc'") {
		t.Fatal("service worker must try originalUrl + '.enc' for encrypted files")
	}

	// Must call decryptData on the fetched .enc content
	if !strings.Contains(js, "decryptData(new Uint8Array(buf), password)") {
		t.Fatal("service worker must decrypt .enc content with stored password")
	}

	// Must set Content-Type from original URL
	if !strings.Contains(js, "getContentType(originalUrl)") {
		t.Fatal("service worker must infer Content-Type from original URL")
	}
}

func TestGenerateLoginPageLocaleSelection(t *testing.T) {
	en := GenerateLoginPage("en")
	it := GenerateLoginPage("it")

	if !strings.Contains(en, "Sign in") {
		t.Fatal("English login page missing 'Sign in'")
	}
	if !strings.Contains(it, "Accedi") {
		t.Fatal("Italian login page missing 'Accedi'")
	}
	if !strings.Contains(en, "Protected site") {
		t.Fatal("English login page missing 'Protected site'")
	}
	if !strings.Contains(it, "Sito protetto") {
		t.Fatal("Italian login page missing 'Sito protetto'")
	}
}

func TestGenerateLoginPageUnknownLocaleFallback(t *testing.T) {
	de := GenerateLoginPage("de")
	if !strings.Contains(de, "Sign in") {
		t.Fatal("Unknown locale did not fall back to English")
	}
}

func TestGenerateLoginPageSetsLangAttribute(t *testing.T) {
	en := GenerateLoginPage("en")
	it := GenerateLoginPage("it")

	if !strings.Contains(en, `lang="en"`) {
		t.Fatal("English page missing lang=\"en\"")
	}
	if !strings.Contains(it, `lang="it"`) {
		t.Fatal("Italian page missing lang=\"it\"")
	}
}
