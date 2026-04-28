package config

import "testing"

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

func TestSiteURLDoesNotProduceDoubleSlash(t *testing.T) {
	cases := []struct {
		name   string
		prefix string
		want   string
	}{
		{"empty prefix", "", "https://s3.cubbit.eu/bucket/index.html"},
		{"trailing slash gets normalized", "weekly/2026-04-27/", "https://s3.cubbit.eu/bucket/weekly/2026-04-27/index.html"},
		{"leading slash gets normalized", "/weekly/2026-04-27", "https://s3.cubbit.eu/bucket/weekly/2026-04-27/index.html"},
		{"both slashes get normalized", "/weekly/2026-04-27/", "https://s3.cubbit.eu/bucket/weekly/2026-04-27/index.html"},
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
