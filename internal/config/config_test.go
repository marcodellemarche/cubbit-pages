package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveWithoutSourceDir(t *testing.T) {
	c := &Config{
		Bucket:    "bucket",
		AccessKey: "ak",
		SecretKey: "sk",
	}
	if err := c.Resolve(); err != nil {
		t.Fatalf("Resolve without SourceDir should succeed for non-deploy commands: %v", err)
	}
}

func TestResolveNormalizesPrefix(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"foo", "foo"},
		{"foo/", "foo"},
		{"/foo", "foo"},
		{"/foo/", "foo"},
		{"foo/bar/", "foo/bar"},
		{"//foo//", "foo"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			c := &Config{
				Bucket:    "bucket",
				AccessKey: "ak",
				SecretKey: "sk",
				SourceDir: "site",
				Prefix:    tc.in,
			}
			if err := c.Resolve(); err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if c.Prefix != tc.want {
				t.Errorf("Prefix = %q, want %q", c.Prefix, tc.want)
			}
		})
	}
}

func TestSiteURLUsesVirtualHostedStyle(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
		want     string
	}{
		{"cubbit default", "https://s3.cubbit.eu", "https://bucket.s3.cubbit.eu/index.html"},
		{"minio local port", "http://localhost:9000", "http://localhost:9000/bucket/index.html"},
		{"custom host with port", "https://custom.example.com:443", "https://custom.example.com:443/bucket/index.html"},
		{"custom host no port", "https://custom.example.com", "https://bucket.custom.example.com/index.html"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{
				Bucket:    "bucket",
				AccessKey: "ak",
				SecretKey: "sk",
				Endpoint:  tc.endpoint,
			}
			if err := c.Resolve(); err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if got := c.SiteURL(); got != tc.want {
				t.Errorf("SiteURL = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSiteURLDoesNotProduceDoubleSlash(t *testing.T) {
	cases := []struct {
		name   string
		prefix string
		want   string
	}{
		{"empty prefix", "", "https://bucket.s3.cubbit.eu/index.html"},
		{"trailing slash gets normalized", "weekly/2026-04-27/", "https://bucket.s3.cubbit.eu/weekly/2026-04-27/index.html"},
		{"leading slash gets normalized", "/weekly/2026-04-27", "https://bucket.s3.cubbit.eu/weekly/2026-04-27/index.html"},
		{"both slashes get normalized", "/weekly/2026-04-27/", "https://bucket.s3.cubbit.eu/weekly/2026-04-27/index.html"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{
				Bucket:    "bucket",
				AccessKey: "ak",
				SecretKey: "sk",
				SourceDir: "site",
				Prefix:    tc.prefix,
			}
			if err := c.Resolve(); err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if got := c.SiteURL(); got != tc.want {
				t.Errorf("SiteURL = %q, want %q", got, tc.want)
			}
		})
	}
}

// withTempHome redirects os.UserHomeDir to a temp dir and clears Cubbit env vars
// so tests are isolated from the external environment.
func withTempHome(t *testing.T, fn func(home string)) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// Clear all Cubbit env vars to prevent external environment from affecting tests.
	for _, key := range []string{
		"CUBBIT_PROFILE", "CUBBIT_ACCESS_KEY", "CUBBIT_SECRET_KEY",
		"CUBBIT_BUCKET", "CUBBIT_ENDPOINT", "CUBBIT_LOCALE",
	} {
		t.Setenv(key, "")
	}
	fn(tmp)
}

func writeConfig(t *testing.T, home, content string) {
	t.Helper()
	dir := filepath.Join(home, ".cubbit", "pages")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestResolveUsesDefaultProfile(t *testing.T) {
	withTempHome(t, func(home string) {
		writeConfig(t, home, `
profiles:
  default:
    access_key: file-ak
    secret_key: file-sk
    bucket: file-bucket
    endpoint: https://s3.cubbit.eu
`)
		c := &Config{}
		if err := c.Resolve(); err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if c.AccessKey != "file-ak" {
			t.Errorf("AccessKey = %q, want %q", c.AccessKey, "file-ak")
		}
		if c.Bucket != "file-bucket" {
			t.Errorf("Bucket = %q, want %q", c.Bucket, "file-bucket")
		}
		if c.Profile != DefaultProfileName {
			t.Errorf("Profile = %q, want %q", c.Profile, DefaultProfileName)
		}
	})
}

func TestResolveUsesNamedProfile(t *testing.T) {
	withTempHome(t, func(home string) {
		writeConfig(t, home, `
profiles:
  default:
    access_key: default-ak
    secret_key: default-sk
    bucket: default-bucket
  staging:
    access_key: staging-ak
    secret_key: staging-sk
    bucket: staging-bucket
`)
		c := &Config{Profile: "staging"}
		if err := c.Resolve(); err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if c.AccessKey != "staging-ak" {
			t.Errorf("AccessKey = %q, want %q", c.AccessKey, "staging-ak")
		}
		if c.Bucket != "staging-bucket" {
			t.Errorf("Bucket = %q, want %q", c.Bucket, "staging-bucket")
		}
		if c.Profile != "staging" {
			t.Errorf("Profile = %q, want %q", c.Profile, "staging")
		}
	})
}

func TestResolveCUBBITPROFILE(t *testing.T) {
	withTempHome(t, func(home string) {
		writeConfig(t, home, `
profiles:
  default:
    access_key: default-ak
    secret_key: default-sk
    bucket: default-bucket
  prod:
    access_key: prod-ak
    secret_key: prod-sk
    bucket: prod-bucket
`)
		t.Setenv("CUBBIT_PROFILE", "prod")

		c := &Config{}
		if err := c.Resolve(); err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if c.AccessKey != "prod-ak" {
			t.Errorf("AccessKey = %q, want %q", c.AccessKey, "prod-ak")
		}
		if c.Profile != "prod" {
			t.Errorf("Profile = %q, want %q", c.Profile, "prod")
		}
	})
}

func TestResolveFlagOverridesProfile(t *testing.T) {
	withTempHome(t, func(home string) {
		writeConfig(t, home, `
profiles:
  default:
    access_key: file-ak
    secret_key: file-sk
    bucket: file-bucket
`)
		c := &Config{
			AccessKey: "flag-ak",
			SecretKey: "flag-sk",
			Bucket:    "flag-bucket",
		}
		if err := c.Resolve(); err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if c.AccessKey != "flag-ak" {
			t.Errorf("AccessKey = %q, want %q (flag should override file)", c.AccessKey, "flag-ak")
		}
		if c.Bucket != "flag-bucket" {
			t.Errorf("Bucket = %q, want %q (flag should override file)", c.Bucket, "flag-bucket")
		}
	})
}

func TestResolveErrorsOnUnknownExplicitProfile(t *testing.T) {
	withTempHome(t, func(home string) {
		writeConfig(t, home, `
profiles:
  default:
    access_key: ak
    secret_key: sk
    bucket: bucket
`)
		c := &Config{Profile: "nonexistent"}
		err := c.Resolve()
		if err == nil {
			t.Fatal("expected error for unknown profile, got nil")
		}
		if !strings.Contains(err.Error(), "nonexistent") {
			t.Errorf("error should mention profile name, got: %v", err)
		}
		if !strings.Contains(err.Error(), "default") {
			t.Errorf("error should list available profiles, got: %v", err)
		}
	})
}

func TestResolveNoErrorOnMissingDefaultProfile(t *testing.T) {
	withTempHome(t, func(home string) {
		// No config file — user provides credentials via flags (normal first-time use).
		c := &Config{
			Bucket:    "my-bucket",
			AccessKey: "ak",
			SecretKey: "sk",
		}
		if err := c.Resolve(); err != nil {
			t.Fatalf("should not error when using default profile with no config file: %v", err)
		}
	})
}

func TestLoadFileConfigMigratesLegacyFormat(t *testing.T) {
	withTempHome(t, func(home string) {
		writeConfig(t, home, `access_key: old-ak
secret_key: old-sk
bucket: old-bucket
endpoint: https://s3.cubbit.eu
locale: it
`)
		fc, err := LoadFileConfig()
		if err != nil {
			t.Fatalf("LoadFileConfig: %v", err)
		}
		if fc == nil {
			t.Fatal("expected non-nil FileConfig")
		}
		pc := fc.GetProfile(DefaultProfileName)
		if pc == nil {
			t.Fatal("expected 'default' profile after migration")
		}
		if pc.AccessKey != "old-ak" {
			t.Errorf("AccessKey = %q, want %q", pc.AccessKey, "old-ak")
		}
		if pc.Bucket != "old-bucket" {
			t.Errorf("Bucket = %q, want %q", pc.Bucket, "old-bucket")
		}
		if pc.Locale != "it" {
			t.Errorf("Locale = %q, want %q", pc.Locale, "it")
		}
	})
}

func TestActiveProfileName(t *testing.T) {
	cases := []struct {
		name     string
		explicit string
		envVar   string
		fcNil    bool   // true = nil *FileConfig receiver
		fcDef    string // non-empty sets fc.Default (only when !fcNil)
		want     string
	}{
		// Explicit flag always wins.
		{"explicit over env and fc.Default", "prod", "staging", false, "other", "prod"},
		{"explicit over env (nil fc)", "prod", "staging", true, "", "prod"},
		// CUBBIT_PROFILE is second priority.
		{"env over fc.Default", "", "staging", false, "other", "staging"},
		{"env over nil fc", "", "staging", true, "", "staging"},
		// fc.Default is third priority (requires non-nil fc).
		{"fc.Default over hardcoded", "", "", false, "custom", "custom"},
		// Fallback to DefaultProfileName.
		{"non-nil fc no Default set", "", "", false, "", DefaultProfileName},
		{"nil fc no env no explicit", "", "", true, "", DefaultProfileName},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CUBBIT_PROFILE", tc.envVar)
			var fc *FileConfig
			if !tc.fcNil {
				fc = &FileConfig{Default: tc.fcDef}
			}
			got := fc.ActiveProfileName(tc.explicit)
			if got != tc.want {
				t.Errorf("ActiveProfileName = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestProfileNames(t *testing.T) {
	fc := &FileConfig{
		Profiles: map[string]*ProfileConfig{
			"zebra":   {},
			"alpha":   {},
			"default": {},
			"beta":    {},
		},
	}
	got := fc.ProfileNames()
	want := []string{"default", "alpha", "beta", "zebra"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ProfileNames[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

func TestProfileNamesNilSafe(t *testing.T) {
	var fc *FileConfig
	if got := fc.ProfileNames(); got != nil {
		t.Errorf("expected nil for nil FileConfig, got %v", got)
	}
}

func TestResolveUsesFcDefault(t *testing.T) {
	withTempHome(t, func(home string) {
		writeConfig(t, home, `
default: work
profiles:
  default:
    access_key: default-ak
    secret_key: default-sk
    bucket: default-bucket
  work:
    access_key: work-ak
    secret_key: work-sk
    bucket: work-bucket
`)
		c := &Config{} // no explicit --profile
		if err := c.Resolve(); err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if c.AccessKey != "work-ak" {
			t.Errorf("AccessKey = %q, want %q (fc.Default should select 'work')", c.AccessKey, "work-ak")
		}
		if c.Profile != "work" {
			t.Errorf("Profile = %q, want %q", c.Profile, "work")
		}
	})
}
