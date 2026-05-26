package main

import (
	"strings"
	"testing"
)

func TestIsValidProfileName(t *testing.T) {
	valid := []string{
		"default",
		"staging",
		"prod",
		"my-profile",
		"my_profile",
		"Profile1",
		"a",
		"0abc",
		strings.Repeat("a", 63),
	}
	invalid := []string{
		"",
		"-starts-with-dash",
		"_starts-with-underscore",
		"has space",
		"has/slash",
		"has.dot",
		"has@at",
		strings.Repeat("a", 64),
	}
	for _, name := range valid {
		if !isValidProfileName(name) {
			t.Errorf("isValidProfileName(%q) = false, want true", name)
		}
	}
	for _, name := range invalid {
		if isValidProfileName(name) {
			t.Errorf("isValidProfileName(%q) = true, want false", name)
		}
	}
}

func TestFormatSize(t *testing.T) {
	cases := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tc := range cases {
		got := formatSize(tc.bytes)
		if got != tc.want {
			t.Errorf("formatSize(%d) = %q, want %q", tc.bytes, got, tc.want)
		}
	}
}
