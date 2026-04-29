package login

import (
	"reflect"
	"testing"
)

func TestAllLocalesHaveAllFields(t *testing.T) {
	types := reflect.TypeOf(Strings{})
	for code, s := range locales {
		v := reflect.ValueOf(s)
		for i := range types.NumField() {
			field := types.Field(i)
			val := v.Field(i).String()
			if val == "" && field.Name != "Lang" {
				t.Errorf("locale %q: field %q is empty", code, field.Name)
			}
		}
	}
}

func TestLocaleStringsFallsBackToEnglish(t *testing.T) {
	s := LocaleStrings("xx")
	if s.Title != LocaleStrings("en").Title {
		t.Fatal("LocaleStrings('xx') did not fall back to English")
	}
}
