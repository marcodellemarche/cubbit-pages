//go:build ignore

// Interactive wizard to add a new login page locale.
//
// Usage:
//   make add-locale LOCALE=fr
// or:
//   go run scripts/add-locale.go fr
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/marcodellemarche/cubbit-pages/internal/login"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: go run scripts/add-locale.go <code>")
	}
	code := strings.TrimSpace(strings.ToLower(os.Args[1]))

	if matched, _ := regexp.MatchString(`^[a-z]{2,5}$`, code); !matched {
		return fmt.Errorf("locale code must be 2-5 lowercase letters, got %q", code)
	}

	if login.IsKnownLocale(code) {
		return fmt.Errorf("locale %q already exists", code)
	}

	srcPath, err := localesPath()
	if err != nil {
		return err
	}

	ref := login.LocaleStrings("en")
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println()
	fmt.Printf("Adding locale %q...\n", code)
	fmt.Println("(Press Enter to keep the English default)")
	fmt.Println()

	var s login.Strings
	s.Lang = code

	fields := []struct {
		prompt string
		ref    string
		target *string
	}{
		{"Title", ref.Title, &s.Title},
		{"Subtitle", ref.Subtitle, &s.Subtitle},
		{"PasswordLabel", ref.PasswordLabel, &s.PasswordLabel},
		{"PasswordPlaceholder", ref.PasswordPlaceholder, &s.PasswordPlaceholder},
		{"ToggleAriaLabel", ref.ToggleAriaLabel, &s.ToggleAriaLabel},
		{"SubmitText", ref.SubmitText, &s.SubmitText},
		{"ErrorText", ref.ErrorText, &s.ErrorText},
		{"NetworkErrorText", ref.NetworkErrorText, &s.NetworkErrorText},
		{"FooterPrefix", ref.FooterPrefix, &s.FooterPrefix},
		{"FooterSuffix", ref.FooterSuffix, &s.FooterSuffix},
	}

	for _, f := range fields {
		for attempts := 0; attempts < 3; attempts++ {
			fmt.Printf("%s [%s]: ", f.prompt, f.ref)
			if !scanner.Scan() {
				return fmt.Errorf("aborted")
			}
			input := strings.TrimSpace(scanner.Text())
			if input == "" && attempts < 2 {
				fmt.Println("  Field cannot be empty. Try again.")
				continue
			}
			if input == "" {
				return fmt.Errorf("field %q cannot be empty", f.prompt)
			}
			*f.target = input
			break
		}
	}

	src, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", srcPath, err)
	}

	content := string(src)

	// Find the closing brace of the locales map specifically.
	mapStart := strings.Index(content, "var locales = map[string]Strings{")
	if mapStart == -1 {
		return fmt.Errorf("cannot find locales map declaration in %s", srcPath)
	}
	// Brace match starting right after the opening `{`.
	openIdx := strings.Index(content[mapStart:], "{")
	if openIdx == -1 {
		return fmt.Errorf("cannot find opening brace of locales map")
	}
	absOpen := mapStart + openIdx
	depth := 0
	closeIdx := -1
	for i := absOpen; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				closeIdx = i
			}
		}
		if closeIdx != -1 {
			break
		}
	}
	if closeIdx == -1 {
		return fmt.Errorf("cannot find closing brace of locales map")
	}

	var entryBuf strings.Builder
	entryBuf.WriteString("\t\"")
	entryBuf.WriteString(code)
	entryBuf.WriteString("\": {\n")
	fmt.Fprintf(&entryBuf, "\t\tLang:                %q,\n", s.Lang)
	fmt.Fprintf(&entryBuf, "\t\tTitle:               %q,\n", s.Title)
	fmt.Fprintf(&entryBuf, "\t\tSubtitle:            %q,\n", s.Subtitle)
	fmt.Fprintf(&entryBuf, "\t\tPasswordLabel:       %q,\n", s.PasswordLabel)
	fmt.Fprintf(&entryBuf, "\t\tPasswordPlaceholder: %q,\n", s.PasswordPlaceholder)
	fmt.Fprintf(&entryBuf, "\t\tToggleAriaLabel:     %q,\n", s.ToggleAriaLabel)
	fmt.Fprintf(&entryBuf, "\t\tSubmitText:          %q,\n", s.SubmitText)
	fmt.Fprintf(&entryBuf, "\t\tErrorText:           %q,\n", s.ErrorText)
	fmt.Fprintf(&entryBuf, "\t\tNetworkErrorText:    %q,\n", s.NetworkErrorText)
	fmt.Fprintf(&entryBuf, "\t\tFooterPrefix:        %q,\n", s.FooterPrefix)
	fmt.Fprintf(&entryBuf, "\t\tFooterSuffix:        %q,\n", s.FooterSuffix)
	entryBuf.WriteString("\t},\n")

	newContent := content[:closeIdx] + entryBuf.String() + content[closeIdx:]

	if err := os.WriteFile(srcPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", srcPath, err)
	}

	fmt.Println()
	fmt.Printf("Locale %q added to %s\n", code, srcPath)
	fmt.Println("Run 'make test' to verify all fields are populated.")
	fmt.Println()

	return nil
}

// localesPath resolves the absolute path to internal/login/locales.go
// relative to this script's own location, so it works regardless of cwd.
func localesPath() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot determine script path")
	}
	scriptsDir := filepath.Dir(file)
	repoRoot := filepath.Dir(scriptsDir)
	path := filepath.Join(repoRoot, "internal", "login", "locales.go")
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("locales.go not found at %s: %w", path, err)
	}
	return path, nil
}
