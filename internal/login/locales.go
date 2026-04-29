package login

type Strings struct {
	Lang                string
	Title               string
	Subtitle            string
	PasswordLabel       string
	PasswordPlaceholder string
	ToggleAriaLabel     string
	SubmitText          string
	ErrorText           string
	NetworkErrorText    string
	FooterText          string
}

var locales = map[string]Strings{
	"en": {
		Lang:                "en",
		Title:               "Sign in — Cubbit Pages",
		Subtitle:            "Protected site — enter the password",
		PasswordLabel:       "Password",
		PasswordPlaceholder: "Enter the password",
		ToggleAriaLabel:     "Show password",
		SubmitText:          "Sign in",
		ErrorText:           "Incorrect password",
		NetworkErrorText:    "Network error — try again",
		FooterText:          "Protected with Cubbit Pages",
	},
	"it": {
		Lang:                "it",
		Title:               "Accedi — Cubbit Pages",
		Subtitle:            "Sito protetto — inserisci la password",
		PasswordLabel:       "Password",
		PasswordPlaceholder: "Inserisci la password",
		ToggleAriaLabel:     "Mostra password",
		SubmitText:          "Accedi",
		ErrorText:           "Password non corretta",
		NetworkErrorText:    "Errore di rete — riprova",
		FooterText:          "Protetto con Cubbit Pages",
	},
}

func KnownLocales() []string {
	codes := make([]string, 0, len(locales))
	for code := range locales {
		codes = append(codes, code)
	}
	return codes
}

func LocaleStrings(code string) Strings {
	if s, ok := locales[code]; ok {
		return s
	}
	return locales["en"]
}

func IsKnownLocale(code string) bool {
	_, ok := locales[code]
	return ok
}
