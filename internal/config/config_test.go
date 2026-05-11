package config

import "testing"

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
